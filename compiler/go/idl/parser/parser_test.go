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

package parser

import (
	"reflect"
	"strings"
	"testing"

	"github.com/apache/thrift/compiler/go/idl/ast"
	"github.com/apache/thrift/compiler/go/idl/token"
)

func parse(t *testing.T, src string) *ast.Program {
	t.Helper()
	prog, err := Parse("test.thrift", []byte(src), nil)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	return prog
}

// one parses src and returns its single definition.
func one(t *testing.T, src string) ast.Definition {
	t.Helper()
	prog := parse(t, src)
	if len(prog.Definitions) != 1 {
		t.Fatalf("%q: %d definitions", src, len(prog.Definitions))
	}
	return prog.Definitions[0]
}

func expectEqual(t *testing.T, got, want interface{}) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("\n got %#v\nwant %#v", got, want)
	}
}

func i32() ast.TypeRef { return &ast.BaseTypeRef{Kind: ast.BaseI32} }

// Positive cases, one per grammar rule.

func TestHeaders(t *testing.T) {
	prog := parse(t, `include "a.thrift"
cpp_include "<map>"
namespace go pkg.sub
namespace * star
namespace java com.x (a = "1", b)
`)
	expectEqual(t, prog.Headers, []ast.Header{
		&ast.Include{Path: "a.thrift", Line: 1},
		&ast.CppInclude{Path: "<map>", Line: 2},
		&ast.Namespace{Scope: "go", Name: "pkg.sub", Line: 3},
		&ast.Namespace{Scope: "*", Name: "star", Line: 4},
		&ast.Namespace{Scope: "java", Name: "com.x", Annotations: []ast.Annotation{{Key: "a", Value: "1"}, {Key: "b", Value: "1"}}, Line: 5},
	})
}

func TestTypedef(t *testing.T) {
	expectEqual(t, one(t, `typedef i32 MyInt (go.type = "int") ;`), &ast.Typedef{
		Type: i32(), Name: "MyInt", Annotations: []ast.Annotation{{Key: "go.type", Value: "int"}}, Line: 1,
	})
	expectEqual(t, one(t, `typedef map<string, list<Foo>> M`), &ast.Typedef{
		Type: &ast.MapTypeRef{
			Key:   &ast.BaseTypeRef{Kind: ast.BaseString},
			Value: &ast.ListTypeRef{Elem: &ast.NamedTypeRef{Name: "Foo", Line: 1}, Line: 1},
		},
		Name: "M", Line: 1,
	})
}

func TestEnum(t *testing.T) {
	expectEqual(t, one(t, "enum E {\n A,\n B = 5;\n C = 0x10 (x = \"y\")\n D = -1\n} (e = \"v\")"), &ast.Enum{
		Name: "E",
		Values: []*ast.EnumValue{
			{Name: "A", Line: 2},
			{Name: "B", Value: 5, HasValue: true, Line: 3},
			{Name: "C", Value: 16, HasValue: true, Annotations: []ast.Annotation{{Key: "x", Value: "y"}}, Line: 4},
			{Name: "D", Value: -1, HasValue: true, Line: 5},
		},
		Annotations: []ast.Annotation{{Key: "e", Value: "v"}},
		Line:        1,
	})
}

