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

// generateJavaUnion is generate_java_union.
func (g *Generator) generateJavaUnion(s *sema.Struct) {
	fStructName := g.packageDir + "/" + s.Name() + ".java"
	var f strings.Builder

	f.WriteString(autogenComment() + g.javaPackage() + g.javaTypeImports() + g.javaThriftImports())

	g.javaDoc(&f, s)

	isFinal := s.Annotations().Has("final")

	f.WriteString(g.indent() + "public ")
	if isFinal {
		f.WriteString("final ")
	}
	f.WriteString("class " + s.Name() + " extends TUnion ")

	g.scopeUp(&f)

	g.generateStructDesc(&f, s)
	g.generateFieldDescs(&f, s)

	f.WriteString("\n")

	g.generateUnionConstructor(&f, s)

	f.WriteString("\n")

	g.generateUnionAbstractMethods(&f, s)

	f.WriteString("\n")

	g.generateUnionGettersAndSetters(&f, s)

	f.WriteString("\n")

	g.generateUnionComparisons(&f, s)

	f.WriteString("\n")

	g.generateUnionHashcode(&f, s)

	f.WriteString("\n")

	g.scopeDown(&f)

	emit.WriteFile(fStructName, f.String())
}

func (g *Generator) generateUnionConstructor(out *strings.Builder, s *sema.Struct) {
	tn := g.tn(s)

	out.WriteString(g.indent() + "public " + tn + "() {\n")
	out.WriteString(g.indent() + "  super();\n")
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "public " + tn + "(_Fields setField, Object value) {\n")
	out.WriteString(g.indent() + "  super(setField, value);\n")
	out.WriteString(g.indent() + "}\n\n")

	out.WriteString(g.indent() + "public " + tn + "(" + tn + " other) {\n")
	out.WriteString(g.indent() + "  super(other);\n")
	out.WriteString(g.indent() + "}\n")

	out.WriteString(g.indent() + "public " + s.Name() + " deepCopy() {\n")
	out.WriteString(g.indent() + "  return new " + s.Name() + "(this);\n")
	out.WriteString(g.indent() + "}\n\n")

	// generate "constructors" for each field
	for _, m := range s.Members() {
		out.WriteString(g.indent() + "public static " + tn + " " + m.Name() + "(" + g.tn(m.Type()) + " value) {\n")
		out.WriteString(g.indent() + "  " + tn + " x = new " + tn + "();\n")
		out.WriteString(g.indent() + "  x.set" + getCapName(m.Name()) + "(value);\n")
		out.WriteString(g.indent() + "  return x;\n")
		out.WriteString(g.indent() + "}\n\n")
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

		g.javaDocField(out, f)
		out.WriteString(g.indent() + "public " + g.tn(f.Type()) + " get" + getCapName(f.Name()) + "() {\n")
		out.WriteString(g.indent() + "  if (getSetField() == _Fields." + constantName(f.Name()) + ") {\n")
		out.WriteString(g.indent() + "    return (" + g.typeName(f.Type(), true) + ")getFieldValue();\n")
		out.WriteString(g.indent() + "  } else {\n")
		out.WriteString(g.indent() + "    throw new RuntimeException(\"Cannot get field '" + f.Name() +
			"' because union is currently set to \" + getFieldDesc(getSetField()).name);\n")
		out.WriteString(g.indent() + "  }\n")
		out.WriteString(g.indent() + "}\n")

		out.WriteString("\n")

		g.javaDocField(out, f)
		out.WriteString(g.indent() + "public void set" + getCapName(f.Name()) + "(" + g.tn(f.Type()) + " value) {\n")
		if typeCanBeNull(f.Type()) {
			out.WriteString(g.indent() + "  if (value == null) throw new NullPointerException();\n")
		}
		out.WriteString(g.indent() + "  setField_ = _Fields." + constantName(f.Name()) + ";\n")
		out.WriteString(g.indent() + "  value_ = value;\n")
		out.WriteString(g.indent() + "}\n")
	}
}

func (g *Generator) generateUnionAbstractMethods(out *strings.Builder, s *sema.Struct) {
	g.generateCheckType(out, s)
	out.WriteString("\n")
	g.generateReadValue(out, s)
	out.WriteString("\n")
	g.generateWriteValue(out, s)
	out.WriteString("\n")
	g.generateGetFieldDesc(out, s)
	out.WriteString("\n")
	g.generateGetStructDesc(out, s)
	out.WriteString("\n")
}

