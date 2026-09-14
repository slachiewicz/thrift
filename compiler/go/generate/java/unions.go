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
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateJavaUnion is generate_java_union.
func (g *Generator) generateJavaUnion(s *sema.Struct) {
	fStructName := g.packageDir + "/" + makeValidJavaFilename(s.Name()) + ".java"
	var f strings.Builder

	f.WriteString(autogenComment() + g.javaPackage())

	g.javaDoc(&f, s)
	f.WriteString(javaSuppressions())

	isFinal := s.Annotations().Has("final")
	deprecated := isDeprecated(s.Annotations())

	if !g.opts.SuppressGeneratedAnnotation {
		g.generateJavaxGeneratedAnnotation(&f)
	}

	if deprecated {
		f.WriteString(g.indent() + "@Deprecated\n")
	}
	id := makeValidJavaIdentifier(s.Name())
	f.WriteString(g.indent() + "public ")
	if isFinal {
		f.WriteString("final ")
	}
	f.WriteString("class " + id + " extends org.apache.thrift.TUnion<" + id + ", " + id + "._Fields> ")

	g.scopeUp(&f)

	g.generateStructDesc(&f, s)
	g.generateFieldDescs(&f, s)

	f.WriteString("\n")

	g.generateFieldNameConstants(&f, s)

	f.WriteString("\n")

	g.generateJavaMetaDataMap(&f, s)

	g.generateUnionConstructor(&f, s)

	f.WriteString("\n")

	g.generateUnionAbstractMethods(&f, s)

	f.WriteString("\n")

	g.generateJavaStructFieldByID(&f, s)

	f.WriteString("\n")

	g.generateUnionGettersAndSetters(&f, s)

	f.WriteString("\n")

	g.generateUnionIsSetMethods(&f, s)

	f.WriteString("\n")

	g.generateUnionComparisons(&f, s)

	f.WriteString("\n")

	g.generateUnionHashcode(&f, s)

	f.WriteString("\n")

	g.generateJavaStructWriteObject(&f, s)

	f.WriteString("\n")

	g.generateJavaStructReadObject(&f, s)

	f.WriteString("\n")

	g.scopeDown(&f)

	emit.WriteFile(fStructName, f.String())
}

