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

package ocaml

import (
	"fmt"
	"os"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateDeserializeField is t_ocaml_generator::generate_deserialize_field:
// deserializes a field of any type.
func (g *Generator) generateDeserializeField(out *strings.Builder, tfield *sema.Field, prefix string) {
	typ := tfield.Type()
	name := decapitalize(tfield.Name())
	out.WriteString(g.indent() + prefix + "#set_" + name + " ")
	g.generateDeserializeType(out, typ)
	out.WriteString("\n")
}

// generateDeserializeType is t_ocaml_generator::generate_deserialize_type:
// deserializes a field of any type.
func (g *Generator) generateDeserializeType(out *strings.Builder, typ sema.Type) {
	typ = sema.TrueType(typ)

	if typ.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE")
	}

	switch {
	case typ.IsStruct() || typ.IsXception():
		g.generateDeserializeStruct(out, typ.(*sema.Struct))
	case typ.IsContainer():
		g.generateDeserializeContainer(out, typ)
	case typ.IsBaseType():
		out.WriteString("iprot#")
		b := typ.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeVoid:
			emit.Throw("compiler error: cannot serialize void field in a struct")
		case sema.TypeString:
			out.WriteString("readString")
		case sema.TypeBool:
			out.WriteString("readBool")
		case sema.TypeI8:
			out.WriteString("readByte")
		case sema.TypeI16:
			out.WriteString("readI16")
		case sema.TypeI32:
			out.WriteString("readI32")
		case sema.TypeI64:
			out.WriteString("readI64")
		case sema.TypeDouble:
			out.WriteString("readDouble")
		default:
			emit.Throw("compiler error: no ocaml name for base type %s", sema.BaseName(b.Base()))
		}
	case typ.IsEnum():
		ename := capitalize(typ.Name())
		out.WriteString("(" + ename + ".of_i iprot#readI32)")
	default:
		fmt.Fprintf(os.Stdout, "DO NOT KNOW HOW TO DESERIALIZE TYPE '%s'\n", typ.Name())
	}
}

// generateDeserializeStruct is
// t_ocaml_generator::generate_deserialize_struct: generates an
// unserializer for a struct, calling read().
func (g *Generator) generateDeserializeStruct(out *strings.Builder, s *sema.Struct) {
	prefix := ""
	program := s.Program()
	if program != nil && program != g.program {
		prefix = capitalize(program.Name()) + "_types."
	}
	name := decapitalize(s.Name())
	out.WriteString("(" + prefix + "read_" + name + " iprot)")
}

// generateDeserializeContainer is
// t_ocaml_generator::generate_deserialize_container: serializes a
// container by writing out the header followed by data and then a footer.
func (g *Generator) generateDeserializeContainer(out *strings.Builder, ttype sema.Type) {
	// C++ builds t_field locals (fsize/fktype/fvtype/fetype) from these
	// tmp() calls and never reads them; the calls are kept so the tmp_
	// counter stays in step for byte parity.
	size := g.tmp("_size")
	ktype := g.tmp("_ktype")
	vtype := g.tmp("_vtype")
	etype := g.tmp("_etype")
	con := g.tmp("_con")

	out.WriteString("\n")
	g.indentUp()
	switch {
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		out.WriteString(g.indent() + "(let (" + ktype + "," + vtype + "," + size + ") = iprot#readMapBegin in" + "\n")
		out.WriteString(g.indent() + "let " + con + " = Hashtbl.create (min " + size + " 1024) in" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + "for i = 1 to " + size + " do" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + "let _k = ")
		g.generateDeserializeType(out, m.KeyType())
		out.WriteString(" in" + "\n")
		out.WriteString(g.indent() + "let _v = ")
		g.generateDeserializeType(out, m.ValType())
		out.WriteString(" in" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + "Hashtbl.add " + con + " _k _v" + "\n")
		g.indentDown()
		g.indentDown()
		out.WriteString(g.indent() + "done; iprot#readMapEnd; " + con + ")")
		g.indentDown()
	case ttype.IsSet():
		s := ttype.(*sema.Set)
		out.WriteString(g.indent() + "(let (" + etype + "," + size + ") = iprot#readSetBegin in" + "\n")
		out.WriteString(g.indent() + "let " + con + " = Hashtbl.create (min " + size + " 1024) in" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + "for i = 1 to " + size + " do" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + "Hashtbl.add " + con + " ")
		g.generateDeserializeType(out, s.ElemType())
		out.WriteString(" true" + "\n")
		g.indentDown()
		out.WriteString(g.indent() + "done; iprot#readSetEnd; " + con + ")")
		g.indentDown()
	case ttype.IsList():
		l := ttype.(*sema.List)
		// The element count is read before any element, so read the
		// elements one at a time into an accumulator rather than
		// allocating an Array.init of the declared count up front;
		// readListBegin has already rejected a negative size.
		out.WriteString(g.indent() + "(let (" + etype + "," + size + ") = iprot#readListBegin in" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + "let " + con + " = ref [] in" + "\n")
		out.WriteString(g.indent() + "for _i = 1 to " + size + " do" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + con + " := (")
		g.generateDeserializeType(out, l.ElemType())
		out.WriteString(") :: !" + con + "\n")
		g.indentDown()
		out.WriteString(g.indent() + "done; iprot#readListEnd; List.rev !" + con + ")")
		g.indentDown()
	}
	g.indentDown()
}

