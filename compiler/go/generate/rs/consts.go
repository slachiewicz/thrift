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

package rs

import (
	"strconv"

	"github.com/apache/thrift/compiler/go/sema"
)

//-----------------------------------------------------------------------------
//
// Consts
//
// NOTE: consider using macros to generate constants
//
//-----------------------------------------------------------------------------

// generateConst is t_rs_generator::generate_const. This is worse than it
// should be because constants aren't (sensibly) limited to scalar types.
func (g *generator) generateConst(c *sema.Const) {
	name := c.Name()
	t := c.Type()
	v := c.Value()

	if canGenerateSimpleConst(t) {
		g.renderConstValueNamed(name, t, v)
	} else if canGenerateConstHolder(t) {
		g.renderConstValueHolder(name, t, v)
	} else {
		throwf("cannot generate const for %s", name)
	}
}

// renderConstValueNamed is the (name, type, value) overload of
// render_const_value.
func (g *generator) renderConstValueNamed(name string, t sema.Type, v *sema.ConstValue) {
	if !canGenerateSimpleConst(t) {
		throwf("cannot generate simple rust constant for %s", t.Name())
	}

	g.indentRaw("pub const " + rustUpperCase(name) + ": " + g.toRustConstType(t) + " = ")
	g.renderConstValue(t, v, false, true)
	g.wl(";")
	g.wl("")
}

// renderConstValueHolder is t_rs_generator::render_const_value_holder.
func (g *generator) renderConstValueHolder(name string, t sema.Type, v *sema.ConstValue) {
	if !canGenerateConstHolder(t) {
		throwf("cannot generate constant holder for %s", t.Name())
	}

	holderName := "Const" + rustCamelCase(name)

	g.line("pub struct " + holderName + ";")
	g.line("impl " + holderName + " {")
	g.up()

	g.line("pub fn const_value() -> " + g.toRustType(t) + " {")
	g.up()
	g.renderConstValue(t, v, true, false)
	g.down()
	g.line("}")

	g.down()
	g.line("}")
	g.wl("")
}

// renderConstValue is the (type, value, isOwned, isInline) overload of
// render_const_value.
func (g *generator) renderConstValue(t sema.Type, v *sema.ConstValue, isOwned, isInline bool) {
	if !isInline {
		g.raw(g.ind())
	}

	switch {
	case t.IsBaseType():
		b := t.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeString:
			if b.IsBinary() {
				if isOwned {
					g.raw("\"" + v.String() + "\"" + ".to_owned().into_bytes()")
				} else {
					g.raw("b\"" + v.String() + "\"")
				}
			} else {
				g.raw("\"" + v.String() + "\"")
				if isOwned {
					g.raw(".to_owned()")
				}
			}
		case sema.TypeUUID:
			g.raw("Uuid::parse_str(\"" + v.String() + "\").unwrap()")
		case sema.TypeBool:
			if v.Integer() != 0 {
				g.raw("true")
			} else {
				g.raw("false")
			}
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			g.raw(strconv.FormatInt(v.Integer(), 10))
		case sema.TypeDouble:
			g.raw("OrderedFloat::from(" + formatDouble(v.Double()) + "_f64)")
		default:
			throwf("cannot generate const value for %s", sema.BaseName(b.Base()))
		}
	case t.IsTypedef():
		g.renderConstValue(sema.TrueType(t), v, isOwned, true)
	case t.IsEnum():
		g.wl("{")
		g.up()
		g.line(g.toRustType(t) + "::from(" + strconv.FormatInt(v.Integer(), 10) + ")")
		g.down()
		g.indentRaw("}")
	case t.IsStruct(), t.IsXception():
		g.renderConstStruct(t, v)
	case t.IsContainer():
		// all of them use vec! or from(), extra block is no longer needed
		switch {
		case t.IsList():
			g.renderConstList(t, v)
		case t.IsSet():
			g.renderConstSet(t, v)
		case t.IsMap():
			g.renderConstMap(t, v)
		default:
			throwf("cannot generate const container value for %s", t.Name())
		}
	default:
		throwf("cannot generate const value for %s", t.Name())
	}

	if !isInline {
		g.wl("")
	}
}

// renderConstStruct is t_rs_generator::render_const_struct. Struct
// constants are not implemented for Rust: both the union and non-union
// branches of the C++ code render the same "unimplemented!()" body.
func (g *generator) renderConstStruct(t sema.Type, v *sema.ConstValue) {
	_ = v
	g.wl("{")
	g.up()
	g.line("unimplemented!()")
	g.down()
	g.indentRaw("}")
}

// renderConstList is t_rs_generator::render_const_list.
func (g *generator) renderConstList(t sema.Type, v *sema.ConstValue) {
	elemType := t.(*sema.List).ElemType()
	g.wl("vec![")
	g.up()
	for _, e := range v.List() {
		g.raw(g.ind())
		g.renderConstValue(elemType, e, true, true)
		g.wl(",")
	}
	g.down()
	g.indentRaw("]")
}

// renderConstSet is t_rs_generator::render_const_set.
func (g *generator) renderConstSet(t sema.Type, v *sema.ConstValue) {
	elemType := t.(*sema.Set).ElemType()
	g.wl("BTreeSet::from([")
	g.up()
	for _, e := range v.List() {
		g.raw(g.ind())
		g.renderConstValue(elemType, e, true, true)
		g.wl(",")
	}
	g.down()
	g.indentRaw("])")
}

