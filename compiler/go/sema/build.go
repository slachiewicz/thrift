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

package sema

import (
	"fmt"
	"math"

	"github.com/apache/thrift/compiler/go/idl/ast"
)

// builder walks one file's syntax tree in declaration order and performs
// the semantic actions of thrifty.yy, in the same order, against the same
// scope state. Order matters: a constant may only reference constants
// declared before it, a service may only extend a service declared before
// it, and a throws clause may only name exceptions declared before it.
type builder struct {
	p      *Program
	parent *Scope
	prefix string
	diag   *Diagnostics

	strict            int
	allowNegFieldKeys bool
	allow64BitConsts  bool

	fieldVal  int32 // y_field_val
	enumVal   int32 // y_enum_val
	inArglist bool  // g_arglist
}

func (b *builder) warn(line int, level int, format string, args ...interface{}) {
	if b.diag != nil {
		b.diag.Line = line
		b.diag.warn(level, fmt.Sprintf(format, args...))
	}
}

func toAnnotations(list []ast.Annotation) Annotations {
	if len(list) == 0 {
		return nil
	}
	a := Annotations{}
	for _, x := range list {
		a[x.Key] = append(a[x.Key], x.Value)
	}
	return a
}

func (b *builder) build(prog *ast.Program) {
	for _, h := range prog.Headers {
		switch h := h.(type) {
		case *ast.Namespace:
			b.p.SetNamespace(h.Scope, h.Name)
			if h.Scope != "*" && h.Annotations != nil {
				b.p.setNamespaceAnnotations(h.Scope, toAnnotations(h.Annotations))
			}
		case *ast.CppInclude:
			b.p.addCppInclude(h.Path)
		}
	}
	for _, d := range prog.Definitions {
		switch d := d.(type) {
		case *ast.Const:
			b.constDef(d)
		case *ast.Typedef:
			b.typedefDef(d)
		case *ast.Enum:
			b.enumDef(d)
		case *ast.Struct:
			b.structDef(d)
		case *ast.Service:
			b.serviceDef(d)
		}
	}
	if prog.Doc != "" {
		b.p.SetDoc(prog.Doc)
	}
}

// addType is the Definition -> TypeDefinition action.
func (b *builder) addType(t Type) {
	b.p.Scope.AddType(t.Name(), t)
	if b.parent != nil {
		b.parent.AddType(b.prefix+t.Name(), t)
	}
	if !b.p.IsUniqueTypename(t) {
		fail("Type \"%s\" is already defined.", t.Name())
	}
}

func (b *builder) constDef(d *ast.Const) {
	validateSimpleIdentifier(d.Name)
	typ := b.resolveType(d.Type)
	val := b.constValue(d.Value)
	b.p.Scope.ResolveConstValue(val, typ)
	c := NewConst(typ, d.Name, val)
	validateConstType(c)
	b.p.Scope.AddConstant(d.Name, c)
	if b.parent != nil {
		b.parent.AddConstant(b.prefix+d.Name, c)
	}
	b.p.addConst(c)
	if d.Doc != "" {
		c.SetDoc(d.Doc)
	}
}

func (b *builder) typedefDef(d *ast.Typedef) {
	validateSimpleIdentifier(d.Name)
	typ := b.resolveType(d.Type)
	td := NewTypedef(b.p, typ, d.Name)
	td.setAnnotations(toAnnotations(d.Annotations))
	b.p.addTypedef(td)
	b.addType(td)
	if d.Doc != "" {
		td.SetDoc(d.Doc)
	}
}

func (b *builder) enumDef(d *ast.Enum) {
	e := NewEnum(b.p)
	b.enumVal = -1
	for _, v := range d.Values {
		if v.HasValue {
			if v.Value < math.MinInt32 || v.Value > math.MaxInt32 {
				fail("64-bit value supplied for enum %s will be truncated.", v.Name)
			}
			b.enumVal = int32(v.Value)
		} else {
			validateSimpleIdentifier(v.Name)
			if b.enumVal == math.MaxInt32 {
				fail("enum value overflow at enum %s", v.Name)
			}
			b.enumVal++
		}
		ev := &EnumValue{name: v.Name, value: b.enumVal}
		if v.Doc != "" {
			ev.SetDoc(v.Doc)
		}
		ev.annotations = toAnnotations(v.Annotations)
		e.append(ev)
	}
	validateSimpleIdentifier(d.Name)
	e.setName(d.Name)
	e.setAnnotations(toAnnotations(d.Annotations))
	for _, ev := range e.Constants() {
		constName := e.Name() + "." + ev.Name()
		cv := NewIntConstValue(int64(ev.Value()))
		cv.SetEnum(e)
		b.p.Scope.AddConstant(constName, NewConst(GlobalI32, ev.Name(), cv))
		if b.parent != nil {
			b.parent.AddConstant(b.prefix+constName, NewConst(GlobalI32, ev.Name(), cv))
		}
	}
	b.p.addEnum(e)
	b.addType(e)
	if d.Doc != "" {
		e.SetDoc(d.Doc)
	}
}

