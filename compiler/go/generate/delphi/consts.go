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

package delphi

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// makeConstantsClassname is t_delphi_generator::make_constants_classname.
func (g *Generator) makeConstantsClassname() string {
	if g.opts.ConstPrefix {
		return makeValidDelphiIdentifier("T" + g.programName + "Constants")
	}
	return "TConstants"
}

// constNeedsVar is t_delphi_generator::const_needs_var.
func constNeedsVar(t sema.Type) bool {
	truetype := sema.TrueType(t)
	if !truetype.IsBaseType() {
		return true
	}
	switch truetype.(*sema.BaseType).Base() {
	case sema.TypeUUID:
		return true
	case sema.TypeString:
		return truetype.IsBinary()
	default:
		return false
	}
}

// generateConsts is t_delphi_generator::generate_consts.
func (g *Generator) generateConsts(consts []*sema.Const) {
	if len(consts) == 0 {
		return
	}

	g.hasConst = true
	constantsClass := g.makeConstantsClassname()

	g.indentUp()
	g.ln(&g.sConst, constantsClass+" = class")
	g.ln(&g.sConst, "private")
	g.indentUp()
	for _, c := range consts {
		if constNeedsVar(c.Type()) {
			g.printPrivateField(&g.sConst, g.normalizeNameSimple(c.Name()), c.Type())
		}
	}
	g.indentDown()
	g.ln(&g.sConst, "public")
	g.indentUp()
	for _, c := range consts {
		g.generateDelphiDoc(&g.sConst, c)
		g.printConstProp(&g.sConst, g.normalizeNameSimple(c.Name()), c.Type(), c.Value())
	}
	g.ln(&g.sConst, "{$IF CompilerVersion >= 21.0}")
	g.ln(&g.sConst, "class constructor Create;")
	g.ln(&g.sConst, "class destructor Destroy;")
	g.ln(&g.sConst, "{$IFEND}")
	g.indentDown()
	g.ln(&g.sConst, "end;")
	g.raw(&g.sConst, "\n")
	g.indentDown()

	var vars, code strings.Builder

	g.indentUpImpl()
	for _, c := range consts {
		g.initializeField(&vars, &code, g.propName(c.Name(), false, "F"), c.Type(), c.Value(), true)
	}
	g.indentDownImpl()

	g.lnI(&g.sConstImpl, "{$IF CompilerVersion >= 21.0}")
	g.lnI(&g.sConstImpl, "class constructor "+constantsClass+".Create;")

	if vars.Len() > 0 {
		g.lnI(&g.sConstImpl, "var")
		g.raw(&g.sConstImpl, vars.String())
	}
	g.lnI(&g.sConstImpl, "begin")
	if code.Len() > 0 {
		g.raw(&g.sConstImpl, code.String())
	}
	g.lnI(&g.sConstImpl, "end;")
	g.raw(&g.sConstImpl, "\n")
	g.lnI(&g.sConstImpl, "class destructor "+constantsClass+".Destroy;")
	g.lnI(&g.sConstImpl, "begin")
	g.indentUpImpl()
	for _, c := range consts {
		if constNeedsVar(c.Type()) {
			g.finalizeField()
		}
	}
	g.lnI(&g.sConstImpl, "inherited;")
	g.indentDownImpl()
	g.lnI(&g.sConstImpl, "end;")
	g.lnI(&g.sConstImpl, "{$ELSE}")

	vars.Reset()
	code.Reset()

	g.indentUpImpl()
	for _, c := range consts {
		if constNeedsVar(c.Type()) {
			g.initializeField(&vars, &code, constantsClass+"."+g.propName(c.Name(), false, "F"), c.Type(), c.Value(), true)
		}
	}
	g.indentDownImpl()

	g.lnI(&g.sConstImpl, "procedure "+constantsClass+"_Initialize;")
	if vars.Len() > 0 {
		g.lnI(&g.sConstImpl, "var")
		g.raw(&g.sConstImpl, vars.String())
	}
	g.lnI(&g.sConstImpl, "begin")
	if code.Len() > 0 {
		g.raw(&g.sConstImpl, code.String())
	}
	g.lnI(&g.sConstImpl, "end;")
	g.raw(&g.sConstImpl, "\n")

	g.lnI(&g.sConstImpl, "procedure "+constantsClass+"_Finalize;")
	g.lnI(&g.sConstImpl, "begin")
	g.indentUpImpl()
	for range consts {
		g.finalizeField()
	}
	g.indentDownImpl()
	g.lnI(&g.sConstImpl, "end;")
	g.lnI(&g.sConstImpl, "{$IFEND}")
	g.raw(&g.sConstImpl, "\n")
}

