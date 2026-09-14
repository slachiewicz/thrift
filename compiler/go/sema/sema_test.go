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
	"bytes"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// One test per rule of the design's section 5.3. Each writes its files to
// a temporary directory, because Load needs real paths, and loads the
// named entry file.

// writeTree writes files (relative path to content) under a fresh
// temporary directory and returns that directory.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, content := range files {
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// compile finishes what Load leaves to the generator: constant resolution
// and the validators. It returns the *Error those raise.
func compile(p *Program) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*Error); ok {
				err = e
				return
			}
			panic(r)
		}
	}()
	p.Scope.ResolveAllConsts()
	ValidateInput(p)
	return nil
}

// load writes files, loads entry with l, and runs compile. warnings
// receives everything the loader printed.
func load(t *testing.T, l *Loader, files map[string]string, entry string) (*Program, string, error) {
	t.Helper()
	dir := writeTree(t, files)
	var warnings bytes.Buffer
	if l.Diag == nil {
		l.Diag = &Diagnostics{Out: &warnings, WarnLevel: 1}
	} else if l.Diag.Out == nil {
		l.Diag.Out = &warnings
	}
	for i, d := range l.IncludeDirs {
		l.IncludeDirs[i] = filepath.Join(dir, d)
	}
	prog, err := l.Load(filepath.Join(dir, entry))
	if err != nil {
		return nil, warnings.String(), err
	}
	return prog, warnings.String(), compile(prog)
}

func mustLoad(t *testing.T, l *Loader, files map[string]string, entry string) (*Program, string) {
	t.Helper()
	prog, warnings, err := load(t, l, files, entry)
	if err != nil {
		t.Fatalf("unexpected error: %v\nwarnings:\n%s", err, warnings)
	}
	return prog, warnings
}

func mustFail(t *testing.T, l *Loader, files map[string]string, entry, want string) {
	t.Helper()
	_, warnings, err := load(t, l, files, entry)
	if err == nil {
		t.Fatalf("expected an error containing %q, got none\nwarnings:\n%s", want, warnings)
	}
	if !strings.Contains(err.Error(), want) {
		t.Fatalf("error %q does not contain %q", err.Error(), want)
	}
}

func constInt(t *testing.T, p *Program, name string) int64 {
	t.Helper()
	c := p.Scope.GetConstant(name)
	if c == nil {
		t.Fatalf("constant %s not in scope", name)
	}
	return c.Value().Integer()
}

// Include resolution.

func TestIncludeRelativeToIncludingFile(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"a/main.thrift":    `include "sub/inc.thrift" struct S { 1: inc.T t }`,
		"a/sub/inc.thrift": `struct T { 1: i32 x }`,
	}, "a/main.thrift")
	if got := prog.Scope.GetType("inc.T"); got == nil {
		t.Fatal("inc.T not registered in the including scope")
	}
	if len(prog.Includes()) != 1 || prog.Includes()[0].Name() != "inc" {
		t.Fatalf("includes = %v", prog.Includes())
	}
}

func TestIncludeDirsSearchedInOrderAfterIncludingDir(t *testing.T) {
	files := map[string]string{
		"main.thrift":      `include "inc.thrift" const i32 X = inc.V`,
		"one/inc.thrift":   `const i32 V = 1`,
		"two/inc.thrift":   `const i32 V = 2`,
		"here/main.thrift": `include "inc.thrift" const i32 X = inc.V`,
		"here/inc.thrift":  `const i32 V = 3`,
	}
	prog, _ := mustLoad(t, &Loader{IncludeDirs: []string{"one", "two"}}, files, "main.thrift")
	if got := constInt(t, prog, "X"); got != 1 {
		t.Fatalf("first -I directory should win, got V=%d", got)
	}
	prog, _ = mustLoad(t, &Loader{IncludeDirs: []string{"two", "one"}}, files, "main.thrift")
	if got := constInt(t, prog, "X"); got != 2 {
		t.Fatalf("-I order should be honoured, got V=%d", got)
	}
	prog, _ = mustLoad(t, &Loader{IncludeDirs: []string{"one", "two"}}, files, "here/main.thrift")
	if got := constInt(t, prog, "X"); got != 3 {
		t.Fatalf("the including file's directory should be searched first, got V=%d", got)
	}
}

