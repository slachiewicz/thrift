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

package cpp

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateDeserializeField is t_cpp_generator::generate_deserialize_field:
// deserializes a field of any type.
func (g *Generator) generateDeserializeField(out *strings.Builder, f *sema.Field, prefix, suffix string) {
	t := sema.TrueType(f.Type())
	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, f.Name())
	}

	name := prefix + f.Name() + suffix

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateDeserializeStruct(out, t.(*sema.Struct), name, isReference(f))
	case t.IsContainer():
		g.generateDeserializeContainer(out, t, name)
	case t.IsBaseType():
		b := t.(*sema.BaseType)
		out.WriteString(g.indent() + "xfer += iprot->")
		switch b.Base() {
		case sema.TypeVoid:
			emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
		case sema.TypeUUID:
			out.WriteString("readUUID(" + name + ");")
		case sema.TypeString:
			if b.IsBinary() {
				out.WriteString("readBinary(" + name + ");")
			} else {
				out.WriteString("readString(" + name + ");")
			}
		case sema.TypeBool:
			out.WriteString("readBool(" + name + ");")
		case sema.TypeI8:
			out.WriteString("readByte(" + name + ");")
		case sema.TypeI16:
			out.WriteString("readI16(" + name + ");")
		case sema.TypeI32:
			out.WriteString("readI32(" + name + ");")
		case sema.TypeI64:
			out.WriteString("readI64(" + name + ");")
		case sema.TypeDouble:
			out.WriteString("readDouble(" + name + ");")
		default:
			emit.Throw("compiler error: no C++ reader for base type %s %s", sema.BaseName(b.Base()), name)
		}
		out.WriteString("\n")
	case t.IsEnum():
		tvar := g.tmp("ecast")
		out.WriteString(g.indent() + "int32_t " + tvar + ";\n" + g.indent() + "xfer += iprot->readI32(" + tvar + ");\n" +
			g.indent() + name + " = static_cast<" + g.typeName(t, false, false) + ">(" + tvar + ");\n")
	default:
		// The C++ generator only prints a diagnostic here; nothing goes
		// into the output stream.
	}
}

// generateDeserializeStruct is t_cpp_generator::generate_deserialize_struct.
// It assumes there is a const char* variable named data pointing at the
// deserialization buffer, and a variable protocol referencing the TProtocol
// serialization object.
func (g *Generator) generateDeserializeStruct(out *strings.Builder, s *sema.Struct, prefix string, pointer bool) {
	if pointer {
		out.WriteString(g.indent() + "if (!" + prefix + ") { \n")
		out.WriteString(g.indent() + "  " + prefix + " = ::std::shared_ptr<" + g.typeName(s, false, false) +
			">(new " + g.typeName(s, false, false) + ");\n")
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "xfer += " + prefix + "->read(iprot);\n")
		out.WriteString(g.indent() + "bool wasSet = false;\n")
		for _, f := range s.Members() {
			out.WriteString(g.indent() + "if (" + prefix + "->__isset." + f.Name() + ") { wasSet = true; }\n")
		}
		out.WriteString(g.indent() + "if (!wasSet) { " + prefix + ".reset(); }\n")
	} else {
		out.WriteString(g.indent() + "xfer += " + prefix + ".read(iprot);\n")
	}
}

