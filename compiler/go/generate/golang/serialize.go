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

package golang

import (
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// tempField makes the unnamed or temporary t_field the C++ code builds on
// the stack for container elements.
func tempField(typ sema.Type, name string) *sema.Field {
	f := sema.NewField(typ, name, 0)
	f.SetReq(sema.OptInReqOut)
	return f
}

func (g *Generator) generateDeserializeField(out *strings.Builder, f *sema.Field, declare bool, prefix string, inkey bool, inContainerValue bool) {
	origType := f.Type()
	typ := sema.TrueType(origType)
	name := prefix + g.publicize(f.Name())
	if typ.IsVoid() {
		throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s", name)
	}
	switch {
	case typ.IsStruct() || typ.IsXception():
		g.generateDeserializeStruct(out, typ.(*sema.Struct), isPointerField(f), declare, name)
	case typ.IsContainer():
		g.generateDeserializeContainer(out, origType, isPointerField(f), declare, name)
	case typ.IsBaseType() || typ.IsEnum():
		if declare {
			var typeName string
			if inkey {
				typeName = g.typeToGoKeyType(f.Type())
			} else {
				typeName = g.typeToGoType(f.Type())
			}
			out.WriteString("var " + f.Name() + " " + typeName + "\n")
		}
		out.WriteString("if v, err := iprot.")
		if typ.IsBaseType() {
			switch typ.(*sema.BaseType).Base() {
			case sema.TypeVoid:
				throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if typ.IsBinary() && !inkey {
					out.WriteString("ReadBinary(ctx)")
				} else {
					out.WriteString("ReadString(ctx)")
				}
			case sema.TypeBool:
				out.WriteString("ReadBool(ctx)")
			case sema.TypeI8:
				out.WriteString("ReadByte(ctx)")
			case sema.TypeI16:
				out.WriteString("ReadI16(ctx)")
			case sema.TypeI32:
				out.WriteString("ReadI32(ctx)")
			case sema.TypeI64:
				out.WriteString("ReadI64(ctx)")
			case sema.TypeDouble:
				out.WriteString("ReadDouble(ctx)")
			case sema.TypeUUID:
				out.WriteString("ReadUUID(ctx)")
			default:
				throw("compiler error: no Go name for base type %s", sema.BaseName(typ.(*sema.BaseType).Base()))
			}
		} else {
			out.WriteString("ReadI32(ctx)")
		}
		out.WriteString("; err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error reading field " + itoa(int64(f.Key())) + ": \", err)\n")
		out.WriteString("} else {\n")
		wrap := ""
		if typ.IsEnum() || origType.IsTypedef() {
			wrap = g.publicize(g.typeName(origType))
		} else if typ.(*sema.BaseType).Base() == sema.TypeI8 {
			wrap = "int8"
		}
		maybeAddress := ""
		if isPointerField(f) {
			maybeAddress = "&"
		}
		if wrap == "" {
			out.WriteString(name + " = " + maybeAddress + "v\n")
		} else {
			out.WriteString("temp := " + wrap + "(v)\n")
			out.WriteString(name + " = " + maybeAddress + "temp\n")
		}
		out.WriteString("}\n")
	default:
		throw("INVALID TYPE IN generate_deserialize_field '%s' for field '%s'", typ.Name(), f.Name())
	}
}

func (g *Generator) generateDeserializeStruct(out *strings.Builder, s *sema.Struct, pointerField, declare bool, prefix string) {
	eq := " = "
	if declare {
		eq = " := "
	}
	amp := ""
	if pointerField {
		amp = "&"
	}
	out.WriteString(prefix + eq + amp)
	g.generateGoStructInitializer(out, s, false)
	out.WriteString("if err := " + prefix + "." + g.readMethodName + "(ctx, iprot); err != nil {\n")
	out.WriteString("return thrift.PrependError(fmt.Sprintf(\"%T error reading struct: \", " + prefix + "), err)\n")
	out.WriteString("}\n")
}

func (g *Generator) generateDeserializeContainer(out *strings.Builder, origType sema.Type, pointerField, declare bool, prefix string) {
	ttype := sema.TrueType(origType)
	eq := " = "
	if declare {
		eq = " := "
	}
	amp := ""
	if pointerField {
		amp = "&"
	}
	switch {
	case ttype.IsMap():
		out.WriteString("_, _, size, err := iprot.ReadMapBegin(ctx)\n")
		out.WriteString("if err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error reading map begin: \", err)\n")
		out.WriteString("}\n")
		if g.isContainerKeyedMap(ttype) {
			out.WriteString("tMap := make(" + g.typeToGoType(origType) + ", 0, thrift.PreallocSize(size))\n")
		} else {
			out.WriteString("tMap := make(" + g.typeToGoType(origType) + ", thrift.PreallocSize(size))\n")
		}
		out.WriteString(prefix + eq + amp + "tMap\n")
	case ttype.IsSet():
		out.WriteString("_, size, err := iprot.ReadSetBegin(ctx)\n")
		out.WriteString("if err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error reading set begin: \", err)\n")
		out.WriteString("}\n")
		out.WriteString("tSet := make(" + g.typeToGoType(origType) + ", 0, thrift.PreallocSize(size))\n")
		out.WriteString(prefix + eq + amp + "tSet\n")
	case ttype.IsList():
		out.WriteString("_, size, err := iprot.ReadListBegin(ctx)\n")
		out.WriteString("if err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error reading list begin: \", err)\n")
		out.WriteString("}\n")
		out.WriteString("tSlice := make(" + g.typeToGoType(origType) + ", 0, thrift.PreallocSize(size))\n")
		out.WriteString(prefix + eq + amp + "tSlice\n")
	default:
		throw("INVALID TYPE IN generate_deserialize_container '%s' for prefix '%s'", ttype.Name(), prefix)
	}
	out.WriteString("for i := 0; i < size; i++ {\n")
	if pointerField {
		prefix = "(*" + prefix + ")"
	}
	switch {
	case ttype.IsMap():
		g.generateDeserializeMapElement(out, ttype.(*sema.Map), prefix)
	case ttype.IsSet():
		g.generateDeserializeSetElement(out, ttype.(*sema.Set), prefix)
	case ttype.IsList():
		g.generateDeserializeListElement(out, ttype.(*sema.List), prefix)
	}
	out.WriteString("}\n")
	switch {
	case ttype.IsMap():
		out.WriteString("if err := iprot.ReadMapEnd(ctx); err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error reading map end: \", err)\n")
		out.WriteString("}\n")
	case ttype.IsSet():
		out.WriteString("if err := iprot.ReadSetEnd(ctx); err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error reading set end: \", err)\n")
		out.WriteString("}\n")
	case ttype.IsList():
		out.WriteString("if err := iprot.ReadListEnd(ctx); err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error reading list end: \", err)\n")
		out.WriteString("}\n")
	}
}

func (g *Generator) generateDeserializeMapElement(out *strings.Builder, m *sema.Map, prefix string) {
	key := g.tmp("_key")
	val := g.tmp("_val")
	fkey := tempField(m.KeyType(), key)
	fval := tempField(m.ValType(), val)
	if g.isContainerKeyedMap(m) {
		out.WriteString("var " + key + " " + g.mapEntryKeyType(m.KeyType()) + "\n")
		out.WriteString("var " + val + " " + g.typeToGoType(m.ValType()) + "\n")
		out.WriteString("{\n")
		g.generateDeserializeField(out, fkey, false, "", true, false)
		out.WriteString("}\n")
		out.WriteString("{\n")
		g.generateDeserializeField(out, fval, false, "", false, true)
		out.WriteString("}\n")
		out.WriteString(prefix + " = append(" + prefix + ", " + g.mapEntryType(m) + "{Key: " + key + ", Value: " + val + "})\n")
		return
	}
	g.generateDeserializeField(out, fkey, true, "", true, false)
	g.generateDeserializeField(out, fval, true, "", false, true)
	out.WriteString(prefix + "[" + key + "] = " + val + "\n")
}

func (g *Generator) generateDeserializeSetElement(out *strings.Builder, s *sema.Set, prefix string) {
	elem := g.tmp("_elem")
	felem := tempField(s.ElemType(), elem)
	g.generateDeserializeField(out, felem, true, "", false, true)
	out.WriteString(prefix + " = append(" + prefix + ", " + elem + ")\n")
}

func (g *Generator) generateDeserializeListElement(out *strings.Builder, l *sema.List, prefix string) {
	elem := g.tmp("_elem")
	felem := tempField(l.ElemType(), elem)
	g.generateDeserializeField(out, felem, true, "", false, true)
	out.WriteString(prefix + " = append(" + prefix + ", " + elem + ")\n")
}

func (g *Generator) generateSerializeField(out *strings.Builder, f *sema.Field, prefix string, inkey bool) {
	typ := sema.TrueType(f.Type())
	name := prefix + g.publicize(f.Name())
	if typ.IsVoid() {
		throw("compiler error: cannot generate serialize for void type: %s", name)
	}
	switch {
	case typ.IsStruct() || typ.IsXception():
		g.generateSerializeStruct(out, name)
	case typ.IsContainer():
		g.generateSerializeContainer(out, typ, isPointerField(f), name)
	case typ.IsBaseType() || typ.IsEnum():
		out.WriteString("if err := oprot.")
		if isPointerField(f) {
			name = "*" + name
		}
		if typ.IsBaseType() {
			switch typ.(*sema.BaseType).Base() {
			case sema.TypeVoid:
				throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if typ.IsBinary() && !inkey {
					out.WriteString("WriteBinary(ctx, " + name + ")")
				} else {
					out.WriteString("WriteString(ctx, string(" + name + "))")
				}
			case sema.TypeBool:
				out.WriteString("WriteBool(ctx, bool(" + name + "))")
			case sema.TypeI8:
				out.WriteString("WriteByte(ctx, int8(" + name + "))")
			case sema.TypeI16:
				out.WriteString("WriteI16(ctx, int16(" + name + "))")
			case sema.TypeI32:
				out.WriteString("WriteI32(ctx, int32(" + name + "))")
			case sema.TypeI64:
				out.WriteString("WriteI64(ctx, int64(" + name + "))")
			case sema.TypeDouble:
				out.WriteString("WriteDouble(ctx, float64(" + name + "))")
			case sema.TypeUUID:
				out.WriteString("WriteUUID(ctx, thrift.Tuuid(" + name + "))")
			default:
				throw("compiler error: no Go name for base type %s", sema.BaseName(typ.(*sema.BaseType).Base()))
			}
		} else {
			out.WriteString("WriteI32(ctx, int32(" + name + "))")
		}
		out.WriteString("; err != nil {\n")
		out.WriteString("return thrift.PrependError(fmt.Sprintf(\"%T." + escapeString(f.Name()) +
			" (" + itoa(int64(f.Key())) + ") field write error: \", p), err)\n")
		out.WriteString("}\n")
	default:
		throw("compiler error: Invalid type in generate_serialize_field '%s' for field '%s'", typ.Name(), name)
	}
}

func (g *Generator) generateSerializeStruct(out *strings.Builder, prefix string) {
	out.WriteString("if err := " + prefix + "." + g.writeMethodName + "(ctx, oprot); err != nil {\n")
	out.WriteString("return thrift.PrependError(fmt.Sprintf(\"%T error writing struct: \", " + prefix + "), err)\n")
	out.WriteString("}\n")
}

func (g *Generator) generateSerializeContainer(out *strings.Builder, ttype sema.Type, pointerField bool, prefix string) {
	if pointerField {
		prefix = "*" + prefix
	}
	switch {
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		out.WriteString("if err := oprot.WriteMapBegin(ctx, " + g.typeToEnum(m.KeyType()) + ", " +
			g.typeToEnum(m.ValType()) + ", " + "len(" + prefix + ")); err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error writing map begin: \", err)\n")
		out.WriteString("}\n")
	case ttype.IsSet():
		out.WriteString("if err := oprot.WriteSetBegin(ctx, " + g.typeToEnum(ttype.(*sema.Set).ElemType()) + ", " +
			"len(" + prefix + ")); err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error writing set begin: \", err)\n")
		out.WriteString("}\n")
	case ttype.IsList():
		out.WriteString("if err := oprot.WriteListBegin(ctx, " + g.typeToEnum(ttype.(*sema.List).ElemType()) + ", " +
			"len(" + prefix + ")); err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error writing list begin: \", err)\n")
		out.WriteString("}\n")
	default:
		throw("compiler error: Invalid type in generate_serialize_container '%s' for prefix '%s'", ttype.Name(), prefix)
	}
	switch {
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		if g.isContainerKeyedMap(m) {
			wrappedPrefix := prefix
			if pointerField {
				wrappedPrefix = "(" + prefix + ")"
			}
			notUnique := "return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, " +
				"fmt.Errorf(\"%T error writing map field %q: keys are not unique\", " +
				wrappedPrefix + ", \"" + escapeString(prefix) + "\"))"
			if g.isComparableStructKey(m.KeyType()) {
				seen := g.tmp("seen")
				sawNil := g.tmp("sawNil")
				entry := g.tmp("e")
				keyValueType := g.publicize(g.typeName(sema.TrueType(m.KeyType())))
				out.WriteString("if len(" + wrappedPrefix + ") > 1 {\n")
				out.WriteString(seen + " := make(map[" + keyValueType + "]struct{}, len(" + wrappedPrefix + "))\n")
				out.WriteString(sawNil + " := false\n")
				// gofmt strips parentheses around a range expression, so a
				// pointer field ranges over *p.Field rather than (*p.Field).
				out.WriteString("for _, " + entry + " := range " + prefix + " {\n")
				out.WriteString("if " + entry + ".Key == nil {\n")
				out.WriteString("if " + sawNil + " {\n")
				out.WriteString(notUnique + "\n")
				out.WriteString("}\n")
				out.WriteString(sawNil + " = true\n")
				out.WriteString("continue\n")
				out.WriteString("}\n")
				out.WriteString("if _, ok := " + seen + "[*" + entry + ".Key]; ok {\n")
				out.WriteString(notUnique + "\n")
				out.WriteString("}\n")
				out.WriteString(seen + "[*" + entry + ".Key] = struct{}{}\n")
				out.WriteString("}\n")
				out.WriteString("}\n")
			} else {
				keyType := g.mapEntryKeyType(m.KeyType())
				out.WriteString("for i := 0; i < len(" + prefix + "); i++ {\n")
				out.WriteString("for j := i + 1; j < len(" + prefix + "); j++ {\n")
				out.WriteString("if func(tgt, src " + keyType + ") bool {\n")
				g.generateGoEquals(out, m.KeyType(), "tgt", "src")
				out.WriteString("return true\n")
				out.WriteString("}(" + wrappedPrefix + "[i].Key, " + wrappedPrefix + "[j].Key) {\n")
				out.WriteString(notUnique + "\n")
				out.WriteString("}\n")
				out.WriteString("}\n")
				out.WriteString("}\n")
			}
			out.WriteString("for _, e := range " + prefix + " {\n")
			g.generateSerializeMapElement(out, m, "e.Key", "e.Value")
		} else {
			out.WriteString("for k, v := range " + prefix + " {\n")
			g.generateSerializeMapElement(out, m, "k", "v")
		}
		out.WriteString("}\n")
	case ttype.IsSet():
		s := ttype.(*sema.Set)
		out.WriteString("for i := 0; i < len(" + prefix + "); i++ {\n")
		out.WriteString("for j := i + 1; j < len(" + prefix + "); j++ {\n")
		wrappedPrefix := prefix
		if pointerField {
			wrappedPrefix = "(" + prefix + ")"
		}
		goType := g.typeToGoType(s.ElemType())
		out.WriteString("if func(tgt, src " + goType + ") bool {\n")
		g.generateGoEquals(out, s.ElemType(), "tgt", "src")
		out.WriteString("return true\n")
		out.WriteString("}(" + wrappedPrefix + "[i], " + wrappedPrefix + "[j]) {\n")
		out.WriteString("return thrift.NewTProtocolExceptionWithType(thrift.INVALID_DATA, " +
			"fmt.Errorf(\"%T error writing set field %q: slice is not unique\", " +
			wrappedPrefix + ", \"" + escapeString(prefix) + "\"))\n")
		out.WriteString("}\n")
		out.WriteString("}\n")
		out.WriteString("}\n")
		out.WriteString("for _, v := range " + prefix + " {\n")
		g.generateSerializeSetElement(out, s, "v")
		out.WriteString("}\n")
	case ttype.IsList():
		l := ttype.(*sema.List)
		out.WriteString("for _, v := range " + prefix + " {\n")
		g.generateSerializeListElement(out, l, "v")
		out.WriteString("}\n")
	}
	switch {
	case ttype.IsMap():
		out.WriteString("if err := oprot.WriteMapEnd(ctx); err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error writing map end: \", err)\n")
		out.WriteString("}\n")
	case ttype.IsSet():
		out.WriteString("if err := oprot.WriteSetEnd(ctx); err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error writing set end: \", err)\n")
		out.WriteString("}\n")
	case ttype.IsList():
		out.WriteString("if err := oprot.WriteListEnd(ctx); err != nil {\n")
		out.WriteString("return thrift.PrependError(\"error writing list end: \", err)\n")
		out.WriteString("}\n")
	}
}

func (g *Generator) generateSerializeMapElement(out *strings.Builder, m *sema.Map, kiter, viter string) {
	g.generateSerializeField(out, tempField(m.KeyType(), ""), kiter, true)
	g.generateSerializeField(out, tempField(m.ValType(), ""), viter, false)
}

func (g *Generator) generateSerializeSetElement(out *strings.Builder, s *sema.Set, prefix string) {
	g.generateSerializeField(out, tempField(s.ElemType(), ""), prefix, false)
}

func (g *Generator) generateSerializeListElement(out *strings.Builder, l *sema.List, prefix string) {
	g.generateSerializeField(out, tempField(l.ElemType(), ""), prefix, false)
}

func (g *Generator) generateGoEquals(out *strings.Builder, oriType sema.Type, tgt, src string) {
	ttype := sema.TrueType(oriType)
	if ttype.IsVoid() {
		throw("compiler error: cannot generate equals for void type: %s", tgt)
	}
	switch {
	case ttype.IsStruct() || ttype.IsXception():
		g.generateGoEqualsStruct(out, tgt, src)
	case ttype.IsContainer():
		g.generateGoEqualsContainer(out, ttype, tgt, src)
	case ttype.IsBaseType() || ttype.IsEnum():
		out.WriteString("if ")
		if ttype.IsBaseType() {
			switch ttype.(*sema.BaseType).Base() {
			case sema.TypeVoid:
				throw("compiler error: cannot equals void: %s", tgt)
			case sema.TypeString:
				if ttype.IsBinary() {
					out.WriteString("bytes.Compare(" + tgt + ", " + src + ") != 0")
				} else {
					out.WriteString(tgt + " != " + src)
				}
			case sema.TypeBool, sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64, sema.TypeDouble, sema.TypeUUID:
				out.WriteString(tgt + " != " + src)
			default:
				throw("compiler error: no Go name for base type %s", sema.BaseName(ttype.(*sema.BaseType).Base()))
			}
		} else {
			out.WriteString(tgt + " != " + src)
		}
		out.WriteString(" {\n")
		out.WriteString("return false\n")
		out.WriteString("}\n")
	default:
		throw("compiler error: Invalid type in generate_go_equals '%s' for '%s'", ttype.Name(), tgt)
	}
}

func (g *Generator) generateGoEqualsStruct(out *strings.Builder, tgt, src string) {
	out.WriteString("if !" + tgt + "." + g.equalsMethodName + "(" + src + ") {\n")
	out.WriteString("return false\n")
	out.WriteString("}\n")
}

func (g *Generator) generateGoEqualsUnordered(out *strings.Builder, goType, tgt, src string) {
	out.WriteString("if !thrift.UnorderedEqual(" + indexableGoExpr(tgt) + ", " +
		indexableGoExpr(src) + ", func(_tgt, _src " + goType + ") bool {\n")
}

func (g *Generator) generateGoEqualsContainer(out *strings.Builder, ttype sema.Type, tgt, src string) {
	out.WriteString("if len(" + tgt + ") != len(" + src + ") {\n")
	out.WriteString("return false\n")
	out.WriteString("}\n")
	switch {
	case g.isContainerKeyedMap(ttype):
		m := ttype.(*sema.Map)
		g.generateGoEqualsUnordered(out, g.mapEntryType(m), tgt, src)
		g.generateGoEquals(out, m.KeyType(), "_tgt.Key", "_src.Key")
		g.generateGoEquals(out, m.ValType(), "_tgt.Value", "_src.Value")
		out.WriteString("return true\n")
		out.WriteString("}) {\n")
		out.WriteString("return false\n")
		out.WriteString("}\n")
	case ttype.IsMap():
		m := ttype.(*sema.Map)
		out.WriteString("for k, _tgt := range " + tgt + " {\n")
		elementSource := g.tmp("_src")
		out.WriteString(elementSource + ", ok := " + indexableGoExpr(src) + "[k]\n")
		out.WriteString("if !ok {\n")
		out.WriteString("return false\n")
		out.WriteString("}\n")
		g.generateGoEquals(out, m.ValType(), "_tgt", elementSource)
		out.WriteString("}\n")
	case ttype.IsSet():
		elem := ttype.(*sema.Set).ElemType()
		g.generateGoEqualsUnordered(out, g.typeToGoType(elem), tgt, src)
		g.generateGoEquals(out, elem, "_tgt", "_src")
		out.WriteString("return true\n")
		out.WriteString("}) {\n")
		out.WriteString("return false\n")
		out.WriteString("}\n")
	case ttype.IsList():
		elem := ttype.(*sema.List).ElemType()
		out.WriteString("for i, _tgt := range " + tgt + " {\n")
		elementSource := g.tmp("_src")
		out.WriteString(elementSource + " := " + indexableGoExpr(src) + "[i]\n")
		g.generateGoEquals(out, elem, "_tgt", elementSource)
		out.WriteString("}\n")
	default:
		throw("INVALID TYPE IN generate_go_equals_container '%s", ttype.Name())
	}
}
