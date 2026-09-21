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

package lua

import (
	"fmt"
	"os"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// ---- Deserialization (Read) ----

// generateDeserializeField is t_lua_generator::generate_deserialize_field.
func (g *Generator) generateDeserializeField(out *strings.Builder, f *sema.Field, local bool, prefix string) {
	t := sema.TrueType(f.Type())

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, f.Name())
	}

	name := prefix + f.Name()

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateDeserializeStruct(out, t.(*sema.Struct), local, name)
	case t.IsContainer():
		g.generateDeserializeContainer(out, t, local, name)
	case t.IsBaseType() || t.IsEnum():
		localKw := ""
		if local {
			localKw = "local "
		}
		out.WriteString(g.indent() + localKw + name + " = iprot:")

		if t.IsBaseType() {
			bt := t.(*sema.BaseType)
			switch bt.Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				out.WriteString("readString()")
			case sema.TypeBool:
				out.WriteString("readBool()")
			case sema.TypeI8:
				out.WriteString("readByte()")
			case sema.TypeI16:
				out.WriteString("readI16()")
			case sema.TypeI32:
				out.WriteString("readI32()")
			case sema.TypeI64:
				out.WriteString("readI64()")
			case sema.TypeDouble:
				out.WriteString("readDouble()")
			case sema.TypeUUID:
				out.WriteString("readUuid()")
			default:
				emit.Throw("compiler error: no PHP name for base type %s", sema.BaseName(bt.Base()))
			}
		} else if t.IsEnum() {
			out.WriteString("readI32()")
		}
		out.WriteString("\n")

	default:
		fmt.Fprintf(os.Stdout, "DO NOT KNOW HOW TO DESERIALIZE FIELD '%s' TYPE '%s'\n", f.Name(), t.Name())
	}
}

// generateDeserializeStruct is t_lua_generator::generate_deserialize_struct.
func (g *Generator) generateDeserializeStruct(out *strings.Builder, s *sema.Struct, local bool, prefix string) {
	localKw := ""
	if local {
		localKw = "local "
	}
	out.WriteString(g.indent() + localKw + prefix + " = " + s.Name() + ":new{}" + "\n" +
		g.indent() + prefix + ":read(iprot)" + "\n")
}

// generateDeserializeContainer is
// t_lua_generator::generate_deserialize_container. The C++ original also
// declares t_field locals for size/ktype/vtype/etype that no code ever
// reads; they are dropped here since they produce no output, but the four
// tmp() calls that name the Lua locals are kept unconditionally, exactly
// as the C++ source makes them regardless of the container kind, so that
// later tmp names stay in sync with the C++ compiler's output.
func (g *Generator) generateDeserializeContainer(out *strings.Builder, t sema.Type, local bool, prefix string) {
	size := g.tmp("_size")
	ktype := g.tmp("_ktype")
	vtype := g.tmp("_vtype")
	etype := g.tmp("_etype")

	localKw := ""
	if local {
		localKw = "local "
	}
	out.WriteString(g.indent() + localKw + prefix + " = {}" + "\n")
	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "local " + ktype + ", " + vtype + ", " + size + " = iprot:readMapBegin() " + "\n")
	case t.IsSet():
		out.WriteString(g.indent() + "local " + etype + ", " + size + " = iprot:readSetBegin()" + "\n")
	case t.IsList():
		out.WriteString(g.indent() + "local " + etype + ", " + size + " = iprot:readListBegin()" + "\n")
	}

	out.WriteString(g.indent() + "for _i=1," + size + " do" + "\n")
	g.indentUp()

	switch {
	case t.IsMap():
		g.generateDeserializeMapElement(out, t.(*sema.Map), prefix)
	case t.IsSet():
		g.generateDeserializeSetElement(out, t.(*sema.Set), prefix)
	case t.IsList():
		g.generateDeserializeListElement(out, t.(*sema.List), prefix)
	}

	g.indentDown()
	g.line(out, "end")

	switch {
	case t.IsMap():
		g.line(out, "iprot:readMapEnd()")
	case t.IsSet():
		g.line(out, "iprot:readSetEnd()")
	case t.IsList():
		g.line(out, "iprot:readListEnd()")
	}
}

// generateDeserializeMapElement is
// t_lua_generator::generate_deserialize_map_element: a map is represented
// by a table indexable by any lua type.
func (g *Generator) generateDeserializeMapElement(out *strings.Builder, m *sema.Map, prefix string) {
	key := g.tmp("_key")
	val := g.tmp("_val")
	fkey := sema.NewField(m.KeyType(), key, 0)
	fval := sema.NewField(m.ValType(), val, 0)

	g.generateDeserializeField(out, fkey, true, "")
	g.generateDeserializeField(out, fval, true, "")

	out.WriteString(g.indent() + prefix + "[" + key + "] = " + val + "\n")
}

// generateDeserializeSetElement is
// t_lua_generator::generate_deserialize_set_element: a set is represented
// by a table indexed by the value.
func (g *Generator) generateDeserializeSetElement(out *strings.Builder, s *sema.Set, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(s.ElemType(), elem, 0)

	g.generateDeserializeField(out, felem, true, "")

	out.WriteString(g.indent() + prefix + "[" + elem + "] = " + elem + "\n")
}

