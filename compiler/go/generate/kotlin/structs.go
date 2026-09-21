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
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateStruct is t_kotlin_generator::generate_struct.
func (g *Generator) generateStruct(s *sema.Struct) {
	if s.IsUnion() {
		g.generateKotlinUnion(s)
	} else {
		g.generateKotlinStruct(s, false)
	}
}

// generateXception is t_kotlin_generator::generate_xception.
func (g *Generator) generateXception(s *sema.Struct) {
	g.generateKotlinStruct(s, true)
}

// generateKotlinStruct is t_kotlin_generator::generate_kotlin_struct.
func (g *Generator) generateKotlinStruct(s *sema.Struct, isException bool) {
	fStructName := g.packageDir + "/" + s.Name() + ".kt"
	var out strings.Builder
	out.WriteString(autogenComment() + warningSuppressions() + g.kotlinPackage())
	g.generateStructDefinition(&out, s, isException, "")
	emit.WriteFile(fStructName, out.String())
}

// generateStructFieldNameConstants is
// t_kotlin_generator::generate_struct_field_name_constants.
func (g *Generator) generateStructFieldNameConstants(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "enum class _Fields(private val thriftFieldId: kotlin.Short, private val fieldName: kotlin.String) : org.apache.thrift.TFieldIdEnum {\n")
	g.indentUp()
	{
		first := true
		for _, f := range s.Members() {
			if !first {
				out.WriteString(",\n")
			}
			first = false
			out.WriteString(g.indent() + constantName(f.Name()) + "(" + strconv.Itoa(int(f.Key())) + ", \"" + f.Name() + "\")")
		}
		if first {
			out.WriteString(g.indent())
		}
		out.WriteString(";\n\n")

		out.WriteString(g.indent() + "override fun getThriftFieldId() = thriftFieldId\n\n")
		out.WriteString(g.indent() + "override fun getFieldName() = fieldName\n\n")

		out.WriteString(g.indent() + "companion object {\n")
		g.indentUp()
		{
			out.WriteString(g.indent() + "@kotlin.jvm.JvmStatic\n")
			out.WriteString(g.indent() + "fun findByValue(value: kotlin.Int): _Fields? {\n")
			g.indentUp()
			{
				out.WriteString(g.indent() + "return when (value) {\n")
				g.indentUp()
				{
					for _, f := range s.Members() {
						out.WriteString(g.indent() + strconv.Itoa(int(f.Key())) + " -> " + constantName(f.Name()) + "\n")
					}
					out.WriteString(g.indent() + "else -> null\n")
				}
				g.scopeDown(out)
			}
			g.scopeDown(out)
		}

		out.WriteString("\n")

		{
			out.WriteString(g.indent() + "@kotlin.jvm.JvmStatic\n")
			out.WriteString(g.indent() + "fun findByName(name: kotlin.String): _Fields? {\n")
			g.indentUp()
			{
				out.WriteString(g.indent() + "return when (name) {\n")
				g.indentUp()
				{
					for _, f := range s.Members() {
						out.WriteString(g.indent() + "\"" + f.Name() + "\" -> " + constantName(f.Name()) + "\n")
					}
					out.WriteString(g.indent() + "else -> null\n")
				}
				g.scopeDown(out)
			}
			g.scopeDown(out)
		}

		g.scopeDown(out)
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructCompanionObject is
// t_kotlin_generator::generate_struct_companion_object.
func (g *Generator) generateStructCompanionObject(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "companion object {\n")
	g.indentUp()
	{
		out.WriteString(g.indent() + "private val STRUCT_DESC: org.apache.thrift.protocol.TStruct = org.apache.thrift.protocol.TStruct(\"" + s.Name() + "\")\n")
		for _, f := range s.Members() {
			out.WriteString(g.indent() + "private val " + constantName(f.Name()) + "_FIELD_DESC: org.apache.thrift.protocol.TField = org.apache.thrift.protocol.TField(\"" + f.Name() + "\", " + g.typeToEnum(f.Type()) + ", " + strconv.Itoa(int(f.Key())) + ")\n")
			out.WriteString(g.indent() + "private val " + constantName(f.Name()) + "_FIELD_META_DATA: org.apache.thrift.meta_data.FieldMetaData = org.apache.thrift.meta_data.FieldMetaData(\n")
			g.indentUp()
			{
				out.WriteString(g.indent() + "\"" + f.Name() + "\",\n")
				out.WriteString(g.indent() + "org.apache.thrift.TFieldRequirementType.")
				switch f.Req() {
				case sema.Required:
					out.WriteString("REQUIRED")
				case sema.Optional:
					out.WriteString("OPTIONAL")
				default:
					out.WriteString("DEFAULT")
				}
				out.WriteString(",\n")
				out.WriteString(g.indent())
				g.generateFieldValueMetaData(out, f.Type())
				out.WriteString(",\n")
				out.WriteString(g.indent())
				g.generateMetadataForFieldAnnotations(out, f)
			}
			out.WriteString(")\n")
			g.indentDown()
		}

		out.WriteString(g.indent() + "private val metadata: Map<_Fields, org.apache.thrift.meta_data.FieldMetaData> = mapOf(\n")
		g.indentUp()
		for _, f := range s.Members() {
			out.WriteString(g.indent() + "_Fields." + constantName(f.Name()) + " to " + constantName(f.Name()) + "_FIELD_META_DATA,\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + ")\n")

		out.WriteString(g.indent() + "init {\n")
		g.indentUp()
		out.WriteString(g.indent() + "org.apache.thrift.meta_data.FieldMetaData.addStructMetaDataMap(" + s.Name() + "::class.java, metadata)\n")
		g.scopeDown(out)
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateMetadataForFieldAnnotations is
// t_kotlin_generator::generate_metadata_for_field_annotations.
func (g *Generator) generateMetadataForFieldAnnotations(out *strings.Builder, f *sema.Field) {
	a := f.Annotations()
	if len(a) == 0 {
		out.WriteString("emptyMap()")
	} else {
		out.WriteString("mapOf(\n")
		g.indentUp()
		for _, k := range a.Keys() {
			v := a[k]
			out.WriteString(g.indent() + "\"" + k + "\" to \"" + v[len(v)-1] + "\",\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + ")")
	}
}

// generateFieldValueMetaData is
// t_kotlin_generator::generate_field_value_meta_data. Its checks
// (is_struct, is_container, is_enum, is_typedef, is_binary) run on the
// type given, not on get_true_type(type): a typedef of, say, a struct
// falls through to the final "else" branch (FieldValueMetaData with the
// typedef's symbolic name), which is intentional in the C++ source and
// reproduced here rather than fixed.
func (g *Generator) generateFieldValueMetaData(out *strings.Builder, t sema.Type) {
	const ttypeClass = "org.apache.thrift.protocol.TType."
	const metaPackage = "org.apache.thrift.meta_data."
	out.WriteString(metaPackage)
	if t.IsStruct() || t.IsXception() {
		out.WriteString("StructMetaData(" + ttypeClass + "STRUCT, " + g.typeName(t, false, false, false) + "::class.java")
	} else if t.IsContainer() {
		if t.IsList() {
			l := t.(*sema.List)
			out.WriteString("ListMetaData(" + ttypeClass + "LIST,\n")
			g.indentUp()
			out.WriteString(g.indent())
			g.generateFieldValueMetaData(out, l.ElemType())
			g.indentDown()
		} else if t.IsSet() {
			s := t.(*sema.Set)
			out.WriteString("SetMetaData(" + ttypeClass + "SET,\n")
			g.indentUp()
			out.WriteString(g.indent())
			g.generateFieldValueMetaData(out, s.ElemType())
			g.indentDown()
		} else {
			m := t.(*sema.Map)
			out.WriteString("MapMetaData(" + ttypeClass + "MAP,\n")
			g.indentUp()
			out.WriteString(g.indent())
			g.generateFieldValueMetaData(out, m.KeyType())
			out.WriteString(",\n")
			out.WriteString(g.indent())
			g.generateFieldValueMetaData(out, m.ValType())
			g.indentDown()
		}
	} else if t.IsEnum() {
		out.WriteString("EnumMetaData(" + ttypeClass + "ENUM, " + g.typeName(t, false, false, false) + "::class.java")
	} else {
		out.WriteString("FieldValueMetaData(" + g.typeToEnum(t))
		if t.IsTypedef() {
			out.WriteString(", \"" + t.(*sema.Typedef).Symbolic() + "\"")
		} else if t.IsBinary() {
			out.WriteString(", true")
		}
	}
	out.WriteString(")")
}

// generateStructMethodDeepCopy is
// t_kotlin_generator::generate_struct_method_deep_copy.
func (g *Generator) generateStructMethodDeepCopy(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "override fun deepCopy(): " + s.Name() + " {\n")
	g.indentUp()
	{
		out.WriteString(g.indent() + "return " + s.Name() + " (\n")
		g.indentUp()
		for _, f := range s.Members() {
			out.WriteString(g.indent() + kotlinSafeName(f.Name()) + ",\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + ")\n")
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructMethodCompareTo is
// t_kotlin_generator::generate_struct_method_compare_to. field_type's
// container/binary checks run on the field's raw type, not its true type,
// so a typedef of a list is not spotted here and gets no
// TBaseHelper::compareTo wrapper; that is reproduced, not fixed.
func (g *Generator) generateStructMethodCompareTo(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "override fun compareTo(other: " + s.Name() + "?): kotlin.Int {\n")
	g.indentUp()
	{
		out.WriteString(g.indent() + "val comparator = compareBy<" + s.Name() + "> { it::class.java.name }\n")
		g.indentUp()
		for _, f := range s.Members() {
			out.WriteString(g.indent() + ".thenBy")
			ft := f.Type()
			if ft.IsList() || ft.IsSet() || ft.IsMap() || ft.IsBinary() {
				out.WriteString("(org.apache.thrift.TBaseHelper::compareTo)")
			}
			out.WriteString(" { it." + kotlinSafeName(f.Name()) + " } \n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "return nullsFirst(comparator).compare(this, other)\n")
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructMethodFieldForID is
// t_kotlin_generator::generate_struct_method_field_for_id. The C++
// original takes an unused t_struct* parameter, dropped here.
func (g *Generator) generateStructMethodFieldForID(out *strings.Builder) {
	out.WriteString(g.indent() + "override fun fieldForId(fieldId: kotlin.Int): _Fields {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return _Fields.findByValue(fieldId) ?: throw kotlin.IllegalArgumentException(\"invalid fieldId $fieldId\")\n")
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructMethodIsSet is
// t_kotlin_generator::generate_struct_method_is_set.
func (g *Generator) generateStructMethodIsSet(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "override fun isSet(field: _Fields): kotlin.Boolean {\n")
	g.indentUp()
	{
		out.WriteString(g.indent() + "return when (field) {\n")
		g.indentUp()
		{
			members := s.Members()
			if len(members) > 0 {
				for _, f := range members {
					out.WriteString(g.indent() + "_Fields." + constantName(f.Name()) + " -> ")
					if f.Req() == sema.Required {
						out.WriteString("this._" + f.Name() + " != null")
					} else {
						out.WriteString("this." + kotlinSafeName(f.Name()) + " != null")
					}
					out.WriteString("\n")
				}
			} else {
				out.WriteString(g.indent() + "else -> false\n")
			}
		}
		g.scopeDown(out)
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructMethodClear is
// t_kotlin_generator::generate_struct_method_clear.
func (g *Generator) generateStructMethodClear(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "override fun clear(): kotlin.Unit {\n")
	g.indentUp()
	for _, f := range s.Members() {
		name := kotlinSafeName(f.Name())
		if f.Req() == sema.Required {
			name = "_" + f.Name()
		}
		out.WriteString(g.indent() + name + " = null\n")
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructMethodValidate is
// t_kotlin_generator::generate_struct_method_validate.
func (g *Generator) generateStructMethodValidate(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "@kotlin.jvm.Throws(org.apache.thrift.TException::class)\n")
	out.WriteString(g.indent() + "fun validate(): kotlin.Unit {\n")
	g.indentUp()
	for _, f := range s.Members() {
		if f.Req() == sema.Required {
			out.WriteString(g.indent() + "if (_" + f.Name() + " == null) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "throw org.apache.thrift.TException(\"Required field `" + f.Name() + "' is null, struct is: $this\")\n")
			g.scopeDown(out)
		}
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructMethodSetFieldValue is
// t_kotlin_generator::generate_struct_method_set_field_value.
func (g *Generator) generateStructMethodSetFieldValue(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "@Suppress(\"UNCHECKED_CAST\")\n")
	out.WriteString(g.indent() + "override fun setFieldValue(field: _Fields, value: kotlin.Any?): kotlin.Unit {\n")
	g.indentUp()
	{
		members := s.Members()
		if len(members) > 0 {
			out.WriteString(g.indent() + "when (field) {\n")
			g.indentUp()
			for _, f := range members {
				name := kotlinSafeName(f.Name())
				if f.Req() == sema.Required {
					name = "_" + f.Name()
				}
				out.WriteString(g.indent() + "_Fields." + constantName(f.Name()) + " -> this." + name + " = value as " + g.typeName(f.Type(), false, false, false) + "?\n")
			}
			g.scopeDown(out)
		} else {
			out.WriteString(g.indent() + "return\n")
		}
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructMethodGetFieldValue is
// t_kotlin_generator::generate_struct_method_get_field_value.
func (g *Generator) generateStructMethodGetFieldValue(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "override fun getFieldValue(field: _Fields): kotlin.Any? {\n")
	g.indentUp()
	{
		members := s.Members()
		if len(members) > 0 {
			out.WriteString(g.indent() + "return when (field) {\n")
			g.indentUp()
			for _, f := range members {
				out.WriteString(g.indent() + "_Fields." + constantName(f.Name()) + " -> this." + kotlinSafeName(f.Name()) + "\n")
			}
			g.scopeDown(out)
		} else {
			out.WriteString(g.indent() + "return null\n")
		}
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructMethodRead is t_kotlin_generator::generate_struct_method_read.
func (g *Generator) generateStructMethodRead(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "override fun read(iproto: org.apache.thrift.protocol.TProtocol): kotlin.Unit {\n")
	g.indentUp()
	{
		out.WriteString(g.indent() + "require(org.apache.thrift.scheme.StandardScheme::class.java == iproto.scheme) { \"only standard scheme is supported for now\" }\n")
		out.WriteString(g.indent() + s.Name() + "StandardScheme.read(iproto, this)\n")
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructMethodWrite is t_kotlin_generator::generate_struct_method_write.
func (g *Generator) generateStructMethodWrite(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "override fun write(oproto: org.apache.thrift.protocol.TProtocol): kotlin.Unit {\n")
	g.indentUp()
	{
		out.WriteString(g.indent() + "require(org.apache.thrift.scheme.StandardScheme::class.java == oproto.scheme) { \"only standard scheme is supported for now\" }\n")
		out.WriteString(g.indent() + s.Name() + "StandardScheme.write(oproto, this)\n")
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructStandardSchemeRead is
// t_kotlin_generator::generate_struct_standard_scheme_read.
func (g *Generator) generateStructStandardSchemeRead(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "override fun read(iproto: org.apache.thrift.protocol.TProtocol, struct: " + s.Name() + ") {\n")
	g.indentUp()
	{
		out.WriteString(g.indent() + "iproto.apply {\n")
		g.indentUp()
		{
			out.WriteString(g.indent() + "readStruct {\n")
			g.indentUp()
			{
				out.WriteString(g.indent() + "var stopped = false\n")
				out.WriteString(g.indent() + "while (!stopped) {\n")
				g.indentUp()
				{
					out.WriteString(g.indent() + "stopped = readField {\n")
					g.indentUp()
					{
						out.WriteString(g.indent() + "val skipNext = { org.apache.thrift.protocol.TProtocolUtil.skip(iproto, it.type) }\n")
						out.WriteString(g.indent() + "when (it.id.toInt()) {\n")
						g.indentUp()
						{
							for _, f := range s.Members() {
								out.WriteString(g.indent() + strconv.Itoa(int(f.Key())) + " -> {\n")
								g.indentUp()
								{
									out.WriteString(g.indent() + "if (it.type == " + g.typeToEnum(f.Type()) + ") {\n")
									g.indentUp()
									g.generateDeserializeField(out, f, "struct.")
									g.indentDown()
									out.WriteString(g.indent() + "} else {\n")
									g.indentUp()
									out.WriteString(g.indent() + "skipNext()\n")
									g.indentDown()
									out.WriteString(g.indent() + "}\n")
								}
								g.scopeDown(out)
							}
							out.WriteString(g.indent() + "else -> skipNext()\n")
						}
						g.scopeDown(out)
					}
					g.scopeDown(out)
				}
				g.scopeDown(out)
				out.WriteString(g.indent() + "struct.validate()\n")
			}
			g.scopeDown(out)
		}
		g.scopeDown(out)
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructStandardSchemeWrite is
// t_kotlin_generator::generate_struct_standard_scheme_write.
func (g *Generator) generateStructStandardSchemeWrite(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "override fun write(oproto: org.apache.thrift.protocol.TProtocol, struct: " + s.Name() + ") {\n")
	g.indentUp()
	{
		out.WriteString(g.indent() + "struct.validate()\n")
		out.WriteString(g.indent() + "oproto.apply {\n")
		g.indentUp()
		{
			out.WriteString(g.indent() + "writeStruct(STRUCT_DESC) {\n")
			g.indentUp()
			{
				for _, f := range s.Members() {
					optMark := "?"
					if f.Req() == sema.Required {
						optMark = ""
					}
					out.WriteString(g.indent() + "struct." + kotlinSafeName(f.Name()) + optMark + ".let { " + kotlinSafeName(f.Name()) + " ->\n")
					g.indentUp()
					{
						out.WriteString(g.indent() + "writeField(" + constantName(f.Name()) + "_FIELD_DESC) {\n")
						g.indentUp()
						g.generateSerializeField(out, f)
						g.scopeDown(out)
					}
					g.scopeDown(out)
				}
			}
			out.WriteString(g.indent() + "writeFieldStop()\n")
			g.scopeDown(out)
		}
		g.scopeDown(out)
	}
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructStandardScheme is
// t_kotlin_generator::generate_struct_standard_scheme.
func (g *Generator) generateStructStandardScheme(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "private object " + s.Name() + "StandardScheme : org.apache.thrift.scheme.StandardScheme<" + s.Name() + ">() {\n")
	g.indentUp()
	g.generateStructStandardSchemeRead(out, s)
	g.generateStructStandardSchemeWrite(out, s)
	g.scopeDown(out)
	out.WriteString("\n")
}

// generateStructDefinition is t_kotlin_generator::generate_struct_definition.
//
// The private backing field for a required field is declared with the
// field's raw name ("private var _"+name), but the public getter's body
// and every other reference to that backing field use "_"+kotlinSafeName
// (a backtick-quoted name for a Kotlin keyword): for a field whose name is
// itself a Kotlin reserved word, the getter references a backing field
// that was never declared under that spelling. That mismatch is in the
// C++ source and is reproduced here, not fixed.
func (g *Generator) generateStructDefinition(out *strings.Builder, s *sema.Struct, isException bool, additionalInterface string) {
	g.generateKdocComment(out, s)
	members := s.Members()
	if len(members) > 0 {
		out.WriteString(g.indent() + "data class ")
	} else {
		out.WriteString(g.indent() + "class ")
	}
	out.WriteString(kotlinSafeName(s.Name()) + "(")

	g.indentUp()
	sep := ""
	for _, f := range members {
		out.WriteString(sep + "\n")
		sep = ","
		g.generateKdocComment(out, f)
		isRequired := f.Req() == sema.Required
		if isRequired {
			out.WriteString(g.indent() + "private var _" + f.Name())
		} else if isException && f.Name() == "message" {
			// special handling for exception when field name is message - needs override
			if !f.Type().IsString() {
				emit.Throw("type error: for `message' field in an exception struct, it must be a string")
			}
			out.WriteString(g.indent() + "override var message")
		} else {
			out.WriteString(g.indent() + "var " + kotlinSafeName(f.Name()))
		}
		out.WriteString(": " + g.typeName(f.Type(), false, false, false) + "? = null")
	}
	g.indentDown()
	out.WriteString("\n")
	out.WriteString(g.indent() + ") : ")
	if isException {
		out.WriteString("org.apache.thrift.TException(), ")
	}
	if additionalInterface != "" {
		additionalInterface = ", " + additionalInterface
	}
	out.WriteString("org.apache.thrift.TBase<" + s.Name() + ", " + s.Name() + "._Fields>" + additionalInterface + " {\n")

	g.indentUp()

	for _, f := range members {
		if f.Req() == sema.Required {
			out.WriteString(g.indent())
			// special handling for exception when field name is message - needs override
			if isException && f.Name() == "message" {
				out.WriteString("override ")
			}
			out.WriteString("val " + kotlinSafeName(f.Name()) + ": " + g.typeName(f.Type(), false, false, false) + " get() = _" + kotlinSafeName(f.Name()) + "!!\n")
		}
	}

	g.generateStructFieldNameConstants(out, s)
	g.generateStructCompanionObject(out, s)
	g.generateStructStandardScheme(out, s)
	g.generateStructMethodCompareTo(out, s)
	g.generateStructMethodFieldForID(out)
	g.generateStructMethodGetFieldValue(out, s)
	g.generateStructMethodSetFieldValue(out, s)
	g.generateStructMethodIsSet(out, s)
	g.generateStructMethodDeepCopy(out, s)
	g.generateStructMethodClear(out, s)
	g.generateStructMethodValidate(out, s)
	g.generateStructMethodRead(out, s)
	g.generateStructMethodWrite(out, s)

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}
