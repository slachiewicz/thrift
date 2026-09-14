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
	"sort"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateEnum is generate_enum: a class with a set of static constants.
func (g *Generator) generateEnum(e *sema.Enum) {
	deprecated := isDeprecated(e.Annotations())
	fEnumName := g.packageDir + "/" + makeValidJavaFilename(e.Name()) + ".java"
	var f strings.Builder

	f.WriteString(autogenComment() + g.javaPackage() + "\n")

	g.javaDoc(&f, e)

	if !g.opts.SuppressGeneratedAnnotation {
		g.generateJavaxGeneratedAnnotation(&f)
	}

	if deprecated {
		f.WriteString(g.indent() + "@Deprecated\n")
	}
	f.WriteString(g.indent() + "public enum " + e.Name() + " implements org.apache.thrift.TEnum ")
	g.scopeUp(&f)

	first := true
	for _, c := range e.Constants() {
		value := int64(c.Value())
		if first {
			first = false
		} else {
			f.WriteString(",\n")
		}
		g.javaDoc(&f, c)
		if isDeprecated(c.Annotations()) {
			f.WriteString(g.indent() + "@Deprecated\n")
		}
		f.WriteString(g.indent() + c.Name() + "(" + itoa(value) + ")")
	}
	f.WriteString(";\n\n")

	f.WriteString(g.indent() + "private final int value;\n\n")

	f.WriteString(g.indent() + "private " + e.Name() + "(int value) {\n")
	f.WriteString(g.indent() + "  this.value = value;\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + "/**\n")
	f.WriteString(g.indent() + " * Get the integer value of this enum value, as defined in the Thrift IDL.\n")
	f.WriteString(g.indent() + " */\n")
	f.WriteString(g.indent() + javaOverrideAnnotation() + "\n")
	f.WriteString(g.indent() + "public int getValue() {\n")
	f.WriteString(g.indent() + "  return value;\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + "/**\n")
	f.WriteString(g.indent() + " * Find a the enum type by its integer value, as defined in the Thrift IDL.\n")
	f.WriteString(g.indent() + " * @return null if the value is not found.\n")
	f.WriteString(g.indent() + " */\n")
	f.WriteString(g.indent() + javaNullableAnnotation() + "\n")
	f.WriteString(g.indent() + "public static " + e.Name() + " findByValue(int value) { \n")

	g.indentUp()
	f.WriteString(g.indent() + "switch (value) {\n")
	g.indentUp()
	for _, c := range e.Constants() {
		f.WriteString(g.indent() + "case " + itoa(int64(c.Value())) + ":\n")
		f.WriteString(g.indent() + "  return " + c.Name() + ";\n")
	}
	f.WriteString(g.indent() + "default:\n")
	f.WriteString(g.indent() + "  return null;\n")
	g.indentDown()
	f.WriteString(g.indent() + "}\n")
	g.indentDown()
	f.WriteString(g.indent() + "}\n")

	g.scopeDown(&f)

	emit.WriteFile(fEnumName, f.String())
}

// generateConsts is generate_consts: a class that holds all the constants.
func (g *Generator) generateConsts(consts []*sema.Const) {
	if len(consts) == 0 {
		return
	}
	fConstsName := g.packageDir + "/" + makeValidJavaFilename(g.programName) + "Constants.java"
	var f strings.Builder

	f.WriteString(autogenComment() + g.javaPackage() + javaSuppressions())
	f.WriteString("public class " + makeValidJavaIdentifier(g.programName) + "Constants {\n\n")
	g.indentUp()
	for _, c := range consts {
		g.javaDoc(&f, c)
		g.printConstValue(&f, c.Name(), c.Type(), c.Value(), false, false)
	}
	g.indentDown()
	f.WriteString(g.indent() + "}\n")
	emit.WriteFile(fConstsName, f.String())
}

// printConstValue is print_const_value.
func (g *Generator) printConstValue(out *strings.Builder, name string, typ sema.Type, value *sema.ConstValue, inStatic, defval bool) {
	typ = trueType(typ)

	out.WriteString(g.indent())
	if !defval {
		if !inStatic {
			out.WriteString("public static final ")
		}
		out.WriteString(g.tn(typ) + " ")
	}
	switch {
	case typ.IsBaseType():
		v2 := g.renderConstValue(out, typ, value)
		out.WriteString(name + " = " + v2 + ";\n\n")
	case typ.IsEnum():
		out.WriteString(name + " = " + g.renderConstValue(out, typ, value) + ";\n\n")
	case typ.IsStruct() || typ.IsXception():
		fields := append([]*sema.Field(nil), typ.(*sema.Struct).Members()...)
		sort.SliceStable(fields, func(i, j int) bool { return fields[i].Key() < fields[j].Key() })
		out.WriteString(name + " = new " + g.typeName(typ, false, true, false, false) + "();\n")
		if !inStatic {
			out.WriteString(g.indent() + "static {\n")
			g.indentUp()
		}
		for _, e := range value.Map() {
			var fieldType sema.Type
			for _, f := range fields {
				if f.Name() == e.Key.String() {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				emit.Throw("type error: %s has no field %s", typ.Name(), e.Key.String())
			}
			val := g.renderConstValue(out, fieldType, e.Value)
			out.WriteString(g.indent() + name + ".")
			out.WriteString("set" + g.capName(e.Key.String()) + "(" + val + ");\n")
		}
		if !inStatic {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		out.WriteString("\n")
	case typ.IsMap():
		constructorArgs := ""
		if g.isEnumMap(typ) {
			constructorArgs = g.innerEnumTypeName(typ)
		}
		out.WriteString(name + " = new " + g.typeName(typ, false, true, false, false) + "(" + constructorArgs + ");\n")
		if !inStatic {
			out.WriteString(g.indent() + "static {\n")
			g.indentUp()
		}
		m := typ.(*sema.Map)
		for _, e := range value.Map() {
			key := g.renderConstValue(out, m.KeyType(), e.Key)
			val := g.renderConstValue(out, m.ValType(), e.Value)
			out.WriteString(g.indent() + name + ".put(" + key + ", " + val + ");\n")
		}
		if !inStatic {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		out.WriteString("\n")
	case typ.IsList() || typ.IsSet():
		if g.isEnumSet(typ) {
			out.WriteString(name + " = " + g.typeName(typ, false, true, true, false) + ".noneOf(" + g.innerEnumTypeName(typ) + ");\n")
		} else {
			out.WriteString(name + " = new " + g.typeName(typ, false, true, false, false) + "();\n")
		}
		if !inStatic {
			out.WriteString(g.indent() + "static {\n")
			g.indentUp()
		}
		var etype sema.Type
		if typ.IsList() {
			etype = typ.(*sema.List).ElemType()
		} else {
			etype = typ.(*sema.Set).ElemType()
		}
		for _, v := range value.List() {
			val := g.renderConstValue(out, etype, v)
			out.WriteString(g.indent() + name + ".add(" + val + ");\n")
		}
		if !inStatic {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		out.WriteString("\n")
	default:
		emit.Throw("compiler error: no const of type %s", typ.Name())
	}
}

// renderConstValue is render_const_value.
func (g *Generator) renderConstValue(out *strings.Builder, typ sema.Type, value *sema.ConstValue) string {
	typ = trueType(typ)
	var render strings.Builder

	if typ.IsBaseType() {
		switch baseOf(typ) {
		case sema.TypeString:
			if typ.IsBinary() {
				render.WriteString("java.nio.ByteBuffer.wrap(\"" + escapedString(value) + "\".getBytes())")
			} else {
				render.WriteString("\"" + escapedString(value) + "\"")
			}
		case sema.TypeUUID:
			render.WriteString("java.util.UUID.fromString(\"" + escapedString(value) + "\")")
		case sema.TypeBool:
			if value.Integer() > 0 {
				render.WriteString("true")
			} else {
				render.WriteString("false")
			}
		case sema.TypeI8:
			render.WriteString("(byte)" + itoa(value.Integer()))
		case sema.TypeI16:
			render.WriteString("(short)" + itoa(value.Integer()))
		case sema.TypeI32:
			render.WriteString(itoa(value.Integer()))
		case sema.TypeI64:
			render.WriteString(itoa(value.Integer()) + "L")
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				render.WriteString(itoa(value.Integer()) + "d")
			} else {
				render.WriteString(emit.DoubleFixed16(value.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", typ.Name())
		}
	} else if typ.IsEnum() {
		namespacePrefix := typ.Program().Namespace("java")
		if len(namespacePrefix) > 0 {
			namespacePrefix += "."
		}
		render.WriteString(namespacePrefix + value.IdentifierWithParent())
	} else {
		t := g.tmp("tmp")
		g.printConstValue(out, t, typ, value, true, false)
		render.WriteString(t)
	}
	return render.String()
}
