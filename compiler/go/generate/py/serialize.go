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

package py

import (
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// generateDeserializeField deserializes a field of any type.
func (g *Generator) generateDeserializeField(out *strings.Builder, f *sema.Field, prefix string) {
	typ := sema.TrueType(f.Type())

	if typ.IsVoid() {
		throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, f.Name())
	}

	name := prefix + maybeEscapeIdentifier(f.Name())

	switch {
	case typ.IsStruct() || typ.IsXception():
		g.generateDeserializeStruct(out, typ.(*sema.Struct), name)
	case typ.IsContainer():
		g.generateDeserializeContainer(out, typ, name)
	case typ.IsBaseType():
		out.WriteString(g.indent() + name + " = iprot.")
		bt := typ.(*sema.BaseType)
		switch bt.Base() {
		case sema.TypeVoid:
			throw("compiler error: cannot serialize void field in a struct: %s", name)
		case sema.TypeString:
			if bt.IsBinary() {
				out.WriteString("readBinary()")
			} else if !g.opts.Utf8Strings {
				out.WriteString("readString()")
			} else {
				out.WriteString("readString().decode('utf-8', errors='replace') if sys.version_info[0] == 2 else iprot.readString()")
			}
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
			throw("compiler error: no Python name for base type %s", sema.BaseName(bt.Base()))
		}
		out.WriteString("\n")
	case typ.IsEnum():
		if g.opts.Enum {
			out.WriteString(g.indent() + name + " = " + g.typeName(typ) + "(iprot.readI32())")
		} else {
			out.WriteString(g.indent() + name + " = iprot.readI32()")
		}
		out.WriteString("\n")
	default:
		// The C++ generator prints a diagnostic to stdout and emits
		// nothing for the field; that silent fallback is reproduced here.
	}
}

// generateDeserializeStruct generates an unserializer for a struct,
// calling read().
func (g *Generator) generateDeserializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	if isImmutable(s) {
		out.WriteString(g.indent() + prefix + " = " + maybeEscapeIdentifier(g.typeName(s)) + ".read(iprot)\n")
	} else {
		out.WriteString(g.indent() + prefix + " = " + maybeEscapeIdentifier(g.typeName(s)) + "()\n" +
			g.indent() + prefix + ".read(iprot)\n")
	}
}

// generateDeserializeContainer serializes a container by writing out the
// header followed by data and then a footer.
func (g *Generator) generateDeserializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	size := g.tmp("_size")
	ktype := g.tmp("_ktype")
	vtype := g.tmp("_vtype")
	etype := g.tmp("_etype")

	// Declare variables, read the header.
	switch {
	case t.IsMap():
		out.WriteString(g.indent() + prefix + " = {}\n" + g.indent() +
			"(" + ktype + ", " + vtype + ", " + size + ") = iprot.readMapBegin()\n")
	case t.IsSet():
		out.WriteString(g.indent() + prefix + " = set()\n" + g.indent() +
			"(" + etype + ", " + size + ") = iprot.readSetBegin()\n")
	case t.IsList():
		out.WriteString(g.indent() + prefix + " = []\n" + g.indent() +
			"(" + etype + ", " + size + ") = iprot.readListBegin()\n")
	}

	// The for loop iterates over elements.
	i := g.tmp("_i")
	out.WriteString(g.indent() + "for " + i + " in range(" + size + "):\n")

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

	// Read the container end.
	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "iprot.readMapEnd()\n")
		if isImmutable(t) {
			out.WriteString(g.indent() + prefix + " = TFrozenDict(" + prefix + ")\n")
		}
	case t.IsSet():
		out.WriteString(g.indent() + "iprot.readSetEnd()\n")
		if isImmutable(t) {
			out.WriteString(g.indent() + prefix + " = frozenset(" + prefix + ")\n")
		}
	case t.IsList():
		if isImmutable(t) {
			out.WriteString(g.indent() + prefix + " = tuple(" + prefix + ")\n")
		}
		out.WriteString(g.indent() + "iprot.readListEnd()\n")
	}
}

// generateDeserializeMapElement generates code to deserialize a map.
func (g *Generator) generateDeserializeMapElement(out *strings.Builder, m *sema.Map, prefix string) {
	key := g.tmp("_key")
	val := g.tmp("_val")
	fkey := sema.NewField(m.KeyType(), key, 0)
	fval := sema.NewField(m.ValType(), val, 0)

	g.generateDeserializeField(out, fkey, "")
	g.generateDeserializeField(out, fval, "")

	out.WriteString(g.indent() + prefix + "[" + key + "] = " + val + "\n")
}

// generateDeserializeSetElement writes a set element.
func (g *Generator) generateDeserializeSetElement(out *strings.Builder, s *sema.Set, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(s.ElemType(), elem, 0)

	g.generateDeserializeField(out, felem, "")

	out.WriteString(g.indent() + prefix + ".add(" + elem + ")\n")
}

