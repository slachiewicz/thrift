/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements. See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership. The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package dart

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateStruct is t_dart_generator::generate_struct.
func (g *Generator) generateStruct(s *sema.Struct) {
	g.generateDartStruct(s, false)
}

// generateXception is t_dart_generator::generate_xception: exceptions
// are structs, but they inherit from Exception.
func (g *Generator) generateXception(s *sema.Struct) {
	g.generateDartStruct(s, true)
}

// generateDartStruct is t_dart_generator::generate_dart_struct.
func (g *Generator) generateDartStruct(s *sema.Struct, isException bool) {
	fileName := getFileName(s.Name())
	fStructName := g.srcDir + "/" + fileName + ".dart"
	var f strings.Builder

	f.WriteString(autogenComment() + g.dartLibrary(fileName) + "\n")
	f.WriteString(g.dartThriftImports() + "\n")

	g.generateDartStructDefinition(&f, s, isException, false, fileName)

	emit.WriteFile(fStructName, f.String())
}

// generateDartStructDefinition is
// t_dart_generator::generate_dart_struct_definition. It has various
// parameters, as it could be generated standalone or inside another
// class as a helper.
func (g *Generator) generateDartStructDefinition(out *strings.Builder, s *sema.Struct, isException, isResult bool, exportFileName string) {
	g.generateDartDoc(out, s)

	className := s.Name()
	if exportFileName != "" {
		g.exportClassToLibrary(exportFileName, className)
	}
	out.WriteString(g.ind() + "class " + className + " ")

	out.WriteString("implements TBase")
	if isException {
		out.WriteString(", Exception ")
	}
	g.scopeUp(out)

	out.WriteString(g.ind() + "static final TStruct _STRUCT_DESC = new TStruct(\"" + className + "\");\n")

	// Members are public for -dart, private for -dartbean
	members := s.Members()

	for _, m := range members {
		out.WriteString(g.ind() + "static final TField _" + constantName(m.Name()) + "_FIELD_DESC = new TField(\"" +
			m.Name() + "\", " + typeToEnum(m.Type()) + ", " + strconv.Itoa(int(m.Key())) + ");\n")
	}

	out.WriteString("\n")

	for _, m := range members {
		g.generateDartDoc(out, m)
		out.WriteString(g.ind() + g.typeName(m.Type()) + getTypeSuffix(m.Type()) + " _" + getMemberName(m.Name()) + initValue(m) + ";\n")
		out.WriteString(g.ind() + "static const int " + upcaseString(m.Name()) + " = " + strconv.Itoa(int(m.Key())) + ";\n")
	}

	out.WriteString("\n")

	// Inner Isset class
	if len(members) > 0 {
		for _, m := range members {
			if !typeCanBeNull(m.Type()) {
				fieldName := getMemberName(m.Name())
				out.WriteString(g.ind() + "bool __isset_" + fieldName + " = false;\n")
			}
		}
	}

	out.WriteString("\n")

	// Default constructor
	out.WriteString(g.ind() + s.Name() + "()")
	g.scopeUp(out)
	for _, m := range members {
		t := sema.TrueType(m.Type())
		if m.Value() != nil {
			g.printConstValue(out, "this."+getMemberName(m.Name()), t, m.Value(), true, true, false)
		}
	}
	g.scopeDown(out)
	out.WriteString("\n")

	g.generateDartBeanBoilerplate(out, s)
	g.generateGenericFieldGetters(out, s)
	g.generateGenericFieldSetters(out, s)
	g.generateGenericIssetMethod(out, s)

	g.generateDartStructReader(out, s)
	if isResult {
		g.generateDartStructResultWriter(out, s)
	} else {
		g.generateDartStructWriter(out, s)
	}
	g.generateDartStructTostring(out, s)
	g.generateDartValidator(out, s)
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateDartStructReader is t_dart_generator::generate_dart_struct_reader:
// generates a function to read all the fields of the struct.
func (g *Generator) generateDartStructReader(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.ind() + "read(TProtocol iprot)")
	g.scopeUp(out)

	fields := s.Members()

	// Declare stack tmp variables and read struct header
	out.WriteString(g.ind() + "TField field;\n")
	out.WriteString(g.ind() + "iprot.incrementRecursionDepth();\n")
	out.WriteString(g.ind() + "try")
	g.scopeUp(out)
	out.WriteString(g.ind() + "iprot.readStructBegin();\n")

	// Loop over reading in fields
	out.WriteString(g.ind() + "while (true)")
	g.scopeUp(out)

	// Read beginning field marker
	out.WriteString(g.ind() + "field = iprot.readFieldBegin();\n")

	// Check for field STOP marker and break
	out.WriteString(g.ind() + "if (field.type == TType.STOP)")
	g.scopeUp(out)
	out.WriteString(g.ind() + "break;\n")
	g.scopeDown(out)

	// Switch statement on the field we are reading
	out.WriteString(g.ind() + "switch (field.id)")
	g.scopeUp(out)

	// Generate deserialization code for known cases
	for _, f := range fields {
		out.WriteString(g.ind() + "case " + upcaseString(f.Name()) + ":\n")
		g.indentUp()

		out.WriteString(g.ind() + "if (field.type == " + typeToEnum(f.Type()) + ")")
		g.scopeUp(out)

		g.generateDeserializeField(out, f, "this.")
		g.generateIssetSet(out, f)

		g.scopeDownPostfix(out, " else")
		g.scopeUp(out)
		out.WriteString(g.ind() + "TProtocolUtil.skip(iprot, field.type);\n")
		g.scopeDown(out)

		out.WriteString(g.ind() + "break;\n")
		g.indentDown()
	}

	// In the default case we skip the field
	out.WriteString(g.ind() + "default:\n")
	g.indentUp()
	out.WriteString(g.ind() + "TProtocolUtil.skip(iprot, field.type);\n")
	out.WriteString(g.ind() + "break;\n")
	g.indentDown()

	g.scopeDown(out)

	// Read field end marker
	out.WriteString(g.ind() + "iprot.readFieldEnd();\n")

	g.scopeDown(out)

	out.WriteString(g.ind() + "iprot.readStructEnd();\n\n")

	// in non-beans style, check for required fields of primitive type
	// (which can be checked here but not in the general validate method)
	out.WriteString(g.ind() + "// check for required fields of primitive type, which can't be checked in the validate method\n")
	for _, f := range fields {
		if f.Req() == sema.Required && !typeCanBeNull(f.Type()) {
			fieldName := getMemberName(f.Name())
			out.WriteString(g.ind() + "if (!__isset_" + fieldName + ")")
			g.scopeUp(out)
			out.WriteString(g.ind() + "  throw new TProtocolError(TProtocolErrorType.UNKNOWN, \"Required field '" +
				fieldName + "' was not found in serialized data! Struct: \" + toString());\n")
			g.scopeDownPostfix(out, "\n\n")
		}
	}

	// performs various checks (e.g. check that all required fields are set)
	out.WriteString(g.ind() + "validate();\n")

	g.scopeDownPostfix(out, " finally") // close try, begin finally
	g.scopeUp(out)
	out.WriteString(g.ind() + "iprot.decrementRecursionDepth();\n")
	g.scopeDown(out) // close finally

	g.scopeDownPostfix(out, "\n\n") // close read() function
}

