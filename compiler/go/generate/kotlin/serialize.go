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

// baseTypeWriteExpression is t_kotlin_generator::base_type_write_expression.
func baseTypeWriteExpression(t *sema.BaseType, it string) string {
	switch t.Base() {
	case sema.TypeVoid:
		emit.Throw("compiler error: no void in base types")
	case sema.TypeString:
		if t.IsBinary() {
			return "writeBinary(java.nio.ByteBuffer.wrap(" + it + "))"
		}
		return "writeString(" + it + ")"
	case sema.TypeBool:
		return "writeBool(" + it + ")"
	case sema.TypeI8:
		return "writeByte(" + it + ")"
	case sema.TypeI16:
		return "writeI16(" + it + ")"
	case sema.TypeI32:
		return "writeI32(" + it + ")"
	case sema.TypeI64:
		return "writeI64(" + it + ")"
	case sema.TypeUUID:
		return "writeUuid(" + it + ")"
	case sema.TypeDouble:
		return "writeDouble(" + it + ")"
	}
	emit.Throw("compiler error: no Kotlin name for base type %s", sema.BaseName(t.Base()))
	return ""
}

// baseTypeReadExpression is t_kotlin_generator::base_type_read_expression.
func baseTypeReadExpression(t *sema.BaseType) string {
	switch t.Base() {
	case sema.TypeVoid:
		emit.Throw("compiler error: no void in base types")
	case sema.TypeString:
		if t.IsBinary() {
			return "org.apache.thrift.TBaseHelper.byteBufferToByteArray(readBinary())"
		}
		return "readString()"
	case sema.TypeBool:
		return "readBool()"
	case sema.TypeI8:
		return "readByte()"
	case sema.TypeI16:
		return "readI16()"
	case sema.TypeI32:
		return "readI32()"
	case sema.TypeI64:
		return "readI64()"
	case sema.TypeUUID:
		return "readUuid()"
	case sema.TypeDouble:
		return "readDouble()"
	}
	emit.Throw("compiler error: no Kotlin name for base type %s", sema.BaseName(t.Base()))
	return ""
}

// generateSerializeValue is t_kotlin_generator::generate_serialize_value.
// Its final "else" branch is a printf to the compiler's own stdout, not
// output written to the generated file, so nothing is written there.
func (g *Generator) generateSerializeValue(out *strings.Builder, t sema.Type, it string) {
	tt := sema.TrueType(t)
	if tt.IsStruct() || tt.IsXception() {
		out.WriteString(it + ".write(this)")
	} else if tt.IsContainer() {
		g.generateSerializeContainer(out, tt, it)
	} else if tt.IsBaseType() {
		out.WriteString(baseTypeWriteExpression(tt.(*sema.BaseType), it))
	} else if tt.IsEnum() {
		out.WriteString("writeI32(" + it + ".value)")
	}
}

// generateDeserializeValue is t_kotlin_generator::generate_deserialize_value.
func (g *Generator) generateDeserializeValue(out *strings.Builder, t sema.Type) {
	tt := sema.TrueType(t)
	if tt.IsStruct() || tt.IsXception() {
		out.WriteString(g.typeName(tt, false, false, false) + "().apply { read(iproto) }")
	} else if tt.IsContainer() {
		g.generateDeserializeContainer(out, tt)
	} else if tt.IsBaseType() {
		out.WriteString(baseTypeReadExpression(tt.(*sema.BaseType)))
	} else if tt.IsEnum() {
		out.WriteString("requireNotNull(" + g.typeName(tt, false, false, true) + ".findByValue(readI32()))")
	}
}

// generateSerializeField is t_kotlin_generator::generate_serialize_field.
func (g *Generator) generateSerializeField(out *strings.Builder, f *sema.Field) {
	t := sema.TrueType(f.Type())
	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s", f.Name())
	}
	out.WriteString(g.indent())
	g.generateSerializeValue(out, t, kotlinSafeName(f.Name()))
	out.WriteString("\n")
}

