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
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateConsts is t_cpp_generator::generate_consts: a class that holds
// all of the program's constants.
func (g *Generator) generateConsts(consts []*sema.Const) {
	if len(consts) == 0 {
		return
	}
	outDir := g.outDir()
	var fConsts, fConstsImpl strings.Builder

	fConsts.WriteString(g.autogenComment())
	fConstsImpl.WriteString(g.autogenComment())

	fConsts.WriteString("#ifndef " + g.programName + "_CONSTANTS_H\n#define " + g.programName +
		"_CONSTANTS_H\n\n#include \"" + g.getIncludePrefix(g.program) + g.programName + "_types.h\"\n\n" + g.nsOpen + "\n\n")

	fConstsImpl.WriteString("#include \"" + g.getIncludePrefix(g.program) + g.programName + "_constants.h\"\n\n" + g.nsOpen + "\n\n")

	fConsts.WriteString("class " + g.programName + "Constants {\n public:\n  " + g.programName + "Constants();\n\n")
	g.indentUp()
	for _, c := range consts {
		fConsts.WriteString(g.indent() + g.typeName(c.Type(), false, false) + " " + c.Name() + ";\n")
	}
	g.indentDown()
	fConsts.WriteString("};\n")

	fConstsImpl.WriteString("const " + g.programName + "Constants g_" + g.programName + "_constants;\n\n" +
		g.programName + "Constants::" + g.programName + "Constants() {\n")
	g.indentUp()
	for _, c := range consts {
		g.printConstValue(&fConstsImpl, c.Name(), c.Type(), c.Value())
	}
	g.indentDown()
	fConstsImpl.WriteString(g.indent() + "}\n")

	fConsts.WriteString("\nextern const " + g.programName + "Constants g_" + g.programName + "_constants;\n\n" +
		g.nsClose + "\n\n#endif\n")
	writeFile(outDir+g.programName+"_constants.h", fConsts.String())

	fConstsImpl.WriteString("\n" + g.nsClose + "\n\n")
	writeFile(outDir+g.programName+"_constants.cpp", fConstsImpl.String())
}

// printConstValue is t_cpp_generator::print_const_value. Note that type
// checking is NOT performed in this function, as it always runs
// beforehand.
func (g *Generator) printConstValue(out *strings.Builder, name string, t sema.Type, value *sema.ConstValue) {
	t = sema.TrueType(t)
	switch {
	case t.IsBaseType():
		v2 := g.renderConstValue(out, name, t, value)
		out.WriteString(g.indent() + name + " = " + v2 + ";\n\n")
	case t.IsEnum():
		out.WriteString(g.indent() + name + " = static_cast<" + g.typeName(t, false, false) + ">(" +
			strconv.FormatInt(value.Integer(), 10) + ");\n\n")
	case t.IsStruct() || t.IsXception():
		st := t.(*sema.Struct)
		fields := st.Members()
		for _, e := range value.Map() {
			var fieldType sema.Type
			isNonrequired := false
			for _, f := range fields {
				if f.Name() == e.Key.String() {
					fieldType = f.Type()
					isNonrequired = f.Req() != sema.Required
				}
			}
			if fieldType == nil {
				emit.Throw("type error: %s has no field %s", t.Name(), e.Key.String())
			}
			itemVal := g.renderConstValue(out, name, fieldType, e.Value)
			out.WriteString(g.indent() + name + "." + e.Key.String() + " = " + itemVal + ";\n")
			if isNonrequired {
				out.WriteString(g.indent() + name + ".__isset." + e.Key.String() + " = true;\n")
			}
		}
		out.WriteString("\n")
	case t.IsMap():
		m := t.(*sema.Map)
		for _, e := range value.Map() {
			key := g.renderConstValue(out, name, m.KeyType(), e.Key)
			itemVal := g.renderConstValue(out, name, m.ValType(), e.Value)
			out.WriteString(g.indent() + name + ".insert(std::make_pair(" + key + ", " + itemVal + "));\n")
		}
		out.WriteString("\n")
	case t.IsList():
		l := t.(*sema.List)
		for _, v := range value.List() {
			itemVal := g.renderConstValue(out, name, l.ElemType(), v)
			out.WriteString(g.indent() + name + ".push_back(" + itemVal + ");\n")
		}
		out.WriteString("\n")
	case t.IsSet():
		s := t.(*sema.Set)
		for _, v := range value.List() {
			itemVal := g.renderConstValue(out, name, s.ElemType(), v)
			out.WriteString(g.indent() + name + ".insert(" + itemVal + ");\n")
		}
		out.WriteString("\n")
	default:
		emit.Throw("INVALID TYPE IN print_const_value: %s", t.Name())
	}
}

// renderConstValue is t_cpp_generator::render_const_value. When out is
// nil, a struct-, map-, list- or set-typed value renders as the empty
// string; that quirk is reproduced from the C++ generator, which only
// takes that branch when out is non-null.
func (g *Generator) renderConstValue(out *strings.Builder, name string, t sema.Type, value *sema.ConstValue) string {
	_ = name
	var render strings.Builder

	if t.IsBaseType() {
		b := t.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeString:
			render.WriteString(`"` + escapeString(value.String()) + `"`)
		case sema.TypeBool:
			if value.Integer() > 0 {
				render.WriteString("true")
			} else {
				render.WriteString("false")
			}
		case sema.TypeI8, sema.TypeI16, sema.TypeI32:
			render.WriteString(strconv.FormatInt(value.Integer(), 10))
		case sema.TypeI64:
			render.WriteString(strconv.FormatInt(value.Integer(), 10) + "LL")
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				render.WriteString("static_cast<double>(" + strconv.FormatInt(value.Integer(), 10) + ")")
			} else {
				render.WriteString(doubleAsString(value.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(b.Base()))
		}
	} else if t.IsEnum() {
		render.WriteString("static_cast<" + g.typeName(t, false, false) + ">(" + strconv.FormatInt(value.Integer(), 10) + ")")
	} else if out != nil {
		tmpName := g.tmp("tmp")
		out.WriteString(g.indent() + g.typeName(t, false, false) + " " + tmpName + ";\n")
		g.printConstValue(out, tmpName, t, value)
		render.WriteString(tmpName)
	}

	return render.String()
}