// generateDartValidator is t_dart_generator::generate_dart_validator:
// generates a method to perform various checks (e.g. check that all
// required fields are set).
func (g *Generator) generateDartValidator(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.ind() + "validate()")
	g.scopeUp(out)

	fields := s.Members()

	out.WriteString(g.ind() + "// check for required fields\n")
	for _, f := range fields {
		if f.Req() == sema.Required {
			fieldName := getMemberName(f.Name())
			if typeCanBeNull(f.Type()) {
				out.WriteString(g.ind() + "if (" + fieldName + " == null)")
				g.scopeUp(out)
				out.WriteString(g.ind() + "throw new TProtocolError(TProtocolErrorType.UNKNOWN, \"Required field '" +
					fieldName + "' was not present! Struct: \" + toString());\n")
				g.scopeDown(out)
			} else {
				out.WriteString(g.ind() + "// alas, we cannot check '" + fieldName +
					"' because it's a primitive and you chose the non-beans generator.\n")
			}
		}
	}

	// check that fields of type enum have valid values
	out.WriteString(g.ind() + "// check that fields of type enum have valid values\n")
	for _, f := range fields {
		// If field is an enum, check that its value is valid. Uses the
		// raw declared type, not TrueType: a typedef-to-enum field is
		// not caught here, matching the C++ generator.
		t := f.Type()
		if t.IsEnum() {
			fieldName := getMemberName(f.Name())
			out.WriteString(g.ind() + "if (" + generateIssetCheckField(f) + " && !" + g.getTtypeClassName(t) +
				".VALID_VALUES.contains(" + fieldName + "))")
			g.scopeUp(out)
			out.WriteString(g.ind() + "throw new TProtocolError(TProtocolErrorType.UNKNOWN, \"The field '" +
				fieldName + "' has been assigned the invalid value $" + fieldName + "\");\n")
			g.scopeDown(out)
		}
	}

	g.scopeDownPostfix(out, "\n\n")
}

