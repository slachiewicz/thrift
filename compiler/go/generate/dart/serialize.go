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

package dart

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateDeserializeField is t_dart_generator::generate_deserialize_field.
func (g *Generator) generateDeserializeField(out *strings.Builder, field *sema.Field, prefix string) {
	t := sema.TrueType(field.Type())
	fieldName := getMemberName(field.Name())

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, fieldName)
	}

	name := prefix + fieldName

	switch {
	case t.IsStruct(), t.IsXception():
		g.generateDeserializeStruct(out, t.(*sema.Struct), name)
	case t.IsContainer():
		g.generateDeserializeContainer(out, t, name)
	case t.IsBaseType(), t.IsEnum():
		out.WriteString(g.ind() + name + " = iprot.")

		if t.IsBaseType() {
			b := t.(*sema.BaseType)
			switch b.Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				if b.IsBinary() {
					out.WriteString("readBinary();")
				} else {
					out.WriteString("readString();")
				}
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
				emit.Throw("compiler error: no Dart name for base type %s", sema.BaseName(b.Base()))
			}
		} else if t.IsEnum() {
			out.WriteString("readI32();")
		}
		out.WriteString("\n")
	default:
		// DO NOT KNOW HOW TO DESERIALIZE FIELD: the C++ generator only
		// printf's a diagnostic here and writes nothing to the file, so
		// there is nothing to reproduce in the output.
	}
}

// generateDeserializeStruct is t_dart_generator::generate_deserialize_struct:
// generates an unserializer for a struct, invokes read().
func (g *Generator) generateDeserializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	out.WriteString(g.ind() + prefix + " = new " + g.typeName(s) + "();\n")
	out.WriteString(g.ind() + prefix + "!.read(iprot);\n")
}

// generateDeserializeContainer is
// t_dart_generator::generate_deserialize_container: deserializes a
// container by reading its size and then iterating.
func (g *Generator) generateDeserializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	out.WriteString(g.ind())
	g.scopeUpPrefix(out, "")

	var obj string
	switch {
	case t.IsMap():
		obj = g.tmp("_map")
	case t.IsSet():
		obj = g.tmp("_set")
	case t.IsList():
		obj = g.tmp("_list")
	}

	// Declare variables, read header
	switch {
	case t.IsMap():
		out.WriteString(g.ind() + "TMap " + obj + " = iprot.readMapBegin();\n")
	case t.IsSet():
		out.WriteString(g.ind() + "TSet " + obj + " = iprot.readSetBegin();\n")
	case t.IsList():
		out.WriteString(g.ind() + "TList " + obj + " = iprot.readListBegin();\n")
	}

	out.WriteString(g.ind() + prefix + " = " + g.typeNameInstantiate(t) + ";\n")

	// For loop iterates over elements
	i := g.tmp("_i")
	out.WriteString(g.ind() + "for (int " + i + " = 0; " + i + " < " + obj + ".length; ++" + i + ")")
	g.scopeUp(out)

	switch {
	case t.IsMap():
		g.generateDeserializeMapElement(out, t.(*sema.Map), prefix)
	case t.IsSet():
		g.generateDeserializeSetElement(out, t.(*sema.Set), prefix)
	case t.IsList():
		g.generateDeserializeListElement(out, t.(*sema.List), prefix)
	}

	g.scopeDown(out)

	// Read container end
	switch {
	case t.IsMap():
		out.WriteString(g.ind() + "iprot.readMapEnd();\n")
	case t.IsSet():
		out.WriteString(g.ind() + "iprot.readSetEnd();\n")
	case t.IsList():
		out.WriteString(g.ind() + "iprot.readListEnd();\n")
	}

	g.scopeDown(out)
}

// generateDeserializeMapElement is
// t_dart_generator::generate_deserialize_map_element.
func (g *Generator) generateDeserializeMapElement(out *strings.Builder, m *sema.Map, prefix string) {
	key := g.tmp("_key")
	val := g.tmp("_val")
	fkey := sema.NewField(m.KeyType(), key, 0)
	fval := sema.NewField(m.ValType(), val, 0)

	out.WriteString(g.ind() + g.declareField(fkey, false) + "\n")
	out.WriteString(g.ind() + g.declareField(fval, false) + "\n")

	g.generateDeserializeField(out, fkey, "")
	g.generateDeserializeField(out, fval, "")

	out.WriteString(g.ind() + prefix + "![" + key + "] = " + val + ";\n")
}

// generateDeserializeSetElement is
// t_dart_generator::generate_deserialize_set_element.
func (g *Generator) generateDeserializeSetElement(out *strings.Builder, s *sema.Set, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(s.ElemType(), elem, 0)

	out.WriteString(g.ind() + g.declareField(felem, false) + "\n")

	g.generateDeserializeField(out, felem, "")

	out.WriteString(g.ind() + prefix + "!.add(" + elem + ");\n")
}

// generateDeserializeListElement is
// t_dart_generator::generate_deserialize_list_element.
func (g *Generator) generateDeserializeListElement(out *strings.Builder, l *sema.List, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(l.ElemType(), elem, 0)

	out.WriteString(g.ind() + g.declareField(felem, false) + "\n")

	g.generateDeserializeField(out, felem, "")

	out.WriteString(g.ind() + prefix + "!.add(" + elem + ");\n")
}