// generateDeserializeField is t_kotlin_generator::generate_deserialize_field.
func (g *Generator) generateDeserializeField(out *strings.Builder, f *sema.Field, prefix string) {
	t := sema.TrueType(f.Type())
	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, f.Name())
	}
	isRequired := f.Req() == sema.Required
	name := prefix
	if isRequired {
		name += "_" + f.Name()
	} else {
		name += kotlinSafeName(f.Name())
	}
	out.WriteString(g.indent() + name + " = ")
	g.generateDeserializeValue(out, t)
	out.WriteString("\n")
}

// generateSerializeContainer is t_kotlin_generator::generate_serialize_container.
func (g *Generator) generateSerializeContainer(out *strings.Builder, t sema.Type, it string) {
	if t.IsMap() {
		m := t.(*sema.Map)
		out.WriteString("writeMap(" + g.typeToEnum(m.KeyType()) + ", " + g.typeToEnum(m.ValType()) + ", " + it + ") { (key, value) ->\n")
		g.indentUp()
		out.WriteString(g.indent())
		g.generateSerializeValue(out, m.KeyType(), "key")
		out.WriteString("\n")
		out.WriteString(g.indent())
		g.generateSerializeValue(out, m.ValType(), "value")
		out.WriteString("\n")
		g.indentDown()
		out.WriteString(g.indent() + "}")
	} else if t.IsSet() {
		s := t.(*sema.Set)
		out.WriteString("writeSet(" + g.typeToEnum(s.ElemType()) + ", " + it + ") {\n")
		g.indentUp()
		out.WriteString(g.indent())
		g.generateSerializeValue(out, s.ElemType(), "it")
		out.WriteString("\n")
		g.indentDown()
		out.WriteString(g.indent() + "}")
	} else if t.IsList() {
		l := t.(*sema.List)
		out.WriteString("writeList(" + g.typeToEnum(l.ElemType()) + ", " + it + ") {\n")
		g.indentUp()
		out.WriteString(g.indent())
		g.generateSerializeValue(out, l.ElemType(), "it")
		out.WriteString("\n")
		g.indentDown()
		out.WriteString(g.indent() + "}")
	} else {
		emit.Throw("not a container type: %s", t.Name())
	}
}

// generateDeserializeContainer is
// t_kotlin_generator::generate_deserialize_container.
//
// A container header's element count is read before any element, so a peer
// can name a count it never backs with data. buildMap/buildSet/buildList
// reserve an initial capacity capped at 1024 (coerceAtMost) rather than the
// raw count, then read every element; larger containers grow as elements
// are added. coerceAtMost keeps the original reject-on-negative behaviour,
// since a negative capacity still throws.
func (g *Generator) generateDeserializeContainer(out *strings.Builder, t sema.Type) {
	if t.IsMap() {
		m := t.(*sema.Map)
		out.WriteString("readMap { tmap ->\n")
		g.indentUp()
		out.WriteString(g.indent() + "buildMap(tmap.size.coerceAtMost(1024)) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "repeat(tmap.size) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "put(")
		g.generateDeserializeValue(out, m.KeyType())
		out.WriteString(", ")
		g.generateDeserializeValue(out, m.ValType())
		out.WriteString(")\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}")
	} else if t.IsSet() {
		s := t.(*sema.Set)
		out.WriteString("readSet { tset ->\n")
		g.indentUp()
		out.WriteString(g.indent() + "buildSet(tset.size.coerceAtMost(1024)) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "repeat(tset.size) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "add(")
		g.generateDeserializeValue(out, s.ElemType())
		out.WriteString(")\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}")
	} else if t.IsList() {
		l := t.(*sema.List)
		out.WriteString("readList { tlist ->\n")
		g.indentUp()
		out.WriteString(g.indent() + "buildList(tlist.size.coerceAtMost(1024)) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "repeat(tlist.size) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "add(")
		g.generateDeserializeValue(out, l.ElemType())
		out.WriteString(")\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}")
	} else {
		emit.Throw("not a container type: %s", t.Name())
	}
}