func TestIncludeAbsolutePath(t *testing.T) {
	dir := writeTree(t, map[string]string{"inc.thrift": `const i32 V = 7`})
	abs, err := filepath.EvalSymlinks(filepath.Join(dir, "inc.thrift"))
	if err != nil {
		t.Fatal(err)
	}
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"main.thrift": `include "` + abs + `" const i32 X = inc.V`,
	}, "main.thrift")
	if got := constInt(t, prog, "X"); got != 7 {
		t.Fatalf("X = %d", got)
	}
}

func TestIncludeMissingIsWarningUnlessStrict(t *testing.T) {
	files := map[string]string{"main.thrift": `include "nope.thrift" struct S { 1: i32 x }`}
	_, warnings := mustLoad(t, &Loader{}, files, "main.thrift")
	if !strings.Contains(warnings, "Could not find include file nope.thrift") {
		t.Fatalf("warnings:\n%s", warnings)
	}
	mustFail(t, &Loader{Strict: 255}, files, "main.thrift", "Could not find include file nope.thrift")
	mustFail(t, &Loader{Strict: 192}, files, "main.thrift", "Could not find include file nope.thrift")
	_, warnings = mustLoad(t, &Loader{Strict: 191}, files, "main.thrift")
	if !strings.Contains(warnings, "Could not find include file") {
		t.Fatalf("191 is below the threshold; warnings:\n%s", warnings)
	}
}

func TestIncludeRecursionDetected(t *testing.T) {
	mustFail(t, &Loader{}, map[string]string{
		"a.thrift": `include "b.thrift"`,
		"b.thrift": `include "a.thrift"`,
	}, "a.thrift", "Recursion detected")
	mustFail(t, &Loader{}, map[string]string{
		"self.thrift": `include "self.thrift"`,
	}, "self.thrift", "Recursion detected")
}

func TestIncludeSameFileTwiceFails(t *testing.T) {
	// realpath canonicalises, so "sub/../inc.thrift" and "inc.thrift" name
	// the same file. The C++ compiler parses it twice and fails on the
	// second registration of its symbols; so does the Go loader.
	mustFail(t, &Loader{}, map[string]string{
		"main.thrift":     `include "inc.thrift" include "sub/../inc.thrift" const i32 X = inc.V`,
		"inc.thrift":      `const i32 V = 4`,
		"sub/placeholder": ``,
	}, "main.thrift", "inc.V is already defined")
}

// Scope.

func TestIncludedSymbolsRegisteredOneLevelOnly(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"top.thrift":  `include "mid.thrift" struct T { 1: mid.M m }`,
		"mid.thrift":  `include "leaf.thrift" struct M { 1: leaf.L l } const i32 C = leaf.LC`,
		"leaf.thrift": `struct L { 1: i32 x } const i32 LC = 9 service LS {} enum LE { A }`,
	}, "top.thrift")
	if prog.Scope.GetType("mid.M") == nil {
		t.Fatal("mid.M missing from top scope")
	}
	if prog.Scope.GetConstant("mid.C") == nil {
		t.Fatal("mid.C missing from top scope")
	}
	if prog.Scope.GetType("leaf.L") != nil || prog.Scope.GetType("mid.leaf.L") != nil {
		t.Fatal("grandchild types must not reach the top scope")
	}
	mid := prog.Includes()[0]
	if mid.Scope.GetType("leaf.L") == nil || mid.Scope.GetService("leaf.LS") == nil {
		t.Fatal("leaf symbols missing from mid scope")
	}
	if mid.Scope.GetConstant("leaf.LE.A") == nil {
		t.Fatal("enum members are registered as constants in the including scope")
	}
}

func TestDuplicateConstantFails(t *testing.T) {
	mustFail(t, &Loader{}, map[string]string{
		"m.thrift": "const i32 A = 1\nconst i32 A = 2",
	}, "m.thrift", "is already defined")
}

func TestLookupFailureIsNil(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{"m.thrift": `struct S {}`}, "m.thrift")
	if prog.Scope.GetType("Nope") != nil || prog.Scope.GetConstant("Nope") != nil || prog.Scope.GetService("Nope") != nil {
		t.Fatal("lookups of unknown names must return nil")
	}
}

// Constant resolution.

func TestConstTypedefFollowed(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `typedef i32 MyInt typedef MyInt MyInt2 const MyInt2 X = 5 enum E { A = 3 } typedef E TE const TE Y = 3`,
	}, "m.thrift")
	if got := constInt(t, prog, "X"); got != 5 {
		t.Fatalf("X = %d", got)
	}
	y := prog.Scope.GetConstant("Y").Value()
	if y.Kind() != CVIdentifier || y.Identifier() != "E.A" || y.Enum() == nil {
		t.Fatalf("Y should resolve to E.A through the typedef, got kind=%v id=%q", y.Kind(), y.Identifier())
	}
}

