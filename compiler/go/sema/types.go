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

// Package sema resolves and validates Thrift IDL programs.
//
// It reproduces the semantic actions of compiler/cpp/src/thrift/thrifty.yy,
// the lookup rules of parse/t_scope.h, the include handling of main.cc and
// the validation performed before code generation. The model it produces
// mirrors the C++ parse tree class for class, so that a generator ported
// from C++ can call the same accessors: Type.IsStruct corresponds to
// t_type::is_struct, TrueType to get_true_type, and so on.
package sema

import (
	"fmt"
	"sort"
)

// Annotations is a set of type annotations. Keys iterate in sorted order,
// as they do from the std::map the C++ compiler uses; the values of one
// key stay in source order.
type Annotations map[string][]string

// Keys returns the annotation keys in sorted order.
func (a Annotations) Keys() []string {
	keys := make([]string, 0, len(a))
	for k := range a {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Has reports whether the key is present.
func (a Annotations) Has(key string) bool {
	_, ok := a[key]
	return ok
}

// First returns the first value of the key and whether it is present.
func (a Annotations) First(key string) (string, bool) {
	v, ok := a[key]
	if !ok || len(v) == 0 {
		return "", false
	}
	return v[0], true
}

func (a Annotations) clone() Annotations {
	if a == nil {
		return nil
	}
	c := make(Annotations, len(a))
	for k, v := range a {
		c[k] = append([]string(nil), v...)
	}
	return c
}

// docBase is t_doc.
type docBase struct {
	doc    string
	hasDoc bool
}

// Doc returns the doc comment text.
func (d *docBase) Doc() string { return d.doc }

// HasDoc reports whether a doc comment was attached.
func (d *docBase) HasDoc() bool { return d.hasDoc }

// SetDoc attaches a doc comment.
func (d *docBase) SetDoc(doc string) {
	d.doc = doc
	d.hasDoc = true
}

// Type is t_type: the common interface of every Thrift type.
type Type interface {
	Name() string
	Program() *Program
	Annotations() Annotations
	Doc() string
	HasDoc() bool

	IsVoid() bool
	IsBaseType() bool
	IsString() bool
	IsUUID() bool
	IsBinary() bool
	IsBool() bool
	IsTypedef() bool
	IsEnum() bool
	IsStruct() bool
	IsXception() bool
	IsMethodXcepts() bool
	IsContainer() bool
	IsList() bool
	IsSet() bool
	IsMap() bool
	IsService() bool

	// Validate is t_doc::validate, run once before generation.
	Validate()

	setName(string)
	setAnnotations(Annotations)
}

// typeBase holds what t_type holds and answers false to every predicate.
type typeBase struct {
	docBase
	program     *Program
	name        string
	annotations Annotations
}

func (b *typeBase) Name() string             { return b.name }
func (b *typeBase) Program() *Program        { return b.program }
func (b *typeBase) Annotations() Annotations { return b.annotations }
func (b *typeBase) IsVoid() bool             { return false }
func (b *typeBase) IsBaseType() bool         { return false }
func (b *typeBase) IsString() bool           { return false }
func (b *typeBase) IsUUID() bool             { return false }
func (b *typeBase) IsBinary() bool           { return false }
func (b *typeBase) IsBool() bool             { return false }
func (b *typeBase) IsTypedef() bool          { return false }
func (b *typeBase) IsEnum() bool             { return false }
func (b *typeBase) IsStruct() bool           { return false }
func (b *typeBase) IsXception() bool         { return false }
func (b *typeBase) IsMethodXcepts() bool     { return false }
func (b *typeBase) IsContainer() bool        { return false }
func (b *typeBase) IsList() bool             { return false }
func (b *typeBase) IsSet() bool              { return false }
func (b *typeBase) IsMap() bool              { return false }
func (b *typeBase) IsService() bool          { return false }
func (b *typeBase) Validate()                {}
func (b *typeBase) setName(n string)         { b.name = n }
func (b *typeBase) setAnnotations(a Annotations) {
	b.annotations = a
}

// TrueType follows typedefs to the underlying type. It is
// t_type::get_true_type.
func TrueType(t Type) Type {
	for t.IsTypedef() {
		t = t.(*Typedef).Type()
	}
	return t
}

// BaseKind is t_base_type::t_base.
type BaseKind int

// Base type kinds.
const (
	TypeVoid BaseKind = iota
	TypeString
	TypeUUID
	TypeBool
	TypeI8
	TypeI16
	TypeI32
	TypeI64
	TypeDouble
)

// BaseName is t_base_type::t_base_name.
func BaseName(k BaseKind) string {
	switch k {
	case TypeVoid:
		return "void"
	case TypeString:
		return "string"
	case TypeUUID:
		return "uuid"
	case TypeBool:
		return "bool"
	case TypeI8:
		return "i8"
	case TypeI16:
		return "i16"
	case TypeI32:
		return "i32"
	case TypeI64:
		return "i64"
	case TypeDouble:
		return "double"
	}
	return "(unknown)"
}

// BaseType is t_base_type.
type BaseType struct {
	typeBase
	base   BaseKind
	binary bool
}

func newBaseType(name string, k BaseKind) *BaseType {
	return &BaseType{typeBase: typeBase{name: name}, base: k}
}

// The global base types of common.cc. Their names are what the C++
// compiler gives them; note that binary and uuid are both named "string".
var (
	GlobalVoid   = newBaseType("void", TypeVoid)
	GlobalString = newBaseType("string", TypeString)
	GlobalBinary = func() *BaseType { b := newBaseType("string", TypeString); b.binary = true; return b }()
	GlobalUUID   = newBaseType("string", TypeUUID)
	GlobalBool   = newBaseType("bool", TypeBool)
	GlobalI8     = newBaseType("i8", TypeI8)
	GlobalI16    = newBaseType("i16", TypeI16)
	GlobalI32    = newBaseType("i32", TypeI32)
	GlobalI64    = newBaseType("i64", TypeI64)
	GlobalDouble = newBaseType("double", TypeDouble)
)

// Base returns the base kind.
func (b *BaseType) Base() BaseKind { return b.base }

func (b *BaseType) IsVoid() bool     { return b.base == TypeVoid }
func (b *BaseType) IsString() bool   { return b.base == TypeString }
func (b *BaseType) IsBool() bool     { return b.base == TypeBool }
func (b *BaseType) IsUUID() bool     { return b.base == TypeUUID }
func (b *BaseType) IsBinary() bool   { return b.binary && b.base == TypeString }
func (b *BaseType) IsBaseType() bool { return true }

// withAnnotations returns an annotated copy, as the BaseType grammar rule
// does when annotations follow a base type keyword.
func (b *BaseType) withAnnotations(a Annotations) *BaseType {
	c := *b
	c.annotations = a
	return &c
}

// Typedef is t_typedef. A forward typedef stands for a name that had not
// been declared when it was referenced; it resolves through the program
// scope on first use, and fails if the name never gets declared.
type Typedef struct {
	typeBase
	target   Type
	symbolic string
	forward  bool
}

// NewTypedef creates a declared typedef.
func NewTypedef(program *Program, target Type, symbolic string) *Typedef {
	return &Typedef{typeBase: typeBase{program: program, name: symbolic}, target: target, symbolic: symbolic}
}

func newForwardTypedef(program *Program, symbolic string) *Typedef {
	return &Typedef{typeBase: typeBase{program: program, name: symbolic}, symbolic: symbolic, forward: true}
}

// Type returns the aliased type. It is t_typedef::get_type.
func (t *Typedef) Type() Type {
	if t.target == nil {
		typ := t.program.Scope.GetType(t.symbolic)
		if typ == nil {
			fail("Type \"%s\" not defined", t.symbolic)
		}
		return typ
	}
	return t.target
}

// Symbolic returns the typedef's name.
func (t *Typedef) Symbolic() string { return t.symbolic }

// IsForwardTypedef reports whether this is an unresolved reference.
func (t *Typedef) IsForwardTypedef() bool { return t.forward }

func (t *Typedef) IsTypedef() bool { return true }

// EnumValue is t_enum_value.
type EnumValue struct {
	docBase
	name        string
	value       int32
	annotations Annotations
}

func (v *EnumValue) Name() string             { return v.name }
func (v *EnumValue) Value() int32             { return v.value }
func (v *EnumValue) Annotations() Annotations { return v.annotations }
func (v *EnumValue) Validate()                {}

// Enum is t_enum.
type Enum struct {
	typeBase
	constants []*EnumValue
}

// NewEnum creates an empty enum.
func NewEnum(program *Program) *Enum {
	return &Enum{typeBase: typeBase{program: program}}
}

func (e *Enum) append(v *EnumValue) { e.constants = append(e.constants, v) }

// Constants returns the members in declaration order.
func (e *Enum) Constants() []*EnumValue { return e.constants }

// ConstantByName finds a member by name.
func (e *Enum) ConstantByName(name string) *EnumValue {
	for _, v := range e.constants {
		if v.name == name {
			return v
		}
	}
	return nil
}

// ConstantByValue finds the first member with the value.
func (e *Enum) ConstantByValue(value int64) *EnumValue {
	for _, v := range e.constants {
		if int64(v.value) == value {
			return v
		}
	}
	return nil
}

// MinValue returns the member with the smallest value, or nil.
func (e *Enum) MinValue() *EnumValue {
	if len(e.constants) == 0 {
		return nil
	}
	m := e.constants[0]
	for _, v := range e.constants {
		if v.value < m.value {
			m = v
		}
	}
	return m
}

// MaxValue returns the member with the largest value, or nil.
func (e *Enum) MaxValue() *EnumValue {
	if len(e.constants) == 0 {
		return nil
	}
	m := e.constants[len(e.constants)-1]
	for _, v := range e.constants {
		if v.value > m.value {
			m = v
		}
	}
	return m
}

func (e *Enum) IsEnum() bool { return true }

// Requiredness is t_field::e_req.
type Requiredness int

// Requiredness values, in the C++ enum order.
const (
	Required Requiredness = iota
	Optional
	OptInReqOut
)

// Field is t_field.
type Field struct {
	docBase
	typ         Type
	name        string
	key         int32
	req         Requiredness
	value       *ConstValue
	xsdOptional bool
	xsdNillable bool
	xsdAttrs    *Struct
	reference   bool
	annotations Annotations
}

// NewField creates a field.
func NewField(typ Type, name string, key int32) *Field {
	return &Field{typ: typ, name: name, key: key, req: OptInReqOut}
}

func (f *Field) Type() Type               { return f.typ }
func (f *Field) Name() string             { return f.name }
func (f *Field) Key() int32               { return f.key }
func (f *Field) Req() Requiredness        { return f.req }
func (f *Field) SetReq(r Requiredness)    { f.req = r }
func (f *Field) Value() *ConstValue       { return f.value }
func (f *Field) XsdOptional() bool        { return f.xsdOptional }
func (f *Field) XsdNillable() bool        { return f.xsdNillable }
func (f *Field) XsdAttrs() *Struct        { return f.xsdAttrs }
func (f *Field) Reference() bool          { return f.reference }
func (f *Field) Annotations() Annotations { return f.annotations }
func (f *Field) Validate()                {}

// Struct is t_struct. It also represents argument lists, throws lists and
// annotation holders, as in the C++ compiler.
type Struct struct {
	typeBase
	members          []*Field
	membersInIDOrder []*Field
	isXception       bool
	isUnion          bool
	isMethodXcepts   bool
	unionValidated   bool
	xceptsValidated  bool
	membersWithValue int
	xsdAll           bool
}

// NewStruct creates an empty struct owned by the program.
func NewStruct(program *Program) *Struct {
	return &Struct{typeBase: typeBase{program: program}}
}

// SetName renames the struct and re-runs the union member checks, as
// t_struct::set_name does.
func (s *Struct) SetName(name string) {
	s.name = name
	s.unionValidated = false
	s.validateMembers()
}

func (s *Struct) setName(name string) { s.SetName(name) }

func (s *Struct) setXception(v bool) { s.isXception = v }

func (s *Struct) setMethodXcepts(v bool) {
	s.isMethodXcepts = v
	s.xceptsValidated = false
	s.validateMembers()
}

func (s *Struct) setUnion(v bool) {
	s.isUnion = v
	s.unionValidated = false
	s.validateMembers()
}

func (s *Struct) setXsdAll(v bool) { s.xsdAll = v }

// XsdAll reports the xsd_all flag.
func (s *Struct) XsdAll() bool { return s.xsdAll }

// Append adds a field. It returns false when the id or the name is
// already used, like t_struct::append.
func (s *Struct) Append(f *Field) bool {
	pos := sort.Search(len(s.membersInIDOrder), func(i int) bool {
		return s.membersInIDOrder[i].key >= f.key
	})
	if pos < len(s.membersInIDOrder) && s.membersInIDOrder[pos].key == f.key {
		return false
	}
	if s.FieldByName(f.name) != nil {
		return false
	}
	s.members = append(s.members, f)
	s.membersInIDOrder = append(s.membersInIDOrder, nil)
	copy(s.membersInIDOrder[pos+1:], s.membersInIDOrder[pos:])
	s.membersInIDOrder[pos] = f
	if s.needsValidation() {
		s.validateMembers()
	} else {
		s.validateMemberField(f)
	}
	return true
}

// Members returns the fields in declaration order.
func (s *Struct) Members() []*Field { return s.members }

// SortedMembers returns the fields in id order.
func (s *Struct) SortedMembers() []*Field { return s.membersInIDOrder }

func (s *Struct) IsStruct() bool       { return !s.isXception }
func (s *Struct) IsXception() bool     { return s.isXception }
func (s *Struct) IsMethodXcepts() bool { return s.isMethodXcepts }

// IsUnion reports whether the struct was declared with the union keyword.
func (s *Struct) IsUnion() bool { return s.isUnion }

// FieldByName finds a member by name.
func (s *Struct) FieldByName(name string) *Field {
	for _, f := range s.membersInIDOrder {
		if f.name == name {
			return f
		}
	}
	return nil
}

// Validate is t_struct::validate.
func (s *Struct) Validate() {
	what := "struct"
	if s.IsUnion() {
		what = "union"
	}
	if s.IsXception() {
		what = "exception"
	}
	for _, f := range s.members {
		f.typ.Validate()
		if !s.isMethodXcepts {
			if TrueType(f.typ).IsXception() {
				fail("%s %s: exception type \"%s\" cannot be used as member field type %s", what, s.name, f.typ.Name(), f.name)
			}
		}
	}
}

func (s *Struct) validateMemberField(f *Field) {
	s.validateUnionMember(f)
	s.validateMethodExceptionField(f)
}

func (s *Struct) validateUnionMember(f *Field) {
	if s.isUnion && s.name != "" {
		s.unionValidated = true
		if f.req != Optional {
			if f.req != OptInReqOut {
				s.warn("Union %s field %s: union members must be optional, ignoring specified requiredness.", s.name, f.name)
			}
			f.req = Optional
		}
		if f.value != nil {
			s.membersWithValue++
			if s.membersWithValue > 1 {
				fail("Error: Field %s provides another default value for union %s", f.name, s.name)
			}
		}
	}
}

func (s *Struct) validateMethodExceptionField(f *Field) {
	if s.isMethodXcepts {
		s.xceptsValidated = true
		if f.req == Required {
			f.req = OptInReqOut
			s.warn("Exception field %s: \"required\" is illegal here, ignoring.", f.name)
		}
	}
}

func (s *Struct) needsValidation() bool {
	if s.isMethodXcepts {
		return !s.xceptsValidated
	}
	if s.isUnion {
		return !s.unionValidated
	}
	return false
}

func (s *Struct) validateMembers() {
	if s.needsValidation() {
		for _, f := range s.membersInIDOrder {
			s.validateMemberField(f)
		}
	}
}

func (s *Struct) warn(format string, args ...interface{}) {
	if s.program != nil && s.program.diag != nil {
		s.program.diag.warn(1, fmt.Sprintf(format, args...))
	}
}

// Function is t_function.
type Function struct {
	docBase
	returnType  Type
	name        string
	arglist     *Struct
	xceptions   *Struct
	oneway      bool
	annotations Annotations
}

// NewFunction is the five-argument t_function constructor.
func NewFunction(returnType Type, name string, arglist, xceptions *Struct, oneway bool, diag *Diagnostics) *Function {
	f := &Function{returnType: returnType, name: name, arglist: arglist, xceptions: xceptions, oneway: oneway}
	xceptions.setMethodXcepts(true)
	if oneway && len(xceptions.Members()) != 0 {
		fail("Oneway methods can't throw exceptions.")
	}
	if oneway && !returnType.IsVoid() && diag != nil {
		diag.warn(1, "Oneway methods should return void.")
	}
	return f
}

func (f *Function) ReturnType() Type         { return f.returnType }
func (f *Function) Name() string             { return f.name }
func (f *Function) Arglist() *Struct         { return f.arglist }
func (f *Function) Xceptions() *Struct       { return f.xceptions }
func (f *Function) IsOneway() bool           { return f.oneway }
func (f *Function) Annotations() Annotations { return f.annotations }

// Validate is t_function::validate.
func (f *Function) Validate() {
	f.returnType.Validate()
	if TrueType(f.returnType).IsXception() {
		fail("method %s(): exception type \"%s\" cannot be used as function return", f.name, f.returnType.Name())
	}
	for _, a := range f.arglist.Members() {
		a.typ.Validate()
		if TrueType(a.typ).IsXception() {
			fail("method %s(): exception type \"%s\" cannot be used as function argument %s", f.name, a.typ.Name(), a.name)
		}
	}
}

// Service is t_service.
type Service struct {
	typeBase
	functions []*Function
	extends   *Service
}

// NewService creates an empty service.
func NewService(program *Program) *Service {
	return &Service{typeBase: typeBase{program: program}}
}

func (s *Service) IsService() bool { return true }

// Extends returns the base service or nil.
func (s *Service) Extends() *Service { return s.extends }

func (s *Service) setExtends(e *Service) { s.extends = e }

// Functions returns the functions declared in this service.
func (s *Service) Functions() []*Function { return s.functions }

func (s *Service) addFunction(f *Function) {
	if s.FunctionByName(f.name) != nil {
		fail("Function %s is already defined", f.name)
	}
	s.functions = append(s.functions, f)
}

func (s *Service) validateUniqueMembers() {
	for _, f := range s.functions {
		if s.extends != nil && s.extends.FunctionByName(f.name) != nil {
			fail("Function %s is already defined in service %s", f.name, s.name)
		}
	}
}

// FunctionByName searches this service and its bases.
func (s *Service) FunctionByName(name string) *Function {
	if s.extends != nil {
		if f := s.extends.FunctionByName(name); f != nil {
			return f
		}
	}
	for _, f := range s.functions {
		if f.name == name {
			return f
		}
	}
	return nil
}

// containerBase is t_container.
type containerBase struct {
	typeBase
	cppName    string
	hasCppName bool
}

func (c *containerBase) IsContainer() bool { return true }

// HasCppName reports whether a cpp_type was given.
func (c *containerBase) HasCppName() bool { return c.hasCppName }

// CppName returns the cpp_type string.
func (c *containerBase) CppName() string { return c.cppName }

func (c *containerBase) setCppName(n string) {
	c.cppName = n
	c.hasCppName = true
}

// Map is t_map.
type Map struct {
	containerBase
	keyType Type
	valType Type
}

// NewMap creates a map type.
func NewMap(key, val Type) *Map { return &Map{keyType: key, valType: val} }

func (m *Map) KeyType() Type { return m.keyType }
func (m *Map) ValType() Type { return m.valType }
func (m *Map) IsMap() bool   { return true }

// Validate is t_map::validate.
func (m *Map) Validate() {
	if TrueType(m.keyType).IsXception() {
		fail("exception type \"%s\" cannot be used inside a map", m.keyType.Name())
	}
	if TrueType(m.valType).IsXception() {
		fail("exception type \"%s\" cannot be used inside a map", m.valType.Name())
	}
}

// List is t_list.
type List struct {
	containerBase
	elemType Type
}

// NewList creates a list type.
func NewList(elem Type) *List { return &List{elemType: elem} }

func (l *List) ElemType() Type { return l.elemType }
func (l *List) IsList() bool   { return true }

// Validate is t_list::validate.
func (l *List) Validate() {
	if TrueType(l.elemType).IsXception() {
		fail("exception type \"%s\" cannot be used inside a list", l.elemType.Name())
	}
}

// Set is t_set.
type Set struct {
	containerBase
	elemType Type
}

// NewSet creates a set type.
func NewSet(elem Type) *Set { return &Set{elemType: elem} }

func (s *Set) ElemType() Type { return s.elemType }
func (s *Set) IsSet() bool    { return true }

// Validate is t_set::validate.
func (s *Set) Validate() {
	if TrueType(s.elemType).IsXception() {
		fail("exception type \"%s\" cannot be used inside a set", s.elemType.Name())
	}
}

// Const is t_const.
type Const struct {
	docBase
	typ   Type
	name  string
	value *ConstValue
}

// NewConst creates a constant.
func NewConst(typ Type, name string, value *ConstValue) *Const {
	return &Const{typ: typ, name: name, value: value}
}

func (c *Const) Type() Type         { return c.typ }
func (c *Const) Name() string       { return c.name }
func (c *Const) Value() *ConstValue { return c.value }
func (c *Const) Validate()          {}