// generateDeserializeListElement is
// t_lua_generator::generate_deserialize_list_element: a list is
// represented by a table indexed by integer values. LUA natively provides
// all of the functions required to maintain a list.
func (g *Generator) generateDeserializeListElement(out *strings.Builder, l *sema.List, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(l.ElemType(), elem, 0)

	g.generateDeserializeField(out, felem, true, "")

	out.WriteString(g.indent() + "table.insert(" + prefix + ", " + elem + ")" + "\n")
}

// ---- Serialization (Write) ----

// generateSerializeField is t_lua_generator::generate_serialize_field.
func (g *Generator) generateSerializeField(out *strings.Builder, f *sema.Field, prefix string) {
	t := sema.TrueType(f.Type())
	name := prefix + f.Name()

	// Do nothing for void types
	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s", name)
	}

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateSerializeStruct(out, t.(*sema.Struct), name)
	case t.IsContainer():
		g.generateSerializeContainer(out, t, name)
	case t.IsBaseType() || t.IsEnum():
		out.WriteString(g.indent() + "oprot:")

		if t.IsBaseType() {
			bt := t.(*sema.BaseType)
			switch bt.Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				out.WriteString("writeString(" + name + ")")
			case sema.TypeBool:
				out.WriteString("writeBool(" + name + ")")
			case sema.TypeI8:
				out.WriteString("writeByte(" + name + ")")
			case sema.TypeI16:
				out.WriteString("writeI16(" + name + ")")
			case sema.TypeI32:
				out.WriteString("writeI32(" + name + ")")
			case sema.TypeI64:
				out.WriteString("writeI64(" + name + ")")
			case sema.TypeDouble:
				out.WriteString("writeDouble(" + name + ")")
			case sema.TypeUUID:
				out.WriteString("writeUuid(" + name + ")")
			default:
				emit.Throw("compiler error: no PHP name for base type %s", sema.BaseName(bt.Base()))
			}
		} else if t.IsEnum() {
			out.WriteString("writeI32(" + name + ")")
		}
		out.WriteString("\n")
	default:
		fmt.Fprintf(os.Stdout, "DO NOT KNOW HOW TO SERIALIZE FIELD '%s' TYPE '%s'\n", name, t.Name())
	}
}

// generateSerializeStruct is t_lua_generator::generate_serialize_struct.
func (g *Generator) generateSerializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	_ = s
	g.line(out, prefix+":write(oprot)")
}

// generateSerializeContainer is
// t_lua_generator::generate_serialize_container.
func (g *Generator) generateSerializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	// Begin writing
	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		g.line(out, "oprot:writeMapBegin("+g.typeToEnum(m.KeyType())+", "+g.typeToEnum(m.ValType())+", "+"ttable_size("+prefix+"))")
	case t.IsSet():
		s := t.(*sema.Set)
		g.line(out, "oprot:writeSetBegin("+g.typeToEnum(s.ElemType())+", "+"ttable_size("+prefix+"))")
	case t.IsList():
		l := t.(*sema.List)
		g.line(out, "oprot:writeListBegin("+g.typeToEnum(l.ElemType())+", "+"#"+prefix+")")
	}

	// Serialize
	switch {
	case t.IsMap():
		kiter := g.tmp("kiter")
		viter := g.tmp("viter")
		g.line(out, "for "+kiter+","+viter+" in pairs("+prefix+") do")
		g.indentUp()
		g.generateSerializeMapElement(out, t.(*sema.Map), kiter, viter)
		g.indentDown()
		g.line(out, "end")
	case t.IsSet():
		iter := g.tmp("iter")
		g.line(out, "for "+iter+",_ in pairs("+prefix+") do")
		g.indentUp()
		g.generateSerializeSetElement(out, t.(*sema.Set), iter)
		g.indentDown()
		g.line(out, "end")
	case t.IsList():
		iter := g.tmp("iter")
		g.line(out, "for _,"+iter+" in ipairs("+prefix+") do")
		g.indentUp()
		g.generateSerializeListElement(out, t.(*sema.List), iter)
		g.indentDown()
		g.line(out, "end")
	}

	// Finish writing
	switch {
	case t.IsMap():
		g.line(out, "oprot:writeMapEnd()")
	case t.IsSet():
		g.line(out, "oprot:writeSetEnd()")
	case t.IsList():
		g.line(out, "oprot:writeListEnd()")
	}
}

// generateSerializeMapElement is
// t_lua_generator::generate_serialize_map_element.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, m *sema.Map, kiter, viter string) {
	kfield := sema.NewField(m.KeyType(), kiter, 0)
	g.generateSerializeField(out, kfield, "")

	vfield := sema.NewField(m.ValType(), viter, 0)
	g.generateSerializeField(out, vfield, "")
}

// generateSerializeSetElement is
// t_lua_generator::generate_serialize_set_element.
func (g *Generator) generateSerializeSetElement(out *strings.Builder, s *sema.Set, iter string) {
	efield := sema.NewField(s.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}

// generateSerializeListElement is
// t_lua_generator::generate_serialize_list_element.
func (g *Generator) generateSerializeListElement(out *strings.Builder, l *sema.List, iter string) {
	efield := sema.NewField(l.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}
