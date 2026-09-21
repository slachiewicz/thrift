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

package haxe

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateTypedef is t_haxe_generator::generate_typedef: this is not done
// in haxe, since it does not support arbitrary name replacements, and it'd
// be a wacky waste of overhead to make wrapper classes.
func (g *generator) generateTypedef(ttypedef *sema.Typedef) {}

// generateEnum is t_haxe_generator::generate_enum: enums are a class with
// a set of static constants.
func (g *generator) generateEnum(tenum *sema.Enum) {
	fEnumName := g.packageDir + "/" + g.makeHaxeUserTypeName(tenum.Name()) + ".hx"
	var f strings.Builder

	f.WriteString(g.autogenComment() + g.haxePackage() + ";\n\n")

	f.WriteString("import org.apache.thrift.helper.*;\n\n")

	g.generateRttiDecoration(&f)
	g.generateMacroDecoration(&f)
	g.indentRaw(&f, "class "+g.makeHaxeUserTypeName(tenum.Name())+" ")
	g.scopeUp(&f)

	constants := tenum.Constants()
	for _, c := range constants {
		g.line(&f, "public static inline var "+c.Name()+" : Int = "+itoa32(c.Value())+";")
	}

	f.WriteString("\n")

	g.indentRaw(&f, "public static var VALID_VALUES = { new IntSet( [")
	g.indentUp()
	firstValue := true
	for _, c := range constants {
		if !firstValue {
			f.WriteString(", ")
		}
		f.WriteString(c.Name())
		firstValue = false
	}
	g.indentDown()
	f.WriteString("]); };\n")

	g.indentRaw(&f, "public static var VALUES_TO_NAMES = { [")
	g.indentUp()
	firstValue = true
	for _, c := range constants {
		if !firstValue {
			f.WriteString(",")
		}
		f.WriteString("\n")
		g.indentRaw(&f, c.Name()+" => \""+c.Name()+"\"")
		firstValue = false
	}
	f.WriteString("\n")
	g.indentDown()
	g.line(&f, "]; };")

	g.scopeDown(&f) // end class

	emit.WriteFile(fEnumName, f.String())
}

// generateStruct is t_haxe_generator::generate_struct.
func (g *generator) generateStruct(tstruct *sema.Struct) {
	g.generateHaxeStruct(tstruct, false, false)
}

// generateXception is t_haxe_generator::generate_xception: exceptions are
// structs, but they inherit from Exception.
func (g *generator) generateXception(txception *sema.Struct) {
	g.generateHaxeStruct(txception, true, false)
}

// generateHaxeStruct is t_haxe_generator::generate_haxe_struct.
func (g *generator) generateHaxeStruct(tstruct *sema.Struct, isException, isResult bool) {
	fStructName := g.packageDir + "/" + g.makeHaxeUserTypeName(tstruct.Name()) + ".hx"
	var f strings.Builder

	f.WriteString(g.autogenComment() + g.haxePackage() + ";\n")

	f.WriteString("\n")

	var imports strings.Builder
	g.haxeThriftGenImportsStruct(tstruct, &imports, g.makeHaxeUserTypeName(tstruct.Name()))

	f.WriteString(g.haxeTypeImports() + g.haxeThriftImports() + imports.String() + "\n")

	g.generateHaxeStructDefinition(&f, tstruct, isException, isResult)

	emit.WriteFile(fStructName, f.String())
}

