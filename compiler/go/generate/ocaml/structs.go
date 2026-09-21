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
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// generateStruct is t_ocaml_generator::generate_struct.
func (g *Generator) generateStruct(s *sema.Struct) {
	g.generateOcamlStruct(s, false)
}

// generateXception is t_ocaml_generator::generate_xception: generates a
// struct definition for a thrift exception. Basically the same as a
// struct, but also has an exception declaration.
func (g *Generator) generateXception(s *sema.Struct) {
	g.generateOcamlStruct(s, true)
}

// generateOcamlStruct is t_ocaml_generator::generate_ocaml_struct.
func (g *Generator) generateOcamlStruct(s *sema.Struct, isException bool) {
	g.generateOcamlStructDefinition(&g.fTypes, s, isException)
	g.generateOcamlStructSig(&g.fTypesI, s, isException)
}

// generateOcamlMethodCopy is t_ocaml_generator::generate_ocaml_method_copy.
func (g *Generator) generateOcamlMethodCopy(out *strings.Builder, members []*sema.Field) {
	// Create a copy of the current object
	out.WriteString(g.indent() + "method copy =" + "\n")
	g.indentUp()
	g.indentUp()
	out.WriteString(g.indent() + "let _new = Oo.copy self in" + "\n")
	for _, m := range members {
		g.generateOcamlMemberCopy(out, m)
	}

	g.indentDown()
	out.WriteString(g.indent() + "_new" + "\n")
	g.indentDown()
}

// structMemberCopyOf is t_ocaml_generator::struct_member_copy_of. It
// deliberately checks the type as given, not its true type: only the
// top-level call site (generateOcamlMemberCopy) resolves typedefs once, so
// a typedef of a container's key/value/element type is copied by reference
// here, exactly as t_type::is_struct() etc. would see it unresolved.
func (g *Generator) structMemberCopyOf(typ sema.Type, what string) string {
	if typ.IsStruct() || typ.IsXception() {
		return what + "#copy"
	}
	if typ.IsMap() {
		m := typ.(*sema.Map)
		copyOfK := g.structMemberCopyOf(m.KeyType(), "k")
		copyOfV := g.structMemberCopyOf(m.ValType(), "v")

		if copyOfK == "k" && copyOfV == "v" {
			return "(Hashtbl.copy " + what + ")"
		}
		return "((fun oh -> let nh = Hashtbl.create (Hashtbl.length oh) in Hashtbl.iter (fun k v -> Hashtbl.add nh " +
			copyOfK + " " + copyOfV + ") oh; nh) " + what + ")"
	}
	if typ.IsSet() {
		s := typ.(*sema.Set)
		copyOf := g.structMemberCopyOf(s.ElemType(), "k")

		if copyOf == "k" {
			return "(Hashtbl.copy " + what + ")"
		}
		return "((fun oh -> let nh = Hashtbl.create (Hashtbl.length oh) in Hashtbl.iter (fun k v -> Hashtbl.add nh " +
			copyOf + " true) oh; nh) " + what + ")"
	}
	if typ.IsList() {
		l := typ.(*sema.List)
		copyOf := g.structMemberCopyOf(l.ElemType(), "x")
		if copyOf != "x" {
			return "(List.map (fun x -> " + copyOf + ") " + what + ")"
		}
		return what
	}
	return what
}

// generateOcamlMemberCopy is t_ocaml_generator::generate_ocaml_member_copy.
func (g *Generator) generateOcamlMemberCopy(out *strings.Builder, tmember *sema.Field) {
	mname := decapitalize(tmember.Name())
	typ := sema.TrueType(tmember.Type())

	grabField := "self#grab_" + mname
	copyOf := g.structMemberCopyOf(typ, grabField)
	if copyOf != grabField {
		out.WriteString(g.indent())
		if !g.structMemberPersistent(tmember) {
			out.WriteString("if _" + mname + " <> None then" + "\n")
			out.WriteString(g.indent() + "  ")
		}
		out.WriteString("_new#set_" + mname + " " + copyOf + ";" + "\n")
	}
}

