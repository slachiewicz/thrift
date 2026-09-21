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

package js

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateDeserializeField is generate_deserialize_field: deserializes a
// field of any type.
func (g *Generator) generateDeserializeField(out *strings.Builder, f *sema.Field, prefix string) {
	t := sema.TrueType(f.Type())

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, f.Name())
	}

	name := prefix + f.Name()

	switch {
	case t.IsStruct(), t.IsXception():
		g.generateDeserializeStruct(out, t.(*sema.Struct), name)
	case t.IsContainer():
		g.generateDeserializeContainer(out, t, name)
	case t.IsBaseType(), t.IsEnum():
		// In bigint mode the protocol still returns a node-int64 Int64;
		// wrap the read site in `thrift.toBigInt(...)` so the assigned
		// value matches the generated `bigint` type. gen_bigint_ is only
		// true when gen_node_ is true, so the `.value` branch below is
		// never reached in this mode.
		wrapI64Bigint := g.opts.Bigint && t.IsBaseType() && t.(*sema.BaseType).Base() == sema.TypeI64

		out.WriteString(g.indent() + name + " = ")
		if wrapI64Bigint {
			out.WriteString("thrift.toBigInt(")
		}
		out.WriteString("input.")

		if t.IsBaseType() {
			switch t.(*sema.BaseType).Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if t.IsBinary() {
					out.WriteString("readBinary()")
				} else {
					out.WriteString("readString()")
				}
			case sema.TypeUUID:
				out.WriteString("readUuid()")
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
			default:
				emit.Throw("compiler error: no JS name for base type %s", sema.BaseName(t.(*sema.BaseType).Base()))
			}
		} else if t.IsEnum() {
			out.WriteString("readI32()")
		}

		if wrapI64Bigint {
			out.WriteString(")")
		}

		if !g.opts.Node {
			out.WriteString(".value")
		}

		out.WriteString(";" + "\n")
	default:
		// DO NOT KNOW HOW TO DESERIALIZE FIELD - unreachable for any
		// resolved type; matches the C++ generator's printf fallback,
		// which writes nothing to the output.
	}
}

// generateDeserializeStruct is generate_deserialize_struct: this makes two
// key assumptions, first that there is a const char* variable named data
// that points to the buffer for deserialization, and that there is a
// variable protocol which is a reference to a TProtocol serialization
// object.
func (g *Generator) generateDeserializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	out.WriteString(g.indent() + prefix + " = new " + g.jsTypeNamespace(s.Program()) + s.Name() + "();" + "\n" +
		g.indent() + prefix + "[Symbol.for(\"read\")](input);" + "\n")
}

// generateDeserializeContainer is generate_deserialize_container.
func (g *Generator) generateDeserializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	size := g.tmp("_size")
	rtmp3 := g.tmp("_rtmp3")
	seen := "" // populated only for sets; gives O(1) per-element dedup

	switch {
	case t.IsMap():
		out.WriteString(g.indent() + prefix + " = {};" + "\n")
		out.WriteString(g.indent() + g.jsConstType + rtmp3 + " = input.readMapBegin();" + "\n")
		out.WriteString(g.indent() + g.jsConstType + size + " = " + rtmp3 + ".size || 0;" + "\n")
	case t.IsSet():
		seen = g.tmp("_seen")
		out.WriteString(g.indent() + prefix + " = [];" + "\n" +
			g.indent() + g.jsConstType + seen + " = new Set();" + "\n" +
			g.indent() + g.jsConstType + rtmp3 + " = input.readSetBegin();" + "\n" +
			g.indent() + g.jsConstType + size + " = " + rtmp3 + ".size || 0;" + "\n")
	case t.IsList():
		out.WriteString(g.indent() + prefix + " = [];" + "\n" +
			g.indent() + g.jsConstType + rtmp3 + " = input.readListBegin();" + "\n" +
			g.indent() + g.jsConstType + size + " = " + rtmp3 + ".size || 0;" + "\n")
	}

	// For loop iterates over elements
	i := g.tmp("_i")
	out.WriteString(g.indent() + "for (" + g.jsLetType + i + " = 0; " + i + " < " + size + "; ++" + i + ") {" + "\n")
	g.indentUp()

	switch {
	case t.IsMap():
		if !g.opts.Node {
			out.WriteString(g.indent() + "if (" + i + " > 0 ) {" + "\n" +
				g.indent() + "  if (input.rstack.length > input.rpos[input.rpos.length -1] + 1) {" + "\n" +
				g.indent() + "    input.rstack.pop();" + "\n" +
				g.indent() + "  }" + "\n" +
				g.indent() + "}" + "\n")
		}
		g.generateDeserializeMapElement(out, t.(*sema.Map), prefix)
	case t.IsSet():
		g.generateDeserializeSetElement(out, t.(*sema.Set), prefix, seen)
	case t.IsList():
		g.generateDeserializeListElement(out, t.(*sema.List), prefix)
	}

	g.scopeDown(out)

	// Read container end
	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "input.readMapEnd();" + "\n")
	case t.IsSet():
		out.WriteString(g.indent() + "input.readSetEnd();" + "\n")
	case t.IsList():
		out.WriteString(g.indent() + "input.readListEnd();" + "\n")
	}
}

