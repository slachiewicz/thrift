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

package ocaml

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateConst is t_ocaml_generator::generate_const.
func (g *Generator) generateConst(tconst *sema.Const) {
	typ := tconst.Type()
	name := decapitalize(tconst.Name())
	value := tconst.Value()

	g.fConsts.WriteString(g.indent() + "let " + name + " = " + g.renderConstValue(typ, value) + "\n" + "\n")
}

// renderConstValue is t_ocaml_generator::render_const_value. Note that type
// checking is NOT performed in this function, as it is always run
// beforehand using sema.ValidateInput. "out.setf(ios::showpoint)" in the
// C++ source applies to the whole function; the one place it actually
// changes anything is the TYPE_DOUBLE base-type case below, so that is
// where formatDoubleShowpoint (rather than a plain %g) is used.
func (g *Generator) renderConstValue(typ sema.Type, value *sema.ConstValue) string {
	typ = sema.TrueType(typ)
	var out strings.Builder

	switch {
	case typ.IsBaseType():
		b := typ.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeString:
			out.WriteString(`"` + emit.EscapeString(value.String()) + `"`)
		case sema.TypeBool:
			if value.Integer() > 0 {
				out.WriteString("true")
			} else {
				out.WriteString("false")
			}
		case sema.TypeI8, sema.TypeI16:
			out.WriteString(strconv.FormatInt(value.Integer(), 10))
		case sema.TypeI32:
			out.WriteString(strconv.FormatInt(value.Integer(), 10) + "l")
		case sema.TypeI64:
			out.WriteString(strconv.FormatInt(value.Integer(), 10) + "L")
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				out.WriteString(strconv.FormatInt(value.Integer(), 10) + ".0")
			} else {
				out.WriteString(formatDoubleShowpoint(value.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(b.Base()))
		}
	case typ.IsEnum():
		e := typ.(*sema.Enum)
		if ev := e.ConstantByValue(value.Integer()); ev != nil {
			out.WriteString(g.indent() + capitalize(e.Name()) + "." + capitalize(ev.Name()))
		}
	case typ.IsStruct() || typ.IsXception():
		s := typ.(*sema.Struct)
		cname := g.typeName(typ)
		ct := g.tmp("_c")
		out.WriteString("\n")
		g.indentUp()
		out.WriteString(g.indent() + "(let " + ct + " = new " + cname + " in" + "\n")
		g.indentUp()
		fields := s.Members()
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
			fname := e.Key.String()
			out.WriteString(g.indent())
			out.WriteString(ct + "#set_" + fname + " ")
			out.WriteString(g.renderConstValue(fieldType, e.Value))
			out.WriteString(";" + "\n")
		}
		out.WriteString(g.indent() + ct + ")")
		g.indentDown()
		g.indentDown()
	case typ.IsMap():
		m := typ.(*sema.Map)
		ktype, vtype := m.KeyType(), m.ValType()
		entries := value.Map()
		hm := g.tmp("_hm")
		out.WriteString("\n")
		g.indentUp()
		out.WriteString(g.indent() + "(let " + hm + " = Hashtbl.create " + strconv.Itoa(len(entries)) + " in" + "\n")
		g.indentUp()
		for _, e := range entries {
			key := g.renderConstValue(ktype, e.Key)
			val := g.renderConstValue(vtype, e.Value)
			out.WriteString(g.indent() + "Hashtbl.add " + hm + " " + key + " " + val + ";" + "\n")
		}
		out.WriteString(g.indent() + hm + ")")
		g.indentDown()
		g.indentDown()
	case typ.IsList():
		l := typ.(*sema.List)
		etype := l.ElemType()
		out.WriteString("[" + "\n")
		g.indentUp()
		for _, v := range value.List() {
			out.WriteString(g.indent())
			out.WriteString(g.renderConstValue(etype, v))
			out.WriteString(";" + "\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "]")
	case typ.IsSet():
		s := typ.(*sema.Set)
		etype := s.ElemType()
		list := value.List()
		hm := g.tmp("_hm")
		out.WriteString(g.indent() + "(let " + hm + " = Hashtbl.create " + strconv.Itoa(len(list)) + " in" + "\n")
		g.indentUp()
		for _, v := range list {
			val := g.renderConstValue(etype, v)
			out.WriteString(g.indent() + "Hashtbl.add " + hm + " " + val + " true;" + "\n")
		}
		out.WriteString(g.indent() + hm + ")" + "\n")
		g.indentDown()
		out.WriteString("\n")
	default:
		emit.Throw("CANNOT GENERATE CONSTANT FOR TYPE: %s", typ.Name())
	}
	return out.String()
}
