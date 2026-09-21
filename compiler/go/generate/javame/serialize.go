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

package javame

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateDeserializeField is generate_deserialize_field: deserializes a
// field of any type.
func (g *Generator) generateDeserializeField(out *strings.Builder, f *sema.Field, prefix string) {
	t := trueType(f.Type())

	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, f.Name())
	}

	name := prefix + f.Name()

	if t.IsStruct() || t.IsXception() {
		g.generateDeserializeStruct(out, t.(*sema.Struct), name)
	} else if t.IsContainer() {
		g.generateDeserializeContainer(out, t, name)
	} else if t.IsBaseType() {
		out.WriteString(g.indent() + name + " = iprot.")

		switch baseOf(t) {
		case sema.TypeVoid:
			emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
		case sema.TypeString:
			if !t.IsBinary() {
				out.WriteString("readString();")
			} else {
				out.WriteString("readBinary();")
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
			emit.Throw("compiler error: no Java name for base type %s", sema.BaseName(baseOf(t)))
		}
		out.WriteString("\n")
	} else if t.IsEnum() {
		out.WriteString(g.indent() + name + " = " + g.typeName(f.Type(), true) + ".findByValue(iprot.readI32());\n")
	}
	// The C++ source's final else prints a diagnostic to the compiler's
	// own stdout, not to the generated file; every resolved type reaches
	// one of the branches above, so it is unreachable.
}

// generateDeserializeStruct is generate_deserialize_struct: invokes
// read().
func (g *Generator) generateDeserializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	out.WriteString(g.indent() + prefix + " = new " + g.tn(s) + "();\n")
	out.WriteString(g.indent() + prefix + ".read(iprot);\n")
}

// generateDeserializeContainer is generate_deserialize_container:
// deserializes a container by reading its size and then iterating.
func (g *Generator) generateDeserializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	g.scopeUp(out)

	var obj string
	if t.IsMap() {
		obj = g.tmp("_map")
	} else if t.IsSet() {
		obj = g.tmp("_set")
	} else if t.IsList() {
		obj = g.tmp("_list")
	}

	// Declare variables, read header
	if t.IsMap() {
		out.WriteString(g.indent() + "TMap " + obj + " = iprot.readMapBegin();\n")
	} else if t.IsSet() {
		out.WriteString(g.indent() + "TSet " + obj + " = iprot.readSetBegin();\n")
	} else if t.IsList() {
		out.WriteString(g.indent() + "TList " + obj + " = iprot.readListBegin();\n")
	}

	twoTimes := "2*"
	if t.IsList() {
		twoTimes = ""
	}
	out.WriteString(g.indent() + prefix + " = new " + g.typeName(t, false) +
		// size the collection correctly, but cap the capacity reserved from the wire-supplied
		// element count so a peer cannot name a count it never backs with data
		"(TBaseHelper.preallocSize(" + twoTimes + obj + ".size));\n")

	// For loop iterates over elements
	i := g.tmp("_i")
	out.WriteString(g.indent() + "for (int " + i + " = 0; " + i + " < " + obj + ".size; ++" + i + ")\n")

	g.scopeUp(out)

	if t.IsMap() {
		g.generateDeserializeMapElement(out, t.(*sema.Map), prefix)
	} else if t.IsSet() {
		g.generateDeserializeSetElement(out, t.(*sema.Set), prefix)
	} else if t.IsList() {
		g.generateDeserializeListElement(out, t.(*sema.List), prefix)
	}

	g.scopeDown(out)

	// Read container end
	if t.IsMap() {
		out.WriteString(g.indent() + "iprot.readMapEnd();\n")
	} else if t.IsSet() {
		out.WriteString(g.indent() + "iprot.readSetEnd();\n")
	} else if t.IsList() {
		out.WriteString(g.indent() + "iprot.readListEnd();\n")
	}

	g.scopeDown(out)
}

