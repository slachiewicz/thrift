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

package kotlin

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateEnum is t_kotlin_generator::generate_enum. It reproduces a
// quirk in the C++ source: the ostream statement "indent(f_enum);"
// immediately before the member loop writes one indent's worth of
// whitespace with no newline, so the opening brace line ends with two
// trailing spaces before the first member's newline (or, for an empty
// enum, the "indent(f_enum);" inside "if (first)" runs a second time and
// doubles that whitespace before the semicolon).
func (g *Generator) generateEnum(e *sema.Enum) {
	fEnumName := g.packageDir + "/" + e.Name() + ".kt"
	var out strings.Builder
	out.WriteString(autogenComment() + g.kotlinPackage())

	out.WriteString(g.indent() + "enum class " + kotlinSafeName(e.Name()) +
		"(private val value: kotlin.Int) : org.apache.thrift.TEnum {")
	g.indentUp()
	out.WriteString(g.indent())

	first := true
	values := e.Constants()
	for _, v := range values {
		if first {
			out.WriteString("\n")
		} else {
			out.WriteString(",\n")
		}
		first = false
		out.WriteString(g.indent() + v.Name() + "(" + strconv.Itoa(int(v.Value())) + ")")
	}
	if first {
		out.WriteString(g.indent())
	}
	out.WriteString(";\n\n")
	out.WriteString(g.indent() + "override fun getValue() = value\n\n")
	{
		out.WriteString(g.indent() + "companion object {\n")
		g.indentUp()
		{
			out.WriteString(g.indent() + "@kotlin.jvm.JvmStatic\n")
			out.WriteString(g.indent() + "fun findByValue(i: kotlin.Int): " + kotlinSafeName(e.Name()) + "? {\n")
			g.indentUp()
			{
				out.WriteString(g.indent() + "return when (i) {\n")
				g.indentUp()
				{
					for _, v := range values {
						out.WriteString(g.indent() + strconv.Itoa(int(v.Value())) + " -> " + v.Name() + "\n")
					}
					out.WriteString(g.indent() + "else -> null\n")
				}
				g.scopeDown(&out)
			}
			g.scopeDown(&out)
		}
		g.scopeDown(&out)
	}
	g.scopeDown(&out)
	emit.WriteFile(fEnumName, out.String())
}

// generateConsts is t_kotlin_generator::generate_consts. It writes into
// f_types_ directly, like the C++ source, and closeGenerator flushes it.
//
// The is_base_type()/is_enum() checks below run on const_type itself, not
// on get_true_type(const_type) as typeName does internally: a const
// declared through a typedef of a base type or an enum falls through to
// neither branch (the C++ "// TODO" comment) and only "val name: Type =
// \n" is written, with no value. That is reproduced here rather than
// fixed.
func (g *Generator) generateConsts(consts []*sema.Const) {
	for _, c := range consts {
		constType := c.Type()
		if constType.IsBaseType() {
			g.fTypes.WriteString("const ")
		}
		g.fTypes.WriteString("val " + kotlinSafeName(c.Name()) + ": " + g.typeName(constType, false, false, false) + " = ")

		value := c.Value()
		if constType.IsBaseType() {
			switch constType.(*sema.BaseType).Base() {
			case sema.TypeString:
				g.fTypes.WriteString("\"" + value.String() + "\"")
			case sema.TypeBool:
				if value.Integer() > 0 {
					g.fTypes.WriteString("true")
				} else {
					g.fTypes.WriteString("false")
				}
			case sema.TypeI8, sema.TypeI16, sema.TypeI32:
				g.fTypes.WriteString(strconv.FormatInt(value.Integer(), 10))
			case sema.TypeDouble:
				if value.Kind() == sema.CVInteger {
					g.fTypes.WriteString(strconv.FormatInt(value.Integer(), 10) + ".")
				} else {
					g.fTypes.WriteString(emit.DoubleFixed16(value.Double()))
				}
			default:
				g.fTypes.WriteString(strconv.FormatInt(value.Integer(), 10))
			}
		} else if constType.IsEnum() {
			namespacePrefix := constType.Program().Namespace("java")
			if len(namespacePrefix) > 0 {
				namespacePrefix += "."
			}
			g.fTypes.WriteString(namespacePrefix + value.IdentifierWithParent())
		}
		g.fTypes.WriteString("\n")
	}
}

// generateTypedef is t_kotlin_generator::generate_typedef. It also writes
// into f_types_ directly.
func (g *Generator) generateTypedef(t *sema.Typedef) {
	g.fTypes.WriteString("typealias " + t.Symbolic() + " = " + g.typeName(t.Type(), true, false, false) + "\n")
}