// renderConstMap is t_rs_generator::render_const_map.
func (g *generator) renderConstMap(t sema.Type, v *sema.ConstValue) {
	m := t.(*sema.Map)
	keyType, valType := m.KeyType(), m.ValType()
	g.wl("BTreeMap::from([")
	g.up()
	for _, e := range v.Map() {
		g.line("(")
		g.up()
		g.raw(g.ind())
		g.renderConstValue(keyType, e.Key, true, true)
		g.wl(",")
		g.raw(g.ind())
		g.renderConstValue(valType, e.Value, true, true)
		g.wl(",")
		g.down()
		g.line("),")
	}
	g.down()
	g.indentRaw("])")
}

//-----------------------------------------------------------------------------
//
// Typedefs
//
//-----------------------------------------------------------------------------

// generateTypedef is t_rs_generator::generate_typedef.
func (g *generator) generateTypedef(t *sema.Typedef) {
	actualType := g.toRustType(t.Type())
	g.wl("pub type " + rustSafeName(t.Symbolic()) + " = " + actualType + ";")
	g.wl("")
}

//-----------------------------------------------------------------------------
//
// Enums
//
//-----------------------------------------------------------------------------

// generateEnum is t_rs_generator::generate_enum.
func (g *generator) generateEnum(e *sema.Enum) {
	enumName := rustCamelCase(e.Name())
	g.renderEnumDefinition(e, enumName)
	g.renderEnumImpl(e, enumName)
	g.renderEnumConversion(e, enumName)
}

func (g *generator) renderEnumDefinition(e *sema.Enum, enumName string) {
	g.renderRustdoc(e)
	g.wl("#[derive(Copy, Clone, Debug, Eq, Hash, Ord, PartialEq, PartialOrd)]")
	g.wl("pub struct " + enumName + "(pub i32);")
	g.wl("")
}

func (g *generator) renderEnumImpl(e *sema.Enum, enumName string) {
	g.wl("impl " + enumName + " {")
	g.up()

	constants := e.Constants()

	// associated constants for each IDL-defined enum variant
	for _, val := range constants {
		g.renderRustdoc(val)
		g.line("pub const " + rustEnumVariantName(val.Name()) + ": " + enumName + " = " + enumName + "(" + strconv.FormatInt(int64(val.Value()), 10) + ");")
	}

	// array containing all IDL-defined enum variants
	g.line("pub const ENUM_VALUES: &'static [Self] = &[")
	g.up()
	for _, val := range constants {
		g.line("Self::" + rustEnumVariantName(val.Name()) + ",")
	}
	g.down()
	g.line("];")

	g.down()
	g.wl("}")
	g.wl("")

	g.wl("impl TSerializable for " + enumName + " {")
	g.up()

	g.line("#[allow(clippy::trivially_copy_pass_by_ref)]")
	g.line("fn write_to_out_protocol(&self, o_prot: &mut dyn TOutputProtocol) -> thrift::Result<()> {")
	g.up()
	g.line("o_prot.write_i32(self.0)")
	g.down()
	g.line("}")

	g.line("fn read_from_in_protocol(i_prot: &mut dyn TInputProtocol) -> thrift::Result<" + enumName + "> {")
	g.up()
	g.line("let enum_value = i_prot.read_i32()?;")
	g.line("Ok(" + enumName + "::from(enum_value))")
	g.down()
	g.line("}")

	g.down()
	g.wl("}")
	g.wl("")
}

func (g *generator) renderEnumConversion(e *sema.Enum, enumName string) {
	constants := e.Constants()

	// From trait: i32 -> ENUM_TYPE
	g.wl("impl From<i32> for " + enumName + " {")
	g.up()
	g.line("fn from(i: i32) -> Self {")
	g.up()
	g.line("match i {")
	g.up()
	for _, val := range constants {
		g.line(strconv.FormatInt(int64(val.Value()), 10) + " => " + enumName + "::" + rustEnumVariantName(val.Name()) + ",")
	}
	g.line("_ => " + enumName + "(i)")
	g.down()
	g.line("}")
	g.down()
	g.line("}")
	g.down()
	g.wl("}")
	g.wl("")

	// From trait: &i32 -> ENUM_TYPE
	g.wl("impl From<&i32> for " + enumName + " {")
	g.up()
	g.line("fn from(i: &i32) -> Self {")
	g.up()
	g.line(enumName + "::from(*i)")
	g.down()
	g.line("}")
	g.down()
	g.wl("}")
	g.wl("")

	// From trait: ENUM_TYPE -> int
	g.wl("impl From<" + enumName + "> for i32 {")
	g.up()
	g.line("fn from(e: " + enumName + ") -> i32 {")
	g.up()
	g.line("e.0")
	g.down()
	g.line("}")
	g.down()
	g.wl("}")
	g.wl("")

	// From trait: &ENUM_TYPE -> int
	g.wl("impl From<&" + enumName + "> for i32 {")
	g.up()
	g.line("fn from(e: &" + enumName + ") -> i32 {")
	g.up()
	g.line("e.0")
	g.down()
	g.line("}")
	g.down()
	g.wl("}")
	g.wl("")
}