func TestConstIdentifierOfEnumTypeBindsEnum(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `enum E { A = 1, B = 2 } const E X = E.B const list<E> L = [E.A, 2]`,
	}, "m.thrift")
	x := prog.Scope.GetConstant("X").Value()
	if x.Enum() == nil || x.Enum().Name() != "E" || x.Integer() != 2 {
		t.Fatalf("X: enum=%v int=%d", x.Enum(), x.Integer())
	}
	l := prog.Scope.GetConstant("L").Value().List()
	if len(l) != 2 || l[1].Kind() != CVIdentifier || l[1].Identifier() != "E.B" {
		t.Fatalf("integer list elements of enum type must map back to the member: %+v", l)
	}
}

func TestConstIdentifierCopiesNamedConstant(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `const i32 I = 3 const i32 I2 = I
const string S = "s" const string S2 = S
const double D = 1.5 const double D2 = D
const list<i32> L = [1, 2] const list<i32> L2 = L
const map<string, i32> M = {"k": 1} const map<string, i32> M2 = M
const bool B = true const bool B2 = B`,
	}, "m.thrift")
	get := func(n string) *ConstValue { return prog.Scope.GetConstant(n).Value() }
	if get("I2").Integer() != 3 || get("B2").Integer() != 1 {
		t.Fatal("integer kinds are copied")
	}
	if get("S2").String() != "s" {
		t.Fatal("string is copied")
	}
	if get("D2").Double() != 1.5 {
		t.Fatal("double is copied")
	}
	// The container branches of resolve_const_value run before the
	// identifier branch, so a container constant named by identifier is
	// left as the identifier and the generator emits a reference.
	if v := get("L2"); v.Kind() != CVIdentifier || v.Identifier() != "L" {
		t.Fatalf("L2 = kind %v %q", v.Kind(), v.Identifier())
	}
	if v := get("M2"); v.Kind() != CVIdentifier || v.Identifier() != "M" {
		t.Fatalf("M2 = kind %v %q", v.Kind(), v.Identifier())
	}
}

func TestConstIdentifierUnknownFails(t *testing.T) {
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `const i32 X = NOPE`}, "m.thrift",
		"No enum value or constant found named \"NOPE\"")
}

func TestConstIntegerMapsBackToEnumMember(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `enum E { A = 1, B = 2 } const E X = 2 struct S { 1: E e = 1 }`,
	}, "m.thrift")
	x := prog.Scope.GetConstant("X").Value()
	if x.Kind() != CVIdentifier || x.Identifier() != "E.B" {
		t.Fatalf("X = kind %v %q", x.Kind(), x.Identifier())
	}
	f := prog.Structs()[0].Members()[0].Value()
	if f.Kind() != CVIdentifier || f.Identifier() != "E.A" {
		t.Fatalf("field default = kind %v %q", f.Kind(), f.Identifier())
	}
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `enum E { A = 1 } const E X = 9`}, "m.thrift",
		"Couldn't find a named value in enum E for value 9")
}

func TestConstStructResolvedFieldWise(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `enum E { A = 1 } struct S { 1: E e, 2: i32 n } const S X = {"e": 1, "n": 2}`,
	}, "m.thrift")
	m := prog.Scope.GetConstant("X").Value().Map()
	if len(m) != 2 || m[0].Value.Identifier() != "E.A" {
		t.Fatalf("struct constant fields resolve by field type: %+v", m)
	}
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `struct S { 1: i32 n } const S X = {"nope": 2}`}, "m.thrift",
		"No field named \"nope\" was found in struct of type \"S\"")
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `struct S { 1: i32 n } const S X = {1: 2}`}, "m.thrift",
		// Resolution runs before validation and reads the key as a string,
		// so this is the message the C++ compiler prints too.
		"No field named \"\" was found in struct of type \"S\"")
}

func TestConstMapKeyAndValueResolved(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `enum E { A = 1 } const map<E, E> M = {1: E.A}`,
	}, "m.thrift")
	m := prog.Scope.GetConstant("M").Value().Map()
	if m[0].Key.Identifier() != "E.A" || m[0].Value.Enum() == nil {
		t.Fatalf("map entries resolve on both sides: %+v", m)
	}
}

// Validation.

