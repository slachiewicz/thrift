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

package golang

import (
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// omitInitialization reports whether a field's initialization can be
// left out because its default equals the Go zero value.
func omitInitialization(f *sema.Field) bool {
	value := f.Value()
	if value == nil {
		return true
	}
	typ := sema.TrueType(f.Type())
	if typ.IsBaseType() {
		switch typ.(*sema.BaseType).Base() {
		case sema.TypeVoid:
			throw("")
		case sema.TypeString:
			if typ.IsBinary() {
				return false
			}
			return value.String() == ""
		case sema.TypeBool, sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64:
			return value.Integer() == 0
		case sema.TypeDouble:
			if value.Kind() == sema.CVInteger {
				return value.Integer() == 0
			}
			return value.Double() == 0
		case sema.TypeUUID:
			return false
		default:
			throw("compiler error: unhandled type")
		}
	}
	return false
}

// typeNeedReference reports whether an optional value of the type without
// a default is held through a pointer.
func typeNeedReference(typ sema.Type) bool {
	typ = sema.TrueType(typ)
	if typ.IsMap() || typ.IsSet() || typ.IsList() || typ.IsStruct() || typ.IsXception() || typ.IsBinary() {
		return false
	}
	return true
}

func indexableGoExpr(expr string) string {
	if expr != "" && expr[0] == '*' {
		return "(" + expr + ")"
	}
	return expr
}

// isPointerField is t_go_generator::is_pointer_field.
func isPointerField(f *sema.Field) bool {
	if f.Annotations().Has("cpp.ref") {
		return true
	}
	typ := sema.TrueType(f.Type())
	if typ.IsStruct() || typ.IsXception() {
		return true
	}
	if f.Req() != sema.Optional {
		return false
	}
	hasDefault := f.Value() != nil
	switch {
	case typ.IsBaseType():
		switch typ.(*sema.BaseType).Base() {
		case sema.TypeVoid:
			throw("")
		case sema.TypeString:
			if typ.IsBinary() {
				return false
			}
			return !hasDefault
		case sema.TypeBool, sema.TypeI8, sema.TypeI16, sema.TypeI32, sema.TypeI64, sema.TypeDouble, sema.TypeUUID:
			return !hasDefault
		}
	case typ.IsEnum():
		return !hasDefault
	case typ.IsStruct() || typ.IsXception():
		return true
	case typ.IsMap(), typ.IsSet(), typ.IsList(), typ.IsTypedef():
		return hasDefault
	}
	throw("INVALID TYPE IN type_to_go_type: %s", typ.Name())
	return false
}

// typeName is t_go_generator::type_name.
func (g *Generator) typeName(t sema.Type) string {
	if module := g.moduleName(t); module != "" {
		return module + "." + t.Name()
	}
	return t.Name()
}

// moduleName returns the import identifier of the program that declares
// the type, or empty for the program being generated.
func (g *Generator) moduleName(t sema.Type) string {
	program := t.Program()
	if program != nil && program != g.program {
		if program.Namespace("go") == "" || g.program.Namespace("go") == "" ||
			program.Namespace("go") != g.program.Namespace("go") {
			module := g.realGoModule(program)
			if module == g.realGoModule(g.program) {
				return ""
			}
			return g.packageIdentifiers[module]
		}
	}
	return ""
}

// typeToEnum is t_go_generator::type_to_enum.
func (g *Generator) typeToEnum(t sema.Type) string {
	t = sema.TrueType(t)
	switch {
	case t.IsBaseType():
		switch t.(*sema.BaseType).Base() {
		case sema.TypeVoid:
			throw("NO T_VOID CONSTRUCT")
		case sema.TypeString:
			return "thrift.STRING"
		case sema.TypeBool:
			return "thrift.BOOL"
		case sema.TypeI8:
			return "thrift.BYTE"
		case sema.TypeI16:
			return "thrift.I16"
		case sema.TypeI32:
			return "thrift.I32"
		case sema.TypeI64:
			return "thrift.I64"
		case sema.TypeDouble:
			return "thrift.DOUBLE"
		case sema.TypeUUID:
			return "thrift.UUID"
		}
	case t.IsEnum():
		return "thrift.I32"
	case t.IsStruct() || t.IsXception():
		return "thrift.STRUCT"
	case t.IsMap():
		return "thrift.MAP"
	case t.IsSet():
		return "thrift.SET"
	case t.IsList():
		return "thrift.LIST"
	}
	throw("INVALID TYPE IN type_to_enum: %s", t.Name())
	return ""
}

// isContainerKeyedMap reports whether the map is generated as an entry
// slice rather than a Go map.
func (g *Generator) isContainerKeyedMap(t sema.Type) bool {
	tt := sema.TrueType(t)
	if !tt.IsMap() {
		return false
	}
	kt := sema.TrueType(tt.(*sema.Map).KeyType())
	if kt.IsMap() || kt.IsList() || kt.IsSet() {
		return true
	}
	return g.opts.StructKeyEntries && (kt.IsStruct() || kt.IsXception())
}

func (g *Generator) mapEntryType(m *sema.Map) string {
	return "thrift.MapEntry[" + g.mapEntryKeyType(m.KeyType()) + ", " + g.typeToGoType(m.ValType()) + "]"
}

func (g *Generator) isComparableStructKey(kt sema.Type) bool {
	tt := sema.TrueType(kt)
	if !tt.IsStruct() && !tt.IsXception() {
		return false
	}
	for _, m := range tt.(*sema.Struct).Members() {
		if isPointerField(m) {
			return false
		}
		ft := sema.TrueType(m.Type())
		if ft.IsEnum() {
			continue
		}
		if !ft.IsBaseType() || ft.IsBinary() {
			return false
		}
	}
	return true
}

func (g *Generator) mapEntryKeyType(kt sema.Type) string {
	tt := sema.TrueType(kt)
	if tt.IsStruct() || tt.IsXception() {
		return g.typeToGoType(tt)
	}
	return g.typeToGoType(kt)
}

func (g *Generator) typeToGoKeyType(t sema.Type) string {
	resolved := sema.TrueType(t)
	if resolved.IsMap() || resolved.IsList() || resolved.IsSet() {
		throw("Cannot produce a valid type for a Go map key: %s - aborting.", g.typeToGoType(t))
	}
	if resolved.IsBinary() {
		return "string"
	}
	return g.typeToGoType(t)
}

func (g *Generator) typeToGoType(t sema.Type) string {
	return g.typeToGoTypeWithOpt(t, false)
}

// typeToGoTypeWithOpt is type_to_go_type_with_opt.
func (g *Generator) typeToGoTypeWithOpt(t sema.Type, optionalField bool) string {
	maybePointer := ""
	if optionalField {
		maybePointer = "*"
	}
	if td, ok := t.(*sema.Typedef); ok && td.IsForwardTypedef() {
		t = sema.TrueType(t)
	}
	switch {
	case t.IsBaseType():
		switch t.(*sema.BaseType).Base() {
		case sema.TypeVoid:
			throw("")
		case sema.TypeString:
			if t.IsBinary() {
				return maybePointer + "[]byte"
			}
			return maybePointer + "string"
		case sema.TypeBool:
			return maybePointer + "bool"
		case sema.TypeI8:
			return maybePointer + "int8"
		case sema.TypeI16:
			return maybePointer + "int16"
		case sema.TypeI32:
			return maybePointer + "int32"
		case sema.TypeI64:
			return maybePointer + "int64"
		case sema.TypeDouble:
			return maybePointer + "float64"
		case sema.TypeUUID:
			return maybePointer + "thrift.Tuuid"
		}
	case t.IsEnum():
		return maybePointer + g.publicize(g.typeName(t))
	case t.IsStruct() || t.IsXception():
		return "*" + g.publicize(g.typeName(t))
	case t.IsMap():
		m := t.(*sema.Map)
		if g.isContainerKeyedMap(m) {
			return maybePointer + "[]" + g.mapEntryType(m)
		}
		keyType := g.typeToGoKeyType(m.KeyType())
		valueType := g.typeToGoType(m.ValType())
		return maybePointer + "map[" + keyType + "]" + valueType
	case t.IsSet():
		return maybePointer + "[]" + g.typeToGoType(t.(*sema.Set).ElemType())
	case t.IsList():
		return maybePointer + "[]" + g.typeToGoType(t.(*sema.List).ElemType())
	case t.IsTypedef():
		return maybePointer + g.publicize(g.typeName(t))
	}
	throw("INVALID TYPE IN type_to_go_type: %s", t.Name())
	return ""
}

// functionSignatureIf renders an interface method signature.
func (g *Generator) functionSignatureIf(f *sema.Function, prefix string, addError bool) string {
	signature := g.publicize(prefix+f.Name()) + "("
	signature += "ctx context.Context"
	if len(f.Arglist().Members()) != 0 {
		signature += ", " + g.argumentList(f.Arglist())
	}
	signature += ") ("
	ret := f.ReturnType()
	errs := g.argumentList(f.Xceptions())
	if !ret.IsVoid() {
		signature += "_r " + g.typeToGoType(ret)
		if addError || len(errs) == 0 {
			signature += ", "
		}
	}
	if addError {
		signature += "_err error"
	}
	signature += ")"
	return signature
}

func (g *Generator) argumentList(s *sema.Struct) string {
	var parts []string
	for _, f := range s.Members() {
		parts = append(parts, variableNameToGoName(f.Name())+" "+g.typeToGoType(f.Type()))
	}
	return strings.Join(parts, ", ")
}