// generateDartStructWriter is t_dart_generator::generate_dart_struct_writer:
// generates a function to write all the fields of the struct.
func (g *Generator) generateDartStructWriter(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.ind() + "write(TProtocol oprot)")
	g.scopeUp(out)

	fields := s.SortedMembers()

	// performs various checks (e.g. check that all required fields are set)
	out.WriteString(g.ind() + "validate();\n\n")

	out.WriteString(g.ind() + "oprot.incrementRecursionDepth();\n")
	out.WriteString(g.ind() + "try")
	g.scopeUp(out)
	out.WriteString(g.ind() + "oprot.writeStructBegin(_STRUCT_DESC);\n")

	for _, f := range fields {
		fieldName := getMemberName(f.Name())
		couldBeUnset := f.Req() == sema.Optional
		if couldBeUnset {
			out.WriteString(g.ind() + "if (" + generateIssetCheckField(f) + ")")
			g.scopeUp(out)
		}
		nullAllowed := typeCanBeNull(f.Type())
		if nullAllowed {
			out.WriteString(g.ind() + "if (this." + fieldName + " != null)")
			g.scopeUp(out)
		}

		out.WriteString(g.ind() + "oprot.writeFieldBegin(_" + constantName(f.Name()) + "_FIELD_DESC);\n")

		// Write field contents
		g.generateSerializeField(out, f, "this.")

		// Write field closer
		out.WriteString(g.ind() + "oprot.writeFieldEnd();\n")

		if nullAllowed {
			g.scopeDown(out)
		}
		if couldBeUnset {
			g.scopeDown(out)
		}
	}
	// Write the struct map
	out.WriteString(g.ind() + "oprot.writeFieldStop();\n")
	out.WriteString(g.ind() + "oprot.writeStructEnd();\n")

	g.scopeDownPostfix(out, " finally") // close try, begin finally
	g.scopeUp(out)
	out.WriteString(g.ind() + "oprot.decrementRecursionDepth();\n")
	g.scopeDown(out) // close finally

	g.scopeDownPostfix(out, "\n\n") // close write() function
}

// generateDartStructResultWriter is
// t_dart_generator::generate_dart_struct_result_writer: generates a
// function to write all the fields of the struct, which is a function
// result. These fields are only written if they are set in the Isset
// array, and only one of them can be set at a time.
func (g *Generator) generateDartStructResultWriter(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.ind() + "write(TProtocol oprot)")
	g.scopeUp(out)

	fields := s.SortedMembers()

	out.WriteString(g.ind() + "oprot.incrementRecursionDepth();\n")
	out.WriteString(g.ind() + "try")
	g.scopeUp(out)
	out.WriteString(g.ind() + "oprot.writeStructBegin(_STRUCT_DESC);\n\n")

	first := true
	for _, f := range fields {
		if first {
			first = false
			out.WriteString(g.ind() + "if ")
		} else {
			out.WriteString(" else if ")
		}

		out.WriteString("(this." + generateIssetCheckField(f) + ")")
		g.scopeUp(out)

		out.WriteString(g.ind() + "oprot.writeFieldBegin(_" + constantName(f.Name()) + "_FIELD_DESC);\n")

		// Write field contents
		g.generateSerializeField(out, f, "this.")

		// Write field closer
		out.WriteString(g.ind() + "oprot.writeFieldEnd();\n")

		g.scopeDownPostfix(out, "")
	}
	out.WriteString("\n")

	// Write the struct map
	out.WriteString(g.ind() + "oprot.writeFieldStop();\n")
	out.WriteString(g.ind() + "oprot.writeStructEnd();\n")

	g.scopeDownPostfix(out, " finally") // close try, begin finally
	g.scopeUp(out)
	out.WriteString(g.ind() + "oprot.decrementRecursionDepth();\n")
	g.scopeDown(out) // close finally

	g.scopeDownPostfix(out, "\n\n") // close write() function
}