// generateHaxeStructDefinition is t_haxe_generator::generate_haxe_struct_definition.
func (g *generator) generateHaxeStructDefinition(out *strings.Builder, tstruct *sema.Struct, isException, isResult bool) {
	g.generateHaxeDoc(out, tstruct)

	clsname := g.makeHaxeUserTypeName(tstruct.Name())

	g.generateRttiDecoration(out)
	g.generateMacroDecoration(out)
	g.indentRaw(out, "class "+clsname+" ")

	if isException {
		out.WriteString("extends TException ")
	}
	out.WriteString("implements TBase {\n\n")
	g.indentUp()

	g.line(out, "static var STRUCT_DESC = { new TStruct(\""+tstruct.Name()+"\"); };")

	members := tstruct.Members()

	for _, m := range members {
		g.line(out, "static var "+constantName(m.Name())+"_FIELD_DESC = { new TField(\""+m.Name()+"\", "+g.typeToEnum(m.Type())+", "+itoa32(m.Key())+"); };")
	}
	out.WriteString("\n")

	for _, m := range members {
		g.generateHaxeDoc(out, m)
		g.line(out, "@:isVar")
		g.line(out, "public var "+escapeHaxeKeyword(m.Name())+"(get,set) : "+getCapName(g.typeName(m.Type()))+";")
	}

	out.WriteString("\n")

	for _, m := range members {
		g.line(out, "inline static var "+upcaseString(m.Name())+"_FIELD_ID : Int = "+itoa32(m.Key())+";")
	}

	out.WriteString("\n")

	// Inner Isset class
	if len(members) > 0 {
		for _, m := range members {
			if !typeCanBeNull(m.Type()) {
				g.line(out, "private var __isset_"+escapeHaxeKeyword(m.Name())+" : Bool = false;")
			}
		}
	}

	out.WriteString("\n")

	// Static initializer to populate global class to struct metadata map:
	// dead code in the C++ source (guarded by `if (false)`, "TODO:
	// reactivate when needed"), so nothing is emitted here.

	// Default constructor
	g.line(out, "public function new() {")
	g.indentUp()
	if isException {
		g.line(out, "super();")
	}
	for _, m := range members {
		if m.Value() != nil {
			g.indentRaw(out, "this."+escapeHaxeKeyword(m.Name())+" = ")
			g.renderConstValue(out, m.Type(), m.Value())
			out.WriteString(";\n")
		}
	}
	g.indentDown()
	g.line(out, "}")
	out.WriteString("\n")

	g.generatePropertyGettersSetters(out, tstruct)
	g.generateGenericFieldGettersSetters(out, tstruct)
	g.generateGenericIssetMethod(out, tstruct)

	g.generateHaxeStructReader(out, tstruct)
	if isResult {
		g.generateHaxeStructResultWriter(out, tstruct)
	} else {
		g.generateHaxeStructWriter(out, tstruct)
	}
	g.generateHaxeStructToString(out, tstruct, isException)
	g.generateHaxeValidator(out, tstruct)
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateHaxeStructReader is t_haxe_generator::generate_haxe_struct_reader:
// generates a function to read all the fields of the struct.
func (g *generator) generateHaxeStructReader(out *strings.Builder, tstruct *sema.Struct) {
	g.line(out, "public function read( iprot : TProtocol) : Void {")
	g.indentUp()

	fields := tstruct.Members()

	g.line(out, "iprot.IncrementRecursionDepth();")
	g.line(out, "try")
	g.scopeUp(out)

	// Declare stack tmp variables and read struct header
	g.line(out, "var field : TField;")
	g.line(out, "iprot.readStructBegin();")

	// Loop over reading in fields
	g.line(out, "while (true)")
	g.scopeUp(out)

	// Read beginning field marker
	g.line(out, "field = iprot.readFieldBegin();")

	// Check for field STOP marker and break
	g.line(out, "if (field.type == TType.STOP) { ")
	g.indentUp()
	g.line(out, "break;")
	g.indentDown()
	g.line(out, "}")

	// Switch statement on the field we are reading
	g.line(out, "switch (field.id)")

	g.scopeUp(out)

	// Generate deserialization code for known cases
	for _, f := range fields {
		g.line(out, "case "+upcaseString(f.Name())+"_FIELD_ID:")
		g.indentUp()
		g.line(out, "if (field.type == "+g.typeToEnum(f.Type())+") {")
		g.indentUp()

		g.generateDeserializeField(out, f, "this.")
		g.generateIssetSet(out, f)
		g.indentDown()
		g.indentRaw(out, "} else { \n")
		g.indentRaw(out, "  TProtocolUtil.skip(iprot, field.type);\n")
		g.line(out, "}")
		g.indentDown()
	}

	// In the default case we skip the field
	g.indentRaw(out, "default:\n")
	g.indentRaw(out, "  TProtocolUtil.skip(iprot, field.type);\n")

	g.scopeDown(out)

	// Read field end marker
	g.line(out, "iprot.readFieldEnd();")

	g.scopeDown(out)

	g.indentRaw(out, "iprot.readStructEnd();\n\n")

	g.line(out, "iprot.DecrementRecursionDepth();")
	g.scopeDown(out)
	g.line(out, "catch(e:Dynamic)")
	g.scopeUp(out)
	g.line(out, "iprot.DecrementRecursionDepth();")
	g.line(out, "throw e;")
	g.scopeDown(out)

	// check for required fields of primitive type
	// (which can be checked here but not in the general validate method)
	out.WriteString("\n" + g.indent() + "// check for required fields of primitive type, which can't be checked in the validate method\n")
	for _, f := range fields {
		if f.Req() == sema.Required && !typeCanBeNull(f.Type()) {
			out.WriteString(g.indent() + "if (!__isset_" + escapeHaxeKeyword(f.Name()) + ") {\n" + g.indent() +
				"  throw new TProtocolException(TProtocolException.UNKNOWN, \"Required field '" +
				f.Name() + "' was not found in serialized data! Struct: \" + toString());\n" + g.indent() +
				"}\n")
		}
	}

	// performs various checks (e.g. check that all required fields are set)
	g.line(out, "validate();")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateHaxeValidator is t_haxe_generator::generate_haxe_validator: a
// method to perform various checks (e.g. check that all required fields
// are set).
func (g *generator) generateHaxeValidator(out *strings.Builder, tstruct *sema.Struct) {
	g.line(out, "public function validate() : Void {")
	g.indentUp()

	fields := tstruct.Members()

	out.WriteString(g.indent() + "// check for required fields\n")
	for _, f := range fields {
		if f.Req() == sema.Required {
			if typeCanBeNull(f.Type()) {
				g.line(out, "if ("+f.Name()+" == null) {")
				g.line(out, "  throw new TProtocolException(TProtocolException.UNKNOWN, \"Required field '"+f.Name()+"' was not present! Struct: \" + toString());")
				g.line(out, "}")
			} else {
				g.line(out, "// alas, we cannot check '"+f.Name()+"' because it's a primitive.")
			}
		}
	}

	// check that fields of type enum have valid values
	out.WriteString(g.indent() + "// check that fields of type enum have valid values\n")
	for _, f := range fields {
		t := f.Type()
		if t.IsEnum() {
			g.line(out, "if ("+generateIssetCheckField(f)+" && !"+getCapName(g.getEnumClassName(t))+".VALID_VALUES.contains("+escapeHaxeKeyword(f.Name())+")){")
			g.indentUp()
			g.line(out, "throw new TProtocolException(TProtocolException.UNKNOWN, \"The field '"+f.Name()+"' has been assigned the invalid value \" + "+escapeHaxeKeyword(f.Name())+");")
			g.indentDown()
			g.line(out, "}")
		}
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateHaxeStructWriter is t_haxe_generator::generate_haxe_struct_writer:
// generates a function to write all the fields of the struct.
func (g *generator) generateHaxeStructWriter(out *strings.Builder, tstruct *sema.Struct) {
	g.line(out, "public function write(oprot:TProtocol) : Void {")
	g.indentUp()

	fields := tstruct.SortedMembers()

	// performs various checks (e.g. check that all required fields are set)
	g.line(out, "validate();")
	g.line(out, "oprot.IncrementRecursionDepth();")
	g.line(out, "try")
	g.scopeUp(out)

	g.line(out, "oprot.writeStructBegin(STRUCT_DESC);")

	for _, f := range fields {
		couldBeUnset := f.Req() == sema.Optional
		if couldBeUnset {
			g.line(out, "if ("+generateIssetCheckField(f)+") {")
			g.indentUp()
		}
		nullAllowed := typeCanBeNull(f.Type())
		if nullAllowed {
			g.line(out, "if (this."+escapeHaxeKeyword(f.Name())+" != null) {")
			g.indentUp()
		}

		g.line(out, "oprot.writeFieldBegin("+constantName(f.Name())+"_FIELD_DESC);")

		// Write field contents
		g.generateSerializeField(out, f, "this.")

		// Write field closer
		g.line(out, "oprot.writeFieldEnd();")

		if nullAllowed {
			g.indentDown()
			g.line(out, "}")
		}
		if couldBeUnset {
			g.indentDown()
			g.line(out, "}")
		}
	}

	g.line(out, "oprot.writeFieldStop();")
	g.line(out, "oprot.writeStructEnd();")

	g.line(out, "oprot.DecrementRecursionDepth();")
	g.scopeDown(out)
	g.line(out, "catch(e:Dynamic)")
	g.scopeUp(out)
	g.line(out, "oprot.DecrementRecursionDepth();")
	g.line(out, "throw e;")
	g.scopeDown(out)

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateHaxeStructResultWriter is
// t_haxe_generator::generate_haxe_struct_result_writer: generates a
// function to write all the fields of the struct, which is a function
// result. These fields are only written if they are set in the Isset
// array, and only one of them can be set at a time.
func (g *generator) generateHaxeStructResultWriter(out *strings.Builder, tstruct *sema.Struct) {
	g.line(out, "public function write(oprot:TProtocol) : Void {")
	g.indentUp()

	fields := tstruct.SortedMembers()

	g.line(out, "oprot.IncrementRecursionDepth();")
	g.line(out, "try")
	g.scopeUp(out)

	g.line(out, "oprot.writeStructBegin(STRUCT_DESC);")

	first := true
	for _, f := range fields {
		if first {
			first = false
			out.WriteString("\n" + g.indent() + "if ")
		} else {
			out.WriteString(" else if ")
		}

		out.WriteString("(this." + generateIssetCheckField(f) + ") {\n")

		g.indentUp()

		g.line(out, "oprot.writeFieldBegin("+constantName(f.Name())+"_FIELD_DESC);")

		// Write field contents
		g.generateSerializeField(out, f, "this.")

		// Write field closer
		g.line(out, "oprot.writeFieldEnd();")

		g.indentDown()
		g.indentRaw(out, "}")
	}

	out.WriteString(g.indent() + "\n")
	g.line(out, "oprot.writeFieldStop();")
	g.line(out, "oprot.writeStructEnd();")

	g.line(out, "oprot.DecrementRecursionDepth();")
	g.scopeDown(out)
	g.line(out, "catch(e:Dynamic)")
	g.scopeUp(out)
	g.line(out, "oprot.DecrementRecursionDepth();")
	g.line(out, "throw e;")
	g.scopeDown(out)

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateReflectionGetters is t_haxe_generator::generate_reflection_getters.
func (g *generator) generateReflectionGetters(out *strings.Builder, origName, fieldName string) {
	g.line(out, "case "+upcaseString(origName)+"_FIELD_ID:")
	g.indentUp()
	g.line(out, "return this."+fieldName+";")
	g.indentDown()
}

// generateReflectionSetters is t_haxe_generator::generate_reflection_setters.
func (g *generator) generateReflectionSetters(out *strings.Builder, origName, fieldName string) {
	g.line(out, "case "+upcaseString(origName)+"_FIELD_ID:")
	g.indentUp()
	g.line(out, "if (value == null) {")
	g.line(out, "  unset"+getCapName(fieldName)+"();")
	g.line(out, "} else {")
	g.line(out, "  this."+fieldName+" = value;")
	out.WriteString(g.indent() + "}\n\n")

	g.indentDown()
}

// generateGenericFieldGettersSetters is
// t_haxe_generator::generate_generic_field_getters_setters.
func (g *generator) generateGenericFieldGettersSetters(out *strings.Builder, tstruct *sema.Struct) {
	var getterStream, setterStream strings.Builder

	// build up the bodies of both the getter and setter at once
	fields := tstruct.Members()
	for _, field := range fields {
		origName := field.Name()
		fieldName := escapeHaxeKeyword(origName)

		g.indentUp()
		g.generateReflectionSetters(&setterStream, origName, fieldName)
		g.generateReflectionGetters(&getterStream, origName, fieldName)
		g.indentDown()
	}

	// create the setter
	g.line(out, "public function setFieldValue(fieldID : Int, value : Dynamic) : Void {")
	g.indentUp()

	if len(fields) > 0 {
		g.line(out, "switch (fieldID) {")
		out.WriteString(setterStream.String())
		g.line(out, "default:")
		g.line(out, "  throw new ArgumentError(\"Field \" + fieldID + \" doesn't exist!\");")
		g.line(out, "}")
	} else {
		g.line(out, "throw new ArgumentError(\"Field \" + fieldID + \" doesn't exist!\");")
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	// create the getter
	g.line(out, "public function getFieldValue(fieldID : Int) : Dynamic {")
	g.indentUp()

	if len(fields) > 0 {
		g.line(out, "switch (fieldID) {")
		out.WriteString(getterStream.String())
		g.line(out, "default:")
		g.line(out, "  throw new ArgumentError(\"Field \" + fieldID + \" doesn't exist!\");")
		g.line(out, "}")
	} else {
		g.line(out, "throw new ArgumentError(\"Field \" + fieldID + \" doesn't exist!\");")
	}

	g.indentDown()

	out.WriteString(g.indent() + "}\n\n")
}

// generateGenericIssetMethod is
// t_haxe_generator::generate_generic_isset_method: creates a generic
// isSet method that takes the field number as argument.
func (g *generator) generateGenericIssetMethod(out *strings.Builder, tstruct *sema.Struct) {
	fields := tstruct.Members()

	out.WriteString(g.indent() + "// Returns true if field corresponding to fieldID is set (has been assigned a value) and false otherwise\n")
	g.line(out, "public function isSet(fieldID : Int) : Bool {")
	g.indentUp()
	if len(fields) > 0 {
		g.line(out, "switch (fieldID) {")

		for _, field := range fields {
			g.line(out, "case "+upcaseString(field.Name())+"_FIELD_ID:")
			g.indentUp()
			g.line(out, "return "+generateIssetCheckField(field)+";")
			g.indentDown()
		}

		g.line(out, "default:")
		g.line(out, "  throw new ArgumentError(\"Field \" + fieldID + \" doesn't exist!\");")
		g.line(out, "}")
	} else {
		g.line(out, "throw new ArgumentError(\"Field \" + fieldID + \" doesn't exist!\");")
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generatePropertyGettersSetters is
// t_haxe_generator::generate_property_getters_setters: generates a set of
// property setters/getters for the given struct.
func (g *generator) generatePropertyGettersSetters(out *strings.Builder, tstruct *sema.Struct) {
	fields := tstruct.Members()
	for _, field := range fields {
		t := sema.TrueType(field.Type())
		fieldName := escapeHaxeKeyword(field.Name())
		capName := getCapName(fieldName)

		// Simple getter
		g.generateHaxeDoc(out, field)
		g.line(out, "public function get_"+fieldName+"() : "+getCapName(g.typeName(t))+" {")
		g.indentUp()
		g.line(out, "return this."+fieldName+";")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		// Simple setter
		g.generateHaxeDoc(out, field)
		g.line(out, "public function set_"+fieldName+"("+fieldName+":"+getCapName(g.typeName(t))+") : "+getCapName(g.typeName(t))+" {")
		g.indentUp()
		g.line(out, "this."+fieldName+" = "+fieldName+";")
		g.generateIssetSet(out, field)
		g.line(out, "return this."+fieldName+";")

		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		// Unsetter
		g.line(out, "public function unset"+capName+"() : Void {")
		g.indentUp()
		if typeCanBeNull(t) {
			g.line(out, "this."+fieldName+" = null;")
		} else {
			g.line(out, "this.__isset_"+fieldName+" = false;")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		// isSet method
		out.WriteString(g.indent() + "// Returns true if field " + field.Name() + " is set (has been assigned a value) and false otherwise\n")
		g.line(out, "public function is"+getCapName("set")+capName+"() : Bool {")
		g.indentUp()
		if typeCanBeNull(t) {
			g.line(out, "return this."+fieldName+" != null;")
		} else {
			g.line(out, "return this.__isset_"+fieldName+";")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}
}

// generateHaxeStructToString is
// t_haxe_generator::generate_haxe_struct_tostring: generates a toString()
// method for the given struct.
func (g *generator) generateHaxeStructToString(out *strings.Builder, tstruct *sema.Struct, isOverride bool) {
	out.WriteString(g.indent() + "public ")
	if isOverride {
		out.WriteString("override ")
	}
	out.WriteString("function toString() : String {\n")
	g.indentUp()

	out.WriteString(g.indent() + "var ret : String = \"" + tstruct.Name() + "(\";\n")
	out.WriteString(g.indent() + "var first : Bool = true;\n\n")

	fields := tstruct.Members()
	first := true
	for _, f := range fields {
		couldBeUnset := f.Req() == sema.Optional
		if couldBeUnset {
			g.line(out, "if ("+generateIssetCheckField(f)+") {")
			g.indentUp()
		}

		if !first {
			g.line(out, "if (!first) ret +=  \", \";")
		}
		g.line(out, "ret += \""+f.Name()+":\";")
		canBeNull := typeCanBeNull(f.Type())
		if canBeNull {
			g.line(out, "if (this."+escapeHaxeKeyword(f.Name())+" == null) {")
			g.line(out, "  ret += \"null\";")
			g.line(out, "} else {")
			g.indentUp()
		}

		switch {
		case f.Type().IsBinary():
			g.line(out, "  ret += \"BINARY\";")
		case f.Type().IsEnum():
			fnVar := f.Name() + "_name"
			g.line(out, "var "+fnVar+" : String = "+getCapName(g.getEnumClassName(f.Type()))+".VALUES_TO_NAMES[this."+escapeHaxeKeyword(f.Name())+"];")
			g.line(out, "if ("+fnVar+" != null) {")
			g.line(out, "  ret += "+fnVar+";")
			g.line(out, "  ret += \" (\";")
			g.line(out, "}")
			g.line(out, "ret += this."+escapeHaxeKeyword(f.Name())+";")
			g.line(out, "if ("+fnVar+" != null) {")
			g.line(out, "  ret += \")\";")
			g.line(out, "}")
		default:
			g.line(out, "ret += this."+escapeHaxeKeyword(f.Name())+";")
		}

		if canBeNull {
			g.indentDown()
			g.line(out, "}")
		}
		g.line(out, "first = false;")

		if couldBeUnset {
			g.indentDown()
			g.line(out, "}")
		}
		first = false
	}
	out.WriteString(g.indent() + "ret += \")\";\n" + g.indent() + "return ret;\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}

// generateHaxeMetaDataMap is t_haxe_generator::generate_haxe_meta_data_map:
// generates a static map with meta data to store information such as
// fieldID to fieldName mapping. Dead code in the C++ source (only reached
// from the `if (false)` block in generate_haxe_struct_definition), kept
// for function-for-function parity.
func (g *generator) generateHaxeMetaDataMap(out *strings.Builder, tstruct *sema.Struct) {
	fields := tstruct.Members()

	// Static Map with fieldID -> FieldMetaData mappings
	g.line(out, "inline static var metaDataMap : IntMap = new IntMap();")

	if len(fields) > 0 {
		// Populate map
		g.scopeUp(out)
		for _, field := range fields {
			fieldName := field.Name()
			g.indentRaw(out, "metaDataMap["+upcaseString(fieldName)+"_FIELD_ID] = new FieldMetaData(\""+fieldName+"\", ")

			// Set field requirement type (required, optional, etc.)
			switch field.Req() {
			case sema.Required:
				out.WriteString("TFieldRequirementType.REQUIRED, ")
			case sema.Optional:
				out.WriteString("TFieldRequirementType.OPTIONAL, ")
			default:
				out.WriteString("TFieldRequirementType.DEFAULT, ")
			}

			// Create value meta data
			g.generateFieldValueMetaData(out, field.Type())
			out.WriteString(");\n")
		}
		g.scopeDown(out)
	}
}

// generateFieldValueMetaData is t_haxe_generator::generate_field_value_meta_data.
func (g *generator) generateFieldValueMetaData(out *strings.Builder, t sema.Type) {
	out.WriteString("\n")
	g.indentUp()
	g.indentUp()
	switch {
	case t.IsStruct():
		g.indentRaw(out, "new StructMetaData(TType.STRUCT, "+g.typeName(t))
	case t.IsContainer():
		switch {
		case t.IsList():
			g.indentRaw(out, "new ListMetaData(TType.LIST, ")
			g.generateFieldValueMetaData(out, t.(*sema.List).ElemType())
		case t.IsSet():
			g.indentRaw(out, "new SetMetaData(TType.SET, ")
			g.generateFieldValueMetaData(out, t.(*sema.Set).ElemType())
		default: // map
			g.indentRaw(out, "new MapMetaData(TType.MAP, ")
			m := t.(*sema.Map)
			g.generateFieldValueMetaData(out, m.KeyType())
			out.WriteString(", ")
			g.generateFieldValueMetaData(out, m.ValType())
		}
	default:
		g.indentRaw(out, "new FieldValueMetaData("+g.getHaxeTypeString(t))
	}
	out.WriteString(")")
	g.indentDown()
	g.indentDown()
}

// declareField is t_haxe_generator::declare_field.
func (g *generator) declareField(tfield *sema.Field, init bool) string {
	result := "var " + tfield.Name() + " : " + g.typeName(tfield.Type())
	if init {
		ttype := sema.TrueType(tfield.Type())
		if ttype.IsBaseType() && tfield.Value() != nil {
			result += " = " + g.renderConstValueStr(ttype, tfield.Value())
		} else {
			result += " = " + g.renderDefaultValueForType(ttype, false)
		}
	}
	return result + ";"
}

// renderDefaultValueForType is t_haxe_generator::render_default_value_for_type.
func (g *generator) renderDefaultValueForType(t sema.Type, allowNull bool) string {
	ttype := sema.TrueType(t)

	switch {
	case ttype.IsBaseType():
		switch ttype.(*sema.BaseType).Base() {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "null"
		case sema.TypeUUID:
			return "uuid.Uuid.NIL"
		case sema.TypeBool:
			return "false"
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			return "0"
		case sema.TypeDouble:
			return "0.0"
		default:
			emit.Throw("unhandled type")
		}
	case ttype.IsEnum():
		return "0"
	case ttype.IsContainer():
		if allowNull {
			return "null"
		}
		return "new " + g.typeName(ttype) + "()"
	default:
		if allowNull {
			return "null"
		}
		return "new " + g.typeName(ttype) + "()"
	}
	return ""
}

// argumentList is t_haxe_generator::argument_list: renders a comma
// separated field list, with type names.
func (g *generator) argumentList(tstruct *sema.Struct) string {
	result := ""

	fields := tstruct.Members()
	first := true
	for _, f := range fields {
		if first {
			first = false
		} else {
			result += ", "
		}
		result += escapeHaxeKeyword(f.Name()) + " : " + getCapName(g.typeName(f.Type()))
	}
	return result
}