func TestValidateSimpleIdentifier(t *testing.T) {
	for _, src := range []string{
		`struct a.b {}`,
		`typedef i32 a.b`,
		`enum E { a.b }`,
		`struct S { 1: i32 a.b }`,
		`service a.b {}`,
		`service S { void a.b() }`,
	} {
		mustFail(t, &Loader{}, map[string]string{"m.thrift": src}, "m.thrift", "can't have a dot")
	}
}

func TestValidateConstType(t *testing.T) {
	cases := map[string]string{
		`const i32 X = "s"`:                 "was declared as i32",
		`const string X = 1`:                "was declared as string",
		`const bool X = "s"`:                "was declared as bool",
		`const double X = "s"`:              "was declared as double",
		`const list<i32> X = ["s"]`:         "was declared as i32",
		`const map<string, i32> X = {1: 1}`: "was declared as string",
		`const set<i32> X = ["s"]`:          "was declared as i32",
		`struct S {} const S X = 1`:         "was declared as struct/xception",
		`const uuid X = "nope"`:             "invalid uuid",
		`const uuid X = 1`:                  "was declared as uuid",
		`const binary X = 1`:                "was declared as string",
	}
	for src, want := range cases {
		mustFail(t, &Loader{}, map[string]string{"m.thrift": src}, "m.thrift", want)
	}
	// Integers are accepted for double, and a string for an enum type is
	// not caught at all, as in the C++ compiler.
	mustLoad(t, &Loader{}, map[string]string{"m.thrift": `const double X = 1`}, "m.thrift")
	mustLoad(t, &Loader{}, map[string]string{"m.thrift": `enum E { A } const E X = "s"`}, "m.thrift")
}

func TestValidateFieldValue(t *testing.T) {
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `struct S { 1: i32 a = "s" }`}, "m.thrift",
		"was declared as i32")
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `service S { void f(1: string a = 1) }`}, "m.thrift",
		"was declared as string")
}

func TestValidateThrows(t *testing.T) {
	mustFail(t, &Loader{}, map[string]string{
		"m.thrift": `struct S {} service X { void f() throws (1: S s) }`,
	}, "m.thrift", "Throws clause may not contain non-exception types")
	mustFail(t, &Loader{}, map[string]string{
		"m.thrift": `service X { void f() throws (1: i32 s) }`,
	}, "m.thrift", "Throws clause may not contain non-exception types")
	mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `exception E {} typedef E TE service X { void f() throws (1: E e, 2: TE te) }`,
	}, "m.thrift")
}

func TestExceptionsAllowedAsTypes(t *testing.T) {
	// ALLOW_EXCEPTIONS_AS_TYPE is defined unconditionally in t_type.h, so
	// the C++ compiler accepts all of these.
	mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `exception E { 1: i32 code }
struct S { 1: E e, 2: list<E> l, 3: map<string, E> m, 4: set<E> s }
service X { E f(1: E e) }`,
	}, "m.thrift")
}

func TestStructMemberRules(t *testing.T) {
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `struct S { 1: i32 a, 1: i32 b }`}, "m.thrift",
		"field identifier/name has already been used")
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `struct S { 1: i32 a, 2: i32 a }`}, "m.thrift",
		"field identifier/name has already been used")
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `union U { 1: i32 a = 1, 2: i32 b = 2 }`}, "m.thrift",
		"provides another default value for union")
	_, warnings := mustLoad(t, &Loader{}, map[string]string{"m.thrift": `union U { 1: required i32 a }`}, "m.thrift")
	if !strings.Contains(warnings, "union members must be optional") {
		t.Fatalf("a required union member warns; warnings:\n%s", warnings)
	}
}

func TestServiceRules(t *testing.T) {
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `service S { void f() void f() }`}, "m.thrift",
		"Function f is already defined")
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `service S extends Nope {}`}, "m.thrift",
		"has not been defined")
	mustFail(t, &Loader{}, map[string]string{
		"m.thrift": `exception E {} service S { oneway void f() throws (1: E e) }`,
	}, "m.thrift", "Oneway methods can't throw exceptions")
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `service Base { void b() } service S extends Base { void f() }`,
	}, "m.thrift")
	if prog.Services()[1].Extends() != prog.Services()[0] {
		t.Fatal("extends must resolve to the base service")
	}
}