func (b *builder) structDef(d *ast.Struct) {
	s := NewStruct(b.p)
	b.fieldList(s, d.Fields)
	validateSimpleIdentifier(d.Name)
	if d.Kind == ast.StructException {
		s.SetName(d.Name)
		s.setXception(true)
	} else {
		s.setXsdAll(d.XsdAll)
		s.setUnion(d.Kind == ast.StructUnion)
		s.SetName(d.Name)
	}
	s.setAnnotations(toAnnotations(d.Annotations))
	if d.Kind == ast.StructException {
		b.p.addXception(s)
	} else {
		b.p.addStruct(s)
	}
	b.addType(s)
	if d.Doc != "" {
		s.SetDoc(d.Doc)
	}
}

// fieldList is the FieldList rule: reset the implicit id counter, then
// build and append each field.
func (b *builder) fieldList(s *Struct, fields []*ast.Field) {
	b.fieldVal = -1
	for _, fd := range fields {
		f := b.field(fd)
		if !s.Append(f) {
			fail("\"%d: %s\" - field identifier/name has already been used", f.Key(), f.Name())
		}
	}
}

func (b *builder) field(d *ast.Field) *Field {
	// FieldIdentifier
	var key int32
	autoAssigned := false
	if d.HasID {
		if d.ID <= 0 {
			if b.allowNegFieldKeys {
				if int32(d.ID) != b.fieldVal {
					b.warn(d.Line, 1, "Nonpositive field key (%d) differs from what would be auto-assigned by thrift (%d).", d.ID, b.fieldVal)
				}
				b.fieldVal = int32(d.ID - 1)
				key = int32(d.ID)
			} else {
				b.warn(d.Line, 1, "Nonpositive value (%d) not allowed as a field key.", d.ID)
				key = b.fieldVal
				b.fieldVal--
				autoAssigned = true
			}
		} else {
			key = int32(d.ID)
		}
	} else {
		key = b.fieldVal
		b.fieldVal--
		autoAssigned = true
	}
	if key < math.MinInt16 || key > math.MaxInt16 {
		b.warn(d.Line, 1, "Field key (%d) exceeds allowed range (%d..%d).", key, math.MinInt16, math.MaxInt16)
	}

	// Field
	if autoAssigned {
		b.warn(d.Line, 1, "No field key specified for %s, resulting protocol may have conflicts or not be backwards compatible!", d.Name)
		if b.strict >= 192 {
			fail("Implicit field keys are deprecated and not allowed with -strict")
		}
	}
	typ := b.resolveType(d.Type)
	validateSimpleIdentifier(d.Name)
	f := NewField(typ, d.Name, key)
	f.reference = d.Reference
	switch d.Req {
	case ast.ReqRequired:
		f.req = Required
	case ast.ReqOptional:
		if b.inArglist {
			b.warn(d.Line, 1, "optional keyword is ignored in argument lists.")
			f.req = OptInReqOut
		} else {
			f.req = Optional
		}
	default:
		f.req = OptInReqOut
	}
	if d.Default != nil {
		cv := b.constValue(d.Default)
		b.p.Scope.ResolveConstValue(cv, typ)
		validateFieldValue(f, cv)
		f.value = cv
	}
	f.xsdOptional = d.XsdOptional
	f.xsdNillable = d.XsdNillable
	if d.Doc != "" {
		f.SetDoc(d.Doc)
	}
	if d.HasXsdAttrs {
		attrs := NewStruct(b.p)
		b.fieldList(attrs, d.XsdAttrs)
		f.xsdAttrs = attrs
	}
	f.annotations = toAnnotations(d.Annotations)
	return f
}

func (b *builder) serviceDef(d *ast.Service) {
	var extends *Service
	if d.HasExtends {
		extends = b.p.Scope.GetService(d.Extends)
		if extends == nil {
			fail("Service \"%s\" has not been defined.", d.Extends)
		}
	}
	s := NewService(b.p)
	b.inArglist = true
	for _, fd := range d.Functions {
		s.addFunction(b.function(fd))
	}
	b.inArglist = false
	validateSimpleIdentifier(d.Name)
	s.setName(d.Name)
	s.setExtends(extends)
	s.setAnnotations(toAnnotations(d.Annotations))
	b.p.Scope.AddService(d.Name, s)
	if b.parent != nil {
		b.parent.AddService(b.prefix+d.Name, s)
	}
	b.p.addService(s)
	if !b.p.IsUniqueTypename(s) {
		fail("Type \"%s\" is already defined.", d.Name)
	}
	if d.Doc != "" {
		s.SetDoc(d.Doc)
	}
}