// structMemberPersistent is t_ocaml_generator::struct_member_persistent:
// checks whether a member of the structure can not have an undefined value.
func (g *Generator) structMemberPersistent(tmember *sema.Field) bool {
	return tmember.Value() != nil
}

// structMemberOmitable is t_ocaml_generator::struct_member_omitable: checks
// whether a member of the structure can be skipped during encoding.
func (g *Generator) structMemberOmitable(tmember *sema.Field) bool {
	return tmember.Req() != sema.Required
}

// structMemberDefaultCheaplyComparable is
// t_ocaml_generator::struct_member_default_cheaply_comparable. It calls
// value.Double() unconditionally, exactly as the C++ source calls
// val->get_double() unconditionally: for a double field whose default was
// written as a bare integer literal, the const value's Kind() is CVInteger
// and its double field was never set, so this reads a plain 0 regardless of
// the literal's actual value - a latent quirk of the C++ source, reproduced
// rather than fixed.
func (g *Generator) structMemberDefaultCheaplyComparable(tmember *sema.Field) bool {
	typ := sema.TrueType(tmember.Type())
	val := tmember.Value()
	if val == nil {
		return false
	} else if typ.IsBaseType() {
		b := typ.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeDouble:
			return val.Double() == 0.0
		default:
			return true
		}
	} else if typ.IsList() {
		return len(val.List()) == 0
	}
	return false
}

// generateOcamlStructDefinition is
// t_ocaml_generator::generate_ocaml_struct_definition.
func (g *Generator) generateOcamlStructDefinition(out *strings.Builder, s *sema.Struct, isException bool) {
	members := s.Members()
	tname := g.typeName(s)
	out.WriteString(g.indent() + "class " + tname + " =" + "\n")
	out.WriteString(g.indent() + "object (self)" + "\n")

	g.indentUp()

	if len(members) > 0 {
		for _, m := range members {
			g.generateOcamlStructMember(out, tname, m)
			out.WriteString("\n")
		}
	}
	g.generateOcamlMethodCopy(out, members)
	g.generateOcamlStructWriter(out, s)
	g.indentDown()
	out.WriteString(g.indent() + "end" + "\n")

	if isException {
		out.WriteString(g.indent() + "exception " + capitalize(tname) + " of " + tname + "\n")
	}

	g.generateOcamlStructReader(out, s)
}

// generateOcamlStructMember is
// t_ocaml_generator::generate_ocaml_struct_member: generates a structure
// member for a thrift data type.
func (g *Generator) generateOcamlStructMember(out *strings.Builder, tname string, tmember *sema.Field) {
	x := g.tmp("_x")
	mname := decapitalize(tmember.Name())

	out.WriteString(g.indent() + "val mutable _" + mname + " : " + g.renderOcamlType(tmember.Type()))
	val := tmember.Value()
	if val != nil {
		if g.structMemberPersistent(tmember) {
			out.WriteString(" = " + g.renderConstValue(tmember.Type(), tmember.Value()) + "\n")
		} else {
			out.WriteString(" option = Some " + g.renderConstValue(tmember.Type(), tmember.Value()) + "\n")
		}
	} else {
		// assert(!struct_member_persistent(tmember))
		out.WriteString(" option = None" + "\n")
	}

	if g.structMemberPersistent(tmember) {
		out.WriteString(g.indent() + "method get_" + mname + " = Some _" + mname + "\n")
		out.WriteString(g.indent() + "method grab_" + mname + " = _" + mname + "\n")
		out.WriteString(g.indent() + "method set_" + mname + " " + x + " = _" + mname + " <- " + x + "\n")
	} else {
		out.WriteString(g.indent() + "method get_" + mname + " = _" + mname + "\n")
		out.WriteString(g.indent() + "method grab_" + mname + " = match _" + mname +
			" with None->raise (Field_empty \"" + tname + "." + mname + "\") | Some " + x + " -> " + x + "\n")
		out.WriteString(g.indent() + "method set_" + mname + " " + x + " = _" + mname + " <- Some " + x + "\n")
		out.WriteString(g.indent() + "method unset_" + mname + " = _" + mname + " <- None" + "\n")
	}

	out.WriteString(g.indent() + "method reset_" + mname + " = _" + mname + " <- ")
	if val != nil {
		if g.structMemberPersistent(tmember) {
			out.WriteString(g.renderConstValue(tmember.Type(), tmember.Value()) + "\n")
		} else {
			out.WriteString("Some " + g.renderConstValue(tmember.Type(), tmember.Value()) + "\n")
		}
	} else {
		out.WriteString("None" + "\n")
	}
}