func TestConstValues(t *testing.T) {
	expectEqual(t, one(t, `const i32 A = 1`).(*ast.Const).Value, &ast.ConstValue{Kind: ast.ConstInt, Int: 1, Line: 1})
	expectEqual(t, one(t, `const double B = 1.5`).(*ast.Const).Value, &ast.ConstValue{Kind: ast.ConstDouble, Double: 1.5, Line: 1})
	expectEqual(t, one(t, `const string C = "s"`).(*ast.Const).Value, &ast.ConstValue{Kind: ast.ConstString, Str: "s", Line: 1})
	expectEqual(t, one(t, `const E D = E.X`).(*ast.Const).Value, &ast.ConstValue{Kind: ast.ConstIdentifier, Ident: "E.X", Line: 1})
	expectEqual(t, one(t, "const list<i32> L = [1, 2; 3\n4]").(*ast.Const).Value, &ast.ConstValue{
		Kind: ast.ConstList, Line: 1,
		List: []*ast.ConstValue{
			{Kind: ast.ConstInt, Int: 1, Line: 1}, {Kind: ast.ConstInt, Int: 2, Line: 1},
			{Kind: ast.ConstInt, Int: 3, Line: 1}, {Kind: ast.ConstInt, Int: 4, Line: 2},
		},
	})
	expectEqual(t, one(t, `const map<string, list<i32>> M = {"a": [1], "b": []}`).(*ast.Const).Value, &ast.ConstValue{
		Kind: ast.ConstMap, Line: 1,
		Map: []ast.ConstMapEntry{
			{Key: &ast.ConstValue{Kind: ast.ConstString, Str: "a", Line: 1}, Value: &ast.ConstValue{Kind: ast.ConstList, List: []*ast.ConstValue{{Kind: ast.ConstInt, Int: 1, Line: 1}}, Line: 1}},
			{Key: &ast.ConstValue{Kind: ast.ConstString, Str: "b", Line: 1}, Value: &ast.ConstValue{Kind: ast.ConstList, Line: 1}},
		},
	})
	// The map form is also how struct constants are written.
	expectEqual(t, one(t, `const S X = {}`).(*ast.Const).Value, &ast.ConstValue{Kind: ast.ConstMap, Line: 1})
}

func TestStructUnionException(t *testing.T) {
	expectEqual(t, one(t, "struct S xsd_all {\n 1: required i32 a = 3,\n 2: optional string b;\n i64 c\n -1: list<i32> d\n} (a = \"b\")"), &ast.Struct{
		Kind: ast.StructStruct, Name: "S", XsdAll: true, Line: 1,
		Fields: []*ast.Field{
			{ID: 1, HasID: true, Req: ast.ReqRequired, Type: i32(), Name: "a", Default: &ast.ConstValue{Kind: ast.ConstInt, Int: 3, Line: 2}, Line: 2},
			{ID: 2, HasID: true, Req: ast.ReqOptional, Type: &ast.BaseTypeRef{Kind: ast.BaseString}, Name: "b", Line: 3},
			{Type: &ast.BaseTypeRef{Kind: ast.BaseI64}, Name: "c", Line: 4},
			{ID: -1, HasID: true, Type: &ast.ListTypeRef{Elem: i32(), Line: 5}, Name: "d", Line: 5},
		},
		Annotations: []ast.Annotation{{Key: "a", Value: "b"}},
	})
	if s := one(t, `union U { 1: i32 a }`).(*ast.Struct); s.Kind != ast.StructUnion {
		t.Fatalf("union kind = %v", s.Kind)
	}
	if s := one(t, `exception X { 1: i32 a }`).(*ast.Struct); s.Kind != ast.StructException {
		t.Fatalf("exception kind = %v", s.Kind)
	}
}

func TestFieldOptions(t *testing.T) {
	f := one(t, `struct S { 1: S &next xsd_optional xsd_nillable xsd_attrs { 1: i32 z } (x = "y") }`).(*ast.Struct).Fields[0]
	expectEqual(t, f, &ast.Field{
		ID: 1, HasID: true, Type: &ast.NamedTypeRef{Name: "S", Line: 1}, Reference: true, Name: "next",
		XsdOptional: true, XsdNillable: true,
		XsdAttrs: []*ast.Field{{ID: 1, HasID: true, Type: i32(), Name: "z", Line: 1}}, HasXsdAttrs: true,
		Annotations: []ast.Annotation{{Key: "x", Value: "y"}}, Line: 1,
	})
	// Keywords the grammar allows as field names.
	s := one(t, `struct S { 1: i32 string, 2: i32 list, 3: i32 service, 4: i32 required, 5: i32 include }`).(*ast.Struct)
	var names []string
	for _, f := range s.Fields {
		names = append(names, f.Name)
	}
	if got := strings.Join(names, " "); got != "string list service required include" {
		t.Fatalf("field names = %q", got)
	}
}

