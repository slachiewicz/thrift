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

// generateDeserializeField is t_haxe_generator::generate_deserialize_field:
// deserializes a field of any type.
func (g *generator) generateDeserializeField(out *strings.Builder, tfield *sema.Field, prefix string) {
	t := sema.TrueType(tfield.Type())

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, tfield.Name())
	}

	name := prefix + escapeHaxeKeyword(tfield.Name())

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateDeserializeStruct(out, t.(*sema.Struct), name)
	case t.IsContainer():
		g.generateDeserializeContainer(out, t, name)
	case t.IsBaseType() || t.IsEnum():
		g.indentRaw(out, name+" = iprot.")

		if t.IsBaseType() {
			switch t.(*sema.BaseType).Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if t.IsBinary() {
					out.WriteString("readBinary();")
				} else {
					out.WriteString("readString();")
				}
			case sema.TypeUUID:
				out.WriteString("readUuid();")
			case sema.TypeBool:
				out.WriteString("readBool();")
			case sema.TypeI8:
				out.WriteString("readByte();")
			case sema.TypeI16:
				out.WriteString("readI16();")
			case sema.TypeI32:
				out.WriteString("readI32();")
			case sema.TypeI64:
				out.WriteString("readI64();")
			case sema.TypeDouble:
				out.WriteString("readDouble();")
			default:
				emit.Throw("compiler error: no Haxe name for base type %s", sema.BaseName(t.(*sema.BaseType).Base()))
			}
		} else if t.IsEnum() {
			out.WriteString("readI32();")
		}
		out.WriteString("\n")
	default:
		// DO NOT KNOW HOW TO DESERIALIZE FIELD: printed by the C++
		// generator to stdout and otherwise ignored; unreachable for any
		// resolved type.
	}
}

// generateDeserializeStruct is t_haxe_generator::generate_deserialize_struct:
// generates an unserializer for a struct, invokes read().
func (g *generator) generateDeserializeStruct(out *strings.Builder, tstruct *sema.Struct, prefix string) {
	out.WriteString(g.indent() + prefix + " = new " + getCapName(g.typeName(tstruct)) + "();\n")
	out.WriteString(g.indent() + prefix + ".read(iprot);\n")
}

// generateDeserializeContainer is
// t_haxe_generator::generate_deserialize_container: deserializes a
// container by reading its size and then iterating.
func (g *generator) generateDeserializeContainer(out *strings.Builder, ttype sema.Type, prefix string) {
	g.scopeUp(out)

	var obj string
	switch {
	case ttype.IsMap():
		obj = g.tmp("_map")
	case ttype.IsSet():
		obj = g.tmp("_set")
	case ttype.IsList():
		obj = g.tmp("_list")
	}

	// Declare variables, read header
	switch {
	case ttype.IsMap():
		g.line(out, "var "+obj+" = iprot.readMapBegin();")
	case ttype.IsSet():
		g.line(out, "var "+obj+" = iprot.readSetBegin();")
	case ttype.IsList():
		g.line(out, "var "+obj+" = iprot.readListBegin();")
	}

	g.line(out, prefix+" = new "+g.typeName(ttype)+
		// size the collection correctly
		"("+
		");")

	// For loop iterates over elements
	i := g.tmp("_i")
	g.line(out, "for( "+i+" in 0 ... "+obj+".size)")

	g.scopeUp(out)

	switch {
	case ttype.IsMap():
		g.generateDeserializeMapElement(out, ttype.(*sema.Map), prefix)
	case ttype.IsSet():
		g.generateDeserializeSetElement(out, ttype.(*sema.Set), prefix)
	case ttype.IsList():
		g.generateDeserializeListElement(out, ttype.(*sema.List), prefix)
	}

	g.scopeDown(out)

	// Read container end
	switch {
	case ttype.IsMap():
		g.line(out, "iprot.readMapEnd();")
	case ttype.IsSet():
		g.line(out, "iprot.readSetEnd();")
	case ttype.IsList():
		g.line(out, "iprot.readListEnd();")
	}

	g.scopeDown(out)
}

// generateDeserializeMapElement is
// t_haxe_generator::generate_deserialize_map_element.
func (g *generator) generateDeserializeMapElement(out *strings.Builder, tmap *sema.Map, prefix string) {
	key := g.tmp("_key")
	val := g.tmp("_val")
	fkey := sema.NewField(tmap.KeyType(), key, 0)
	fval := sema.NewField(tmap.ValType(), val, 0)

	g.line(out, g.declareField(fkey, false))
	g.line(out, g.declareField(fval, false))

	g.generateDeserializeField(out, fkey, "")
	g.generateDeserializeField(out, fval, "")

	g.line(out, prefix+".set( "+key+", "+val+");")
}

// generateDeserializeSetElement is
// t_haxe_generator::generate_deserialize_set_element.
func (g *generator) generateDeserializeSetElement(out *strings.Builder, tset *sema.Set, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(tset.ElemType(), elem, 0)

	g.line(out, g.declareField(felem, false))

	g.generateDeserializeField(out, felem, "")

	g.line(out, prefix+".add("+elem+");")
}

// generateDeserializeListElement is
// t_haxe_generator::generate_deserialize_list_element.
func (g *generator) generateDeserializeListElement(out *strings.Builder, tlist *sema.List, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(tlist.ElemType(), elem, 0)

	g.line(out, g.declareField(felem, false))

	g.generateDeserializeField(out, felem, "")

	g.line(out, prefix+".add("+elem+");")
}