// generateOcamlStructSig is t_ocaml_generator::generate_ocaml_struct_sig.
func (g *Generator) generateOcamlStructSig(out *strings.Builder, s *sema.Struct, isException bool) {
	members := s.Members()
	tname := g.typeName(s)
	out.WriteString(g.indent() + "class " + tname + " :" + "\n")
	out.WriteString(g.indent() + "object ('a)" + "\n")

	g.indentUp()

	// C++ declares "string x = tmp(\"_x\");" here and never reads it; the
	// call is kept so the tmp_ counter stays in step for byte parity.
	_ = g.tmp("_x")
	if len(members) > 0 {
		for _, m := range members {
			mname := decapitalize(m.Name())
			typ := g.renderOcamlType(m.Type())
			out.WriteString(g.indent() + "method get_" + mname + " : " + typ + " option" + "\n")
			out.WriteString(g.indent() + "method grab_" + mname + " : " + typ + "\n")
			out.WriteString(g.indent() + "method set_" + mname + " : " + typ + " -> unit" + "\n")
			if !g.structMemberPersistent(m) {
				out.WriteString(g.indent() + "method unset_" + mname + " : unit" + "\n")
			}
			out.WriteString(g.indent() + "method reset_" + mname + " : unit" + "\n")
		}
	}
	out.WriteString(g.indent() + "method copy : 'a" + "\n")
	out.WriteString(g.indent() + "method write : Protocol.t -> unit" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "end" + "\n")

	if isException {
		out.WriteString(g.indent() + "exception " + capitalize(tname) + " of " + tname + "\n")
	}

	out.WriteString(g.indent() + "val read_" + tname + " : Protocol.t -> " + tname + "\n")
}

// generateOcamlStructReader is t_ocaml_generator::generate_ocaml_struct_reader.
func (g *Generator) generateOcamlStructReader(out *strings.Builder, s *sema.Struct) {
	fields := s.Members()
	sname := g.typeName(s)
	str := g.tmp("_str")
	t := g.tmp("_t")
	id := g.tmp("_id")
	out.WriteString(g.indent() + "let rec read_" + sname + " (iprot : Protocol.t) =" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "let " + str + " = new " + sname + " in" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "iprot#increment_recursion_depth;" + "\n")
	out.WriteString(g.indent() + "(Fun.protect ~finally:(fun () -> iprot#decrement_recursion_depth) (fun () ->" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "ignore(iprot#readStructBegin);" + "\n")

	// Loop over reading in fields
	out.WriteString(g.indent() + "(try while true do" + "\n")
	g.indentUp()
	g.indentUp()

	// Read beginning field marker
	out.WriteString(g.indent() + "let (_," + t + "," + id + ") = iprot#readFieldBegin in" + "\n")

	// Check for field STOP marker and break
	out.WriteString(g.indent() + "if " + t + " = Protocol.T_STOP then" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "raise Break" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "else ();" + "\n")

	out.WriteString(g.indent() + "(match " + id + " with " + "\n")
	g.indentUp()
	// Generate deserialization code for known cases
	for _, f := range fields {
		out.WriteString(g.indent() + "| " + strconv.Itoa(int(f.Key())) + " -> (")
		out.WriteString("if " + t + " = " + g.typeToEnum(f.Type()) + " then" + "\n")
		g.indentUp()
		g.indentUp()
		g.generateDeserializeField(out, f, str)
		g.indentDown()
		out.WriteString(g.indent() + "else" + "\n" + g.indent() + "  iprot#skip " + t + ")" + "\n")
		g.indentDown()
	}

	// In the default case we skip the field
	out.WriteString(g.indent() + "| _ -> " + "iprot#skip " + t + ");" + "\n")
	g.indentDown()
	// Read field end marker
	out.WriteString(g.indent() + "iprot#readFieldEnd;" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "done; ()" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "with Break -> ());" + "\n")

	out.WriteString(g.indent() + "iprot#readStructEnd" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "));" + "\n")

	out.WriteString(g.indent() + str + "\n" + "\n")
	g.indentDown()
	g.indentDown()
}

