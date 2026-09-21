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
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateConsts is t_dart_generator::generate_consts: generates a class
// that holds all the constants.
func (g *Generator) generateConsts(consts []*sema.Const) {
	if len(consts) == 0 {
		return
	}

	className := getConstantsClassName(g.programName)
	fileName := getFileName(className)

	fConstsName := g.srcDir + "/" + fileName + ".dart"
	var f strings.Builder

	// Print header
	f.WriteString(autogenComment() + g.dartLibrary(fileName) + "\n")
	f.WriteString(g.dartThriftImports() + "\n")

	g.exportClassToLibrary(fileName, className)
	f.WriteString(g.ind() + "class " + className)
	g.scopeUp(&f)

	for _, c := range consts {
		g.printConstValue(&f, c.Name(), c.Type(), c.Value(), false, false, false)
		f.WriteString("\n")
	}

	g.scopeDown(&f)

	emit.WriteFile(fConstsName, f.String())
}

// printConstValue is t_dart_generator::print_const_value.
func (g *Generator) printConstValue(out *strings.Builder, name string, t sema.Type, value *sema.ConstValue, inStatic, defval, inlineOnly bool) {
	t = sema.TrueType(t)

	out.WriteString(g.ind())
	if !defval && !inlineOnly {
		if inStatic {
			out.WriteString("var ")
		} else {
			out.WriteString("static final ")
		}
	}
	switch {
	case t.IsBaseType():
		if !defval {
			out.WriteString(g.typeName(t) + " ")
		}
		v2 := g.renderConstValue(out, name, t, value)
		out.WriteString(name)
		out.WriteString(" = " + v2 + ";\n\n")
	case t.IsEnum():
		if !defval {
			out.WriteString(g.typeName(t) + " ")
		}
		out.WriteString(name)
		out.WriteString(" = " + strconv.FormatInt(value.Integer(), 10) + ";\n\n")
	case t.IsStruct(), t.IsXception():
		s := t.(*sema.Struct)
		fields := s.Members()
		if !inlineOnly {
			out.WriteString(g.typeName(t) + " " + name + " = new " + g.typeName(t) + "()")
		} else {
			out.WriteString("new " + g.typeName(t) + "()")
		}
		g.indentUp()
		for _, entry := range value.Map() {
			var fieldType sema.Type
			for _, f := range fields {
				if f.Name() == entry.Key.String() {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				emit.Throw("type error: %s has no field %s", t.Name(), entry.Key.String())
			}
			val2 := g.renderConstValue(out, name, fieldType, entry.Value)
			out.WriteString("\n")
			out.WriteString(g.ind() + ".." + entry.Key.String() + " = " + val2)
		}
		g.indentDown()
		if !inlineOnly {
			out.WriteString(";")
		}
		out.WriteString("\n")
	case t.IsMap():
		if !defval && !inlineOnly {
			out.WriteString(g.typeName(t) + " ")
		}
		if !inlineOnly {
			out.WriteString(name + " =")
		}
		g.scopeUp(out)

		m := t.(*sema.Map)
		for _, entry := range value.Map() {
			key2 := g.renderConstValue(out, name, m.KeyType(), entry.Key)
			val2 := g.renderConstValue(out, name, m.ValType(), entry.Value)
			out.WriteString(g.ind() + key2 + ": " + val2 + ",\n")
		}
		postfix := ";\n"
		if inlineOnly {
			postfix = "\n"
		}
		g.scopeDownPostfix(out, postfix)

		out.WriteString("\n")
	case t.IsList(), t.IsSet():
		if !defval && !inlineOnly {
			out.WriteString(g.typeName(t) + " ")
		}
		if !inlineOnly {
			out.WriteString(name + " =")
		}

		var etype sema.Type
		if t.IsList() {
			out.WriteString("[\n")
			etype = t.(*sema.List).ElemType()
		} else {
			out.WriteString("new " + g.typeName(t) + ".from([\n")
			etype = t.(*sema.Set).ElemType()
		}

		g.indentUp()
		for _, v := range value.List() {
			val2 := g.renderConstValue(out, name, etype, v)
			out.WriteString(g.ind() + val2 + ",\n")
		}
		g.indentDown()

		suffix := ";"
		if inlineOnly {
			suffix = ""
		}
		if t.IsList() {
			out.WriteString(g.ind() + "]" + suffix + "\n")
		} else {
			out.WriteString(g.ind() + "])" + suffix + "\n")
		}
	default:
		emit.Throw("compiler error: no const of type %s", t.Name())
	}
}

// renderConstValue is t_dart_generator::render_const_value. The out and
// name parameters are unused, as in the C++ original (marked with
// (void) casts there): the struct/map/list/set branch renders into its
// own local buffer via printConstValue instead.
func (g *Generator) renderConstValue(out *strings.Builder, name string, t sema.Type, value *sema.ConstValue) string {
	_ = out
	_ = name
	t = sema.TrueType(t)
	var render strings.Builder

	if t.IsBaseType() {
		b := t.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeString:
			render.WriteString("'" + emit.EscapeString(value.String()) + "'")
		case sema.TypeBool:
			if value.Integer() > 0 {
				render.WriteString("true")
			} else {
				render.WriteString("false")
			}
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			render.WriteString(strconv.FormatInt(value.Integer(), 10))
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				render.WriteString(strconv.FormatInt(value.Integer(), 10))
			} else {
				render.WriteString(formatDouble(value.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(b.Base()))
		}
	} else if t.IsEnum() {
		render.WriteString(strconv.FormatInt(value.Integer(), 10))
	} else {
		tname := g.tmp("tmp")
		g.printConstValue(&render, tname, t, value, true, false, true)
	}

	return render.String()
}