func TestTypes(t *testing.T) {
	s := one(t, `struct S {
 1: byte a, 2: i8 b, 3: binary c, 4: uuid d, 5: bool e, 6: double f, 7: string (x = "1") g
 8: map cpp_type "std::unordered_map" <string, i32> h
 9: set cpp_type "hs" <i32> (s = "1") i
 10: list cpp_type "vec" <i32> j
 11: list<i32> cpp_type "vec" k
}`).(*ast.Struct)
	var got []ast.TypeRef
	for _, f := range s.Fields {
		got = append(got, f.Type)
	}
	expectEqual(t, got, []ast.TypeRef{
		&ast.BaseTypeRef{Kind: ast.BaseI8},
		&ast.BaseTypeRef{Kind: ast.BaseI8},
		&ast.BaseTypeRef{Kind: ast.BaseBinary},
		&ast.BaseTypeRef{Kind: ast.BaseUUID},
		&ast.BaseTypeRef{Kind: ast.BaseBool},
		&ast.BaseTypeRef{Kind: ast.BaseDouble},
		&ast.BaseTypeRef{Kind: ast.BaseString, Annotations: []ast.Annotation{{Key: "x", Value: "1"}}},
		&ast.MapTypeRef{CppType: "std::unordered_map", HasCppType: true, Key: &ast.BaseTypeRef{Kind: ast.BaseString}, Value: i32()},
		&ast.SetTypeRef{CppType: "hs", HasCppType: true, Elem: i32(), Annotations: []ast.Annotation{{Key: "s", Value: "1"}}},
		&ast.ListTypeRef{CppType: "vec", HasCppType: true, Elem: i32(), Line: 5},
		&ast.ListTypeRef{Elem: i32(), TrailingCppType: "vec", HasTrailingCpp: true, Line: 6},
	})
}

func TestService(t *testing.T) {
	expectEqual(t, one(t, "service S extends Base {\n void a(),\n oneway void b(1: i32 x);\n async void c()\n i32 d(1: i32 x, string y) throws (1: E e) (f = \"1\")\n list<i32> e() throws ()\n} (s = \"1\")"), &ast.Service{
		Name: "S", Extends: "Base", HasExtends: true, Line: 1,
		Functions: []*ast.Function{
			{Name: "a", Line: 2},
			{Oneway: true, Name: "b", Args: []*ast.Field{{ID: 1, HasID: true, Type: i32(), Name: "x", Line: 3}}, Line: 3},
			{Oneway: true, Name: "c", Line: 4},
			{ReturnType: i32(), Name: "d", Line: 5,
				Args:        []*ast.Field{{ID: 1, HasID: true, Type: i32(), Name: "x", Line: 5}, {Type: &ast.BaseTypeRef{Kind: ast.BaseString}, Name: "y", Line: 5}},
				Throws:      []*ast.Field{{ID: 1, HasID: true, Type: &ast.NamedTypeRef{Name: "E", Line: 5}, Name: "e", Line: 5}},
				HasThrows:   true,
				Annotations: []ast.Annotation{{Key: "f", Value: "1"}}},
			{ReturnType: &ast.ListTypeRef{Elem: i32(), Line: 6}, Name: "e", HasThrows: true, Line: 6},
		},
		Annotations: []ast.Annotation{{Key: "s", Value: "1"}},
	})
}