// generateOcamlStructWriter is t_ocaml_generator::generate_ocaml_struct_writer.
func (g *Generator) generateOcamlStructWriter(out *strings.Builder, s *sema.Struct) {
	name := s.Name()
	fields := s.SortedMembers()
	// C++ declares "string str = tmp(\"_str\");" and "string f = tmp(\"_f\");"
	// here and never reads either; the calls are kept so the tmp_ counter
	// stays in step for byte parity.
	_ = g.tmp("_str")
	_ = g.tmp("_f")

	out.WriteString(g.indent() + "method write (oprot : Protocol.t) =" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "oprot#increment_recursion_depth;" + "\n")
	out.WriteString(g.indent() + "Fun.protect ~finally:(fun () -> oprot#decrement_recursion_depth) (fun () ->" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "oprot#writeStructBegin \"" + name + "\";" + "\n")

	for _, tmember := range fields {
		mname := "_" + decapitalize(tmember.Name())
		var v string

		if g.structMemberPersistent(tmember) {
			if g.structMemberOmitable(tmember) && g.structMemberDefaultCheaplyComparable(tmember) {
				v = "_v"
				// Avoid redundant encoding of members having default values.
				out.WriteString(g.indent() + "(match " + mname + " with " +
					g.renderConstValue(tmember.Type(), tmember.Value()) + " -> () | " + v + " -> " + "\n")
			} else {
				v = mname
				out.WriteString(g.indent() + "(" + "\n")
			}
		} else {
			out.WriteString(g.indent() + "(match " + mname + " with ")

			if g.structMemberOmitable(tmember) {
				out.WriteString("None -> ()")

				if g.structMemberDefaultCheaplyComparable(tmember) {
					// Avoid redundant encoding of members having default values.
					out.WriteString(" | Some " + g.renderConstValue(tmember.Type(), tmember.Value()) + " -> ()")
				}
				out.WriteString(" | Some _v -> " + "\n")
			} else {
				out.WriteString("\n")
				out.WriteString(g.indent() + "| None -> raise (Field_empty \"" + g.typeName(s) + "." + mname + "\")" + "\n")
				out.WriteString(g.indent() + "| Some _v -> " + "\n")
			}

			v = "_v"
		}
		g.indentUp()
		// Write field header
		out.WriteString(g.indent() + "oprot#writeFieldBegin(\"" + tmember.Name() + "\"," +
			g.typeToEnum(tmember.Type()) + "," + strconv.Itoa(int(tmember.Key())) + ");" + "\n")

		// Write field contents
		g.generateSerializeField(out, tmember, v)

		// Write field closer
		out.WriteString(g.indent() + "oprot#writeFieldEnd" + "\n")

		g.indentDown()
		out.WriteString(g.indent() + ");" + "\n")
	}

	// Write the struct map
	out.WriteString(g.indent() + "oprot#writeFieldStop;" + "\n" + g.indent() + "oprot#writeStructEnd" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + ")" + "\n")

	g.indentDown()
}
