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

package java

import (
	"fmt"
	"os"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateDeserializeField is generate_deserialize_field.
func (g *Generator) generateDeserializeField(out *strings.Builder, f *sema.Field, prefix string, hasMetadata bool) {
	typ := trueType(f.Type())

	if typ.IsVoid() {
		emit.Throw("CANNOT GENERATE DESERIALIZE CODE FOR void TYPE: %s%s", prefix, f.Name())
	}

	name := prefix + makeValidJavaIdentifier(f.Name())

	switch {
	case typ.IsStruct() || typ.IsXception():
		g.generateDeserializeStruct(out, typ.(*sema.Struct), name)
	case typ.IsContainer():
		g.generateDeserializeContainer(out, typ, name, hasMetadata)
	case typ.IsBaseType():
		out.WriteString(g.indent() + name + " = iprot.")
		switch baseOf(typ) {
		case sema.TypeVoid:
			emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
		case sema.TypeString:
			if typ.IsBinary() {
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
		case sema.TypeUUID:
			out.WriteString("readUuid();")
		case sema.TypeDouble:
			out.WriteString("readDouble();")
		default:
			emit.Throw("compiler error: no Java name for base type %s", typ.Name())
		}
		out.WriteString("\n")
	case typ.IsEnum():
		out.WriteString(g.indent() + name + " = " + g.typeName(f.Type(), true, false, false, true) + ".findByValue(iprot.readI32());\n")
	default:
		fmt.Fprintf(os.Stdout, "DO NOT KNOW HOW TO DESERIALIZE FIELD '%s' TYPE '%s'\n", f.Name(), g.tn(typ))
	}
}

func (g *Generator) generateDeserializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	if g.opts.ReuseObjects {
		out.WriteString(g.indent() + "if (" + prefix + " == null) {\n")
		g.indentUp()
	}
	out.WriteString(g.indent() + prefix + " = new " + g.tn(s) + "();\n")
	if g.opts.ReuseObjects {
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	out.WriteString(g.indent() + prefix + ".read(iprot);\n")
}

func (g *Generator) generateDeserializeContainer(out *strings.Builder, t sema.Type, prefix string, hasMetadata bool) {
	g.scopeUp(out)

	obj := ""
	switch {
	case t.IsMap():
		obj = g.tmp("_map")
	case t.IsSet():
		obj = g.tmp("_set")
	case t.IsList():
		obj = g.tmp("_list")
	}

	if hasMetadata {
		switch {
		case t.IsMap():
			out.WriteString(g.indent() + "org.apache.thrift.protocol.TMap " + obj + " = iprot.readMapBegin();\n")
		case t.IsSet():
			out.WriteString(g.indent() + "org.apache.thrift.protocol.TSet " + obj + " = iprot.readSetBegin();\n")
		case t.IsList():
			out.WriteString(g.indent() + "org.apache.thrift.protocol.TList " + obj + " = iprot.readListBegin();\n")
		}
	} else {
		switch {
		case t.IsMap():
			m := t.(*sema.Map)
			out.WriteString(g.indent() + "org.apache.thrift.protocol.TMap " + obj + " = iprot.readMapBegin(" +
				g.typeToEnum(m.KeyType()) + ", " + g.typeToEnum(m.ValType()) + "); \n")
		case t.IsSet():
			out.WriteString(g.indent() + "org.apache.thrift.protocol.TSet " + obj + " = iprot.readSetBegin(" +
				g.typeToEnum(t.(*sema.Set).ElemType()) + ");\n")
		case t.IsList():
			out.WriteString(g.indent() + "org.apache.thrift.protocol.TList " + obj + " = iprot.readListBegin(" +
				g.typeToEnum(t.(*sema.List).ElemType()) + ");\n")
		}
	}

	if g.opts.ReuseObjects {
		out.WriteString(g.indent() + "if (" + prefix + " == null) {\n")
		g.indentUp()
	}

	if g.isEnumSet(t) {
		out.WriteString(g.indent() + prefix + " = " + g.typeName(t, false, true, true, false) + ".noneOf")
	} else {
		out.WriteString(g.indent() + prefix + " = new " + g.typeName(t, false, true, false, false))
	}

	if g.isEnumSet(t) || g.isEnumMap(t) {
		out.WriteString("(" + g.innerEnumTypeName(t) + ");\n")
	} else if g.opts.SortedContainers && (t.IsMap() || t.IsSet()) {
		out.WriteString("();\n")
	} else {
		mul := "2*"
		if t.IsList() {
			mul = ""
		}
		out.WriteString("(org.apache.thrift.TBaseHelper.preallocSize(" + mul + obj + ".size));\n")
	}

	if g.opts.ReuseObjects {
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}

	switch {
	case t.IsMap():
		g.generateDeserializeMapElement(out, t.(*sema.Map), prefix, obj, hasMetadata)
	case t.IsSet():
		g.generateDeserializeSetElement(out, t.(*sema.Set), prefix, obj, hasMetadata)
	case t.IsList():
		g.generateDeserializeListElement(out, t.(*sema.List), prefix, obj, hasMetadata)
	}

	g.scopeDown(out)

	if hasMetadata {
		switch {
		case t.IsMap():
			out.WriteString(g.indent() + "iprot.readMapEnd();\n")
		case t.IsSet():
			out.WriteString(g.indent() + "iprot.readSetEnd();\n")
		case t.IsList():
			out.WriteString(g.indent() + "iprot.readListEnd();\n")
		}
	}
	g.scopeDown(out)
}

func (g *Generator) generateDeserializeMapElement(out *strings.Builder, m *sema.Map, prefix, obj string, hasMetadata bool) {
	key := g.tmp("_key")
	val := g.tmp("_val")
	fkey := sema.NewField(m.KeyType(), key, 0)
	fval := sema.NewField(m.ValType(), val, 0)

	out.WriteString(g.indent() + g.declareField(fkey, g.opts.ReuseObjects, false) + "\n")
	out.WriteString(g.indent() + g.declareField(fval, g.opts.ReuseObjects, false) + "\n")

	i := g.tmp("_i")
	out.WriteString(g.indent() + "for (int " + i + " = 0; " + i + " < " + obj + ".size" + "; " + "++" + i + ")\n")

	g.scopeUp(out)

	g.generateDeserializeField(out, fkey, "", hasMetadata)
	g.generateDeserializeField(out, fval, "", hasMetadata)

	if trueType(fkey.Type()).IsEnum() {
		out.WriteString(g.indent() + "if (" + key + " != null)\n")
		g.scopeUp(out)
	}

	out.WriteString(g.indent() + prefix + ".put(" + key + ", " + val + ");\n")

	if trueType(fkey.Type()).IsEnum() {
		g.scopeDown(out)
	}

	if g.opts.ReuseObjects && !trueType(fkey.Type()).IsBaseType() {
		out.WriteString(g.indent() + key + " = null;\n")
	}
	if g.opts.ReuseObjects && !trueType(fval.Type()).IsBaseType() {
		out.WriteString(g.indent() + val + " = null;\n")
	}
}

func (g *Generator) generateDeserializeSetElement(out *strings.Builder, s *sema.Set, prefix, obj string, hasMetadata bool) {
	g.generateDeserializeElement(out, s.ElemType(), prefix, obj, hasMetadata)
}

func (g *Generator) generateDeserializeListElement(out *strings.Builder, l *sema.List, prefix, obj string, hasMetadata bool) {
	g.generateDeserializeElement(out, l.ElemType(), prefix, obj, hasMetadata)
}

// generateDeserializeElement is the body shared by the set and list
// element deserializers, which are identical in the C++ code.
func (g *Generator) generateDeserializeElement(out *strings.Builder, elemType sema.Type, prefix, obj string, hasMetadata bool) {
	elem := g.tmp("_elem")
	felem := sema.NewField(elemType, elem, 0)

	out.WriteString(g.indent() + g.declareField(felem, g.opts.ReuseObjects, false) + "\n")

	i := g.tmp("_i")
	out.WriteString(g.indent() + "for (int " + i + " = 0; " + i + " < " + obj + ".size" + "; " + "++" + i + ")\n")
	g.scopeUp(out)

	g.generateDeserializeField(out, felem, "", hasMetadata)

	if trueType(felem.Type()).IsEnum() {
		out.WriteString(g.indent() + "if (" + elem + " != null)\n")
		g.scopeUp(out)
	}

	out.WriteString(g.indent() + prefix + ".add(" + elem + ");\n")

	if trueType(felem.Type()).IsEnum() {
		g.scopeDown(out)
	}

	if g.opts.ReuseObjects && !trueType(felem.Type()).IsBaseType() {
		out.WriteString(g.indent() + elem + " = null;\n")
	}
}

// generateSerializeField is generate_serialize_field.
func (g *Generator) generateSerializeField(out *strings.Builder, f *sema.Field, prefix, postfix string, hasMetadata bool) {
	typ := trueType(f.Type())

	if typ.IsVoid() {
		emit.Throw("CANNOT GENERATE SERIALIZE CODE FOR void TYPE: %s%s%s", prefix, f.Name(), postfix)
	}

	full := prefix + makeValidJavaIdentifier(f.Name()) + postfix
	switch {
	case typ.IsStruct() || typ.IsXception():
		g.generateSerializeStruct(out, typ.(*sema.Struct), full)
	case typ.IsContainer():
		g.generateSerializeContainer(out, typ, full, hasMetadata)
	case typ.IsEnum():
		out.WriteString(g.indent() + "oprot.writeI32(" + full + ".getValue());\n")
	case typ.IsBaseType():
		name := full
		out.WriteString(g.indent() + "oprot.")
		switch baseOf(typ) {
		case sema.TypeVoid:
			emit.Throw("compiler error: cannot serialize void field in a struct: %s", name)
		case sema.TypeString:
			if typ.IsBinary() {
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
		case sema.TypeUUID:
			out.WriteString("writeUuid(" + name + ");")
		case sema.TypeDouble:
			out.WriteString("writeDouble(" + name + ");")
		default:
			emit.Throw("compiler error: no Java name for base type %s", typ.Name())
		}
		out.WriteString("\n")
	default:
		fmt.Fprintf(os.Stdout, "DO NOT KNOW HOW TO SERIALIZE FIELD '%s%s%s' TYPE '%s'\n", prefix, f.Name(), postfix, g.tn(typ))
	}
}

func (g *Generator) generateSerializeStruct(out *strings.Builder, s *sema.Struct, prefix string) {
	_ = s
	out.WriteString(g.indent() + prefix + ".write(oprot);\n")
}

func (g *Generator) generateSerializeContainer(out *strings.Builder, t sema.Type, prefix string, hasMetadata bool) {
	g.scopeUp(out)

	if hasMetadata {
		switch {
		case t.IsMap():
			m := t.(*sema.Map)
			out.WriteString(g.indent() + "oprot.writeMapBegin(new org.apache.thrift.protocol.TMap(" +
				g.typeToEnum(m.KeyType()) + ", " + g.typeToEnum(m.ValType()) + ", " + prefix + ".size()));\n")
		case t.IsSet():
			out.WriteString(g.indent() + "oprot.writeSetBegin(new org.apache.thrift.protocol.TSet(" +
				g.typeToEnum(t.(*sema.Set).ElemType()) + ", " + prefix + ".size()));\n")
		case t.IsList():
			out.WriteString(g.indent() + "oprot.writeListBegin(new org.apache.thrift.protocol.TList(" +
				g.typeToEnum(t.(*sema.List).ElemType()) + ", " + prefix + ".size()));\n")
		}
	} else {
		out.WriteString(g.indent() + "oprot.writeI32(" + prefix + ".size());\n")
	}

	iter := g.tmp("_iter")
	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		out.WriteString(g.indent() + "for (java.util.Map.Entry<" +
			g.typeName(m.KeyType(), true, false, false, false) + ", " +
			g.typeName(m.ValType(), true, false, false, false) + "> " + iter + " : " + prefix + ".entrySet())")
	case t.IsSet():
		out.WriteString(g.indent() + "for (" + g.tn(t.(*sema.Set).ElemType()) + " " + iter + " : " + prefix + ")")
	case t.IsList():
		out.WriteString(g.indent() + "for (" + g.tn(t.(*sema.List).ElemType()) + " " + iter + " : " + prefix + ")")
	}

	out.WriteString("\n")
	g.scopeUp(out)
	switch {
	case t.IsMap():
		m := t.(*sema.Map)
		g.generateSerializeField(out, sema.NewField(m.KeyType(), iter, 0), "", ".getKey()", hasMetadata)
		g.generateSerializeField(out, sema.NewField(m.ValType(), iter, 0), "", ".getValue()", hasMetadata)
	case t.IsSet():
		g.generateSerializeField(out, sema.NewField(t.(*sema.Set).ElemType(), iter, 0), "", "", hasMetadata)
	case t.IsList():
		g.generateSerializeField(out, sema.NewField(t.(*sema.List).ElemType(), iter, 0), "", "", hasMetadata)
	}
	g.scopeDown(out)

	if hasMetadata {
		switch {
		case t.IsMap():
			out.WriteString(g.indent() + "oprot.writeMapEnd();\n")
		case t.IsSet():
			out.WriteString(g.indent() + "oprot.writeSetEnd();\n")
		case t.IsList():
			out.WriteString(g.indent() + "oprot.writeListEnd();\n")
		}
	}

	g.scopeDown(out)
}

// generateDeepCopyContainer is generate_deep_copy_container.
func (g *Generator) generateDeepCopyContainer(out *strings.Builder, sourceNameP1, sourceNameP2, resultName string, t sema.Type) {
	sourceName := sourceNameP1
	if sourceNameP2 != "" {
		sourceName = sourceNameP1 + "." + sourceNameP2
	}

	var copyConstructContainer bool
	if t.IsMap() {
		m := t.(*sema.Map)
		copyConstructContainer = m.KeyType().IsBaseType() && m.ValType().IsBaseType()
	} else {
		var elemType sema.Type
		if t.IsList() {
			elemType = t.(*sema.List).ElemType()
		} else {
			elemType = t.(*sema.Set).ElemType()
		}
		copyConstructContainer = elemType.IsBaseType()
	}

	if copyConstructContainer {
		out.WriteString(g.indent() + g.typeName(t, true, false, false, false) + " " + resultName + " = new " +
			g.typeName(t, false, true, false, false) + "(" + sourceName + ");\n")
		return
	}

	constructorArgs := ""
	if g.isEnumSet(t) || g.isEnumMap(t) {
		constructorArgs = g.innerEnumTypeName(t)
	} else if !(g.opts.SortedContainers && (t.IsMap() || t.IsSet())) {
		constructorArgs = sourceName + ".size()"
	}

	if g.isEnumSet(t) {
		out.WriteString(g.indent() + g.typeName(t, true, false, false, false) + " " + resultName + " = " +
			g.typeName(t, false, true, true, false) + ".noneOf(" + constructorArgs + ");\n")
	} else {
		out.WriteString(g.indent() + g.typeName(t, true, false, false, false) + " " + resultName + " = new " +
			g.typeName(t, false, true, false, false) + "(" + constructorArgs + ");\n")
	}

	iteratorElementName := sourceNameP1 + "_element"
	resultElementName := resultName + "_copy"

	if t.IsMap() {
		m := t.(*sema.Map)
		keyType, valType := m.KeyType(), m.ValType()

		out.WriteString(g.indent() + "for (java.util.Map.Entry<" + g.typeName(keyType, true, false, false, false) + ", " +
			g.typeName(valType, true, false, false, false) + "> " + iteratorElementName + " : " + sourceName + ".entrySet()) {\n")
		g.indentUp()

		out.WriteString("\n")

		out.WriteString(g.indent() + g.typeName(keyType, true, false, false, false) + " " + iteratorElementName + "_key = " + iteratorElementName + ".getKey();\n")
		out.WriteString(g.indent() + g.typeName(valType, true, false, false, false) + " " + iteratorElementName + "_value = " + iteratorElementName + ".getValue();\n")

		out.WriteString("\n")

		if keyType.IsContainer() {
			g.generateDeepCopyContainer(out, iteratorElementName+"_key", "", resultElementName+"_key", keyType)
		} else {
			out.WriteString(g.indent() + g.typeName(keyType, true, false, false, false) + " " + resultElementName + "_key = ")
			g.generateDeepCopyNonContainer(out, iteratorElementName+"_key", resultElementName+"_key", keyType)
			out.WriteString(";\n")
		}

		out.WriteString("\n")

		if valType.IsContainer() {
			g.generateDeepCopyContainer(out, iteratorElementName+"_value", "", resultElementName+"_value", valType)
		} else {
			out.WriteString(g.indent() + g.typeName(valType, true, false, false, false) + " " + resultElementName + "_value = ")
			g.generateDeepCopyNonContainer(out, iteratorElementName+"_value", resultElementName+"_value", valType)
			out.WriteString(";\n")
		}

		out.WriteString("\n")

		out.WriteString(g.indent() + resultName + ".put(" + resultElementName + "_key, " + resultElementName + "_value);\n")

		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	} else {
		var elemType sema.Type
		if t.IsSet() {
			elemType = t.(*sema.Set).ElemType()
		} else {
			elemType = t.(*sema.List).ElemType()
		}

		out.WriteString(g.indent() + "for (" + g.typeName(elemType, true, false, false, false) + " " + iteratorElementName + " : " + sourceName + ") {\n")
		g.indentUp()

		if elemType.IsContainer() {
			g.generateDeepCopyContainer(out, iteratorElementName, "", resultElementName, elemType)
			out.WriteString(g.indent() + resultName + ".add(" + resultElementName + ");\n")
		} else {
			if elemType.IsBinary() {
				out.WriteString(g.indent() + "java.nio.ByteBuffer temp_binary_element = ")
				g.generateDeepCopyNonContainer(out, iteratorElementName, "temp_binary_element", elemType)
				out.WriteString(";\n")
				out.WriteString(g.indent() + resultName + ".add(temp_binary_element);\n")
			} else {
				out.WriteString(g.indent() + resultName + ".add(")
				g.generateDeepCopyNonContainer(out, iteratorElementName, resultName, elemType)
				out.WriteString(");\n")
			}
		}

		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
}

func (g *Generator) generateDeepCopyNonContainer(out *strings.Builder, sourceName, destName string, t sema.Type) {
	_ = destName
	t = trueType(t)
	if t.IsBaseType() || t.IsEnum() || t.IsTypedef() {
		if t.IsBinary() {
			out.WriteString("org.apache.thrift.TBaseHelper.copyBinary(" + sourceName + ")")
		} else {
			out.WriteString(sourceName)
		}
	} else {
		out.WriteString("new " + g.typeName(t, true, true, false, false) + "(" + sourceName + ")")
	}
}
