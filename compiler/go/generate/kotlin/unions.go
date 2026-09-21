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

package kotlin

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateKotlinUnion is t_kotlin_generator::generate_kotlin_union.
func (g *Generator) generateKotlinUnion(s *sema.Struct) {
	fUnionName := g.packageDir + "/" + s.Name() + ".kt"
	var out strings.Builder
	out.WriteString(autogenComment() + warningSuppressions() + g.kotlinPackage())
	g.generateUnionDefinition(&out, s, "")
	emit.WriteFile(fUnionName, out.String())
}

// generateUnionDefinition is t_kotlin_generator::generate_union_definition.
// The C++ original's additional_interface parameter is commented out
// (unused); it is kept here only to mirror the signature.
func (g *Generator) generateUnionDefinition(out *strings.Builder, s *sema.Struct, additionalInterface string) {
	_ = additionalInterface
	unionClassName := kotlinSafeName(s.Name())
	out.WriteString(g.indent() + "class " + unionClassName + " : org.apache.thrift.TUnion<" + unionClassName + ", " + unionClassName + "._Fields> {\n")
	g.indentUp()
	out.WriteString(g.indent() + "constructor(setField: _Fields, value: kotlin.Any) : super(setField, value)\n")
	out.WriteString(g.indent() + "constructor(other: " + unionClassName + ") : super(other)\n")
	out.WriteString(g.indent() + "constructor() : super()\n")

	g.generateStructFieldNameConstants(out, s)
	g.generateStructCompanionObject(out, s)
	g.generateStructMethodFieldForID(out)
	g.generateUnionMethodsDefinitions(out, s)
	g.generateUnionMethodCheckType(out, s)
	g.generateUnionStandardScheme(out, s)
	g.generateUnionTupleScheme(out)
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

// generateUnionStandardScheme is
// t_kotlin_generator::generate_union_standard_scheme.
func (g *Generator) generateUnionStandardScheme(out *strings.Builder, s *sema.Struct) {
	g.generateUnionStandardSchemeRead(out, s)
	g.generateUnionStandardSchemeWrite(out, s)
}

// generateUnionStandardSchemeRead is
// t_kotlin_generator::generate_union_standard_scheme_read. Unlike its
// write counterpart, this ends as a `=` expression body with no trailing
// brace: the final indent_down() in the C++ source only rebalances the
// indent level and writes nothing.
func (g *Generator) generateUnionStandardSchemeRead(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "override fun standardSchemeReadValue(iproto: org.apache.thrift.protocol.TProtocol, field: org.apache.thrift.protocol.TField): Any? =\n")
	g.indentUp()
	out.WriteString(g.indent() + "when (_Fields.findByValue(field.id.toInt())) {\n")
	g.indentUp()
	for _, m := range s.Members() {
		out.WriteString(g.indent() + "_Fields." + constantName(m.Name()) + " -> {\n")
		g.indentUp()
		{
			out.WriteString(g.indent() + "if (field.type == " + constantName(m.Name()) + "_FIELD_DESC.type) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "iproto.run {\n")
			g.indentUp()
			out.WriteString(g.indent())
			g.generateDeserializeValue(out, m.Type())
			out.WriteString("\n")
			g.scopeDown(out)
			g.indentDown()
			out.WriteString(g.indent() + "} else {\n")
			g.indentUp()
			out.WriteString(g.indent() + "org.apache.thrift.protocol.TProtocolUtil.skip(iproto, field.type)\n")
			out.WriteString(g.indent() + "null\n")
			g.scopeDown(out)
		}
		g.scopeDown(out)
	}
	out.WriteString(g.indent() + "null -> {\n")
	g.indentUp()
	out.WriteString(g.indent() + "org.apache.thrift.protocol.TProtocolUtil.skip(iproto, field.type)\n")
	out.WriteString(g.indent() + "null\n")
	g.scopeDown(out)
	g.scopeDown(out)
	g.indentDown()
}

// generateUnionStandardSchemeWrite is
// t_kotlin_generator::generate_union_standard_scheme_write.
func (g *Generator) generateUnionStandardSchemeWrite(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "@Suppress(\"UNCHECKED_CAST\")\n")
	out.WriteString(g.indent() + "override fun standardSchemeWriteValue(oproto: org.apache.thrift.protocol.TProtocol) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "when (setField_) {\n")
	g.indentUp()
	for _, m := range s.Members() {
		out.WriteString(g.indent() + "_Fields." + constantName(m.Name()) + " -> {\n")
		g.indentUp()
		{
			out.WriteString(g.indent() + "val it = value_ as " + g.typeName(m.Type(), false, false, false) + "\n")
			out.WriteString(g.indent() + "oproto.apply {\n")
			g.indentUp()
			{
				out.WriteString(g.indent())
				g.generateSerializeValue(out, m.Type(), "it")
				out.WriteString("\n")
			}
			g.scopeDown(out)
		}
		g.scopeDown(out)
	}
	out.WriteString(g.indent() + "null -> throw kotlin.IllegalStateException(\"Cannot write union with unknown field $setField_\")\n")
	g.scopeDown(out)
	g.scopeDown(out)
}

// generateUnionTupleScheme is t_kotlin_generator::generate_union_tuple_scheme.
// The C++ original takes an unused t_struct* parameter, dropped here.
func (g *Generator) generateUnionTupleScheme(out *strings.Builder) {
	out.WriteString(g.indent() + "override fun tupleSchemeReadValue(iproto: org.apache.thrift.protocol.TProtocol, fieldID: kotlin.Short) = throw kotlin.UnsupportedOperationException(\"only standard scheme is supported for now\")\n")
	out.WriteString(g.indent() + "override fun tupleSchemeWriteValue(oproto: org.apache.thrift.protocol.TProtocol) = throw kotlin.UnsupportedOperationException(\"only standard scheme is supported for now\")\n")
}

// generateUnionMethodsDefinitions is
// t_kotlin_generator::generate_union_methods_definitions.
//
// The C++ source builds a throwaway t_struct named after the union with
// two synthetic fields, "setField_" (an enum-typed field) and "value_" (a
// binary-string-typed field), purely to reuse
// generate_struct_method_compare_to's field-name and container/binary
// checks against the TUnion base class's own setField_/value_ properties.
// The same trick is reproduced with sema.NewStruct/sema.NewField;
// sema.GlobalBinary supplies the pre-built binary string type.
func (g *Generator) generateUnionMethodsDefinitions(out *strings.Builder, s *sema.Struct) {
	{
		unionFields := sema.NewStruct(g.program)
		unionFields.SetName(s.Name())
		enumType := sema.NewEnum(g.program)
		setField := sema.NewField(enumType, "setField_", 0)
		value := sema.NewField(sema.GlobalBinary, "value_", 1)
		unionFields.Append(setField)
		unionFields.Append(value)
		g.generateStructMethodCompareTo(out, unionFields)
	}

	unionClassName := kotlinSafeName(s.Name())
	out.WriteString(g.indent() + "override fun deepCopy() = " + unionClassName + "(this)\n")
	out.WriteString(g.indent() + "override fun enumForId(id: kotlin.Short) = fieldForId(id.toInt())\n")
	out.WriteString(g.indent() + "override fun getStructDesc() = STRUCT_DESC\n")
	{
		out.WriteString(g.indent() + "override fun getFieldDesc(setField: _Fields) = when (setField) {\n")
		g.indentUp()
		for _, m := range s.Members() {
			out.WriteString(g.indent() + "_Fields." + constantName(m.Name()) + " -> " + constantName(m.Name()) + "_FIELD_DESC\n")
		}
		g.scopeDown(out)
	}
}

// generateUnionMethodCheckType is
// t_kotlin_generator::generate_union_method_check_type.
func (g *Generator) generateUnionMethodCheckType(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "@Suppress(\"UNCHECKED_CAST\")\n")
	out.WriteString(g.indent() + "override fun checkType(setField: _Fields, value: kotlin.Any?) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "when (setField) {\n")
	g.indentUp()
	for _, m := range s.Members() {
		expectType := g.typeName(m.Type(), false, false, false)
		out.WriteString(g.indent() + "_Fields." + constantName(m.Name()) + " -> value as? " + expectType + " ?: throw kotlin.ClassCastException(\"Was expecting value of type `" + expectType + "' for field `" + m.Name() + "', but got ${value?.javaClass}\")\n")
	}
	g.scopeDown(out)
	g.scopeDown(out)
}