func TestUnknownTypeFailsOnFirstUse(t *testing.T) {
	// An unknown name is a forward reference, which fails only when
	// something asks for the type, as t_typedef::get_type does. The
	// generator is the first to ask, so the reject corpus covers the
	// end-to-end failure.
	for _, src := range []string{
		`struct S { 1: Nope n }`,
		`typedef Nope T struct S { 1: T n }`,
	} {
		prog, _ := mustLoad(t, &Loader{}, map[string]string{"m.thrift": src}, "m.thrift")
		err := catch(func() { TrueType(prog.Structs()[0].Members()[0].Type()) })
		if err == nil || !strings.Contains(err.Error(), `Type "Nope" not defined`) {
			t.Fatalf("%s: err = %v", src, err)
		}
	}
}

func TestForwardTypedefResolvesLazily(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `typedef Later T struct Later { 1: i32 x } struct S { 1: T t, 2: list<T> l }`,
	}, "m.thrift")
	td := prog.Typedefs()[0]
	fwd, ok := td.Type().(*Typedef)
	if !ok || !fwd.IsForwardTypedef() || fwd.Symbolic() != "Later" {
		t.Fatalf("a not yet defined name becomes a forward typedef, got %T %v", td.Type(), td.Type())
	}
	if tt := TrueType(td); !tt.IsStruct() || tt.Name() != "Later" {
		t.Fatalf("the forward typedef resolves to the later struct, got %v", tt)
	}
}

// catch runs fn and returns the *Error it raised, if any.
func catch(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*Error); ok {
				err = e
				return
			}
			panic(r)
		}
	}()
	fn()
	return nil
}

// Field and enum numbering.

func TestImplicitFieldIdsCountDown(t *testing.T) {
	prog, warnings := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `struct S { i32 a, 5: i32 b, i32 c } service X { void f(i32 a, i32 b) }`,
	}, "m.thrift")
	keys := []int32{}
	for _, f := range prog.Structs()[0].Members() {
		keys = append(keys, f.Key())
	}
	if want := []int32{-1, 5, -2}; !equalInt32(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if strings.Count(warnings, "No field key specified for") != 4 {
		t.Fatalf("one warning per implicit id; warnings:\n%s", warnings)
	}
	args := prog.Services()[0].Functions()[0].Arglist().Members()
	if args[0].Key() != -1 || args[1].Key() != -2 {
		t.Fatalf("the counter restarts per field list: %d %d", args[0].Key(), args[1].Key())
	}
}

func TestImplicitFieldIdIsErrorWithStrict(t *testing.T) {
	files := map[string]string{"m.thrift": `struct S { i32 a }`}
	mustFail(t, &Loader{Strict: 255}, files, "m.thrift", "Implicit field keys are deprecated and not allowed with -strict")
	mustFail(t, &Loader{Strict: 192}, files, "m.thrift", "Implicit field keys")
	mustLoad(t, &Loader{Strict: 191}, files, "m.thrift")
}

func TestNonpositiveFieldIds(t *testing.T) {
	files := map[string]string{"m.thrift": `struct S { -3: i32 a, i32 b, 0: i32 c }`}
	prog, warnings := mustLoad(t, &Loader{}, files, "m.thrift")
	keys := []int32{}
	for _, f := range prog.Structs()[0].Members() {
		keys = append(keys, f.Key())
	}
	// Without -allow-neg-keys a nonpositive id is replaced by the next
	// implicit one.
	if want := []int32{-1, -2, -3}; !equalInt32(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if !strings.Contains(warnings, "Nonpositive value (-3) not allowed as a field key") {
		t.Fatalf("warnings:\n%s", warnings)
	}

	prog, warnings = mustLoad(t, &Loader{AllowNegFieldKeys: true}, files, "m.thrift")
	keys = keys[:0]
	for _, f := range prog.Structs()[0].Members() {
		keys = append(keys, f.Key())
	}
	// With -allow-neg-keys the id is kept and the counter resumes below it.
	if want := []int32{-3, -4, 0}; !equalInt32(keys, want) {
		t.Fatalf("keys = %v, want %v", keys, want)
	}
	if !strings.Contains(warnings, "Nonpositive field key (-3) differs from what would be auto-assigned by thrift (-1)") {
		t.Fatalf("warnings:\n%s", warnings)
	}
}

func TestFieldIdRangeWarning(t *testing.T) {
	_, warnings := mustLoad(t, &Loader{}, map[string]string{"m.thrift": `struct S { 40000: i32 a }`}, "m.thrift")
	if !strings.Contains(warnings, "Field key (40000) exceeds allowed range") {
		t.Fatalf("warnings:\n%s", warnings)
	}
}

func TestEnumNumbering(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `enum E { A, B, C = 10, D, E = -2, F }`,
	}, "m.thrift")
	got := []int32{}
	for _, v := range prog.Enums()[0].Constants() {
		got = append(got, v.Value())
	}
	if want := []int32{0, 1, 10, 11, -2, -1}; !equalInt32(got, want) {
		t.Fatalf("values = %v, want %v", got, want)
	}
	if prog.Scope.GetConstant("E.D").Value().Integer() != 11 {
		t.Fatal("members are registered as constants")
	}
}