// generateGenericFieldGetters is
// t_dart_generator::generate_generic_field_getters.
func (g *Generator) generateGenericFieldGetters(out *strings.Builder, s *sema.Struct) {
	// create the getter
	out.WriteString(g.ind() + "getFieldValue(int fieldID)")
	g.scopeUp(out)

	out.WriteString(g.ind() + "switch (fieldID)")
	g.scopeUp(out)

	for _, f := range s.Members() {
		fieldName := getMemberName(f.Name())

		out.WriteString(g.ind() + "case " + upcaseString(fieldName) + ":\n")
		g.indentUp()
		out.WriteString(g.ind() + "return this." + fieldName + ";\n")
		g.indentDown()
	}

	out.WriteString(g.ind() + "default:\n")
	g.indentUp()
	out.WriteString(g.ind() + "throw new ArgumentError(\"Field $fieldID doesn't exist!\");\n")
	g.indentDown()

	g.scopeDown(out)                // switch
	g.scopeDownPostfix(out, "\n\n") // method
}

// generateGenericFieldSetters is
// t_dart_generator::generate_generic_field_setters.
func (g *Generator) generateGenericFieldSetters(out *strings.Builder, s *sema.Struct) {
	// create the setter
	out.WriteString(g.ind() + "setFieldValue(int fieldID, Object? value)")
	g.scopeUp(out)

	out.WriteString(g.ind() + "switch (fieldID)")
	g.scopeUp(out)

	// build up the bodies of both the getter and setter at once
	for _, f := range s.Members() {
		t := sema.TrueType(f.Type())
		fieldName := getMemberName(f.Name())

		out.WriteString(g.ind() + "case " + upcaseString(fieldName) + ":\n")
		g.indentUp()

		out.WriteString(g.ind() + "if (value == null)")
		g.scopeUp(out)
		out.WriteString(g.ind() + "unset" + getCapName(fieldName) + "();\n")

		g.scopeDownPostfix(out, " else")
		g.scopeUp(out)

		out.WriteString(g.ind() + "this." + fieldName + " = value as " + g.typeName(t) + ";\n")
		g.scopeDown(out)

		out.WriteString(g.ind() + "break;\n")

		g.indentDown()
		out.WriteString("\n")
	}

	out.WriteString(g.ind() + "default:\n")
	g.indentUp()
	out.WriteString(g.ind() + "throw new ArgumentError(\"Field $fieldID doesn't exist!\");\n")
	g.indentDown()

	g.scopeDown(out)                // switch
	g.scopeDownPostfix(out, "\n\n") // method
}

// generateGenericIssetMethod is
// t_dart_generator::generate_generic_isset_method: creates a generic
// isSet method that takes the field number as argument.
func (g *Generator) generateGenericIssetMethod(out *strings.Builder, s *sema.Struct) {
	// create the isSet method
	out.WriteString(g.ind() + "// Returns true if field corresponding to fieldID is set (has been assigned a value) and false otherwise\n")
	out.WriteString(g.ind() + "bool isSet(int fieldID)")
	g.scopeUp(out)

	out.WriteString(g.ind() + "switch (fieldID)")
	g.scopeUp(out)

	for _, f := range s.Members() {
		out.WriteString(g.ind() + "case " + upcaseString(f.Name()) + ":\n")
		g.indentUp()
		out.WriteString(g.ind() + "return " + generateIssetCheckField(f) + ";\n")
		g.indentDown()
	}

	out.WriteString(g.ind() + "default:\n")
	g.indentUp()
	out.WriteString(g.ind() + "throw new ArgumentError(\"Field $fieldID doesn't exist!\");\n")
	g.indentDown()

	g.scopeDown(out)                // switch
	g.scopeDownPostfix(out, "\n\n") // method
}