func (b *builder) function(d *ast.Function) *Function {
	var returnType Type = GlobalVoid
	if d.ReturnType != nil {
		returnType = b.resolveType(d.ReturnType)
	}
	args := NewStruct(b.p)
	b.fieldList(args, d.Args)
	throws := NewStruct(b.p)
	if d.HasThrows {
		b.fieldList(throws, d.Throws)
		if !validateThrows(throws) {
			fail("Throws clause may not contain non-exception types")
		}
	}
	validateSimpleIdentifier(d.Name)
	args.SetName(d.Name + "_args")
	f := NewFunction(returnType, d.Name, args, throws, d.Oneway, b.diag)
	if d.Doc != "" {
		f.SetDoc(d.Doc)
	}
	f.annotations = toAnnotations(d.Annotations)
	return f
}

var baseTypes = map[ast.BaseKind]*BaseType{
	ast.BaseVoid:   GlobalVoid,
	ast.BaseString: GlobalString,
	ast.BaseBinary: GlobalBinary,
	ast.BaseUUID:   GlobalUUID,
	ast.BaseBool:   GlobalBool,
	ast.BaseI8:     GlobalI8,
	ast.BaseI16:    GlobalI16,
	ast.BaseI32:    GlobalI32,
	ast.BaseI64:    GlobalI64,
	ast.BaseDouble: GlobalDouble,
}

// resolveType is the FieldType rule. An identifier that is not yet in
// scope becomes a forward typedef that resolves on first use.
func (b *builder) resolveType(ref ast.TypeRef) Type {
	switch r := ref.(type) {
	case *ast.NamedTypeRef:
		if t := b.p.Scope.GetType(r.Name); t != nil {
			return t
		}
		return newForwardTypedef(b.p, r.Name)
	case *ast.BaseTypeRef:
		bt := baseTypes[r.Kind]
		if r.Annotations != nil {
			return bt.withAnnotations(toAnnotations(r.Annotations))
		}
		return bt
	case *ast.MapTypeRef:
		m := NewMap(b.resolveType(r.Key), b.resolveType(r.Value))
		if r.HasCppType {
			m.setCppName(r.CppType)
		}
		if r.Annotations != nil {
			m.setAnnotations(toAnnotations(r.Annotations))
		}
		return m
	case *ast.SetTypeRef:
		s := NewSet(b.resolveType(r.Elem))
		if r.HasCppType {
			s.setCppName(r.CppType)
		}
		if r.Annotations != nil {
			s.setAnnotations(toAnnotations(r.Annotations))
		}
		return s
	case *ast.ListTypeRef:
		elem := b.resolveType(r.Elem)
		// check_for_list_of_bytes looks at the element type as written,
		// without following typedefs, so it never forces a forward
		// reference to resolve.
		if elem.IsBaseType() && elem.(*BaseType).Base() == TypeI8 {
			b.warn(r.Line, 1, "Consider using the more efficient \"binary\" type instead of \"list<byte>\".")
		}
		l := NewList(elem)
		if r.HasCppType {
			l.setCppName(r.CppType)
		}
		if r.HasTrailingCpp {
			l.setCppName(r.TrailingCppType)
			b.warn(r.Line, 1, "The syntax 'list<type> cpp_type \"c++ type\"' is deprecated. Use 'list cpp_type \"c++ type\" <type>' instead.")
		}
		if r.Annotations != nil {
			l.setAnnotations(toAnnotations(r.Annotations))
		}
		return l
	}
	fail("compiler error: unknown type reference")
	return nil
}

// constValue is the ConstValue rule.
func (b *builder) constValue(v *ast.ConstValue) *ConstValue {
	cv := NewConstValue()
	switch v.Kind {
	case ast.ConstInt:
		cv.SetInteger(v.Int)
		if !b.allow64BitConsts && (v.Int < math.MinInt32 || v.Int > math.MaxInt32) {
			b.warn(v.Line, 1, "64-bit constant \"%d\" may not work in all languages.", v.Int)
		}
	case ast.ConstDouble:
		cv.SetDouble(v.Double)
	case ast.ConstString:
		cv.SetString(v.Str)
	case ast.ConstIdentifier:
		cv.SetIdentifier(v.Ident)
	case ast.ConstList:
		cv.SetList()
		for _, e := range v.List {
			cv.AddList(b.constValue(e))
		}
	case ast.ConstMap:
		cv.SetMap()
		for _, e := range v.Map {
			cv.AddMap(b.constValue(e.Key), b.constValue(e.Value))
		}
	}
	return cv
}
