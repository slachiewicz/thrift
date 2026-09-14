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

// Package ast holds the syntax tree of one Thrift IDL file.
//
// The tree is purely syntactic: type names are unresolved strings, enum
// values and field ids that were left implicit are marked as such, and no
// validation has run. Package sema turns it into the resolved model.
package ast

// Annotation is one "key = value" pair from a type annotation list. A bare
// key has the value "1", as in the C++ compiler.
type Annotation struct {
	Key   string
	Value string
}

// Program is one parsed file.
type Program struct {
	Path        string
	Headers     []Header
	Definitions []Definition
	// Doc is the program-level doc comment, or empty.
	Doc string
}

// Header is an include, namespace, or cpp_include declaration.
type Header interface{ header() }

// Include is `include "path"`.
type Include struct {
	Path string
	Line int
}

// Namespace is `namespace scope name` or `namespace * name`.
type Namespace struct {
	Scope       string // "*" for the wildcard form
	Name        string
	Annotations []Annotation
	Line        int
}

// CppInclude is `cpp_include "path"`.
type CppInclude struct {
	Path string
	Line int
}

func (*Include) header()    {}
func (*Namespace) header()  {}
func (*CppInclude) header() {}

// TypeRef is a syntactic type reference.
type TypeRef interface{ typeRef() }

// BaseKind enumerates the base type keywords.
type BaseKind int

// Base type kinds, in the order of t_base_type::t_base.
const (
	BaseVoid BaseKind = iota
	BaseString
	BaseUUID
	BaseBool
	BaseI8
	BaseI16
	BaseI32
	BaseI64
	BaseDouble
	BaseBinary // string with the binary flag
)

// BaseTypeRef is one of the base type keywords, with optional annotations.
type BaseTypeRef struct {
	Kind        BaseKind
	Annotations []Annotation
}

// NamedTypeRef refers to a declared type by name, possibly qualified with
// an include name ("shared.SharedStruct").
type NamedTypeRef struct {
	Name string
	Line int
}

// MapTypeRef is `map cpp_type "x" <K, V> (annotations)`.
type MapTypeRef struct {
	CppType     string
	HasCppType  bool
	Key, Value  TypeRef
	Annotations []Annotation
}

// SetTypeRef is `set cpp_type "x" <T> (annotations)`.
type SetTypeRef struct {
	CppType     string
	HasCppType  bool
	Elem        TypeRef
	Annotations []Annotation
}

// ListTypeRef is `list cpp_type "x" <T> cpp_type "y" (annotations)`. The
// trailing cpp_type is a deprecated spelling that the grammar still accepts.
type ListTypeRef struct {
	CppType         string
	HasCppType      bool
	Elem            TypeRef
	TrailingCppType string
	HasTrailingCpp  bool
	Annotations     []Annotation
	Line            int
}

func (*BaseTypeRef) typeRef()  {}
func (*NamedTypeRef) typeRef() {}
func (*MapTypeRef) typeRef()   {}
func (*SetTypeRef) typeRef()   {}
func (*ListTypeRef) typeRef()  {}

// Definition is a top-level declaration.
type Definition interface{ definition() }

// Typedef is `typedef Type Name (annotations)`.
type Typedef struct {
	Type        TypeRef
	Name        string
	Annotations []Annotation
	Doc         string
	Line        int
}

// Enum is `enum Name { values } (annotations)`.
type Enum struct {
	Name        string
	Values      []*EnumValue
	Annotations []Annotation
	Doc         string
	Line        int
}

// EnumValue is one enum member. HasValue is false when the value was left
// implicit and must be assigned by counting.
type EnumValue struct {
	Name        string
	Value       int64
	HasValue    bool
	Annotations []Annotation
	Doc         string
	Line        int
}

// Const is `const Type Name = Value`.
type Const struct {
	Type  TypeRef
	Name  string
	Value *ConstValue
	Doc   string
	Line  int
}

// StructKind distinguishes struct, union and exception declarations.
type StructKind int

// Struct kinds.
const (
	StructStruct StructKind = iota
	StructUnion
	StructException
)

// Struct is a struct, union or exception declaration.
type Struct struct {
	Kind        StructKind
	Name        string
	XsdAll      bool
	Fields      []*Field
	Annotations []Annotation
	Doc         string
	Line        int
}

// Requiredness is the explicit requiredness keyword on a field.
type Requiredness int

// Requiredness values.
const (
	ReqDefault Requiredness = iota
	ReqRequired
	ReqOptional
)

// Field is one member of a struct, argument list, throws list or xsd
// attribute list.
type Field struct {
	ID          int64
	HasID       bool
	Req         Requiredness
	Type        TypeRef
	Reference   bool
	Name        string
	Default     *ConstValue
	XsdOptional bool
	XsdNillable bool
	XsdAttrs    []*Field
	HasXsdAttrs bool
	Annotations []Annotation
	Doc         string
	Line        int
}

// Service is `service Name extends Base { functions } (annotations)`.
type Service struct {
	Name        string
	Extends     string
	HasExtends  bool
	Functions   []*Function
	Annotations []Annotation
	Doc         string
	Line        int
}

// Function is one service method. ReturnType is nil for void.
type Function struct {
	Oneway      bool
	ReturnType  TypeRef
	Name        string
	Args        []*Field
	Throws      []*Field
	HasThrows   bool
	Annotations []Annotation
	Doc         string
	Line        int
}

func (*Typedef) definition() {}
func (*Enum) definition()    {}
func (*Const) definition()   {}
func (*Struct) definition()  {}
func (*Service) definition() {}

// ConstKind is the syntactic form of a constant value.
type ConstKind int

// Constant value kinds.
const (
	ConstInt ConstKind = iota
	ConstDouble
	ConstString
	ConstIdentifier
	ConstList
	ConstMap
)

// ConstValue is a literal constant expression.
type ConstValue struct {
	Kind   ConstKind
	Int    int64
	Double float64
	Str    string
	Ident  string
	List   []*ConstValue
	Map    []ConstMapEntry
	Line   int
}

// ConstMapEntry is one key/value pair of a map constant, in source order.
type ConstMapEntry struct {
	Key, Value *ConstValue
}
