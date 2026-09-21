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

package javame

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

func (g *Generator) generateStruct(s *sema.Struct) {
	if s.IsUnion() {
		g.generateJavaUnion(s)
	} else {
		g.generateJavaStruct(s, false)
	}
}

// generateXception is generate_xception: exceptions are structs, but
// they inherit from Exception.
func (g *Generator) generateXception(s *sema.Struct) {
	g.generateJavaStruct(s, true)
}

// generateJavaStruct is generate_java_struct: one file per struct.
func (g *Generator) generateJavaStruct(s *sema.Struct, isException bool) {
	fStructName := g.packageDir + "/" + s.Name() + ".java"
	var f strings.Builder

	f.WriteString(autogenComment() + g.javaPackage() + g.javaTypeImports() + g.javaThriftImports())

	g.generateJavaStructDefinition(&f, s, isException, false, false)
	emit.WriteFile(fStructName, f.String())
}

// generateJavaStructDefinition is generate_java_struct_definition. It has
// various parameters, as it could be generated standalone or inside
// another class as a helper. If it is a helper, it is a static class.
func (g *Generator) generateJavaStructDefinition(out *strings.Builder, s *sema.Struct, isException, inClass, isResult bool) {
	g.javaDoc(out, s)

	isFinal := s.Annotations().Has("final")

	out.WriteString(g.indent() + "public ")
	if isFinal {
		out.WriteString("final ")
	}
	if inClass {
		out.WriteString("static ")
	}
	out.WriteString("class " + s.Name() + " ")

	if isException {
		out.WriteString("extends Exception ")
	}
	out.WriteString("implements TBase ")

	g.scopeUp(out)

	g.generateStructDesc(out, s)

	// Members are public for -java, private for -javabean
	members := s.Members()

	out.WriteString("\n")

	g.generateFieldDescs(out, s)

	out.WriteString("\n")

	for _, m := range members {
		out.WriteString(g.indent() + "private ")
		out.WriteString(g.declareField(m, false) + "\n")
	}

	// isset data
	if len(members) > 0 {
		out.WriteString("\n")

		out.WriteString(g.indent() + "// isset id assignments\n")

		i := 0
		for _, m := range members {
			if !typeCanBeNull(m.Type()) {
				out.WriteString(g.indent() + "private static final int " + issetFieldID(m) + " = " + itoa(int64(i)) + ";\n")
				i++
			}
		}

		if i > 0 {
			out.WriteString(g.indent() + "private boolean[] __isset_vector = new boolean[" + itoa(int64(i)) + "];\n")
		}

		out.WriteString("\n")
	}

	allOptionalMembers := true

	// Default constructor
	out.WriteString(g.indent() + "public " + s.Name() + "() {\n")
	g.indentUp()
	for _, m := range members {
		t := trueType(m.Type())
		if m.Value() != nil {
			g.printConstValue(out, "this."+m.Name(), t, m.Value(), true, true)
		}
		if m.Req() != sema.Optional {
			allOptionalMembers = false
		}
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	if len(members) > 0 && !allOptionalMembers {
		// Full constructor for all fields
		out.WriteString(g.indent() + "public " + s.Name() + "(\n")
		g.indentUp()
		first := true
		for _, m := range members {
			if m.Req() != sema.Optional {
				if !first {
					out.WriteString(",\n")
				}
				first = false
				out.WriteString(g.indent() + g.tn(m.Type()) + " " + m.Name())
			}
		}
		out.WriteString(")\n")
		g.indentDown()
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		out.WriteString(g.indent() + "this();\n")
		for _, m := range members {
			if m.Req() != sema.Optional {
				out.WriteString(g.indent() + "this." + m.Name() + " = " + m.Name() + ";\n")
				g.issetSet(out, m)
			}
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}

	// copy constructor
	out.WriteString(g.indent() + "/**\n")
	out.WriteString(g.indent() + " * Performs a deep copy on <i>other</i>.\n")
	out.WriteString(g.indent() + " */\n")
	out.WriteString(g.indent() + "public " + s.Name() + "(" + s.Name() + " other) {\n")
	g.indentUp()

	if g.hasBitVector(s) {
		out.WriteString(g.indent() + "System.arraycopy(other.__isset_vector, 0, __isset_vector, 0, other.__isset_vector.length);\n")
	}

	for _, m := range members {
		fieldName := m.Name()
		// The declared (possibly typedef) type, not the resolved type:
		// t_javame_generator::generate_java_struct_definition checks
		// type->is_container() on field->get_type() directly, so a
		// typedef'd container is copied through the non-container path
		// below, matching that.
		t := m.Type()
		canBeNull := typeCanBeNull(t)

		if canBeNull {
			out.WriteString(g.indent() + "if (other." + g.issetCheck(m) + ") {\n")
			g.indentUp()
		}

		if t.IsContainer() {
			g.generateDeepCopyContainer(out, "other", fieldName, "__this__"+fieldName, t)
			out.WriteString(g.indent() + "this." + fieldName + " = __this__" + fieldName + ";\n")
		} else {
			out.WriteString(g.indent() + "this." + fieldName + " = ")
			g.generateDeepCopyNonContainer(out, "other."+fieldName, fieldName, t)
			out.WriteString(";\n")
		}

		if canBeNull {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	// clone method, so that you can deep copy an object when you don't know its class.
	out.WriteString(g.indent() + "public " + s.Name() + " deepCopy() {\n")
	out.WriteString(g.indent() + "  return new " + s.Name() + "(this);\n")
	out.WriteString(g.indent() + "}\n\n")

	g.generateJavaStructClear(out, s)

	g.generateJavaBeanBoilerplate(out, s)
	g.generateGenericFieldGettersSetters(out, s)

	g.generateJavaStructEquality(out, s)
	g.generateJavaStructCompareTo(out, s)

	g.generateJavaStructReader(out, s)
	if isResult {
		g.generateJavaStructResultWriter(out, s)
	} else {
		g.generateJavaStructWriter(out, s)
	}
	g.generateJavaStructToString(out, s)
	g.generateJavaValidator(out, s)
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateJavaStructEquality is generate_java_struct_equality: equals
// methods and a hashCode method for a structure.
func (g *Generator) generateJavaStructEquality(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public boolean equals(Object that) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "if (that == null)\n" + g.indent() + "  return false;\n" +
		g.indent() + "if (that instanceof " + s.Name() + ")\n" + g.indent() +
		"  return this.equals((" + s.Name() + ")that);\n" + g.indent() +
		"return false;\n")
	g.scopeDown(out)
	out.WriteString("\n")

	out.WriteString(g.indent() + "public boolean equals(" + s.Name() + " that) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "if (that == null)\n" + g.indent() + "  return false;\n" +
		g.indent() + "if (this == that)\n" + g.indent() + "  return true;\n")

	for _, m := range s.Members() {
		out.WriteString("\n")

		t := trueType(m.Type())
		// Most existing Thrift code does not use isset or optional/required,
		// so we treat "default" fields as required.
		isOptional := m.Req() == sema.Optional
		canBeNull := typeCanBeNull(t)
		name := m.Name()

		thisPresent := "true"
		thatPresent := "true"
		var unequal string

		if isOptional || canBeNull {
			thisPresent += " && this." + g.issetCheck(m)
			thatPresent += " && that." + g.issetCheck(m)
		}

		out.WriteString(g.indent() + "boolean this_present_" + name + " = " + thisPresent + ";\n" +
			g.indent() + "boolean that_present_" + name + " = " + thatPresent + ";\n" +
			g.indent() + "if (this_present_" + name + " || that_present_" + name + ") {\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (!(this_present_" + name + " && that_present_" + name + "))\n" +
			g.indent() + "  return false;\n")

		if t.IsBinary() {
			unequal = "TBaseHelper.compareTo(this." + name + ", that." + name + ") != 0"
		} else if canBeNull {
			unequal = "!this." + name + ".equals(that." + name + ")"
		} else {
			unequal = "this." + name + " != that." + name
		}

		out.WriteString(g.indent() + "if (" + unequal + ")\n" + g.indent() + "  return false;\n")

		g.scopeDown(out)
	}
	out.WriteString("\n")
	out.WriteString(g.indent() + "return true;\n")
	g.scopeDown(out)
	out.WriteString("\n")

	out.WriteString(g.indent() + "public int hashCode() {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return 0;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateJavaStructCompareTo is generate_java_struct_compare_to. Note
// that the "TypeName other = (TypeName)otherObject;" line is written
// without a trailing newline in the C++ source, so the next line's
// indentation lands mid-line; this port reproduces that exactly.
func (g *Generator) generateJavaStructCompareTo(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public int compareTo(Object otherObject) {\n")
	g.indentUp()

	out.WriteString(g.indent() + "if (!getClass().equals(otherObject.getClass())) {\n")
	out.WriteString(g.indent() + "  return getClass().getName().compareTo(otherObject.getClass().getName());\n")
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + g.tn(s) + " other = (" + g.tn(s) + ")otherObject;")

	out.WriteString(g.indent() + "int lastComparison = 0;\n")
	out.WriteString("\n")

	for _, m := range s.Members() {
		out.WriteString(g.indent() + "lastComparison = TBaseHelper.compareTo(" + g.issetCheck(m) + ", other." + g.issetCheck(m) + ");\n")
		out.WriteString(g.indent() + "if (lastComparison != 0) {\n")
		out.WriteString(g.indent() + "  return lastComparison;\n")
		out.WriteString(g.indent() + "}\n")

		out.WriteString(g.indent() + "if (" + g.issetCheck(m) + ") {\n")
		if m.Type().IsStruct() || m.Type().IsXception() {
			out.WriteString(g.indent() + "  lastComparison = this." + m.Name() + ".compareTo(other." + m.Name() + ");\n")
		} else {
			out.WriteString(g.indent() + "  lastComparison = TBaseHelper.compareTo(this." + m.Name() + ", other." + m.Name() + ");\n")
		}
		out.WriteString(g.indent() + "  if (lastComparison != 0) {\n")
		out.WriteString(g.indent() + "    return lastComparison;\n")
		out.WriteString(g.indent() + "  }\n")
		out.WriteString(g.indent() + "}\n")
	}

	out.WriteString(g.indent() + "return 0;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateJavaStructReader is generate_java_struct_reader: reads all the
// fields of the struct.
func (g *Generator) generateJavaStructReader(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public void read(TProtocol iprot) throws TException {\n")
	g.indentUp()

	out.WriteString(g.indent() + "TField field;\n")
	out.WriteString(g.indent() + "iprot.incrementRecursionDepth();\n")
	out.WriteString(g.indent() + "try {\n")
	g.indentUp()
	out.WriteString(g.indent() + "iprot.readStructBegin();\n")

	out.WriteString(g.indent() + "while (true)\n")
	g.scopeUp(out)

	out.WriteString(g.indent() + "field = iprot.readFieldBegin();\n")

	out.WriteString(g.indent() + "if (field.type == TType.STOP) { \n")
	g.indentUp()
	out.WriteString(g.indent() + "break;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	out.WriteString(g.indent() + "switch (field.id) {\n")
	g.indentUp()

	for _, f := range s.Members() {
		out.WriteString(g.indent() + "case " + itoa(int64(f.Key())) + ": // " + constantName(f.Name()) + "\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (field.type == " + g.typeToEnum(f.Type()) + ") {\n")
		g.indentUp()

		g.generateDeserializeField(out, f, "this.")
		g.issetSet(out, f)
		g.indentDown()
		out.WriteString(g.indent() + "} else { \n" + g.indent() + "  TProtocolUtil.skip(iprot, field.type);\n" + g.indent() + "}\n" + g.indent() + "break;\n")
		g.indentDown()
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  TProtocolUtil.skip(iprot, field.type);\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	out.WriteString(g.indent() + "iprot.readFieldEnd();\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	out.WriteString(g.indent() + "iprot.readStructEnd();\n")

	// performs various checks (e.g. check that all required fields are set)
	out.WriteString(g.indent() + "validate();\n")

	g.indentDown()
	out.WriteString(g.indent() + "} finally {\n")
	g.indentUp()
	out.WriteString(g.indent() + "iprot.decrementRecursionDepth();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateJavaValidator is generate_java_validator: performs various
// checks (e.g. check that all required fields are set).
func (g *Generator) generateJavaValidator(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public void validate() throws TException {\n")
	g.indentUp()

	out.WriteString(g.indent() + "// check for required fields\n")
	for _, f := range s.Members() {
		if f.Req() == sema.Required {
			out.WriteString(g.indent() + "if (!" + g.issetCheck(f) + ") {\n" + g.indent() +
				"  throw new TProtocolException(\"Required field '" + f.Name() +
				"' is unset! Struct:\" + toString());\n" + g.indent() + "}\n\n")
		}
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateJavaStructWriter is generate_java_struct_writer: writes all
// the fields of the struct.
func (g *Generator) generateJavaStructWriter(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public void write(TProtocol oprot) throws TException {\n")
	g.indentUp()

	fields := s.SortedMembers()

	// performs various checks (e.g. check that all required fields are set)
	out.WriteString(g.indent() + "validate();\n\n")

	out.WriteString(g.indent() + "oprot.incrementRecursionDepth();\n")
	out.WriteString(g.indent() + "try {\n")
	g.indentUp()
	out.WriteString(g.indent() + "oprot.writeStructBegin(STRUCT_DESC);\n")

	for _, f := range fields {
		nullAllowed := typeCanBeNull(f.Type())
		if nullAllowed {
			out.WriteString(g.indent() + "if (this." + f.Name() + " != null) {\n")
			g.indentUp()
		}
		optional := f.Req() == sema.Optional
		if optional {
			out.WriteString(g.indent() + "if (" + g.issetCheck(f) + ") {\n")
			g.indentUp()
		}

		out.WriteString(g.indent() + "oprot.writeFieldBegin(" + constantName(f.Name()) + "_FIELD_DESC);\n")

		// Write field contents
		g.generateSerializeField(out, f, "this.")

		// Write field closer
		out.WriteString(g.indent() + "oprot.writeFieldEnd();\n")

		if optional {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		if nullAllowed {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
	}
	// Write the struct map
	out.WriteString(g.indent() + "oprot.writeFieldStop();\n" + g.indent() + "oprot.writeStructEnd();\n")

	g.indentDown()
	out.WriteString(g.indent() + "} finally {\n")
	g.indentUp()
	out.WriteString(g.indent() + "oprot.decrementRecursionDepth();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateJavaStructResultWriter is generate_java_struct_result_writer:
// writes all the fields of the struct, which is a function result. These
// fields are only written if they are set in the isset array, and only
// one of them can be set at a time.
func (g *Generator) generateJavaStructResultWriter(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public void write(TProtocol oprot) throws TException {\n")
	g.indentUp()

	fields := s.SortedMembers()

	out.WriteString(g.indent() + "oprot.incrementRecursionDepth();\n")
	out.WriteString(g.indent() + "try {\n")
	g.indentUp()
	out.WriteString(g.indent() + "oprot.writeStructBegin(STRUCT_DESC);\n")

	first := true
	for _, f := range fields {
		if first {
			first = false
			out.WriteString("\n" + g.indent() + "if ")
		} else {
			out.WriteString(" else if ")
		}

		out.WriteString("(this." + g.issetCheck(f) + ") {\n")

		g.indentUp()

		out.WriteString(g.indent() + "oprot.writeFieldBegin(" + constantName(f.Name()) + "_FIELD_DESC);\n")

		// Write field contents
		g.generateSerializeField(out, f, "this.")

		// Write field closer
		out.WriteString(g.indent() + "oprot.writeFieldEnd();\n")

		g.indentDown()
		out.WriteString(g.indent() + "}")
	}
	// Write the struct map
	out.WriteString("\n" + g.indent() + "oprot.writeFieldStop();\n" + g.indent() + "oprot.writeStructEnd();\n")

	g.indentDown()
	out.WriteString(g.indent() + "} finally {\n")
	g.indentUp()
	out.WriteString(g.indent() + "oprot.decrementRecursionDepth();\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateReflectionGetters is generate_reflection_getters.
func (g *Generator) generateReflectionGetters(out *strings.Builder, t sema.Type, fieldName, capName string) {
	out.WriteString(g.indent() + "case " + constantName(fieldName) + ":\n")
	g.indentUp()

	if t.IsBaseType() && !t.IsString() {
		bt := t.(*sema.BaseType)
		getter := "get"
		if bt.Base() == sema.TypeBool {
			getter = "is"
		}
		out.WriteString(g.indent() + "return new " + g.typeName(t, true) + "(" + getter + capName + "());\n\n")
	} else {
		out.WriteString(g.indent() + "return get" + capName + "();\n\n")
	}

	g.indentDown()
}

// generateReflectionSetters is generate_reflection_setters.
func (g *Generator) generateReflectionSetters(out *strings.Builder, t sema.Type, fieldName, capName string) {
	out.WriteString(g.indent() + "case " + constantName(fieldName) + ":\n")
	g.indentUp()
	out.WriteString(g.indent() + "if (value == null) {\n")
	out.WriteString(g.indent() + "  unset" + getCapName(fieldName) + "();\n")
	out.WriteString(g.indent() + "} else {\n")
	out.WriteString(g.indent() + "  set" + capName + "((" + g.typeName(t, true) + ")value);\n")
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "break;\n\n")

	g.indentDown()
}

// generateGenericFieldGettersSetters is
// generate_generic_field_getters_setters. In the C++ source it builds
// both a getter and setter body into local ostringstreams and then never
// writes them anywhere (out is unused, cast to void), so despite the
// balanced indent_up()/indent_down() calls this has no effect on the
// generated file; this port keeps that no-op shape for fidelity.
func (g *Generator) generateGenericFieldGettersSetters(out *strings.Builder, s *sema.Struct) {
	_ = out
	var getterStream, setterStream strings.Builder

	for _, f := range s.Members() {
		t := trueType(f.Type())
		fieldName := f.Name()
		capName := getCapName(fieldName)

		g.indentUp()
		g.generateReflectionSetters(&setterStream, t, fieldName, capName)
		g.generateReflectionGetters(&getterStream, t, fieldName, capName)
		g.indentDown()
	}
}

// generateJavaBeanBoilerplate is generate_java_bean_boilerplate: a set of
// Java Bean boilerplate functions (setters, getters, etc.) for the given
// struct.
func (g *Generator) generateJavaBeanBoilerplate(out *strings.Builder, s *sema.Struct) {
	for _, f := range s.Members() {
		t := trueType(f.Type())
		fieldName := f.Name()
		capName := getCapName(fieldName)

		if t.IsContainer() {
			// Method to return the size of the collection
			out.WriteString(g.indent() + "public int get" + capName)
			out.WriteString(getCapName("size() {") + "\n")

			g.indentUp()
			out.WriteString(g.indent() + "return (this." + fieldName + " == null) ? 0 : this." + fieldName + ".size();\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")
		}

		if t.IsSet() || t.IsList() {
			var elementType sema.Type
			if t.IsSet() {
				elementType = t.(*sema.Set).ElemType()
			} else {
				elementType = t.(*sema.List).ElemType()
			}

			// Iterator getter for sets and lists
			out.WriteString(g.indent() + "public Enumeration get" + capName)
			out.WriteString(getCapName("Enumeration() {") + "\n")

			g.indentUp()
			out.WriteString(g.indent() + "return (this." + fieldName + " == null) ? null : this." + fieldName + ".elements();\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")

			// Add to set or list, create if the set/list is null
			out.WriteString(g.indent())
			out.WriteString("public void add" + getCapName("to"))
			out.WriteString(capName + "(" + g.tn(elementType) + " elem) {\n")

			g.indentUp()
			out.WriteString(g.indent() + "if (this." + fieldName + " == null) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "this." + fieldName + " = new " + g.typeName(t, false) + "();\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
			if t.IsSet() {
				out.WriteString(g.indent() + "this." + fieldName + ".put(" + boxType(elementType, "elem") + ", " + boxType(elementType, "elem") + ");\n")
			} else {
				out.WriteString(g.indent() + "this." + fieldName + ".addElement(" + boxType(elementType, "elem") + ");\n")
			}
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")

		} else if t.IsMap() {
			// Put to map
			m := t.(*sema.Map)
			keyType := m.KeyType()
			valType := m.ValType()

			out.WriteString(g.indent())
			out.WriteString("public void putTo" + capName + "(" + g.typeName(keyType, true) + " key, " + g.typeName(valType, true) + " val) {\n")

			g.indentUp()
			out.WriteString(g.indent() + "if (this." + fieldName + " == null) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "this." + fieldName + " = new " + g.typeName(t, false) + "();\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
			out.WriteString(g.indent() + "this." + fieldName + ".put(key, val);\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")
		}

		// Simple getter
		g.javaDocField(out, f)
		out.WriteString(g.indent() + "public " + g.tn(t))
		if t.IsBaseType() && t.(*sema.BaseType).Base() == sema.TypeBool {
			out.WriteString(" is")
		} else {
			out.WriteString(" get")
		}
		out.WriteString(capName + "() {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return this." + fieldName + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		// Simple setter
		g.javaDocField(out, f)
		out.WriteString(g.indent() + "public ")
		out.WriteString("void")
		out.WriteString(" set" + capName + "(" + g.tn(t) + " " + fieldName + ") {\n")
		g.indentUp()
		out.WriteString(g.indent() + "this." + fieldName + " = " + fieldName + ";\n")
		g.issetSet(out, f)

		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		// Unsetter
		out.WriteString(g.indent() + "public void unset" + capName + "() {\n")
		g.indentUp()
		if typeCanBeNull(t) {
			out.WriteString(g.indent() + "this." + fieldName + " = null;\n")
		} else {
			out.WriteString(g.indent() + "__isset_vector[" + issetFieldID(f) + "] = false;\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		// isSet method
		out.WriteString(g.indent() + "/** Returns true if field " + fieldName + " is set (has been assigned a value) and false otherwise */\n")
		out.WriteString(g.indent() + "public boolean is" + getCapName("set") + capName + "() {\n")
		g.indentUp()
		if typeCanBeNull(t) {
			out.WriteString(g.indent() + "return this." + fieldName + " != null;\n")
		} else {
			out.WriteString(g.indent() + "return __isset_vector[" + issetFieldID(f) + "];\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		out.WriteString(g.indent() + "public void set" + capName + getCapName("isSet") + "(boolean value) {\n")
		g.indentUp()
		if typeCanBeNull(t) {
			out.WriteString(g.indent() + "if (!value) {\n")
			out.WriteString(g.indent() + "  this." + fieldName + " = null;\n")
			out.WriteString(g.indent() + "}\n")
		} else {
			out.WriteString(g.indent() + "__isset_vector[" + issetFieldID(f) + "] = value;\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}
}

// generateJavaStructToString is generate_java_struct_tostring: a
// toString() method for the given struct.
func (g *Generator) generateJavaStructToString(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public String toString() {\n")
	g.indentUp()

	out.WriteString(g.indent() + "StringBuffer sb = new StringBuffer(\"" + s.Name() + "(\");\n")
	out.WriteString(g.indent() + "boolean first = true;\n\n")

	first := true
	for _, f := range s.Members() {
		couldBeUnset := f.Req() == sema.Optional
		if couldBeUnset {
			out.WriteString(g.indent() + "if (" + g.issetCheck(f) + ") {\n")
			g.indentUp()
		}

		if !first {
			out.WriteString(g.indent() + "if (!first) sb.append(\", \");\n")
		}
		out.WriteString(g.indent() + "sb.append(\"" + f.Name() + ":\");\n")
		canBeNull := typeCanBeNull(f.Type())
		if canBeNull {
			out.WriteString(g.indent() + "if (this." + f.Name() + " == null) {\n")
			out.WriteString(g.indent() + "  sb.append(\"null\");\n")
			out.WriteString(g.indent() + "} else {\n")
			g.indentUp()
		}

		if f.Type().IsBinary() {
			out.WriteString(g.indent() + "TBaseHelper.toString(this." + f.Name() + ", sb);\n")
		} else {
			out.WriteString(g.indent() + "sb.append(this." + f.Name() + ");\n")
		}

		if canBeNull {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		out.WriteString(g.indent() + "first = false;\n")

		if couldBeUnset {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		first = false
	}
	out.WriteString(g.indent() + "sb.append(\")\");\n" + g.indent() + "return sb.toString();\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// getJavaTypeString is get_java_type_string, and generateFieldValueMetaData
// below is generate_field_value_meta_data. Both are unreachable in
// t_javame_generator.cc: generate_field_value_meta_data is declared and
// defined but never invoked from anywhere else in the file, so this port
// keeps them for fidelity even though they produce no output.
func (g *Generator) getJavaTypeString(t sema.Type) string {
	if t.IsList() {
		return "TType.LIST"
	} else if t.IsMap() {
		return "TType.MAP"
	} else if t.IsSet() {
		return "TType.SET"
	} else if t.IsStruct() || t.IsXception() {
		return "TType.STRUCT"
	} else if t.IsEnum() {
		return "TType.ENUM"
	} else if t.IsTypedef() {
		return g.getJavaTypeString(t.(*sema.Typedef).Type())
	} else if t.IsBaseType() {
		switch baseOf(t) {
		case sema.TypeVoid:
			return "TType.VOID"
		case sema.TypeString:
			return "TType.STRING"
		case sema.TypeBool:
			return "TType.BOOL"
		case sema.TypeI8:
			return "TType.BYTE"
		case sema.TypeI16:
			return "TType.I16"
		case sema.TypeI32:
			return "TType.I32"
		case sema.TypeI64:
			return "TType.I64"
		case sema.TypeDouble:
			return "TType.DOUBLE"
		default:
			emit.Throw("Unknown thrift type \"%s\" passed to t_javame_generator::get_java_type_string!", t.Name())
		}
	} else {
		emit.Throw("Unknown thrift type \"%s\" passed to t_javame_generator::get_java_type_string!", t.Name())
	}
	return ""
}

func (g *Generator) generateFieldValueMetaData(out *strings.Builder, t sema.Type) {
	out.WriteString("\n")
	g.indentUp()
	g.indentUp()
	if t.IsStruct() || t.IsXception() {
		out.WriteString(g.indent() + "new StructMetaData(TType.STRUCT, " + g.tn(t) + ".class")
	} else if t.IsContainer() {
		if t.IsList() {
			out.WriteString(g.indent() + "new ListMetaData(TType.LIST, ")
			g.generateFieldValueMetaData(out, t.(*sema.List).ElemType())
		} else if t.IsSet() {
			out.WriteString(g.indent() + "new SetMetaData(TType.SET, ")
			// The C++ source casts to t_list* here even in the is_set()
			// branch (reading get_elem_type() through the wrong class);
			// harmless because this function is unreachable, so this
			// port uses the correct accessor.
			g.generateFieldValueMetaData(out, t.(*sema.Set).ElemType())
		} else { // map
			out.WriteString(g.indent() + "new MapMetaData(TType.MAP, ")
			m := t.(*sema.Map)
			g.generateFieldValueMetaData(out, m.KeyType())
			out.WriteString(", ")
			g.generateFieldValueMetaData(out, m.ValType())
		}
	} else if t.IsEnum() {
		out.WriteString(g.indent() + "new EnumMetaData(TType.ENUM, " + g.tn(t) + ".class")
	} else {
		out.WriteString(g.indent() + "new FieldValueMetaData(" + g.getJavaTypeString(t))
		if t.IsTypedef() {
			out.WriteString(", \"" + t.(*sema.Typedef).Symbolic() + "\"")
		}
	}
	out.WriteString(")")
	g.indentDown()
	g.indentDown()
}

func (g *Generator) generateStructDesc(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "private static final TStruct STRUCT_DESC = new TStruct(\"" + s.Name() + "\");\n")
}

func (g *Generator) generateFieldDescs(out *strings.Builder, s *sema.Struct) {
	for _, m := range s.Members() {
		out.WriteString(g.indent() + "private static final TField " + constantName(m.Name()) +
			"_FIELD_DESC = new TField(\"" + m.Name() + "\", " + g.typeToEnum(m.Type()) + ", " +
			"(short)" + itoa(int64(m.Key())) + ");\n")
	}
}

func (g *Generator) hasBitVector(s *sema.Struct) bool {
	for _, m := range s.Members() {
		if !typeCanBeNull(trueType(m.Type())) {
			return true
		}
	}
	return false
}

// generateJavaStructClear is generate_java_struct_clear.
func (g *Generator) generateJavaStructClear(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "public void clear() {\n")

	g.indentUp()
	for _, m := range s.Members() {
		t := trueType(m.Type())
		if m.Value() != nil {
			g.printConstValue(out, "this."+m.Name(), t, m.Value(), true, true)
		} else {
			if typeCanBeNull(t) {
				out.WriteString(g.indent() + "this." + m.Name() + " = null;\n")
			} else {
				// must be a base type
				// means it also needs to be explicitly unset
				out.WriteString(g.indent() + "set" + getCapName(m.Name()) + getCapName("isSet") + "(false);\n")
				switch baseOf(t) {
				case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
					out.WriteString(g.indent() + "this." + m.Name() + " = 0;\n")
				case sema.TypeDouble:
					out.WriteString(g.indent() + "this." + m.Name() + " = 0.0;\n")
				case sema.TypeBool:
					out.WriteString(g.indent() + "this." + m.Name() + " = false;\n")
				}
			}
		}
	}
	g.indentDown()

	out.WriteString(g.indent() + "}\n\n")
}
