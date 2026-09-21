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
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// typeName is t_delphi_generator::type_name.
func (g *Generator) typeName(ttype sema.Type, bCls, bNoPostfix bool) string {
	if ttype.IsTypedef() {
		tdef := ttype.(*sema.Typedef)
		if tdef.IsForwardTypedef() {
			if inner, ok := forwardTypedefTarget(tdef); ok {
				return g.typeName(inner, bCls, bNoPostfix)
			}
			emit.Throw("unresolved forward declaration: %s", tdef.Symbolic())
		}
		return g.normalizeNameSimple("T" + tdef.Symbolic())
	}

	if ttype.IsBaseType() {
		return g.baseTypeName(ttype.(*sema.BaseType))
	} else if ttype.IsEnum() {
		bCls = true
		bNoPostfix = true
	} else if ttype.IsMap() {
		m := ttype.(*sema.Map)
		typNm := "IThriftDictionary"
		if bCls {
			typNm = "TThriftDictionaryImpl"
		}
		return typNm + "<" + g.typeName(m.KeyType(), false, false) + ", " + g.typeName(m.ValType(), false, false) + ">"
	} else if ttype.IsSet() {
		s := ttype.(*sema.Set)
		typNm := "IThriftHashSet"
		if bCls {
			typNm = "TThriftHashSetImpl"
		}
		return typNm + "<" + g.typeName(s.ElemType(), false, false) + ">"
	} else if ttype.IsList() {
		l := ttype.(*sema.List)
		typNm := "IThriftList"
		if bCls {
			typNm = "TThriftListImpl"
		}
		return typNm + "<" + g.typeName(l.ElemType(), false, false) + ">"
	}

	typePrefix := "I"
	if bCls {
		typePrefix = "T"
	}

	nm := g.normalizeClsnm(ttype.Name(), typePrefix)

	if bCls && !bNoPostfix {
		nm += "Impl"
	}

	return nm
}

// forwardTypedefTarget resolves a forward typedef's target the way
// tdef->get_type() != nullptr guards it: the sema Typedef.Type() call
// itself resolves through the scope and fails (panics with *sema.Error)
// if the name was never declared, which corresponds to the C++ nullptr
// branch throwing "unresolved forward declaration".
func forwardTypedefTarget(tdef *sema.Typedef) (t sema.Type, ok bool) {
	defer func() {
		if r := recover(); r != nil {
			ok = false
		}
	}()
	return tdef.Type(), true
}

// baseTypeName is t_delphi_generator::base_type_name.
func (g *Generator) baseTypeName(b *sema.BaseType) string {
	switch b.Base() {
	case sema.TypeVoid:
		return ""
	case sema.TypeString:
		if b.IsBinary() {
			if g.opts.ComTypes {
				return "IThriftBytes"
			}
			if g.opts.RTTI {
				return "Thrift.Protocol.TThriftBytes"
			}
			return "SysUtils.TBytes"
		}
		if g.opts.ComTypes {
			return "System.WideString"
		}
		return "System.UnicodeString"
	case sema.TypeUUID:
		return "System.TGuid"
	case sema.TypeBool:
		return "System.Boolean"
	case sema.TypeI8:
		return "System.ShortInt"
	case sema.TypeI16:
		return "System.SmallInt"
	case sema.TypeI32:
		return "System.Integer"
	case sema.TypeI64:
		return "System.Int64"
	case sema.TypeDouble:
		return "System.Double"
	}
	emit.Throw("compiler error: no Delphi name for base type %s", sema.BaseName(b.Base()))
	return ""
}

// inputArgPrefix is t_delphi_generator::input_arg_prefix. Note that it
// does not resolve typedefs: a typedef of an i32 still falls through to
// the final "const " default, since a *sema.Typedef answers false to
// IsBaseType/IsEnum/IsMap/IsSet/IsList regardless of what it aliases,
// exactly as t_typedef does in C++.
func (g *Generator) inputArgPrefix(ttype sema.Type) string {
	if ttype.IsBaseType() {
		b := ttype.(*sema.BaseType)
		switch b.Base() {
		case sema.TypeString, sema.TypeUUID, sema.TypeI64, sema.TypeDouble:
			return "const "
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeBool, sema.TypeVoid:
			return ""
		default:
			emit.Throw("compiler error: no input_arg_prefix() for base type %s", sema.BaseName(b.Base()))
		}
	} else if ttype.IsEnum() {
		return ""
	} else if ttype.IsMap() || ttype.IsSet() || ttype.IsList() {
		return "const "
	}
	return "const "
}

// declareField is t_delphi_generator::declare_field with is_xception_class
// defaulted to false at most call sites; callers pass it explicitly.
func (g *Generator) declareField(tfield *sema.Field, prefix string, isXceptionClass bool) string {
	ftype := tfield.Type()
	return g.propNameF(tfield, isXceptionClass, prefix) + ": " + g.typeName(ftype, false, true) + ";"
}