func (g *Generator) generateUnionConstructor(out *strings.Builder, s *sema.Struct) {
	members := s.Members()
	tn := g.tn(s)

	out.WriteString(g.indent() + "public " + tn + "() {\n")
	g.indentUp()
	defaultValue := false
	for _, m := range members {
		t := trueType(m.Type())
		if m.Value() != nil {
			out.WriteString(g.indent() + "super(_Fields." + constantName(m.Name()) + ", " + g.renderConstValue(out, t, m.Value()) + ");\n")
			defaultValue = true
			break
		}
	}
	if !defaultValue {
		out.WriteString(g.indent() + "super();\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "public " + tn + "(_Fields setField, java.lang.Object value) {\n")
	out.WriteString(g.indent() + "  super(setField, value);\n")
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "public " + tn + "(" + tn + " other) {\n")
	out.WriteString(g.indent() + "  super(other);\n")
	out.WriteString(g.indent() + "}\n")

	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public " + makeValidJavaIdentifier(s.Name()) + " deepCopy() {\n")
	out.WriteString(g.indent() + "  return new " + s.Name() + "(this);\n")
	out.WriteString(g.indent() + "}\n\n")

	for _, m := range members {
		t := m.Type()
		out.WriteString(g.indent() + "public static " + tn + " " + m.Name() + "(" + g.tn(t) + " value) {\n")
		out.WriteString(g.indent() + "  " + tn + " x = new " + tn + "();\n")
		out.WriteString(g.indent() + "  x.set" + g.capName(m.Name()) + "(value);\n")
		out.WriteString(g.indent() + "  return x;\n")
		out.WriteString(g.indent() + "}\n\n")

		if t.IsBinary() {
			out.WriteString(g.indent() + "public static " + tn + " " + m.Name() + "(byte[] value) {\n")
			out.WriteString(g.indent() + "  " + tn + " x = new " + tn + "();\n")
			out.WriteString(g.indent() + "  x.set" + g.capName(m.Name()))
			if g.opts.UnsafeBinaries {
				out.WriteString(g.indent() + "(java.nio.ByteBuffer.wrap(value));\n")
			} else {
				out.WriteString(g.indent() + "(java.nio.ByteBuffer.wrap(value.clone()));\n")
			}
			out.WriteString(g.indent() + "  return x;\n")
			out.WriteString(g.indent() + "}\n\n")
		}
	}
}

func (g *Generator) generateUnionGettersAndSetters(out *strings.Builder, s *sema.Struct) {
	first := true
	for _, f := range s.Members() {
		if first {
			first = false
		} else {
			out.WriteString("\n")
		}

		t := f.Type()
		capName := g.capName(f.Name())
		deprecated := isDeprecated(f.Annotations())

		g.javaDocField(out, f)
		if t.IsBinary() {
			if deprecated {
				out.WriteString(g.indent() + "@Deprecated\n")
			}
			out.WriteString(g.indent() + "public byte[] get" + capName + "() {\n")
			out.WriteString(g.indent() + "  set" + capName + "(org.apache.thrift.TBaseHelper.rightSize(buffer" + g.capName("for") + capName + "()));\n")
			out.WriteString(g.indent() + "  java.nio.ByteBuffer b = buffer" + g.capName("for") + capName + "();\n")
			out.WriteString(g.indent() + "  return b == null ? null : b.array();\n")
			out.WriteString(g.indent() + "}\n")

			out.WriteString("\n")

			out.WriteString(g.indent() + "public java.nio.ByteBuffer buffer" + g.capName("for") + g.capName(f.Name()) + "() {\n")
			out.WriteString(g.indent() + "  if (getSetField() == _Fields." + constantName(f.Name()) + ") {\n")

			if g.opts.UnsafeBinaries {
				out.WriteString(g.indent() + "    return (java.nio.ByteBuffer)getFieldValue();\n")
			} else {
				out.WriteString(g.indent() + "    return org.apache.thrift.TBaseHelper.copyBinary((java.nio.ByteBuffer)getFieldValue());\n")
			}

			out.WriteString(g.indent() + "  } else {\n")
			out.WriteString(g.indent() + "    throw new java.lang.RuntimeException(\"Cannot get field '" + f.Name() +
				"' because union is currently set to \" + getFieldDesc(getSetField()).name);\n")
			out.WriteString(g.indent() + "  }\n")
			out.WriteString(g.indent() + "}\n")
		} else {
			if deprecated {
				out.WriteString(g.indent() + "@Deprecated\n")
			}
			out.WriteString(g.indent() + "public " + g.tn(f.Type()) + " get" + g.capName(f.Name()) + "() {\n")
			out.WriteString(g.indent() + "  if (getSetField() == _Fields." + constantName(f.Name()) + ") {\n")
			out.WriteString(g.indent() + "    return (" + g.typeName(f.Type(), true, false, false, false) + ")getFieldValue();\n")
			out.WriteString(g.indent() + "  } else {\n")
			out.WriteString(g.indent() + "    throw new java.lang.RuntimeException(\"Cannot get field '" + f.Name() +
				"' because union is currently set to \" + getFieldDesc(getSetField()).name);\n")
			out.WriteString(g.indent() + "  }\n")
			out.WriteString(g.indent() + "}\n")
		}

		out.WriteString("\n")

		g.javaDocField(out, f)
		if t.IsBinary() {
			if deprecated {
				out.WriteString(g.indent() + "@Deprecated\n")
			}
			out.WriteString(g.indent() + "public void set" + g.capName(f.Name()) + "(byte[] value) {\n")
			out.WriteString(g.indent() + "  set" + g.capName(f.Name()))

			if g.opts.UnsafeBinaries {
				out.WriteString(g.indent() + "(java.nio.ByteBuffer.wrap(value));\n")
			} else {
				out.WriteString(g.indent() + "(java.nio.ByteBuffer.wrap(value.clone()));\n")
			}

			out.WriteString(g.indent() + "}\n")

			out.WriteString("\n")
		}
		if deprecated {
			out.WriteString(g.indent() + "@Deprecated\n")
		}
		out.WriteString(g.indent() + "public void set" + g.capName(f.Name()) + "(" + g.tn(f.Type()) + " value) {\n")

		out.WriteString(g.indent() + "  setField_ = _Fields." + constantName(f.Name()) + ";\n")

		if typeCanBeNull(f.Type()) {
			out.WriteString(g.indent() + "  value_ = java.util.Objects.requireNonNull(value,\"" + "_Fields." + constantName(f.Name()) + "\");\n")
		} else {
			out.WriteString(g.indent() + "  value_ = value;\n")
		}

		out.WriteString(g.indent() + "}\n")
	}
}

func (g *Generator) generateUnionIsSetMethods(out *strings.Builder, s *sema.Struct) {
	first := true
	for _, m := range s.Members() {
		if first {
			first = false
		} else {
			out.WriteString("\n")
		}

		fieldName := m.Name()

		out.WriteString(g.indent() + "public boolean is" + g.capName("set") + g.capName(fieldName) + "() {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return setField_ == _Fields." + constantName(fieldName) + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	}
}

func (g *Generator) generateUnionAbstractMethods(out *strings.Builder, s *sema.Struct) {
	g.generateCheckType(out, s)
	out.WriteString("\n")
	g.generateStandardSchemeReadValue(out, s)
	out.WriteString("\n")
	g.generateStandardSchemeWriteValue(out, s)
	out.WriteString("\n")
	g.generateTupleSchemeReadValue(out, s)
	out.WriteString("\n")
	g.generateTupleSchemeWriteValue(out, s)
	out.WriteString("\n")
	g.generateGetFieldDesc(out, s)
	out.WriteString("\n")
	g.generateGetStructDesc(out, s)
	out.WriteString("\n")
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "protected _Fields enumForId(short id) {\n")
	out.WriteString(g.indent() + "  return _Fields.findByThriftIdOrThrow(id);\n")
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateCheckType(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "protected void checkType(_Fields setField, java.lang.Object value) throws java.lang.ClassCastException {\n")
	g.indentUp()

	out.WriteString(g.indent() + "switch (setField) {\n")
	g.indentUp()

	for _, f := range s.Members() {
		out.WriteString(g.indent() + "case " + constantName(f.Name()) + ":\n")
		out.WriteString(g.indent() + "  if (value instanceof " + g.typeName(f.Type(), true, false, true, false) + ") {\n")
		out.WriteString(g.indent() + "    break;\n")
		out.WriteString(g.indent() + "  }\n")
		out.WriteString(g.indent() + "  throw new java.lang.ClassCastException(\"Was expecting value of type " +
			g.typeName(f.Type(), true, false, false, false) + " for field '" + f.Name() +
			"', but got \" + value.getClass().getSimpleName());\n")
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  throw new java.lang.IllegalArgumentException(\"Unknown field id \" + setField);\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateStandardSchemeReadValue(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "protected java.lang.Object standardSchemeReadValue(org.apache.thrift.protocol.TProtocol iprot, org.apache.thrift.protocol.TField field) throws org.apache.thrift.TException {\n")

	g.indentUp()

	out.WriteString(g.indent() + "_Fields setField = _Fields.findByThriftId(field.id);\n")
	out.WriteString(g.indent() + "if (setField != null) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "switch (setField) {\n")
	g.indentUp()

	for _, f := range s.Members() {
		out.WriteString(g.indent() + "case " + constantName(f.Name()) + ":\n")
		g.indentUp()
		out.WriteString(g.indent() + "if (field.type == " + constantName(f.Name()) + "_FIELD_DESC.type) {\n")
		g.indentUp()
		out.WriteString(g.indent() + g.typeName(f.Type(), true, false, false, false) + " " + f.Name() + ";\n")
		g.generateDeserializeField(out, f, "", true)
		out.WriteString(g.indent() + "return " + f.Name() + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "} else {\n")
		out.WriteString(g.indent() + "  org.apache.thrift.protocol.TProtocolUtil.skip(iprot, field.type);\n")
		out.WriteString(g.indent() + "  return null;\n")
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  throw new java.lang.IllegalStateException(\"setField wasn't null, but didn't match any of the case statements!\");\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "} else {\n")
	g.indentUp()
	out.WriteString(g.indent() + "org.apache.thrift.protocol.TProtocolUtil.skip(iprot, field.type);\n")
	out.WriteString(g.indent() + "return null;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateStandardSchemeWriteValue(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "protected void standardSchemeWriteValue(org.apache.thrift.protocol.TProtocol oprot) throws org.apache.thrift.TException {\n")

	g.indentUp()

	out.WriteString(g.indent() + "switch (setField_) {\n")
	g.indentUp()

	for _, f := range s.Members() {
		out.WriteString(g.indent() + "case " + constantName(f.Name()) + ":\n")
		g.indentUp()
		tn := g.typeName(f.Type(), true, false, false, false)
		out.WriteString(g.indent() + tn + " " + f.Name() + " = (" + tn + ")value_;\n")
		g.generateSerializeField(out, f, "", "", true)
		out.WriteString(g.indent() + "return;\n")
		g.indentDown()
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  throw new java.lang.IllegalStateException(\"Cannot write union with unknown field \" + setField_);\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()

	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateTupleSchemeReadValue(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "protected java.lang.Object tupleSchemeReadValue(org.apache.thrift.protocol.TProtocol iprot, short fieldID) throws org.apache.thrift.TException {\n")

	g.indentUp()

	out.WriteString(g.indent() + "_Fields setField = _Fields.findByThriftId(fieldID);\n")
	out.WriteString(g.indent() + "if (setField != null) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "switch (setField) {\n")
	g.indentUp()

	for _, f := range s.Members() {
		out.WriteString(g.indent() + "case " + constantName(f.Name()) + ":\n")
		g.indentUp()
		out.WriteString(g.indent() + g.typeName(f.Type(), true, false, false, false) + " " + f.Name() + ";\n")
		g.generateDeserializeField(out, f, "", true)
		out.WriteString(g.indent() + "return " + f.Name() + ";\n")
		g.indentDown()
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  throw new java.lang.IllegalStateException(\"setField wasn't null, but didn't match any of the case statements!\");\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "} else {\n")
	g.indentUp()
	out.WriteString(g.indent() + "throw new org.apache.thrift.protocol.TProtocolException(\"Couldn't find a field with field id \" + fieldID);\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateTupleSchemeWriteValue(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "protected void tupleSchemeWriteValue(org.apache.thrift.protocol.TProtocol oprot) throws org.apache.thrift.TException {\n")

	g.indentUp()

	out.WriteString(g.indent() + "switch (setField_) {\n")
	g.indentUp()

	for _, f := range s.Members() {
		out.WriteString(g.indent() + "case " + constantName(f.Name()) + ":\n")
		g.indentUp()
		tn := g.typeName(f.Type(), true, false, false, false)
		out.WriteString(g.indent() + tn + " " + f.Name() + " = (" + tn + ")value_;\n")
		g.generateSerializeField(out, f, "", "", true)
		out.WriteString(g.indent() + "return;\n")
		g.indentDown()
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  throw new java.lang.IllegalStateException(\"Cannot write union with unknown field \" + setField_);\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()

	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateGetFieldDesc(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "protected org.apache.thrift.protocol.TField getFieldDesc(_Fields setField) {\n")
	g.indentUp()

	out.WriteString(g.indent() + "switch (setField) {\n")
	g.indentUp()

	for _, f := range s.Members() {
		out.WriteString(g.indent() + "case " + constantName(f.Name()) + ":\n")
		out.WriteString(g.indent() + "  return " + constantName(f.Name()) + "_FIELD_DESC;\n")
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  throw new java.lang.IllegalArgumentException(\"Unknown field id \" + setField);\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateGetStructDesc(out *strings.Builder, s *sema.Struct) {
	_ = s
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "protected org.apache.thrift.protocol.TStruct getStructDesc() {\n")
	out.WriteString(g.indent() + "  return STRUCT_DESC;\n")
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateUnionComparisons(out *strings.Builder, s *sema.Struct) {
	id := makeValidJavaIdentifier(s.Name())
	out.WriteString(g.indent() + "public boolean equals(java.lang.Object other) {\n")
	out.WriteString(g.indent() + "  if (other instanceof " + id + ") {\n")
	out.WriteString(g.indent() + "    return equals((" + id + ")other);\n")
	out.WriteString(g.indent() + "  } else {\n")
	out.WriteString(g.indent() + "    return false;\n")
	out.WriteString(g.indent() + "  }\n")
	out.WriteString(g.indent() + "}\n")

	out.WriteString("\n")

	out.WriteString(g.indent() + "public boolean equals(" + id + " other) {\n")
	out.WriteString(g.indent() + "  return other != null && getSetField() == other.getSetField() && getFieldValue().equals(other.getFieldValue());\n")
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")

	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public int compareTo(" + g.tn(s) + " other) {\n")
	out.WriteString(g.indent() + "  int lastComparison = org.apache.thrift.TBaseHelper.compareTo(getSetField(), other.getSetField());\n")
	out.WriteString(g.indent() + "  if (lastComparison == 0) {\n")
	out.WriteString(g.indent() + "    return org.apache.thrift.TBaseHelper.compareTo(getFieldValue(), other.getFieldValue());\n")
	out.WriteString(g.indent() + "  }\n")
	out.WriteString(g.indent() + "  return lastComparison;\n")
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
}

func (g *Generator) generateUnionHashcode(out *strings.Builder, s *sema.Struct) {
	_ = s
	out.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	out.WriteString(g.indent() + "public int hashCode() {\n")
	out.WriteString(g.indent() + "  java.util.List<java.lang.Object> list = new java.util.ArrayList<java.lang.Object>();\n")
	out.WriteString(g.indent() + "  list.add(this.getClass().getName());\n")
	out.WriteString(g.indent() + "  org.apache.thrift.TFieldIdEnum setField = getSetField();\n")
	out.WriteString(g.indent() + "  if (setField != null) {\n")
	out.WriteString(g.indent() + "    list.add(setField.getThriftFieldId());\n")
	out.WriteString(g.indent() + "    java.lang.Object value = getFieldValue();\n")
	out.WriteString(g.indent() + "    if (value instanceof org.apache.thrift.TEnum) {\n")
	out.WriteString(g.indent() + "      list.add(((org.apache.thrift.TEnum)getFieldValue()).getValue());\n")
	out.WriteString(g.indent() + "    } else {\n")
	out.WriteString(g.indent() + "      list.add(value);\n")
	out.WriteString(g.indent() + "    }\n")
	out.WriteString(g.indent() + "  }\n")
	out.WriteString(g.indent() + "  return list.hashCode();\n")
	out.WriteString(g.indent() + "}")
}