// generateSerializeField is t_dart_generator::generate_serialize_field.
func (g *Generator) generateSerializeField(out *strings.Builder, field *sema.Field, prefix string) {
	t := sema.TrueType(field.Type())
	fieldName := getMemberName(field.Name())
	nullAllowed := typeCanBeNull(t)

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s%s", prefix, fieldName)
	}

	switch {
	case t.IsStruct(), t.IsXception():
		g.generateSerializeStruct(out, t.(*sema.Struct), prefix+fieldName)
	case t.IsContainer():
		g.generateSerializeContainer(out, t, prefix+fieldName)
	case t.IsBaseType(), t.IsEnum():
		name := prefix + fieldName
		out.WriteString(g.ind() + "oprot.")

		if t.IsBaseType() {
			b := t.(*sema.BaseType)
			switch b.Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeString:
				bang := ");"
				if nullAllowed {
					bang = "!);"
				}
				if b.IsBinary() {
					out.WriteString("writeBinary(" + name + bang)
				} else {
					out.WriteString("writeString(" + name + bang)
				}
			case sema.TypeBool:
				out.WriteString("writeBool(" + name + "!);")
			case sema.TypeI8:
				out.WriteString("writeByte(" + name + "!);")
			case sema.TypeI16:
				out.WriteString("writeI16(" + name + "!);")
			case sema.TypeI32:
				out.WriteString("writeI32(" + name + "!);")
			case sema.TypeI64:
				out.WriteString("writeI64(" + name + "!);")
			case sema.TypeDouble:
				out.WriteString("writeDouble(" + name + "!);")
			default:
				emit.Throw("compiler error: no Dart name for base type %s", sema.BaseName(b.Base()))
			}
		} else if t.IsEnum() {
			out.WriteString("writeI32(" + name + ");")
		}
		out.WriteString("\n")
	default:
		// DO NOT KNOW HOW TO SERIALIZE FIELD: printf-only diagnostic in
		// the C++ generator, nothing written to the file.
	}
}

// generateSerializeStruct is t_dart_generator::generate_serialize_struct:
// serializes all the members of a struct.
func (g *Generator) generateSerializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	_ = s
	out.WriteString(g.ind() + prefix + "?.write(oprot);\n")
}

// generateSerializeContainer is
// t_dart_generator::generate_serialize_container: serializes a
// container by writing its size then the elements.
func (g *Generator) generateSerializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	out.WriteString(g.ind())
	g.scopeUpPrefix(out, "")

	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		// The C++ source declares "string iter = tmp("_key");" here,
		// shadowed and unused inside this branch only; the tmp counter
		// still advances, so the call is reproduced for numbering
		// parity even though its result is discarded.
		_ = g.tmp("_key")
		out.WriteString(g.ind() + "oprot.writeMapBegin(new TMap(" + typeToEnum(m.KeyType()) + ", " + typeToEnum(m.ValType()) + ", " + prefix + "!.length));\n")
	case t.IsSet():
		s := t.(*sema.Set)
		out.WriteString(g.ind() + "oprot.writeSetBegin(new TSet(" + typeToEnum(s.ElemType()) + ", " + prefix + "!.length));\n")
	case t.IsList():
		l := t.(*sema.List)
		out.WriteString(g.ind() + "oprot.writeListBegin(new TList(" + typeToEnum(l.ElemType()) + ", " + prefix + "!.length));\n")
	}

	iter := g.tmp("elem")
	switch {
	case t.IsMap():
		out.WriteString(g.ind() + "for (var " + iter + " in " + prefix + "!.keys)")
	case t.IsSet(), t.IsList():
		out.WriteString(g.ind() + "for (var " + iter + " in " + prefix + "!)")
	}

	g.scopeUp(out)

	switch {
	case t.IsMap():
		g.generateSerializeMapElement(out, t.(*sema.Map), iter, prefix)
	case t.IsSet():
		g.generateSerializeSetElement(out, t.(*sema.Set), iter)
	case t.IsList():
		g.generateSerializeListElement(out, t.(*sema.List), iter)
	}

	g.scopeDown(out)

	switch {
	case t.IsMap():
		out.WriteString(g.ind() + "oprot.writeMapEnd();\n")
	case t.IsSet():
		out.WriteString(g.ind() + "oprot.writeSetEnd();\n")
	case t.IsList():
		out.WriteString(g.ind() + "oprot.writeListEnd();\n")
	}

	g.scopeDown(out)
}

// generateSerializeMapElement is
// t_dart_generator::generate_serialize_map_element.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, m *sema.Map, iter, mapName string) {
	kfield := sema.NewField(m.KeyType(), iter, 0)
	g.generateSerializeField(out, kfield, "")
	vfield := sema.NewField(m.ValType(), mapName+"!["+iter+"]", 0)
	g.generateSerializeField(out, vfield, "")
}

// generateSerializeSetElement is
// t_dart_generator::generate_serialize_set_element.
func (g *Generator) generateSerializeSetElement(out *strings.Builder, s *sema.Set, iter string) {
	efield := sema.NewField(s.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}

// generateSerializeListElement is
// t_dart_generator::generate_serialize_list_element.
func (g *Generator) generateSerializeListElement(out *strings.Builder, l *sema.List, iter string) {
	efield := sema.NewField(l.ElemType(), iter, 0)
	g.generateSerializeField(out, efield, "")
}