func (g *Generator) generateDeserializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	g.scopeUp(out)

	// t_generator::tmp is called for all four names regardless of the
	// container kind, so the temporary counter advances the same way the
	// C++ generator's does.
	size := g.tmp("_size")
	ktype := g.tmp("_ktype")
	vtype := g.tmp("_vtype")
	etype := g.tmp("_etype")

	usePush := false
	if cn, ok := t.(cppNamed); ok {
		usePush = cn.HasCppName()
	}

	out.WriteString(g.indent() + prefix + ".clear();\n" + g.indent() + "uint32_t " + size + ";\n")

	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "::apache::thrift::protocol::TType " + ktype + ";\n" +
			g.indent() + "::apache::thrift::protocol::TType " + vtype + ";\n" +
			g.indent() + "xfer += iprot->readMapBegin(" + ktype + ", " + vtype + ", " + size + ");\n")
	case t.IsSet():
		out.WriteString(g.indent() + "::apache::thrift::protocol::TType " + etype + ";\n" +
			g.indent() + "xfer += iprot->readSetBegin(" + etype + ", " + size + ");\n")
	case t.IsList():
		out.WriteString(g.indent() + "::apache::thrift::protocol::TType " + etype + ";\n" +
			g.indent() + "xfer += iprot->readListBegin(" + etype + ", " + size + ");\n")
		if !usePush {
			out.WriteString(g.indent() + prefix + ".reserve(::apache::thrift::protocol::preallocSize(" + size + "));\n")
		}
	}

	// The for loop iterates over the elements.
	i := g.tmp("_i")
	out.WriteString(g.indent() + "uint32_t " + i + ";\n" + g.indent() + "for (" + i + " = 0; " + i +
		" < " + size + "; ++" + i + ")\n")

	g.scopeUp(out)

	switch {
	case t.IsMap():
		g.generateDeserializeMapElement(out, t.(*sema.Map), prefix)
	case t.IsSet():
		g.generateDeserializeSetElement(out, t.(*sema.Set), prefix)
	case t.IsList():
		g.generateDeserializeListElement(out, t.(*sema.List), prefix, usePush, i)
	}

	g.scopeDown(out)

	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "xfer += iprot->readMapEnd();\n")
	case t.IsSet():
		out.WriteString(g.indent() + "xfer += iprot->readSetEnd();\n")
	case t.IsList():
		out.WriteString(g.indent() + "xfer += iprot->readListEnd();\n")
	}

	g.scopeDown(out)
}

func (g *Generator) generateDeserializeMapElement(out *strings.Builder, m *sema.Map, prefix string) {
	key := g.tmp("_key")
	val := g.tmp("_val")
	fkey := sema.NewField(m.KeyType(), key, 0)
	fval := sema.NewField(m.ValType(), val, 0)

	out.WriteString(g.indent() + g.declareField(fkey, false, false, false, false) + "\n")

	g.generateDeserializeField(out, fkey, "", "")
	out.WriteString(g.indent() + g.declareField(fval, false, false, false, true) + " = " + prefix + "[" + key + "];\n")

	g.generateDeserializeField(out, fval, "", "")
}

func (g *Generator) generateDeserializeSetElement(out *strings.Builder, s *sema.Set, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(s.ElemType(), elem, 0)

	out.WriteString(g.indent() + g.declareField(felem, false, false, false, false) + "\n")

	g.generateDeserializeField(out, felem, "", "")

	out.WriteString(g.indent() + prefix + ".insert(" + elem + ");\n")
}

func (g *Generator) generateDeserializeListElement(out *strings.Builder, l *sema.List, prefix string, usePush bool, index string) {
	if usePush {
		elem := g.tmp("_elem")
		felem := sema.NewField(l.ElemType(), elem, 0)
		out.WriteString(g.indent() + g.declareField(felem, false, false, false, false) + "\n")
		g.generateDeserializeField(out, felem, "", "")
		out.WriteString(g.indent() + prefix + ".push_back(" + elem + ");\n")
	} else {
		// The vector's capacity was reserved (capped) from the declared
		// count, so grow it one element at a time and read into the
		// element just appended, rather than indexing into a full resize.
		_ = index
		out.WriteString(g.indent() + prefix + ".emplace_back();\n")
		felem := sema.NewField(l.ElemType(), prefix+".back()", 0)
		g.generateDeserializeField(out, felem, "", "")
	}
}

// generateSerializeField is t_cpp_generator::generate_serialize_field:
// serializes a field of any type.
func (g *Generator) generateSerializeField(out *strings.Builder, f *sema.Field, prefix, suffix string) {
	t := sema.TrueType(f.Type())
	name := prefix + f.Name() + suffix

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s", name)
	}

	switch {
	case t.IsStruct() || t.IsXception():
		g.generateSerializeStruct(out, t.(*sema.Struct), name, isReference(f))
	case t.IsContainer():
		g.generateSerializeContainer(out, t, name)
	case t.IsBaseType() || t.IsEnum():
		out.WriteString(g.indent() + "xfer += oprot->")
		if t.IsBaseType() {
			b := t.(*sema.BaseType)
			switch b.Base() {
			case sema.TypeVoid:
				emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
			case sema.TypeUUID:
				out.WriteString("writeUUID(" + name + ");")
			case sema.TypeString:
				if b.IsBinary() {
					out.WriteString("writeBinary(" + name + ");")
				} else {
					out.WriteString("writeString(" + name + ");")
				}
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
				emit.Throw("compiler error: no C++ writer for base type %s %s", sema.BaseName(b.Base()), name)
			}
		} else if t.IsEnum() {
			out.WriteString("writeI32(static_cast<int32_t>(" + name + "));")
		}
		out.WriteString("\n")
	default:
		// The C++ generator only prints a diagnostic here; nothing goes
		// into the output stream.
	}
}