// generateDeserializeMapElement is generate_deserialize_map_element.
func (g *Generator) generateDeserializeMapElement(out *strings.Builder, m *sema.Map, prefix string) {
	key := g.tmp("_key")
	val := g.tmp("_val")
	fkey := sema.NewField(m.KeyType(), key, 0)
	fval := sema.NewField(m.ValType(), val, 0)

	out.WriteString(g.indent() + g.declareField(fkey, false) + "\n")
	out.WriteString(g.indent() + g.declareField(fval, false) + "\n")

	g.generateDeserializeField(out, fkey, "")
	g.generateDeserializeField(out, fval, "")

	out.WriteString(g.indent() + prefix + ".put(" + boxType(m.KeyType(), key) + ", " + boxType(m.ValType(), val) + ");\n")
}

// generateDeserializeSetElement is generate_deserialize_set_element.
func (g *Generator) generateDeserializeSetElement(out *strings.Builder, s *sema.Set, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(s.ElemType(), elem, 0)

	out.WriteString(g.indent() + g.declareField(felem, false) + "\n")

	g.generateDeserializeField(out, felem, "")

	out.WriteString(g.indent() + prefix + ".put(" + boxType(s.ElemType(), elem) + ", " + boxType(s.ElemType(), elem) + ");\n")
}

// generateDeserializeListElement is generate_deserialize_list_element.
func (g *Generator) generateDeserializeListElement(out *strings.Builder, l *sema.List, prefix string) {
	elem := g.tmp("_elem")
	felem := sema.NewField(l.ElemType(), elem, 0)

	out.WriteString(g.indent() + g.declareField(felem, false) + "\n")

	g.generateDeserializeField(out, felem, "")

	out.WriteString(g.indent() + prefix + ".addElement(" + boxType(l.ElemType(), elem) + ");\n")
}

// generateSerializeField is generate_serialize_field: serializes a field
// of any type.
func (g *Generator) generateSerializeField(out *strings.Builder, f *sema.Field, prefix string) {
	t := trueType(f.Type())

	// Do nothing for void types
	if t.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s%s", prefix, f.Name())
	}

	if t.IsStruct() || t.IsXception() {
		g.generateSerializeStruct(out, t.(*sema.Struct), prefix+f.Name())
	} else if t.IsContainer() {
		g.generateSerializeContainer(out, t, prefix+f.Name())
	} else if t.IsEnum() {
		out.WriteString(g.indent() + "oprot.writeI32(" + prefix + f.Name() + ".getValue());\n")
	} else if t.IsBaseType() {
		name := prefix + f.Name()
		out.WriteString(g.indent() + "oprot.")

		switch baseOf(t) {
		case sema.TypeVoid:
			emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
		case sema.TypeString:
			if t.IsBinary() {
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
			emit.Throw("compiler error: no Java name for base type %s", sema.BaseName(baseOf(t)))
		}
		out.WriteString("\n")
	}
	// As in generateDeserializeField, the C++ source's final else only
	// prints a diagnostic to the compiler's own stdout; unreachable for
	// a resolved type.
}

// generateSerializeStruct is generate_serialize_struct.
func (g *Generator) generateSerializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	_ = s
	out.WriteString(g.indent() + prefix + ".write(oprot);\n")
}