// generateDartBeanBoilerplate is
// t_dart_generator::generate_dart_bean_boilerplate: generates a set of
// Dart Bean boilerplate functions (setters, getters, etc.) for the
// given struct.
func (g *Generator) generateDartBeanBoilerplate(out *strings.Builder, s *sema.Struct) {
	for _, f := range s.Members() {
		t := sema.TrueType(f.Type())
		fieldName := getMemberName(f.Name())
		capName := getCapName(fieldName)

		out.WriteString(g.ind() + "// " + fieldName + "\n")

		// Simple getter
		g.generateDartDoc(out, f)
		out.WriteString(g.ind() + g.typeName(t) + getTypeSuffix(t) + " get " + fieldName + " => this._" + fieldName + ";\n\n")

		// Simple setter
		g.generateDartDoc(out, f)
		out.WriteString(g.ind() + "set " + fieldName + "(" + g.typeName(t) + getTypeSuffix(t) + " " + fieldName + ")")
		g.scopeUp(out)
		out.WriteString(g.ind() + "this._" + fieldName + " = " + fieldName + ";\n")
		g.generateIssetSet(out, f)
		g.scopeDownPostfix(out, "\n\n")

		// isSet method
		out.WriteString(g.ind() + "bool is" + getCapName("set") + capName + "()")
		if typeCanBeNull(t) {
			out.WriteString(" => this." + fieldName + " != null;\n\n")
		} else {
			out.WriteString(" => this.__isset_" + fieldName + ";\n\n")
		}

		// Unsetter
		out.WriteString(g.ind() + "unset" + capName + "()")
		g.scopeUp(out)
		if typeCanBeNull(t) {
			out.WriteString(g.ind() + "this." + fieldName + " = null;\n")
		} else {
			out.WriteString(g.ind() + "this.__isset_" + fieldName + " = false;\n")
		}
		g.scopeDownPostfix(out, "\n\n")
	}
}

// generateDartStructTostring is
// t_dart_generator::generate_dart_struct_tostring: generates a
// toString() method for the given struct.
func (g *Generator) generateDartStructTostring(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.ind() + "String toString()")
	g.scopeUp(out)

	out.WriteString(g.ind() + "StringBuffer ret = new StringBuffer(\"" + s.Name() + "(\");\n\n")

	fields := s.Members()

	first := true
	for _, f := range fields {
		couldBeUnset := f.Req() == sema.Optional
		if couldBeUnset {
			out.WriteString(g.ind() + "if (" + generateIssetCheckField(f) + ")")
			g.scopeUp(out)
		}

		fieldName := getMemberName(f.Name())

		if !first {
			out.WriteString(g.ind() + "ret.write(\", \");\n")
		}
		out.WriteString(g.ind() + "ret.write(\"" + fieldName + ":\");\n")
		canBeNull := typeCanBeNull(f.Type())
		if canBeNull {
			out.WriteString(g.ind() + "if (this." + fieldName + " == null)")
			g.scopeUp(out)
			out.WriteString(g.ind() + "ret.write(\"null\");\n")
			g.scopeDownPostfix(out, " else")
			g.scopeUp(out)
		}

		// Uses the raw declared type for is_binary/is_enum, not
		// TrueType, matching the C++ generator: a typedef to binary or
		// to an enum falls through to the plain ret.write below.
		switch {
		case f.Type().IsBinary():
			out.WriteString(g.ind() + "ret.write(\"BINARY\");\n")
		case f.Type().IsEnum():
			out.WriteString(g.ind() + "String? " + fieldName + "_name = " + g.getTtypeClassName(f.Type()) +
				".VALUES_TO_NAMES[this." + fieldName + "];\n")
			out.WriteString(g.ind() + "if (" + fieldName + "_name != null)")
			g.scopeUp(out)
			out.WriteString(g.ind() + "ret.write(" + fieldName + "_name);\n")
			out.WriteString(g.ind() + "ret.write(\" (\");\n")
			g.scopeDown(out)
			out.WriteString(g.ind() + "ret.write(this." + fieldName + ");\n")
			out.WriteString(g.ind() + "if (" + fieldName + "_name != null)")
			g.scopeUp(out)
			out.WriteString(g.ind() + "ret.write(\")\");\n")
			g.scopeDown(out)
		default:
			out.WriteString(g.ind() + "ret.write(this." + fieldName + ");\n")
		}

		if canBeNull {
			g.scopeDown(out)
		}
		if couldBeUnset {
			g.scopeDown(out)
		}

		out.WriteString("\n")
		first = false
	}

	out.WriteString(g.ind() + "ret.write(\")\");\n\n")

	out.WriteString(g.ind() + "return ret.toString();\n")

	g.scopeDownPostfix(out, "\n\n")
}