// generateSerializeStruct is t_cpp_generator::generate_serialize_struct:
// serializes all the members of a struct.
func (g *Generator) generateSerializeStruct(out *strings.Builder, s *sema.Struct, prefix string, pointer bool) {
	if pointer {
		out.WriteString(g.indent() + "if (" + prefix + ") {\n")
		out.WriteString(g.indent() + "  xfer += " + prefix + "->write(oprot); \n")
		// This line reproduces a C++ generator quirk: the "} else {" and
		// the following statement land on the same line, with no
		// indent() call between them.
		out.WriteString(g.indent() + "} else {" + "oprot->writeStructBegin(\"" + s.Name() + "\"); \n")
		out.WriteString(g.indent() + "  oprot->writeStructEnd();\n")
		out.WriteString(g.indent() + "  oprot->writeFieldStop();\n")
		out.WriteString(g.indent() + "}\n")
	} else {
		out.WriteString(g.indent() + "xfer += " + prefix + ".write(oprot);\n")
	}
}

func (g *Generator) generateSerializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	g.scopeUp(out)

	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		out.WriteString(g.indent() + "xfer += oprot->writeMapBegin(" + typeToEnum(m.KeyType()) + ", " + typeToEnum(m.ValType()) + ", " +
			"static_cast<uint32_t>(" + prefix + ".size()));\n")
	case t.IsSet():
		s := t.(*sema.Set)
		out.WriteString(g.indent() + "xfer += oprot->writeSetBegin(" + typeToEnum(s.ElemType()) + ", " +
			"static_cast<uint32_t>(" + prefix + ".size()));\n")
	case t.IsList():
		l := t.(*sema.List)
		out.WriteString(g.indent() + "xfer += oprot->writeListBegin(" + typeToEnum(l.ElemType()) + ", " +
			"static_cast<uint32_t>(" + prefix + ".size()));\n")
	}

	iter := g.tmp("_iter")
	out.WriteString(g.indent() + g.typeName(t, false, false) + "::const_iterator " + iter + ";\n" + g.indent() +
		"for (" + iter + " = " + prefix + ".begin(); " + iter + " != " + prefix + ".end(); ++" + iter + ")\n")
	g.scopeUp(out)
	switch {
	case t.IsMap():
		g.generateSerializeMapElement(out, t.(*sema.Map), iter)
	case t.IsSet():
		g.generateSerializeSetElement(out, t.(*sema.Set), iter)
	case t.IsList():
		g.generateSerializeListElement(out, t.(*sema.List), iter)
	}
	g.scopeDown(out)

	switch {
	case t.IsMap():
		out.WriteString(g.indent() + "xfer += oprot->writeMapEnd();\n")
	case t.IsSet():
		out.WriteString(g.indent() + "xfer += oprot->writeSetEnd();\n")
	case t.IsList():
		out.WriteString(g.indent() + "xfer += oprot->writeListEnd();\n")
	}

	g.scopeDown(out)
}

// generateSerializeMapElement is t_cpp_generator::generate_serialize_map_element.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, m *sema.Map, iter string) {
	kfield := sema.NewField(m.KeyType(), iter+"->first", 0)
	g.generateSerializeField(out, kfield, "", "")

	vfield := sema.NewField(m.ValType(), iter+"->second", 0)
	g.generateSerializeField(out, vfield, "", "")
}

// generateSerializeSetElement is t_cpp_generator::generate_serialize_set_element.
func (g *Generator) generateSerializeSetElement(out *strings.Builder, s *sema.Set, iter string) {
	efield := sema.NewField(s.ElemType(), "(*"+iter+")", 0)
	g.generateSerializeField(out, efield, "", "")
}

// generateSerializeListElement is t_cpp_generator::generate_serialize_list_element.
func (g *Generator) generateSerializeListElement(out *strings.Builder, l *sema.List, iter string) {
	efield := sema.NewField(l.ElemType(), "(*"+iter+")", 0)
	g.generateSerializeField(out, efield, "", "")
}
