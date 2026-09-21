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

// generateEnum is generate_enum: a class with a set of static constants
// (Java ME predates the "enum" keyword's use here, so this is a plain
// class, unlike t_java_generator's Java 5 enum).
func (g *Generator) generateEnum(e *sema.Enum) {
	fEnumName := g.packageDir + "/" + e.Name() + ".java"
	var f strings.Builder

	f.WriteString(autogenComment() + g.javaPackage())

	g.javaDoc(&f, e)
	f.WriteString(g.indent() + "public class " + e.Name() + " implements org.apache.thrift.TEnum ")
	g.scopeUp(&f)
	f.WriteString("\n")

	for _, c := range e.Constants() {
		g.javaDoc(&f, c)
		f.WriteString(g.indent() + "public static final " + e.Name() + " " + c.Name() + " = new " + e.Name() + "(" + itoa(int64(c.Value())) + ");\n")
	}
	f.WriteString("\n")

	f.WriteString(g.indent() + "private final int value;\n\n")

	f.WriteString(g.indent() + "private " + e.Name() + "(int value) {\n")
	f.WriteString(g.indent() + "  this.value = value;\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + "/**\n")
	f.WriteString(g.indent() + " * Get the integer value of this enum value, as defined in the Thrift IDL.\n")
	f.WriteString(g.indent() + " */\n")
	f.WriteString(g.indent() + "public int getValue() {\n")
	f.WriteString(g.indent() + "  return value;\n")
	f.WriteString(g.indent() + "}\n\n")

	f.WriteString(g.indent() + "/**\n")
	f.WriteString(g.indent() + " * Find a the enum type by its integer value, as defined in the Thrift IDL.\n")
	f.WriteString(g.indent() + " * @return null if the value is not found.\n")
	f.WriteString(g.indent() + " */\n")
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

// generateConsts is generate_consts: a class that holds all the
// constants. Unlike generate_enum and generate_java_struct, it takes no
// java_doc pass over each const.
func (g *Generator) generateConsts(consts []*sema.Const) {
	if len(consts) == 0 {
		return
	}
	fConstsName := g.packageDir + "/" + g.programName + "Constants.java"
	var f strings.Builder

	f.WriteString(autogenComment() + g.javaPackage() + g.javaTypeImports())
	f.WriteString("public class " + g.programName + "Constants {\n\n")
	g.indentUp()
	for _, c := range consts {
		g.printConstValue(&f, c.Name(), c.Type(), c.Value(), false, false)
	}
	g.indentDown()
	f.WriteString(g.indent() + "}\n")
	emit.WriteFile(fConstsName, f.String())
}

// printConstValue is print_const_value. Note that type checking is NOT
// performed here, as it is always run beforehand by validate_types.
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
		fields := typ.(*sema.Struct).Members()
		out.WriteString(name + " = new " + g.typeName(typ, false) + "();\n")
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
			out.WriteString("set" + getCapName(e.Key.String()) + "(" + val + ");\n")
		}
		if !inStatic {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		out.WriteString("\n")
	case typ.IsMap():
		out.WriteString(name + " = new " + g.typeName(typ, false) + "();\n")
		if !inStatic {
			out.WriteString(g.indent() + "static {\n")
			g.indentUp()
		}
		m := typ.(*sema.Map)
		for _, e := range value.Map() {
			key := g.renderConstValue(out, m.KeyType(), e.Key)
			val := g.renderConstValue(out, m.ValType(), e.Value)
			out.WriteString(g.indent() + name + ".put(" + boxType(m.KeyType(), key) + ", " + boxType(m.ValType(), val) + ");\n")
		}
		if !inStatic {
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		}
		out.WriteString("\n")
	case typ.IsList() || typ.IsSet():
		out.WriteString(name + " = new " + g.typeName(typ, false) + "();\n")
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
			if typ.IsList() {
				out.WriteString(g.indent() + name + ".addElement(" + boxType(etype, val) + ");\n")
			} else {
				out.WriteString(g.indent() + name + ".put(" + boxType(etype, val) + ", " + boxType(etype, val) + ");\n")
			}
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

// renderConstValue is render_const_value. Unlike t_java_generator, the
// double case streams the value directly instead of going through
// emit_double_as_string, and the enum case uses the identifier exactly
// as written (get_identifier, not get_identifier_with_parent).
func (g *Generator) renderConstValue(out *strings.Builder, typ sema.Type, value *sema.ConstValue) string {
	typ = trueType(typ)
	var render strings.Builder

	if typ.IsBaseType() {
		switch baseOf(typ) {
		case sema.TypeString:
			render.WriteString(`"` + emit.EscapeString(value.String()) + `"`)
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
				render.WriteString("(double)" + itoa(value.Integer()))
			} else {
				render.WriteString(formatDouble(value.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(baseOf(typ)))
		}
	} else if typ.IsEnum() {
		render.WriteString(g.tn(typ) + "." + value.Identifier())
	} else {
		t := g.tmp("tmp")
		g.printConstValue(out, t, typ, value, true, false)
		render.WriteString(t)
	}
	return render.String()
}

// boxType is box_type. It checks the type as given, without resolving a
// typedef first, so a typedef'd primitive is not boxed here (matching
// t_javame_generator's own inconsistency).
func boxType(t sema.Type, value string) string {
	if t.IsBaseType() {
		switch baseOf(t) {
		case sema.TypeBool:
			return "new Boolean(" + value + ")"
		case sema.TypeI8:
			return "new Byte(" + value + ")"
		case sema.TypeI16:
			return "new Short(" + value + ")"
		case sema.TypeI32:
			return "new Integer(" + value + ")"
		case sema.TypeI64:
			return "new Long(" + value + ")"
		case sema.TypeDouble:
			return "new Double(" + value + ")"
		}
	}
	return value
}
