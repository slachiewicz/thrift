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

package haxe

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateConsts is t_haxe_generator::generate_consts: generates a class
// that holds all the constants.
func (g *generator) generateConsts(consts []*sema.Const) {
	if len(consts) == 0 {
		return
	}

	fConstsName := g.packageDir + "/" + getCapName(g.programName) + "Constants.hx"
	var f strings.Builder

	// Print header
	f.WriteString(g.autogenComment() + g.haxePackage() + ";\n\n")

	f.WriteString("\n")

	f.WriteString(g.haxeTypeImports())

	g.generateRttiDecoration(&f)
	g.generateMacroDecoration(&f)
	g.indentRaw(&f, "class "+getCapName(g.programName)+"Constants {\n\n")
	g.indentUp()
	for _, c := range consts {
		g.printConstValue(&f, c.Name(), c.Type(), c.Value())
	}
	g.indentDown()
	g.line(&f, "}")
	emit.WriteFile(fConstsName, f.String())
}

// printConstValue is t_haxe_generator::print_const_value.
func (g *generator) printConstValue(out *strings.Builder, name string, t sema.Type, value *sema.ConstValue) {
	t = sema.TrueType(t)
	complex := t.IsStruct() || t.IsXception() || t.IsMap() || t.IsList() || t.IsSet()

	out.WriteString(g.indent())

	out.WriteString("public static ")
	if !complex {
		out.WriteString("inline ")
	}
	out.WriteString("var " + name)
	if complex {
		out.WriteString(" (default,null)")
	}
	out.WriteString(" : " + getCapName(g.typeName(t)) + " = ")
	g.renderConstValue(out, t, value)
	out.WriteString(";\n\n")
}

// renderConstValueStr is t_haxe_generator::render_const_value_str.
func (g *generator) renderConstValueStr(t sema.Type, value *sema.ConstValue) string {
	var render strings.Builder
	g.renderConstValue(&render, t, value)
	return render.String()
}

// renderConstValue is t_haxe_generator::render_const_value.
func (g *generator) renderConstValue(out *strings.Builder, t sema.Type, value *sema.ConstValue) {
	t = sema.TrueType(t)

	switch {
	case t.IsBaseType():
		switch t.(*sema.BaseType).Base() {
		case sema.TypeString, sema.TypeUUID:
			out.WriteString("\"" + emit.EscapeString(value.String()) + "\"")
		case sema.TypeBool:
			if value.Integer() > 0 {
				out.WriteString("true")
			} else {
				out.WriteString("false")
			}
		case sema.TypeI8:
			out.WriteString("(byte)" + strconv.FormatInt(value.Integer(), 10))
		case sema.TypeI16:
			out.WriteString("(short)" + strconv.FormatInt(value.Integer(), 10))
		case sema.TypeI32:
			out.WriteString(strconv.FormatInt(value.Integer(), 10))
		case sema.TypeI64:
			out.WriteString(strconv.FormatInt(value.Integer(), 10) + "L")
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				out.WriteString("(double)" + strconv.FormatInt(value.Integer(), 10))
			} else {
				out.WriteString(formatDouble(value.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(t.(*sema.BaseType).Base()))
		}
	case t.IsEnum():
		out.WriteString(strconv.FormatInt(value.Integer(), 10))
	case t.IsStruct() || t.IsXception():
		g.renderStructInitializer(out, t.(*sema.Struct), value)
	case t.IsMap():
		g.renderMapInitializer(out, t.(*sema.Map), value)
	case t.IsList():
		g.renderListInitializer(out, t.(*sema.List), value)
	case t.IsSet():
		g.renderSetInitializer(out, t.(*sema.Set), value)
	default:
		emit.Throw("compiler error: no const of type %s", t.Name())
	}
}

// renderStructInitializer is t_haxe_generator::render_struct_initializer.
func (g *generator) renderStructInitializer(out *strings.Builder, t *sema.Struct, value *sema.ConstValue) {
	out.WriteString("(function() : " + getCapName(g.typeName(t)) + " {\n")
	g.indentUp()
	g.line(out, "var tmp = new "+getCapName(g.typeName(t))+"();")

	fields := t.Members()
	for _, e := range value.Map() {
		var fieldType sema.Type
		for _, f := range fields {
			if f.Name() == e.Key.String() {
				fieldType = f.Type()
				break
			}
		}
		if fieldType == nil {
			emit.Throw("type error: %s has no field %s", t.Name(), e.Key.String())
		}
		g.indentRaw(out, "tmp."+e.Key.String()+" = ")
		g.renderConstValue(out, fieldType, e.Value)
		out.WriteString(";\n")
	}

	g.line(out, "return tmp;")
	g.indentDown()
	g.indentRaw(out, "})()") // no line break
}

// renderMapInitializer is t_haxe_generator::render_map_initializer.
func (g *generator) renderMapInitializer(out *strings.Builder, t *sema.Map, value *sema.ConstValue) {
	out.WriteString("(function() : " + getCapName(g.typeName(t)) + " {\n")
	g.indentUp()
	g.line(out, "var tmp = new "+getCapName(g.typeName(t))+"();")

	keyType := t.KeyType()
	valType := t.ValType()

	for _, e := range value.Map() {
		g.indentRaw(out, "tmp.set(")
		g.renderConstValue(out, keyType, e.Key)
		out.WriteString(", ")
		g.renderConstValue(out, valType, e.Value)
		out.WriteString(");\n")
	}

	g.line(out, "return tmp;")
	g.indentDown()
	g.indentRaw(out, "})()") // no line break
}

// renderListInitializer is t_haxe_generator::render_list_initializer.
func (g *generator) renderListInitializer(out *strings.Builder, t *sema.List, value *sema.ConstValue) {
	out.WriteString("(function() : " + getCapName(g.typeName(t)) + " {\n")
	g.indentUp()
	g.line(out, "var tmp = new "+getCapName(g.typeName(t))+"();")

	elmType := t.ElemType()

	for _, v := range value.List() {
		g.indentRaw(out, "tmp.add(")
		g.renderConstValue(out, elmType, v)
		out.WriteString(");\n")
	}

	g.line(out, "return tmp;")
	g.indentDown()
	g.indentRaw(out, "})()") // no line break
}

// renderSetInitializer is t_haxe_generator::render_set_initializer.
func (g *generator) renderSetInitializer(out *strings.Builder, t *sema.Set, value *sema.ConstValue) {
	out.WriteString("(function() : " + getCapName(g.typeName(t)) + " {\n")
	g.indentUp()
	g.line(out, "var tmp = new "+getCapName(g.typeName(t))+"();")

	elmType := t.ElemType()

	for _, v := range value.List() {
		g.indentRaw(out, "tmp.add(")
		g.renderConstValue(out, elmType, v)
		out.WriteString(");\n")
	}

	g.line(out, "return tmp;")
	g.indentDown()
	g.indentRaw(out, "})()") // no line break
}