// generateSerializeContainer is generate_serialize_container: writes its
// size then the elements.
func (g *Generator) generateSerializeContainer(out *strings.Builder, t sema.Type, prefix string) {
	g.scopeUp(out)

	if t.IsMap() {
		m := t.(*sema.Map)
		out.WriteString(g.indent() + "oprot.writeMapBegin(new TMap(" + g.typeToEnum(m.KeyType()) +
			", " + g.typeToEnum(m.ValType()) + ", " + prefix + ".size()));\n")
	} else if t.IsSet() {
		s := t.(*sema.Set)
		out.WriteString(g.indent() + "oprot.writeSetBegin(new TSet(" + g.typeToEnum(s.ElemType()) +
			", " + prefix + ".size()));\n")
	} else if t.IsList() {
		l := t.(*sema.List)
		out.WriteString(g.indent() + "oprot.writeListBegin(new TList(" + g.typeToEnum(l.ElemType()) +
			", " + prefix + ".size()));\n")
	}

	iter := g.tmp("_iter")
	if t.IsMap() {
		m := t.(*sema.Map)
		enumer := iter + "_enum"
		keyType := g.typeName(m.KeyType(), true)
		out.WriteString(g.indent() + "for (Enumeration " + enumer + " = " + prefix + ".keys(); " + enumer + ".hasMoreElements(); ) ")
		g.scopeUp(out)
		out.WriteString(g.indent() + keyType + " " + iter + " = (" + keyType + ")" + enumer + ".nextElement();\n")
	} else if t.IsSet() {
		s := t.(*sema.Set)
		enumer := iter + "_enum"
		eleType := g.typeName(s.ElemType(), true)
		out.WriteString(g.indent() + "for (Enumeration " + enumer + " = " + prefix + ".keys(); " + enumer + ".hasMoreElements(); ) ")
		g.scopeUp(out)
		out.WriteString(g.indent() + eleType + " " + iter + " = (" + eleType + ")" + enumer + ".nextElement();\n")
	} else if t.IsList() {
		l := t.(*sema.List)
		enumer := iter + "_enum"
		out.WriteString(g.indent() + "for (Enumeration " + enumer + " = " + prefix + ".elements(); " + enumer + ".hasMoreElements(); ) ")
		g.scopeUp(out)
		eleType := g.typeName(l.ElemType(), true)
		out.WriteString(g.indent() + eleType + " " + iter + " = (" + eleType + ")" + enumer + ".nextElement();\n")
	}

	if t.IsMap() {
		g.generateSerializeMapElement(out, t.(*sema.Map), iter, prefix)
	} else if t.IsSet() {
		g.generateSerializeSetElement(out, t.(*sema.Set), iter)
	} else if t.IsList() {
		g.generateSerializeListElement(out, t.(*sema.List), iter)
	}
	g.scopeDown(out)

	if t.IsMap() {
		out.WriteString(g.indent() + "oprot.writeMapEnd();\n")
	} else if t.IsSet() {
		out.WriteString(g.indent() + "oprot.writeSetEnd();\n")
	} else if t.IsList() {
		out.WriteString(g.indent() + "oprot.writeListEnd();\n")
	}

	g.scopeDown(out)
}