// generateDeserializeMapElement is generate_deserialize_map_element: code
// to deserialize a map.
func (g *Generator) generateDeserializeMapElement(out *strings.Builder, m *sema.Map, prefix string) {
	key := g.tmp("key")
	val := g.tmp("val")
	fkey := sema.NewField(m.KeyType(), key, 0)
	fval := sema.NewField(m.ValType(), val, 0)

	out.WriteString(g.indent() + g.declareField(fkey, false, false) + ";" + "\n")
	out.WriteString(g.indent() + g.declareField(fval, false, false) + ";" + "\n")

	g.generateDeserializeField(out, fkey, "")
	g.generateDeserializeField(out, fval, "")

	// A map key comes off the wire, and the map is a plain object, on
	// which "__proto__" is not an ordinary member name: a plain
	// assignment under it runs the setter inherited from
	// Object.prototype, which swaps the object's prototype when the
	// value is an object and silently drops the entry when it is not.
	// Neither stores the pair. defineProperty always does.
	out.WriteString(g.indent() + "if (" + key + " === \"__proto__\") {" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "Object.defineProperty(" + prefix + ", " + key + ", {" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + "value: " + val + ", writable: true, enumerable: true, configurable: true" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "});" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "} else {" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + prefix + "[" + key + "] = " + val + ";" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "}" + "\n")
}

// generateDeserializeSetElement is generate_deserialize_set_element.
func (g *Generator) generateDeserializeSetElement(out *strings.Builder, s *sema.Set, prefix, seen string) {
	elem := g.tmp("elem")
	felem := sema.NewField(s.ElemType(), elem, 0)

	out.WriteString(g.indent() + g.jsLetType + elem + " = null;" + "\n")

	g.generateDeserializeField(out, felem, "")

	// O(1) dedup against a parallel Set, allocated once in the container
	// header. SameValueZero on Set matches indexOf-with-=== for
	// primitives and is equivalent (reference equality) for complex
	// element types.
	out.WriteString(g.indent() + "if (!" + seen + ".has(" + elem + ")) {" + "\n")
	g.indentUp()
	out.WriteString(g.indent() + seen + ".add(" + elem + ");" + "\n")
	out.WriteString(g.indent() + prefix + ".push(" + elem + ");" + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "}" + "\n")
}

// generateDeserializeListElement is generate_deserialize_list_element.
func (g *Generator) generateDeserializeListElement(out *strings.Builder, l *sema.List, prefix string) {
	elem := g.tmp("elem")
	felem := sema.NewField(l.ElemType(), elem, 0)

	out.WriteString(g.indent() + g.jsLetType + elem + " = null;" + "\n")

	g.generateDeserializeField(out, felem, "")

	out.WriteString(g.indent() + prefix + ".push(" + elem + ");" + "\n")
}

// generateSerializeField is generate_serialize_field: serializes a field
// of any type.
func (g *Generator) generateSerializeField(out *strings.Builder, f *sema.Field, prefix string) {
	t := sema.TrueType(f.Type())

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s%s", prefix, f.Name())
	}

	switch {
	case t.IsStruct(), t.IsXception():
		g.generateSerializeStruct(out, t.(*sema.Struct), prefix+f.Name())
	case t.IsContainer():
		g.generateSerializeContainer(out, t, prefix+f.Name())
	case t.IsBaseType(), t.IsEnum():
		name := f.Name()
		// Hack for when prefix is defined (always a hash ref)
		if prefix != "" {
			name = prefix + f.Name()
		}

		out.WriteString(g.indent() + "output.")

		if t.IsBaseType() {
			switch t.(*sema.BaseType).Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if t.IsBinary() {
					out.WriteString("writeBinary(" + name + ")")
				} else {
					out.WriteString("writeString(" + name + ")")
				}
			case sema.TypeUUID:
				out.WriteString("writeUuid(" + name + ")")
			case sema.TypeBool:
				out.WriteString("writeBool(" + name + ")")
			case sema.TypeI8:
				out.WriteString("writeByte(" + name + ")")
			case sema.TypeI16:
				out.WriteString("writeI16(" + name + ")")
			case sema.TypeI32:
				out.WriteString("writeI32(" + name + ")")
			case sema.TypeI64:
				// In bigint mode the generated field holds a `bigint`;
				// convert back to a node-int64 Int64 before handing to
				// `writeI64`, which expects either an Int64 or a Number.
				if g.opts.Bigint {
					out.WriteString("writeI64(thrift.fromBigInt(" + name + "))")
				} else {
					out.WriteString("writeI64(" + name + ")")
				}
			case sema.TypeDouble:
				out.WriteString("writeDouble(" + name + ")")
			default:
				emit.Throw("compiler error: no JS name for base type %s", sema.BaseName(t.(*sema.BaseType).Base()))
			}
		} else if t.IsEnum() {
			out.WriteString("writeI32(" + name + ")")
		}
		out.WriteString(";" + "\n")
	default:
		// DO NOT KNOW HOW TO SERIALIZE FIELD - unreachable for any
		// resolved type; matches the C++ generator's printf fallback.
	}
}