// generateSerializeField is t_haxe_generator::generate_serialize_field:
// serializes a field of any type.
func (g *generator) generateSerializeField(out *strings.Builder, tfield *sema.Field, prefix string) {
	t := sema.TrueType(tfield.Type())

	// Do nothing for void types
	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s%s", prefix, tfield.Name())
	}

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateSerializeStruct(out, t.(*sema.Struct), prefix+escapeHaxeKeyword(tfield.Name()))
	case t.IsContainer():
		g.generateSerializeContainer(out, t, prefix+escapeHaxeKeyword(tfield.Name()))
	case t.IsBaseType() || t.IsEnum():
		name := prefix + escapeHaxeKeyword(tfield.Name())
		g.indentRaw(out, "oprot.")

		if t.IsBaseType() {
			switch t.(*sema.BaseType).Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if t.IsBinary() {
					out.WriteString("writeBinary(" + name + ");")
				} else {
					out.WriteString("writeString(" + name + ");")
				}
			case sema.TypeUUID:
				out.WriteString("writeUuid(" + name + ");")
			case sema.TypeBool:
				out.WriteString("writeBool(" + name + ");")
			case sema.TypeI8:
				out.WriteString("writeByte(" + name + ");")
			case sema.TypeI16:
				out.WriteString("writeI16(" + name + ");")
			case sema.TypeI32:
				out.WriteString("writeI32(" + name + ");")
			case sema.TypeI64:
				out.WriteString("writeI64(" + name + ");")
			case sema.TypeDouble:
				out.WriteString("writeDouble(" + name + ");")
			default:
				emit.Throw("compiler error: no Haxe name for base type %s", sema.BaseName(t.(*sema.BaseType).Base()))
			}
		} else if t.IsEnum() {
			out.WriteString("writeI32(" + name + ");")
		}
		out.WriteString("\n")
	default:
		// DO NOT KNOW HOW TO SERIALIZE FIELD: printed by the C++ generator
		// to stdout and otherwise ignored; unreachable for any resolved
		// type.
	}
}

// generateSerializeStruct is t_haxe_generator::generate_serialize_struct:
// serializes all the members of a struct.
func (g *generator) generateSerializeStruct(out *strings.Builder, tstruct *sema.Struct, prefix string) {
	out.WriteString(g.indent() + prefix + ".write(oprot);\n")
}

// generateSerializeContainer is
// t_haxe_generator::generate_serialize_container: serializes a container
// by writing its size then the elements.
func (g *generator) generateSerializeContainer(out *strings.Builder, ttype sema.Type, prefix string) {
	switch {
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		iter := g.tmp("_key")
		counter := g.tmp("_sizeCounter")
		g.line(out, "var "+counter+" : Int = 0;")
		g.line(out, "for( "+iter+" in "+prefix+") {")
		g.indentRaw(out, "  "+counter+"++;\n")
		g.line(out, "}")

		g.line(out, "oprot.writeMapBegin(new TMap("+g.typeToEnum(m.KeyType())+", "+g.typeToEnum(m.ValType())+", "+counter+"));")
	case ttype.IsSet():
		s := ttype.(*sema.Set)
		g.line(out, "oprot.writeSetBegin(new TSet("+g.typeToEnum(s.ElemType())+", "+prefix+".size));")
	case ttype.IsList():
		l := ttype.(*sema.List)
		g.line(out, "oprot.writeListBegin(new TList("+g.typeToEnum(l.ElemType())+", "+prefix+".length));")
	}

	iter := g.tmp("elem")
	switch {
	case ttype.IsMap():
		g.line(out, "for( "+iter+" in "+prefix+".keys())")
	case ttype.IsSet():
		g.line(out, "for( "+iter+" in "+prefix+".toArray())")
	case ttype.IsList():
		g.line(out, "for( "+iter+" in "+prefix+")")
	}

	g.scopeUp(out)

	switch {
	case ttype.IsMap():
		g.generateSerializeMapElement(out, ttype.(*sema.Map), iter, prefix)
	case ttype.IsSet():
		g.generateSerializeSetElement(out, ttype.(*sema.Set), iter)
	case ttype.IsList():
		g.generateSerializeListElement(out, ttype.(*sema.List), iter)
	}

	g.scopeDown(out)

	switch {
	case ttype.IsMap():
		g.line(out, "oprot.writeMapEnd();")
	case ttype.IsSet():
		g.line(out, "oprot.writeSetEnd();")
	case ttype.IsList():
		g.line(out, "oprot.writeListEnd();")
	}
}

// generateSerializeMapElement is
// t_haxe_generator::generate_serialize_map_element.
func (g *generator) generateSerializeMapElement(out *strings.Builder, tmap *sema.Map, iter, mapVar string) {
	kfield := sema.NewField(tmap.KeyType(), iter, 0)
	g.generateSerializeField(out, kfield, "")
	vfield := sema.NewField(tmap.ValType(), mapVar+".get("+iter+")", 0)
	g.generateSerializeField(out, vfield, "")
}

// generateSerializeSetElement is
// t_haxe_generator::generate_serialize_set_element.
func (g *generator) generateSerializeSetElement(out *strings.Builder, tset *sema.Set, iter string) {
	efield := sema.NewField(tset.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}

// generateSerializeListElement is
// t_haxe_generator::generate_serialize_list_element.
func (g *generator) generateSerializeListElement(out *strings.Builder, tlist *sema.List, iter string) {
	efield := sema.NewField(tlist.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}