func (g *Generator) generateCheckType(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "protected void checkType(_Fields setField, Object value) throws ClassCastException {\n")
	g.indentUp()

	out.WriteString(g.indent() + "switch (setField) {\n")
	g.indentUp()

	for _, f := range s.Members() {
		out.WriteString(g.indent() + "case " + constantName(f.Name()) + ":\n")
		out.WriteString(g.indent() + "  if (value instanceof " + g.typeName(f.Type(), true) + ") {\n")
		out.WriteString(g.indent() + "    break;\n")
		out.WriteString(g.indent() + "  }\n")
		out.WriteString(g.indent() + "  throw new ClassCastException(\"Was expecting value of type " +
			g.typeName(f.Type(), true) + " for field '" + f.Name() + "', but got \" + value.getClass().getSimpleName());\n")
		// do the real check here
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  throw new IllegalArgumentException(\"Unknown field id \" + setField);\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateReadValue(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "protected Object readValue(TProtocol iprot, TField field) throws TException {\n")

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
		out.WriteString(g.indent() + g.typeName(f.Type(), true) + " " + f.Name() + ";\n")
		g.generateDeserializeField(out, f, "")
		out.WriteString(g.indent() + "return " + f.Name() + ";\n")
		g.indentDown()
		out.WriteString(g.indent() + "} else {\n")
		out.WriteString(g.indent() + "  TProtocolUtil.skip(iprot, field.type);\n")
		out.WriteString(g.indent() + "  return null;\n")
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  throw new IllegalStateException(\"setField wasn't null, but didn't match any of the case statements!\");\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "} else {\n")
	g.indentUp()
	out.WriteString(g.indent() + "TProtocolUtil.skip(iprot, field.type);\n")
	out.WriteString(g.indent() + "return null;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateWriteValue(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "protected void writeValue(TProtocol oprot) throws TException {\n")

	g.indentUp()

	out.WriteString(g.indent() + "switch (setField_) {\n")
	g.indentUp()

	for _, f := range s.Members() {
		out.WriteString(g.indent() + "case " + constantName(f.Name()) + ":\n")
		g.indentUp()
		out.WriteString(g.indent() + g.typeName(f.Type(), true) + " " + f.Name() + " = (" +
			g.typeName(f.Type(), true) + ")value_;\n")
		g.generateSerializeField(out, f, "")
		out.WriteString(g.indent() + "return;\n")
		g.indentDown()
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  throw new IllegalStateException(\"Cannot write union with unknown field \" + setField_);\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()

	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateGetFieldDesc(out *strings.Builder, s *sema.Struct) {
	out.WriteString(g.indent() + "protected TField getFieldDesc(_Fields setField) {\n")
	g.indentUp()

	out.WriteString(g.indent() + "switch (setField) {\n")
	g.indentUp()

	for _, f := range s.Members() {
		out.WriteString(g.indent() + "case " + constantName(f.Name()) + ":\n")
		out.WriteString(g.indent() + "  return " + constantName(f.Name()) + "_FIELD_DESC;\n")
	}

	out.WriteString(g.indent() + "default:\n")
	out.WriteString(g.indent() + "  throw new IllegalArgumentException(\"Unknown field id \" + setField);\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateGetStructDesc(out *strings.Builder, s *sema.Struct) {
	_ = s
	out.WriteString(g.indent() + "protected TStruct getStructDesc() {\n")
	out.WriteString(g.indent() + "  return STRUCT_DESC;\n")
	out.WriteString(g.indent() + "}\n")
}

func (g *Generator) generateUnionComparisons(out *strings.Builder, s *sema.Struct) {
	// equality
	out.WriteString(g.indent() + "public boolean equals(Object other) {\n")
	out.WriteString(g.indent() + "  if (other instanceof " + s.Name() + ") {\n")
	out.WriteString(g.indent() + "    return equals((" + s.Name() + ")other);\n")
	out.WriteString(g.indent() + "  } else {\n")
	out.WriteString(g.indent() + "    return false;\n")
	out.WriteString(g.indent() + "  }\n")
	out.WriteString(g.indent() + "}\n")

	out.WriteString("\n")

	out.WriteString(g.indent() + "public boolean equals(" + s.Name() + " other) {\n")
	out.WriteString(g.indent() + "  return other != null && getSetField() == other.getSetField() && getFieldValue().equals(other.getFieldValue());\n")
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")

	out.WriteString(g.indent() + "public int compareTo(" + g.tn(s) + " other) {\n")
	out.WriteString(g.indent() + "  int lastComparison = TBaseHelper.compareTo(getSetField(), other.getSetField());\n")
	out.WriteString(g.indent() + "  if (lastComparison == 0) {\n")
	out.WriteString(g.indent() + "    return TBaseHelper.compareTo(getFieldValue(), other.getFieldValue());\n")
	out.WriteString(g.indent() + "  }\n")
	out.WriteString(g.indent() + "  return lastComparison;\n")
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
}

func (g *Generator) generateUnionHashcode(out *strings.Builder, s *sema.Struct) {
	_ = s
	out.WriteString(g.indent() + "/**\n")
	out.WriteString(g.indent() + " * If you'd like this to perform more respectably, use the hashcode generator option.\n")
	out.WriteString(g.indent() + " */\n")
	out.WriteString(g.indent() + "public int hashCode() {\n")
	out.WriteString(g.indent() + "  return 0;\n")
	out.WriteString(g.indent() + "}\n")
}