// generateDeserializeListElement writes a list element.
func (g *Generator) generateDeserializeListElement(out *strings.Builder, l *sema.List, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(l.ElemType(), elem, 0)

	g.generateDeserializeField(out, felem, "")

	out.WriteString(g.indent() + prefix + ".append(" + elem + ")\n")
}

// generateSerializeField serializes a field of any type. prefix is the
// name to prepend to the field name.
func (g *Generator) generateSerializeField(out *strings.Builder, f *sema.Field, prefix string) {
	typ := sema.TrueType(f.Type())

	if typ.IsVoid() {
		throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s%s", prefix, f.Name())
	}

	switch {
	case typ.IsStruct() || typ.IsXception():
		g.generateSerializeStruct(out, typ.(*sema.Struct), prefix+maybeEscapeIdentifier(f.Name()))
	case typ.IsContainer():
		g.generateSerializeContainer(out, typ, prefix+maybeEscapeIdentifier(f.Name()))
	case typ.IsBaseType() || typ.IsEnum():
		name := prefix + maybeEscapeIdentifier(f.Name())

		out.WriteString(g.indent() + "oprot.")

		if typ.IsBaseType() {
			bt := typ.(*sema.BaseType)
			switch bt.Base() {
			case sema.TypeVoid:
				throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if bt.IsBinary() {
					out.WriteString("writeBinary(" + name + ")")
				} else if !g.opts.Utf8Strings {
					out.WriteString("writeString(" + name + ")")
				} else {
					out.WriteString("writeString(" + name + ".encode('utf-8') if sys.version_info[0] == 2 else " + name + ")")
				}
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
				throw("compiler error: no Python name for base type %s", sema.BaseName(bt.Base()))
			}
		} else if typ.IsEnum() {
			if g.opts.Enum {
				out.WriteString("writeI32(" + name + ".value)")
			} else {
				out.WriteString("writeI32(" + name + ")")
			}
		}
		out.WriteString("\n")
	default:
		// Silent fallback, matching the C++ generator's printf-only branch.
	}
}

// generateSerializeStruct serializes all the members of a struct. prefix
// is the string prefix to attach to all fields.
func (g *Generator) generateSerializeStruct(out *strings.Builder, _ *sema.Struct, prefix string) {
	out.WriteString(g.indent() + prefix + ".write(oprot)\n")
}

func (g *Generator) generateSerializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		out.WriteString(g.indent() + "oprot.writeMapBegin(" + g.typeToEnum(m.KeyType()) + ", " +
			g.typeToEnum(m.ValType()) + ", " + "len(" + prefix + "))\n")
	case t.IsSet():
		s := t.(*sema.Set)
		out.WriteString(g.indent() + "oprot.writeSetBegin(" + g.typeToEnum(s.ElemType()) + ", " +
			"len(" + prefix + "))\n")
	case t.IsList():
		l := t.(*sema.List)
		out.WriteString(g.indent() + "oprot.writeListBegin(" + g.typeToEnum(l.ElemType()) + ", " +
			"len(" + prefix + "))\n")
	}

	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		kiter := g.tmp("kiter")
		viter := g.tmp("viter")
		out.WriteString(g.indent() + "for " + kiter + ", " + viter + " in " + prefix + ".items():\n")
		g.indentUp()
		g.generateSerializeMapElement(out, m, kiter, viter)
		g.indentDown()
	case t.IsSet():
		s := t.(*sema.Set)
		iter := g.tmp("iter")
		out.WriteString(g.indent() + "for " + iter + " in " + prefix + ":\n")
		g.indentUp()
		g.generateSerializeSetElement(out, s, iter)
		g.indentDown()
	case t.IsList():
		l := t.(*sema.List)
		iter := g.tmp("iter")
		out.WriteString(g.indent() + "for " + iter + " in " + prefix + ":\n")
		g.indentUp()
		g.generateSerializeListElement(out, l, iter)
		g.indentDown()
	}

	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "oprot.writeMapEnd()\n")
	case t.IsSet():
		out.WriteString(g.indent() + "oprot.writeSetEnd()\n")
	case t.IsList():
		out.WriteString(g.indent() + "oprot.writeListEnd()\n")
	}
}

// generateSerializeMapElement serializes the members of a map.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, m *sema.Map, kiter, viter string) {
	kfield := sema.NewField(m.KeyType(), kiter, 0)
	g.generateSerializeField(out, kfield, "")

	vfield := sema.NewField(m.ValType(), viter, 0)
	g.generateSerializeField(out, vfield, "")
}

// generateSerializeSetElement serializes the members of a set.
func (g *Generator) generateSerializeSetElement(out *strings.Builder, s *sema.Set, iter string) {
	efield := sema.NewField(s.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}

// generateSerializeListElement serializes the members of a list.
func (g *Generator) generateSerializeListElement(out *strings.Builder, l *sema.List, iter string) {
	efield := sema.NewField(l.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}
