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

package netstd

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

func (g *Generator) generateTypedef(t *sema.Typedef) { _ = t }

// generateEnum is generate_enum(t_enum*): one file per enum.
func (g *Generator) generateEnum(e *sema.Enum) {
	fEnumName := g.namespaceDir + "/" + e.Name() + ".cs"
	var f strings.Builder
	g.generateEnumTo(&f, e)
	emit.WriteFile(fEnumName, f.String())
}

// generateEnumTo is generate_enum(ostream&, t_enum*).
func (g *Generator) generateEnumTo(out *strings.Builder, e *sema.Enum) {
	g.resetIndent()
	out.WriteString(g.autogenComment())
	out.WriteString("using System;\n\n") // needed for Obsolete() attribute

	g.pragmasAndDirectives(out)
	g.startNetstdNamespace(out)
	g.netstdDoc(out, e)

	g.generateDeprecationAttribute(out, e.Annotations())
	out.WriteString(g.indent() + "public enum " + g.typeName(e, false) + "\n")
	g.scopeUp(out)

	for _, c := range e.Constants() {
		g.netstdDoc(out, c)
		g.generateDeprecationAttribute(out, c.Annotations())
		out.WriteString(g.indent() + g.normalizeName(c.Name(), false) + " = " + itoa(int64(c.Value())) + ",\n")
	}

	g.scopeDown(out)
	g.endNetstdNamespace(out)
}

// generateConsts is generate_consts(vector<t_const*>): one file for the
// whole program's constants.
func (g *Generator) generateConsts(consts []*sema.Const) {
	if len(consts) == 0 {
		return
	}
	fConstsName := g.namespaceDir + "/" + g.programName + ".Constants.cs"
	var f strings.Builder
	g.generateConstsTo(&f, consts)
	emit.WriteFile(fConstsName, f.String())
}

// generateConstsTo is generate_consts(ostream&, vector<t_const*>).
func (g *Generator) generateConstsTo(out *strings.Builder, consts []*sema.Const) {
	if len(consts) == 0 {
		return
	}

	g.resetIndent()
	out.WriteString(g.autogenComment() + g.netstdTypeUsings() + "\n\n")

	g.pragmasAndDirectives(out)
	g.startNetstdNamespace(out)

	out.WriteString(g.indent() + "public static class " + makeValidCSharpIdentifier(g.programName) + "Constants\n")

	g.scopeUp(out)

	needStaticConstructor := false
	for _, c := range consts {
		g.netstdDoc(out, c)
		if g.printConstValue(out, g.normalizeName(c.Name(), false), c.Type(), c.Value(), false, false, false) {
			needStaticConstructor = true
		}
	}

	if needStaticConstructor {
		g.printConstConstructor(out, consts)
	}

	g.scopeDown(out)
	g.endNetstdNamespace(out)
}

// printConstDefValue is print_const_def_value.
func (g *Generator) printConstDefValue(out *strings.Builder, name string, t sema.Type, value *sema.ConstValue) {
	if t.IsStruct() || t.IsXception() {
		s := t.(*sema.Struct)
		fields := s.Members()
		g.collectExtensionsTypesStruct(s)
		g.prepareMemberNameMappingStruct(s)

		for _, e := range value.Map() {
			var field *sema.Field
			for _, f := range fields {
				if f.Name() == e.Key.String() {
					field = f
				}
			}
			if field == nil {
				emit.Throw("type error: %s has no field %s", t.Name(), e.Key.String())
			}
			fieldType := field.Type()
			val := g.renderConstValue(out, name, fieldType, e.Value)
			out.WriteString(g.indent() + name + "." + g.propName(field, false) + " = " + val + ";\n")
		}

		g.cleanupMemberNameMapping()
	} else if t.IsMap() {
		m := t.(*sema.Map)
		for _, e := range value.Map() {
			key := g.renderConstValue(out, name, m.KeyType(), e.Key)
			val := g.renderConstValue(out, name, m.ValType(), e.Value)
			out.WriteString(g.indent() + name + "[" + key + "]" + " = " + val + ";\n")
		}
	} else if t.IsList() || t.IsSet() {
		var elemType sema.Type
		if t.IsList() {
			elemType = t.(*sema.List).ElemType()
		} else {
			elemType = t.(*sema.Set).ElemType()
		}
		for _, e := range value.List() {
			val := g.renderConstValue(out, name, elemType, e)
			out.WriteString(g.indent() + name + ".Add(" + val + ");\n")
		}
	}
}