func TestWarningsForwarded(t *testing.T) {
	var got []string
	_, err := Parse("t.thrift", []byte("struct S { 1: byte a } service X { async void f() }"), func(line, level int, msg string) {
		got = append(got, msg)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("warnings = %q", got)
	}
}

// Doc comments.

func TestDocComments(t *testing.T) {
	prog := parse(t, `/** Program doc. */

namespace go x

/** Struct doc.
 * Second line.
 */
struct S {
  /** field a */
  1: i32 a
  /** ignored: an empty definition list follows */
}

/** enum */
enum E {
  /** value */
  A
}

/** const */
const i32 C = 1

/** typedef */
typedef i32 T

/** service */
service X {
  /** function */
  void f()
}
`)
	if prog.Doc != "Program doc.\n" {
		t.Fatalf("program doc = %q", prog.Doc)
	}
	s := prog.Definitions[0].(*ast.Struct)
	if s.Doc != "Struct doc.\nSecond line.\n" || s.Fields[0].Doc != "field a\n" {
		t.Fatalf("struct doc = %q, field doc = %q", s.Doc, s.Fields[0].Doc)
	}
	e := prog.Definitions[1].(*ast.Enum)
	if e.Doc != "enum\n" || e.Values[0].Doc != "value\n" {
		t.Fatalf("enum doc = %q, value doc = %q", e.Doc, e.Values[0].Doc)
	}
	if d := prog.Definitions[2].(*ast.Const).Doc; d != "const\n" {
		t.Fatalf("const doc = %q", d)
	}
	if d := prog.Definitions[3].(*ast.Typedef).Doc; d != "typedef\n" {
		t.Fatalf("typedef doc = %q", d)
	}
	sv := prog.Definitions[4].(*ast.Service)
	if sv.Doc != "service\n" || sv.Functions[0].Doc != "function\n" {
		t.Fatalf("service doc = %q, function doc = %q", sv.Doc, sv.Functions[0].Doc)
	}
}

func TestEmptyDocIsStillADoc(t *testing.T) {
	// clean_up_doctext returns "" rather than NULL for a whitespace-only
	// comment, and set_doc("") still marks the element as documented.
	s := one(t, "/** \t\n */\nstruct S {\n /**\n *\n */\n 1: i32 a\n}").(*ast.Struct)
	if !s.HasDoc || s.Doc != "" {
		t.Fatalf("struct: HasDoc=%v Doc=%q", s.HasDoc, s.Doc)
	}
	if !s.Fields[0].HasDoc || s.Fields[0].Doc != "\n" {
		t.Fatalf("field: HasDoc=%v Doc=%q", s.Fields[0].HasDoc, s.Fields[0].Doc)
	}
	if u := one(t, "struct U {}").(*ast.Struct); u.HasDoc {
		t.Fatal("no comment, no doc")
	}
}

func TestProgramDocRules(t *testing.T) {
	// The first doc comment is the program's only when a header separates
	// it from the first definition; otherwise the definition takes it.
	prog := parse(t, "/** mine */\nstruct S {}")
	if prog.Doc != "" || prog.Definitions[0].(*ast.Struct).Doc != "mine\n" {
		t.Fatalf("program %q, struct %q", prog.Doc, prog.Definitions[0].(*ast.Struct).Doc)
	}
	prog = parse(t, "/** mine */\nnamespace go x\nstruct S {}")
	if prog.Doc != "mine\n" || prog.Definitions[0].(*ast.Struct).Doc != "" {
		t.Fatalf("program %q, struct %q", prog.Doc, prog.Definitions[0].(*ast.Struct).Doc)
	}
	// A doc comment before a header, followed by another before the
	// definition: the first is the program's, the second the struct's.
	prog = parse(t, "/** prog */\nnamespace go x\n/** s */\nstruct S {}")
	if prog.Doc != "prog\n" || prog.Definitions[0].(*ast.Struct).Doc != "s\n" {
		t.Fatalf("program %q, struct %q", prog.Doc, prog.Definitions[0].(*ast.Struct).Doc)
	}
	// A doc comment right after the last header is read as lookahead
	// before the header's action runs, so it is still a candidate when
	// declare_valid_program_doctext fires. Both the program and the
	// struct get it; the C++ compiler does the same.
	prog = parse(t, "namespace go x\n/** s */\nstruct S {}")
	if prog.Doc != "s\n" || prog.Definitions[0].(*ast.Struct).Doc != "s\n" {
		t.Fatalf("program %q, struct %q", prog.Doc, prog.Definitions[0].(*ast.Struct).Doc)
	}
	// With a definition between the header and the comment there is no
	// candidate left.
	prog = parse(t, "namespace go x\nstruct A {}\n/** s */\nstruct S {}")
	if prog.Doc != "" {
		t.Fatalf("program %q", prog.Doc)
	}
	// A file with only a doc comment keeps it as program doc.
	prog = parse(t, "/** only */\n")
	if prog.Doc != "only\n" {
		t.Fatalf("program %q", prog.Doc)
	}
}

func TestCleanUpDoctext(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{" one line ", "one line\n", true},
		{"\n * a\n * b\n ", "a\nb\n", true},
		{"\n * a\n *\n * b\n ", "a\n\nb\n", true},
		{"\n   a\n   b\n ", "a\nb\n", true},
		{"\n * a\r\n * b\r\n ", "a\nb\n", true},
		{" ", "", false},
		// An empty comment body is not NULL: the star line survives as an
		// empty line. Whitespace-only lines keep their whitespace.
		{"\n *\n ", "\n", true},
		{"\n * \t\n ", " \t\n", true},
		{"\n * real\n *\n ", "real\n\n", true},
	}
	for _, c := range cases {
		got, ok := cleanUpDoctext(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("%q: got %q %v, want %q %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

// Negative cases: the error names the line and the offending token.

func TestSyntaxErrors(t *testing.T) {
	cases := []struct {
		src  string
		line int
		want string
	}{
		{"struct {", 1, `unexpected "{"`},
		{"struct S\n{ 1: i32 }", 2, "expecting a field name"},
		{"struct S { 1 i32 a }", 1, `expecting ":"`},
		{"struct S { 1: a }", 1, "expecting a field name"},
		{"enum E { A = x }", 1, `expecting "integer constant"`},
		{"const i32 A 1", 1, `expecting "="`},
		{"const i32 A = ", 1, `unexpected "end of file"`},
		{"const list<i32> A = [1", 1, `unexpected "end of file"`},
		{"const map<string, i32> A = {\"a\" 1}", 1, `expecting ":"`},
		{"service S { void f( }", 1, "expecting a type"},
		{"service S { void f() throws 1: E e }", 1, `expecting "("`},
		{"typedef i32", 1, `expecting "identifier"`},
		{"namespace go", 1, `expecting "identifier"`},
		{"include a.thrift", 1, `expecting "string literal"`},
		{"struct S {}\nfoo", 2, `unexpected identifier "foo"`},
		{"struct S {} namespace go x", 1, `unexpected "namespace"`},
		{"struct S { 1: map<i32> m }", 1, `expecting ","`},
		{"struct S { 1: i32 a (x = 1) }", 1, `expecting "string literal"`},
		{"struct S { 1: i32 a }}", 1, `unexpected "}"`},
		{"struct S { 1: i32 a = @ }", 1, "Unexpected token in input"},
		{"struct S { 1: i32 a = \"x }", 1, "End of file while reading string"},
		{"/* x", 1, "Unexpected end of file in multiline comment"},
	}
	for _, c := range cases {
		_, err := Parse("t.thrift", []byte(c.src), nil)
		if err == nil {
			t.Errorf("%q: accepted", c.src)
			continue
		}
		e, ok := err.(*Error)
		if !ok {
			t.Errorf("%q: error type %T", c.src, err)
			continue
		}
		if e.Line != c.line || !strings.Contains(e.Msg, c.want) {
			t.Errorf("%q: got line %d %q, want line %d containing %q", c.src, e.Line, e.Msg, c.line, c.want)
		}
	}
}

func TestSeparators(t *testing.T) {
	// Commas and semicolons are optional everywhere the grammar allows
	// them, and at most one may appear.
	parse(t, "struct S { 1: i32 a; 2: i32 b, 3: i32 c }")
	parse(t, "enum E { A; B, C }")
	parse(t, "service S { void a(); void b(), void c() }")
	parse(t, "typedef i32 A; typedef i32 B, typedef i32 C")
	parse(t, "const i32 A = 1; const i32 B = 2,")
	for _, src := range []string{"struct S { 1: i32 a;; }", "enum E { A,, }", "struct S { , }"} {
		if _, err := Parse("t.thrift", []byte(src), nil); err == nil {
			t.Errorf("%q: accepted", src)
		}
	}
}

func TestEmptyProgram(t *testing.T) {
	for _, src := range []string{"", "\n", "// comment only\n", "# hash\n/* block */"} {
		prog := parse(t, src)
		if len(prog.Headers) != 0 || len(prog.Definitions) != 0 || prog.Doc != "" {
			t.Errorf("%q: %+v", src, prog)
		}
	}
}

func TestSyntaxErrorPosition(t *testing.T) {
	_, err := Parse("t.thrift", []byte("struct S {\n  1: i32 a\n  2 i32 b\n}"), nil)
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("got %v, want *Error", err)
	}
	if want := (token.Pos{Line: 3, Col: 5}); e.Pos != want {
		t.Errorf("error at %v (%s), want %v: the i32 after the missing colon", e.Pos, e.Msg, want)
	}
}