// generateSerializeStruct is generate_serialize_struct: serializes all the
// members of a struct.
func (g *Generator) generateSerializeStruct(out *strings.Builder, _ *sema.Struct, prefix string) {
	out.WriteString(g.indent() + prefix + "[Symbol.for(\"write\")](output);" + "\n")
}

// generateSerializeContainer is generate_serialize_container.
func (g *Generator) generateSerializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		out.WriteString(g.indent() + "output.writeMapBegin(" + typeToEnum(m.KeyType()) + ", " + typeToEnum(m.ValType()) + ", " +
			"Thrift.objectLength(" + prefix + "));" + "\n")
	case t.IsSet():
		out.WriteString(g.indent() + "Thrift.checkSetUniqueness(" + prefix + ");" + "\n")
		out.WriteString(g.indent() + "output.writeSetBegin(" + typeToEnum(t.(*sema.Set).ElemType()) + ", " + prefix + ".length);" + "\n")
	case t.IsList():
		out.WriteString(g.indent() + "output.writeListBegin(" + typeToEnum(t.(*sema.List).ElemType()) + ", " + prefix + ".length);" + "\n")
	}

	switch {
	case t.IsMap():
		kiter := g.tmp("kiter")
		viter := g.tmp("viter")
		out.WriteString(g.indent() + "for (" + g.jsLetType + kiter + " in " + prefix + ") {" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (" + prefix + ".hasOwnProperty(" + kiter + ")) {" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + g.jsLetType + viter + " = " + prefix + "[" + kiter + "];" + "\n")
		g.generateSerializeMapElement(out, t.(*sema.Map), kiter, viter)
		g.scopeDown(out)
		g.scopeDown(out)
	case t.IsSet():
		iter := g.tmp("iter")
		out.WriteString(g.indent() + "for (" + g.jsLetType + iter + " in " + prefix + ") {" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (" + prefix + ".hasOwnProperty(" + iter + ")) {" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + iter + " = " + prefix + "[" + iter + "];" + "\n")
		g.generateSerializeSetElement(out, t.(*sema.Set), iter)
		g.scopeDown(out)
		g.scopeDown(out)
	case t.IsList():
		iter := g.tmp("iter")
		out.WriteString(g.indent() + "for (" + g.jsLetType + iter + " in " + prefix + ") {" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (" + prefix + ".hasOwnProperty(" + iter + ")) {" + "\n")
		g.indentUp()
		out.WriteString(g.indent() + iter + " = " + prefix + "[" + iter + "];" + "\n")
		g.generateSerializeListElement(out, t.(*sema.List), iter)
		g.scopeDown(out)
		g.scopeDown(out)
	}

	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "output.writeMapEnd();" + "\n")
	case t.IsSet():
		out.WriteString(g.indent() + "output.writeSetEnd();" + "\n")
	case t.IsList():
		out.WriteString(g.indent() + "output.writeListEnd();" + "\n")
	}
}

// generateSerializeMapElement is generate_serialize_map_element.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, m *sema.Map, kiter, viter string) {
	kfield := sema.NewField(m.KeyType(), kiter, 0)
	g.generateSerializeField(out, kfield, "")

	vfield := sema.NewField(m.ValType(), viter, 0)
	g.generateSerializeField(out, vfield, "")
}

// generateSerializeSetElement is generate_serialize_set_element.
func (g *Generator) generateSerializeSetElement(out *strings.Builder, s *sema.Set, iter string) {
	efield := sema.NewField(s.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}

// generateSerializeListElement is generate_serialize_list_element.
func (g *Generator) generateSerializeListElement(out *strings.Builder, l *sema.List, iter string) {
	efield := sema.NewField(l.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}