// printConstConstructor is print_const_constructor.
func (g *Generator) printConstConstructor(out *strings.Builder, consts []*sema.Const) {
	out.WriteString(g.indent() + "static " + makeValidCSharpIdentifier(g.programName) + "Constants()\n")
	g.scopeUp(out)

	for _, c := range consts {
		g.printConstDefValue(out, c.Name(), c.Type(), c.Value())
	}
	g.scopeDown(out)
}

// printConstValue is print_const_value. It returns need_static_construction.
func (g *Generator) printConstValue(out *strings.Builder, name string, t sema.Type, value *sema.ConstValue, inStatic, defval, needtype bool) bool {
	out.WriteString(g.indent())
	needStaticConstruction := !inStatic

	t = trueType(t)

	if !defval || needtype {
		switch {
		case inStatic:
			out.WriteString(g.tn(t) + " ")
		case t.IsBaseType():
			bt := t.(*sema.BaseType)
			// binary (byte[]) and uuid (Guid) cannot be C# `const`; use static readonly
			canBeConst := !bt.IsBinary() && bt.Base() != sema.TypeUUID
			if canBeConst {
				out.WriteString("public const " + g.tn(t) + " ")
			} else {
				readonly := ""
				if g.opts.TargetNetVersion >= 6 {
					readonly = "readonly "
				}
				out.WriteString("public static " + readonly + g.tn(t) + " ")
			}
		default:
			readonly := ""
			if g.opts.TargetNetVersion >= 6 {
				readonly = "readonly "
			}
			out.WriteString("public static " + readonly + g.tn(t) + " ")
		}
	}

	switch {
	case t.IsBaseType():
		v2 := g.renderConstValue(out, name, t, value)
		out.WriteString(name + " = " + v2 + ";\n")
		needStaticConstruction = false
	case t.IsEnum():
		out.WriteString(name + " = " + g.tn(t) + "." + value.IdentifierName() + ";\n")
		needStaticConstruction = false
	case t.IsStruct() || t.IsXception():
		if g.opts.TargetNetVersion >= 6 {
			out.WriteString(name + " = new();\n")
		} else {
			out.WriteString(name + " = new " + g.tn(t) + "();\n")
		}
	case t.IsMap() || t.IsList() || t.IsSet():
		switch {
		case g.opts.TargetNetVersion >= 8:
			out.WriteString(name + " = [];\n")
		case g.opts.TargetNetVersion >= 6:
			out.WriteString(name + " = new();\n")
		default:
			out.WriteString(name + " = new " + g.tn(t) + "();\n")
		}
	}

	if defval && !t.IsBaseType() && !t.IsEnum() {
		g.printConstDefValue(out, name, t, value)
	}

	return needStaticConstruction
}

// renderConstValue is render_const_value.
func (g *Generator) renderConstValue(out *strings.Builder, name string, t sema.Type, value *sema.ConstValue) string {
	_ = name
	_ = out

	if t.IsBaseType() {
		switch baseOf(t) {
		case sema.TypeString:
			if t.IsBinary() {
				return `System.Text.Encoding.UTF8.GetBytes("` + escapedString(value) + `")`
			}
			return `"` + escapedString(value) + `"`
		case sema.TypeUUID:
			return `new System.Guid("` + escapedString(value) + `")`
		case sema.TypeBool:
			if value.Integer() > 0 {
				return "true"
			}
			return "false"
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			return strconv.FormatInt(value.Integer(), 10)
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				return strconv.FormatInt(value.Integer(), 10)
			}
			return formatDouble(value.Double())
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(baseOf(t)))
		}
	} else if t.IsEnum() {
		return g.tn(t) + "." + value.IdentifierName()
	} else {
		tname := g.normalizeName(g.tmp("tmp"), false)
		g.printConstValue(out, tname, t, value, true, true, true)
		return tname
	}
	return ""
}