// generateSerializeMapElement is generate_serialize_map_element.
func (g *Generator) generateSerializeMapElement(out *strings.Builder, m *sema.Map, iter, mapVar string) {
	kfield := sema.NewField(m.KeyType(), iter, 0)
	g.generateSerializeField(out, kfield, "")
	valType := g.typeName(m.ValType(), true)
	vfield := sema.NewField(m.ValType(), "(("+valType+")"+mapVar+".get("+iter+"))", 0)
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

// generateDeepCopyContainer is generate_deep_copy_container.
func (g *Generator) generateDeepCopyContainer(out *strings.Builder, sourceNameP1, sourceNameP2, resultName string, container sema.Type) {
	sourceName := sourceNameP1
	if sourceNameP2 != "" {
		sourceName = sourceNameP1 + "." + sourceNameP2
	}

	out.WriteString(g.indent() + g.typeName(container, true) + " " + resultName + " = new " + g.typeName(container, false) + "();\n")

	iteratorElementName := sourceNameP1 + "_element"
	enumerationName := sourceNameP1 + "_enum"
	resultElementName := resultName + "_copy"

	if container.IsMap() {
		m := container.(*sema.Map)
		keyType := m.KeyType()
		valType := m.ValType()

		out.WriteString(g.indent() + "for (Enumeration " + enumerationName + " = " + sourceName + ".keys(); " + enumerationName + ".hasMoreElements(); ) {\n")
		g.indentUp()

		out.WriteString("\n")

		out.WriteString(g.indent() + g.typeName(keyType, true) + " " + iteratorElementName + "_key = (" + g.typeName(keyType, true) + ")" + enumerationName + ".nextElement();\n")
		out.WriteString(g.indent() + g.typeName(valType, true) + " " + iteratorElementName + "_value = (" + g.typeName(valType, true) + ")" + sourceName + ".get(" + iteratorElementName + "_key);\n")

		out.WriteString("\n")

		if keyType.IsContainer() {
			g.generateDeepCopyContainer(out, iteratorElementName+"_key", "", resultElementName+"_key", keyType)
		} else {
			out.WriteString(g.indent() + g.typeName(keyType, true) + " " + resultElementName + "_key = ")
			g.generateDeepCopyNonContainer(out, iteratorElementName+"_key", resultElementName+"_key", keyType)
			out.WriteString(";\n")
		}

		out.WriteString("\n")

		if valType.IsContainer() {
			g.generateDeepCopyContainer(out, iteratorElementName+"_value", "", resultElementName+"_value", valType)
		} else {
			out.WriteString(g.indent() + g.typeName(valType, true) + " " + resultElementName + "_value = ")
			g.generateDeepCopyNonContainer(out, iteratorElementName+"_value", resultElementName+"_value", valType)
			out.WriteString(";\n")
		}

		out.WriteString("\n")

		out.WriteString(g.indent() + resultName + ".put(" + resultElementName + "_key, " + resultElementName + "_value);\n")

		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	} else {
		var elemType sema.Type
		if container.IsSet() {
			elemType = container.(*sema.Set).ElemType()
		} else {
			elemType = container.(*sema.List).ElemType()
		}

		out.WriteString(g.indent() + "for (Enumeration " + enumerationName + " = " + sourceName + ".elements(); " + enumerationName + ".hasMoreElements(); ) {\n")
		g.indentUp()
		out.WriteString(g.indent() + g.typeName(elemType, true) + " " + iteratorElementName + " = (" + g.typeName(elemType, true) + ")" + enumerationName + ".nextElement();\n")
		if elemType.IsContainer() {
			// recursive deep copy
			g.generateDeepCopyContainer(out, iteratorElementName, "", resultElementName, elemType)
			if elemType.IsList() {
				out.WriteString(g.indent() + resultName + ".addElement(" + resultElementName + ");\n")
			} else {
				out.WriteString(g.indent() + resultName + ".put(" + resultElementName + ", " + resultElementName + ");\n")
			}
		} else {
			// iterative copy
			if elemType.IsBinary() {
				out.WriteString(g.indent() + g.typeName(elemType, true) + " temp_binary_element = ")
				g.generateDeepCopyNonContainer(out, iteratorElementName, "temp_binary_element", elemType)
				out.WriteString(";\n")
				if elemType.IsList() {
					out.WriteString(g.indent() + resultName + ".addElement(temp_binary_element);\n")
				} else {
					out.WriteString(g.indent() + resultName + ".put(temp_binary_element, temp_binary_element);\n")
				}
			} else {
				out.WriteString(g.indent() + resultName + ".addElement(")
				g.generateDeepCopyNonContainer(out, iteratorElementName, resultName, elemType)
				out.WriteString(");\n")
			}
		}

		g.indentDown()

		out.WriteString(g.indent() + "}\n")
	}
}

// generateDeepCopyNonContainer is generate_deep_copy_non_container. Like
// box_type, it checks the type exactly as given: a typedef of a binary
// field takes the plain-assignment branch instead of the
// System.arraycopy one, since is_typedef() short-circuits before
// is_binary() is even asked (matching t_javame_generator.cc).
func (g *Generator) generateDeepCopyNonContainer(out *strings.Builder, sourceName, destName string, t sema.Type) {
	if t.IsBaseType() || t.IsEnum() || t.IsTypedef() {
		if t.IsBinary() {
			out.WriteString("new byte[" + sourceName + ".length];\n")
			out.WriteString(g.indent() + "System.arraycopy(" + sourceName + ", 0, " + destName + ", 0, " + sourceName + ".length)")
		} else {
			out.WriteString(sourceName)
		}
	} else {
		out.WriteString("new " + g.typeName(t, true) + "(" + sourceName + ")")
	}
}