// printConstDefValue is t_delphi_generator::print_const_def_value.
func (g *Generator) printConstDefValue(vars, out *strings.Builder, name string, t sema.Type, value *sema.ConstValue, clsNm string) {
	clsPrefix := ""
	if clsNm != "" {
		clsPrefix = clsNm + "."
	}
	nm := g.normalizeNameSimple(name)

	if t.IsStruct() || t.IsXception() {
		s := t.(*sema.Struct)
		for _, entry := range value.Map() {
			var fieldType sema.Type
			for _, f := range s.Members() {
				if f.Name() == entry.Key.String() {
					fieldType = f.Type()
				}
			}
			if fieldType == nil {
				emit.Throw("type error: %s has no field %s", t.Name(), entry.Key.String())
			}
			val := g.renderConstValue(vars, out, fieldType, entry.Value, false)
			g.lnI(out, clsPrefix+nm+"."+g.propName(entry.Key.String(), t.IsXception(), "")+" := "+val+";")
		}
	} else if t.IsMap() {
		m := t.(*sema.Map)
		for _, entry := range value.Map() {
			key := g.renderConstValue(vars, out, m.KeyType(), entry.Key, false)
			val := g.renderConstValue(vars, out, m.ValType(), entry.Value, false)
			g.lnI(out, clsPrefix+nm+"["+key+"] := "+val+";")
		}
	} else if t.IsList() || t.IsSet() {
		var etype sema.Type
		if t.IsList() {
			etype = t.(*sema.List).ElemType()
		} else {
			etype = t.(*sema.Set).ElemType()
		}
		for _, v := range value.List() {
			val := g.renderConstValue(vars, out, etype, v, false)
			g.lnI(out, clsPrefix+nm+".Add("+val+");")
		}
	}
}

// printPrivateField is t_delphi_generator::print_private_field.
func (g *Generator) printPrivateField(out *strings.Builder, name string, t sema.Type) {
	g.ln(out, "class var F"+name+": "+g.typeName(t, false, false)+";")
}

// printConstProp is t_delphi_generator::print_const_prop.
func (g *Generator) printConstProp(out *strings.Builder, name string, t sema.Type, value *sema.ConstValue) {
	if constNeedsVar(t) {
		g.ln(out, "class property "+name+": "+g.typeName(t, false, false)+" read F"+name+";")
	} else {
		var vars strings.Builder
		v2 := g.renderConstValue(&vars, out, t, value, true)
		g.ln(out, "const "+name+" = "+v2+";")
	}
}

// printConstValue is t_delphi_generator::print_const_value.
func (g *Generator) printConstValue(vars, out *strings.Builder, name string, t sema.Type, value *sema.ConstValue, isConstClass bool) {
	truetype := sema.TrueType(t)

	if truetype.IsBaseType() {
		if constNeedsVar(t) || !isConstClass {
			theValue := g.renderConstValue(vars, out, t, value, false)
			g.lnI(out, name+" := "+theValue+";")
		}
	} else if truetype.IsEnum() {
		g.lnI(out, name+" := "+g.typeName(t, false, false)+"."+value.IdentifierName()+";")
	} else {
		typname := g.typeName(truetype, true, false)
		g.lnI(out, name+" := "+typname+".Create;")
		g.printConstDefValue(vars, out, name, truetype, value, "")
	}
}

// initializeField is t_delphi_generator::initialize_field.
func (g *Generator) initializeField(vars, out *strings.Builder, name string, t sema.Type, value *sema.ConstValue, isConstClass bool) {
	g.printConstValue(vars, out, name, t, value, isConstClass)
}

// finalizeField is t_delphi_generator::finalize_field: a no-op in the
// C++ source (every parameter is cast to void).
func (g *Generator) finalizeField() {}

// renderConstValue is t_delphi_generator::render_const_value.
func (g *Generator) renderConstValue(vars, out *strings.Builder, t sema.Type, value *sema.ConstValue, guidAsLiteral bool) string {
	truetype := sema.TrueType(t)

	var render strings.Builder

	if truetype.IsBaseType() {
		switch truetype.(*sema.BaseType).Base() {
		case sema.TypeString:
			if truetype.IsBinary() {
				render.WriteString("TEncoding.UTF8.GetBytes('" + delphiEscapeString(value.String()) + "')")
			} else {
				render.WriteString("'" + delphiEscapeString(value.String()) + "'")
			}
		case sema.TypeUUID:
			if guidAsLiteral {
				render.WriteString("['{" + value.UUID() + "}']")
			} else {
				render.WriteString("StringToGUID('{" + value.UUID() + "}')")
			}
		case sema.TypeBool:
			if value.Integer() > 0 {
				render.WriteString("True")
			} else {
				render.WriteString("False")
			}
		case sema.TypeI8:
			render.WriteString("ShortInt( " + itoa64(value.Integer()) + ")")
		case sema.TypeI16:
			render.WriteString("SmallInt( " + itoa64(value.Integer()) + ")")
		case sema.TypeI32:
			render.WriteString("LongInt( " + itoa64(value.Integer()) + ")")
		case sema.TypeI64:
			render.WriteString("Int64( " + itoa64(value.Integer()) + ")")
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				render.WriteString(itoa64(value.Integer()) + ".0")
			} else {
				render.WriteString(formatDouble(value.Double()))
			}
		default:
			emit.Throw("compiler error: no const of base type %s", sema.BaseName(truetype.(*sema.BaseType).Base()))
		}
	} else if truetype.IsEnum() {
		render.WriteString(g.typeName(t, false, false) + "." + value.IdentifierName())
	} else {
		tvar := g.tmp("tmp")
		vars.WriteString("  " + tvar + " : " + g.typeName(t, false, false) + ";\n")
		g.printConstValue(vars, out, tvar, t, value, true)
		render.WriteString(tvar)
	}

	return render.String()
}