// generateSerializeField is t_ocaml_generator::generate_serialize_field:
// serializes a field of any type.
func (g *Generator) generateSerializeField(out *strings.Builder, tfield *sema.Field, name string) {
	typ := sema.TrueType(tfield.Type())

	// Do nothing for void types
	if typ.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s", tfield.Name())
	}

	if name == "" {
		name = decapitalize(tfield.Name())
	}

	switch {
	case typ.IsStruct() || typ.IsXception():
		g.generateSerializeStruct(out, typ.(*sema.Struct), name)
	case typ.IsContainer():
		g.generateSerializeContainer(out, typ, name)
	case typ.IsBaseType() || typ.IsEnum():
		out.WriteString(g.indent() + "oprot#")

		if typ.IsBaseType() {
			b := typ.(*sema.BaseType)
			switch b.Base() {
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
			default:
				emit.Throw("compiler error: no ocaml name for base type %s", sema.BaseName(b.Base()))
			}
		} else if typ.IsEnum() {
			ename := capitalize(typ.Name())
			out.WriteString("writeI32(" + ename + ".to_i " + name + ")")
		}
	default:
		fmt.Fprintf(os.Stdout, "DO NOT KNOW HOW TO SERIALIZE FIELD '%s' TYPE '%s'\n", tfield.Name(), typ.Name())
	}
	out.WriteString(";" + "\n")
}

// generateSerializeStruct is t_ocaml_generator::generate_serialize_struct:
// serializes all the members of a struct.
func (g *Generator) generateSerializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	_ = s
	out.WriteString(g.indent() + prefix + "#write(oprot)")
}

// generateSerializeContainer is
// t_ocaml_generator::generate_serialize_container.
func (g *Generator) generateSerializeContainer(out *strings.Builder, ttype sema.Type, prefix string) {
	switch {
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		out.WriteString(g.indent() + "oprot#writeMapBegin(" + g.typeToEnum(m.KeyType()) + ",")
		out.WriteString(g.typeToEnum(m.ValType()) + ",")
		out.WriteString("Hashtbl.length " + prefix + ");" + "\n")
	case ttype.IsSet():
		s := ttype.(*sema.Set)
		out.WriteString(g.indent() + "oprot#writeSetBegin(" + g.typeToEnum(s.ElemType()) + ",")
		out.WriteString("Hashtbl.length " + prefix + ");" + "\n")
	case ttype.IsList():
		l := ttype.(*sema.List)
		out.WriteString(g.indent() + "oprot#writeListBegin(" + g.typeToEnum(l.ElemType()) + ",")
		out.WriteString("List.length " + prefix + ");" + "\n")
	}

	switch {
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		kiter := g.tmp("_kiter")
		viter := g.tmp("_viter")
		out.WriteString(g.indent() + "Hashtbl.iter (fun " + kiter + " -> fun " + viter + " -> " + "\n")
		g.indentUp()
		g.generateSerializeMapElement(out, m, kiter, viter)
		g.indentDown()
		out.WriteString(g.indent() + ") " + prefix + ";" + "\n")
	case ttype.IsSet():
		s := ttype.(*sema.Set)
		iter := g.tmp("_iter")
		out.WriteString(g.indent() + "Hashtbl.iter (fun " + iter + " -> fun _ -> ")
		g.indentUp()
		g.generateSerializeSetElement(out, s, iter)
		g.indentDown()
		out.WriteString(g.indent() + ") " + prefix + ";" + "\n")
	case ttype.IsList():
		l := ttype.(*sema.List)
		iter := g.tmp("_iter")
		out.WriteString(g.indent() + "List.iter (fun " + iter + " -> ")
		g.indentUp()
		g.generateSerializeListElement(out, l, iter)
		g.indentDown()
		out.WriteString(g.indent() + ") " + prefix + ";" + "\n")
	}

	switch {
	case ttype.IsMap():
		out.WriteString(g.indent() + "oprot#writeMapEnd")
	case ttype.IsSet():
		out.WriteString(g.indent() + "oprot#writeSetEnd")
	case ttype.IsList():
		out.WriteString(g.indent() + "oprot#writeListEnd")
	}
}

// generateSerializeMapElement is
// t_ocaml_generator::generate_serialize_map_element: serializes the
// members of a map.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, tmap *sema.Map, kiter, viter string) {
	kfield := sema.NewField(tmap.KeyType(), kiter, 0)
	g.generateSerializeField(out, kfield, "")

	vfield := sema.NewField(tmap.ValType(), viter, 0)
	g.generateSerializeField(out, vfield, "")
}

// generateSerializeSetElement is
// t_ocaml_generator::generate_serialize_set_element: serializes the
// members of a set.
func (g *Generator) generateSerializeSetElement(out *strings.Builder, tset *sema.Set, iter string) {
	efield := sema.NewField(tset.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}

// generateSerializeListElement is
// t_ocaml_generator::generate_serialize_list_element: serializes the
// members of a list.
func (g *Generator) generateSerializeListElement(out *strings.Builder, tlist *sema.List, iter string) {
	efield := sema.NewField(tlist.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}