func TestEnumOverflowAndTruncation(t *testing.T) {
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `enum E { A = 2147483647, B }`}, "m.thrift",
		"enum value overflow at enum B")
	mustLoad(t, &Loader{}, map[string]string{"m.thrift": `enum E { A = 2147483647 }`}, "m.thrift")
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `enum E { A = 2147483648 }`}, "m.thrift",
		"64-bit value supplied for enum A will be truncated")
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `enum E { A = -2147483649 }`}, "m.thrift",
		"64-bit value supplied for enum A will be truncated")
	mustLoad(t, &Loader{}, map[string]string{"m.thrift": `enum E { A = -2147483648 }`}, "m.thrift")
	_ = math.MaxInt32
}

func TestDuplicateTypeFails(t *testing.T) {
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `struct S {} struct S {}`}, "m.thrift", "is already defined")
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `enum E { A } enum E { B }`}, "m.thrift", "is already defined")
	mustFail(t, &Loader{}, map[string]string{"m.thrift": `typedef i32 T typedef i64 T`}, "m.thrift", "is already defined")
}

// Namespaces.

func TestNamespaceWildcardFallback(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `namespace * star namespace go gopkg namespace java jpkg`,
	}, "m.thrift")
	if prog.Namespace("go") != "gopkg" || prog.Namespace("java") != "jpkg" {
		t.Fatal("explicit namespaces win")
	}
	if prog.Namespace("py") != "star" {
		t.Fatalf("unset language falls back to *, got %q", prog.Namespace("py"))
	}
	prog, _ = mustLoad(t, &Loader{}, map[string]string{"m.thrift": `namespace go gopkg`}, "m.thrift")
	if prog.Namespace("py") != "" {
		t.Fatal("no fallback without *")
	}
}

func TestNamespaceAnnotations(t *testing.T) {
	prog, _ := mustLoad(t, &Loader{}, map[string]string{
		"m.thrift": `namespace go pkg (a = "1", b)`,
	}, "m.thrift")
	a := prog.NamespaceAnnotations("go")
	if v, ok := a.First("a"); !ok || v != "1" {
		t.Fatalf("annotation a = %v", a)
	}
	if v, ok := a.First("b"); !ok || v != "1" {
		t.Fatalf("a bare annotation key has the value \"1\", got %v", a)
	}
}

// Warning levels.

func TestWarningLevels(t *testing.T) {
	files := map[string]string{
		"m.thrift":   `include "inc.thrift" struct S { i32 a, 1: list<byte> b, 2: byte c } service X { async void f() }`,
		"inc.thrift": `struct I { 1: byte b }`,
	}
	// -nowarn: only level 0 survives.
	var out bytes.Buffer
	mustLoad(t, &Loader{Diag: &Diagnostics{Out: &out, WarnLevel: 0}}, files, "m.thrift")
	got := out.String()
	if strings.Contains(got, "No field key specified") || strings.Contains(got, "compatibility alias") {
		t.Fatalf("-nowarn drops level 1 warnings:\n%s", got)
	}
	if !strings.Contains(got, `"async" is deprecated`) {
		t.Fatalf("level 0 warnings are always printed:\n%s", got)
	}
	// Default level.
	_, got = mustLoad(t, &Loader{}, files, "m.thrift")
	if strings.Count(got, "compatibility alias") != 1 {
		t.Fatalf("the byte alias warning is printed once per run:\n%s", got)
	}
	if !strings.Contains(got, "No field key specified for a") {
		t.Fatalf("level 1 warnings print by default:\n%s", got)
	}
	if !strings.Contains(got, `Consider using the more efficient "binary" type instead of "list<byte>"`) {
		t.Fatalf("list<byte> warning:\n%s", got)
	}
	if !strings.HasPrefix(got, "[WARNING:") || !strings.Contains(got, "m.thrift:") {
		t.Fatalf("warning format:\n%s", got)
	}
}

func equalInt32(a, b []int32) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
