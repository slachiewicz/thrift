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

package php

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateTypedef generates a typedef. This is not done in PHP, types
// are all implicit.
func (g *Generator) generateTypedef(t *sema.Typedef) { _ = t }

// generateEnum generates code for an enumerated type. Since define is
// expensive to lookup in PHP, we use a global array for this.
func (g *Generator) generateEnum(e *sema.Enum) {
	var out *strings.Builder
	if g.opts.Classmap {
		out = &g.fTypes
	} else {
		out = &strings.Builder{}
		g.generateProgramHeader(out, nil)
	}

	// We're also doing it this way to see how it performs. It's more
	// legible code but you can't do things like an 'extract' on it,
	// which is a bit of a downer.
	g.phpDoc(out, e)
	out.WriteString("final class " + e.Name() + "\n" + "{\n")
	g.indentUp()

	for _, c := range e.Constants() {
		g.phpDoc(out, c)
		out.WriteString(g.indent() + "public const " + c.Name() + " = " + itoa(int64(c.Value())) + ";\n\n")
	}

	out.WriteString(g.indent() + "public static $names = [\n")
	g.indentUp()
	for _, c := range e.Constants() {
		out.WriteString(g.indent() + itoa(int64(c.Value())) + " => '" + c.Name() + "',\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + "];\n")

	g.indentDown()
	out.WriteString("}\n")

	if !g.opts.Classmap {
		fEnumName := g.packageDir + e.Name() + ".php"
		emit.WriteFile(fEnumName, out.String())
	}
}

// generateConsts generates the constant class. Override of the one from
// t_generator.
func (g *Generator) generateConsts(consts []*sema.Const) {
	if len(consts) == 0 {
		return
	}

	var out *strings.Builder
	if g.opts.Classmap {
		out = &g.fTypes
	} else {
		out = &strings.Builder{}
		g.generateProgramHeader(out, phpcsDisablesSnakeCaseMethods)
	}
	out.WriteString("final class Constant extends \\Thrift\\Type\\TConstant" + "\n" + "{\n")
	g.indentUp()

	// Create static property.
	for _, c := range consts {
		out.WriteString(g.indent() + "protected static $" + c.Name() + ";\n")
	}

	// Create init function.
	for _, c := range consts {
		out.WriteString("\n")
		out.WriteString(g.indent() + "protected static function init_" + c.Name() + "()\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()

		out.WriteString(g.indent() + "return ")
		g.phpDoc(out, c)
		out.WriteString(g.renderConstValue(c.Type(), c.Value()))
		out.WriteString(";\n")

		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}

	g.indentDown()
	out.WriteString("}\n")

	if !g.opts.Classmap {
		fConstsName := g.packageDir + "Constant.php"
		emit.WriteFile(fConstsName, out.String())
	}
}

// renderConstValue prints the value of a constant with the given type.
// Note that type checking is NOT performed in this function as it is
// always run beforehand using the validate_types method in main.cc.
func (g *Generator) renderConstValue(t sema.Type, value *sema.ConstValue) string {
	var out strings.Builder
	t = sema.TrueType(t)
	switch {
	case t.IsBaseType():
		bt := t.(*sema.BaseType)
		switch bt.Base() {
		case sema.TypeString:
			out.WriteString(`"` + getEscapedString(value) + `"`)
		case sema.TypeBool:
			if value.Integer() > 0 {
				out.WriteString("true")
			} else {
				out.WriteString("false")
			}
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			out.WriteString(itoa(value.Integer()))
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				out.WriteString(itoa(value.Integer()))
			} else {
				out.WriteString(formatDouble(value.Double()))
			}
		case sema.TypeUUID:
			out.WriteString(`"` + getEscapedString(value) + `"`)
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(bt.Base()))
		}
	case t.IsEnum():
		out.WriteString(g.indent() + itoa(value.Integer()))
	case t.IsStruct() || t.IsXception():
		s := t.(*sema.Struct)
		out.WriteString("new " + g.phpNamespace(t.Program()) + t.Name() + "([\n")
		g.indentUp()
		fields := s.Members()
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
			out.WriteString(g.indent())
			out.WriteString(g.renderConstValue(sema.GlobalString, entry.Key))
			out.WriteString(" => ")
			out.WriteString(g.renderConstValue(fieldType, entry.Value))
			out.WriteString(",\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "])")
	case t.IsMap():
		m := t.(*sema.Map)
		out.WriteString("[\n")
		g.indentUp()
		for _, entry := range value.Map() {
			out.WriteString(g.indent())
			out.WriteString(g.renderConstValue(m.KeyType(), entry.Key))
			out.WriteString(" => ")
			out.WriteString(g.renderConstValue(m.ValType(), entry.Value))
			out.WriteString(",\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "]")
	case t.IsList() || t.IsSet():
		var elemType sema.Type
		if t.IsList() {
			elemType = t.(*sema.List).ElemType()
		} else {
			elemType = t.(*sema.Set).ElemType()
		}
		out.WriteString("[\n")
		g.indentUp()
		for _, e := range value.List() {
			out.WriteString(g.indent())
			out.WriteString(g.renderConstValue(elemType, e))
			if t.IsSet() {
				out.WriteString(" => true")
			}
			out.WriteString(",\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "]")
	}
	return out.String()
}