// typeToEnum is t_delphi_generator::type_to_enum.
func (g *Generator) typeToEnum(t sema.Type) string {
	t = sema.TrueType(t)
	if t.IsBaseType() {
		switch t.(*sema.BaseType).Base() {
		case sema.TypeVoid:
			emit.Throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "TType.String_"
		case sema.TypeUUID:
			return "TType.Uuid"
		case sema.TypeBool:
			return "TType.Bool_"
		case sema.TypeI8:
			return "TType.Byte_"
		case sema.TypeI16:
			return "TType.I16"
		case sema.TypeI32:
			return "TType.I32"
		case sema.TypeI64:
			return "TType.I64"
		case sema.TypeDouble:
			return "TType.Double_"
		}
	} else if t.IsEnum() {
		return "TType.I32"
	} else if t.IsStruct() || t.IsXception() {
		return "TType.Struct"
	} else if t.IsMap() {
		return "TType.Map"
	} else if t.IsSet() {
		return "TType.Set_"
	} else if t.IsList() {
		return "TType.List"
	}
	emit.Throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}

// emptyValue is t_delphi_generator::empty_value.
func (g *Generator) emptyValue(t sema.Type) string {
	t = sema.TrueType(t)
	if t.IsBaseType() {
		switch t.(*sema.BaseType).Base() {
		case sema.TypeVoid:
			return "0"
		case sema.TypeString:
			if t.IsBinary() {
				return "nil"
			}
			return "''"
		case sema.TypeUUID:
			return "System.TGuid.Empty"
		case sema.TypeBool:
			return "False"
		case sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			return "0"
		case sema.TypeDouble:
			return "0.0"
		}
	} else if t.IsEnum() {
		return "T" + t.Name() + "(0)"
	} else if t.IsStruct() || t.IsXception() || t.IsMap() || t.IsSet() || t.IsList() {
		return "nil"
	}
	emit.Throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}

// isVoid is t_delphi_generator::is_void.
func isVoid(t sema.Type) bool {
	t = sema.TrueType(t)
	return t.IsBaseType() && t.(*sema.BaseType).Base() == sema.TypeVoid
}

// typeCanBeNull is t_delphi_generator::type_can_be_null.
func typeCanBeNull(t sema.Type) bool {
	t = sema.TrueType(t)
	return t.IsContainer() || t.IsStruct() || t.IsXception()
}

// initKnownTypesList is t_delphi_generator::init_known_types_list.
func (g *Generator) initKnownTypesList() {
	g.typesKnown[g.typeName(sema.GlobalString, false, false)] = true
	g.typesKnown[g.typeName(sema.GlobalBinary, false, false)] = true
	g.typesKnown[g.typeName(sema.GlobalUUID, false, false)] = true
	g.typesKnown[g.typeName(sema.GlobalBool, false, false)] = true
	g.typesKnown[g.typeName(sema.GlobalI8, false, false)] = true
	g.typesKnown[g.typeName(sema.GlobalI16, false, false)] = true
	g.typesKnown[g.typeName(sema.GlobalI32, false, false)] = true
	g.typesKnown[g.typeName(sema.GlobalI64, false, false)] = true
	g.typesKnown[g.typeName(sema.GlobalDouble, false, false)] = true
}

// isFullyDefinedType is t_delphi_generator::is_fully_defined_type.
func (g *Generator) isFullyDefinedType(ttype sema.Type) bool {
	if ttype.Program() != nil && ttype.Program() != g.program {
		if ttype.Program().Scope.GetType(ttype.Name()) != nil {
			return true
		}
	}

	if ttype.IsTypedef() {
		return g.typesKnown[g.typeName(ttype, false, false)]
	}

	if ttype.IsBaseType() {
		return g.typesKnown[g.baseTypeName(ttype.(*sema.BaseType))]
	} else if ttype.IsEnum() {
		return true
	} else if ttype.IsMap() {
		m := ttype.(*sema.Map)
		return g.isFullyDefinedType(m.KeyType()) && g.isFullyDefinedType(m.ValType())
	} else if ttype.IsSet() {
		return g.isFullyDefinedType(ttype.(*sema.Set).ElemType())
	} else if ttype.IsList() {
		return g.isFullyDefinedType(ttype.(*sema.List).ElemType())
	}

	return g.typesKnown[g.typeName(ttype, false, false)]
}

// addDefinedType is t_delphi_generator::add_defined_type.
func (g *Generator) addDefinedType(ttype sema.Type) {
	g.typesKnown[g.typeName(ttype, false, false)] = true

	more := true
	for more && len(g.typedefsPending) > 0 {
		more = false
		for i, ttypedef := range g.typedefsPending {
			if g.isFullyDefinedType(ttypedef.Type()) {
				g.typedefsPending = append(append([]*sema.Typedef{}, g.typedefsPending[:i]...), g.typedefsPending[i+1:]...)
				g.generateTypedef(ttypedef)
				more = true
				break
			}
		}
	}
}
