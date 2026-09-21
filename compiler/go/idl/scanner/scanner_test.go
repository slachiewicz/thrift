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

package scanner

import (
	"math"
	"strings"
	"testing"

	"github.com/apache/thrift/compiler/go/idl/token"
)

// tok is the part of a token the tests compare.
type tok struct {
	kind  token.Kind
	text  string
	line  int
	i     int64
	f     float64
	isDub bool
}

// scanAll returns every token up to and including EOF, or the error.
func scanAll(src string, opts ...func(*Scanner)) ([]tok, error) {
	s := New([]byte(src))
	for _, o := range opts {
		o(s)
	}
	var out []tok
	for {
		t, err := s.Next()
		if err != nil {
			return out, err
		}
		out = append(out, tok{kind: t.Kind, text: t.Text, line: t.Line, i: t.Int, f: t.Float, isDub: t.Kind == token.DubConstant})
		if t.Kind == token.EOF {
			return out, nil
		}
	}
}

func kinds(ts []tok) []token.Kind {
	var ks []token.Kind
	for _, t := range ts {
		ks = append(ks, t.kind)
	}
	return ks
}

func equalKinds(a, b []token.Kind) bool {
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

func mustScan(t *testing.T, src string) []tok {
	t.Helper()
	ts, err := scanAll(src)
	if err != nil {
		t.Fatalf("%q: %v", src, err)
	}
	return ts
}

func TestKeywordsAndSymbols(t *testing.T) {
	src := "include namespace cpp_include cpp_type xsd_all xsd_optional xsd_nillable xsd_attrs " +
		"void bool string binary uuid byte i8 i16 i32 i64 double map list set oneway async " +
		"typedef struct exception throws extends service enum const required optional union " +
		": ; , { } ( ) = < > [ ] * &"
	want := []token.Kind{
		token.Include, token.Namespace, token.CppInclude, token.CppType, token.XsdAll, token.XsdOptional, token.XsdNillable, token.XsdAttrs,
		token.Void, token.Bool, token.String, token.Binary, token.UUID, token.Byte, token.I8, token.I16, token.I32, token.I64, token.Double, token.Map, token.List, token.Set, token.Oneway, token.Async,
		token.Typedef, token.Struct, token.Xception, token.Throws, token.Extends, token.Service, token.Enum, token.Const, token.Required, token.Optional, token.Union,
		token.Colon, token.Semicolon, token.Comma, token.LBrace, token.RBrace, token.LParen, token.RParen, token.Equals, token.Less, token.Greater, token.LBracket, token.RBracket, token.Star, token.Reference,
		token.EOF,
	}
	var warned []string
	ts, err := scanAll(src, func(s *Scanner) {
		s.OnWarning = func(line, level int, msg string) { warned = append(warned, msg) }
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := kinds(ts); !equalKinds(got, want) {
		t.Fatalf("kinds:\n got %v\nwant %v", got, want)
	}
	if len(warned) != 2 || warned[0] != ByteAliasWarning || warned[1] != AsyncWarning {
		t.Fatalf("warnings = %q", warned)
	}
	// Keywords are case sensitive: "Struct" is an identifier.
	if ts := mustScan(t, "Struct STRING"); ts[0].kind != token.Identifier || ts[1].kind != token.Identifier {
		t.Fatalf("keywords are case sensitive, got %v", kinds(ts))
	}
}

func TestWarningLevelsAndLines(t *testing.T) {
	type w struct {
		line, level int
		msg         string
	}
	var got []w
	_, err := scanAll("i32\nbyte\n\nasync byte", func(s *Scanner) {
		s.OnWarning = func(line, level int, msg string) { got = append(got, w{line, level, msg}) }
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []w{{2, 1, ByteAliasWarning}, {4, 0, AsyncWarning}, {4, 1, ByteAliasWarning}}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("warning %d = %v, want %v", i, got[i], want[i])
		}
	}
}

func TestIdentifiers(t *testing.T) {
	cases := map[string]string{
		"foo":         "foo",
		"_x1":         "_x1",
		"a.b.c":       "a.b.c",
		"e10":         "e10", // longest match: identifier beats the exponent-only double
		"E":           "E",
		"a1.2b":       "a1.2b",
		"Thrift.Test": "Thrift.Test",
	}
	for src, want := range cases {
		ts := mustScan(t, src)
		if ts[0].kind != token.Identifier || ts[0].text != want {
			t.Errorf("%q: got %v %q, want identifier %q", src, ts[0].kind, ts[0].text, want)
		}
	}
	// A dot is only consumed when an identifier character follows it, and
	// a lone dot matches no rule.
	ts, err := scanAll("a.b.")
	if len(ts) != 1 || ts[0].text != "a.b" || err == nil || !strings.Contains(err.Error(), `Unexpected token in input: "."`) {
		t.Fatalf("a.b.: %v %v", ts, err)
	}
	// "1abc" is an integer followed by an identifier.
	ts = mustScan(t, "1abc")
	if got := kinds(ts); !equalKinds(got, []token.Kind{token.IntConstant, token.Identifier, token.EOF}) {
		t.Fatalf("1abc: %v", got)
	}
}

func TestIntegers(t *testing.T) {
	cases := map[string]int64{
		"0":                    0,
		"42":                   42,
		"+42":                  42,
		"-42":                  -42,
		"0x1F":                 31,
		"0Xff":                 0, // 0X is not hex: "0" then identifier "Xff"
		"-0x10":                -16,
		"+0x10":                16,
		"9223372036854775807":  math.MaxInt64,
		"-9223372036854775808": math.MinInt64,
		"007":                  7,
		"true":                 1,
		"false":                0,
	}
	for src, want := range cases {
		ts := mustScan(t, src)
		if ts[0].kind != token.IntConstant || ts[0].i != want {
			t.Errorf("%q: got %v %d, want integer %d", src, ts[0].kind, ts[0].i, want)
		}
	}
	if ts := mustScan(t, "0Xff"); len(ts) != 3 || ts[1].kind != token.Identifier || ts[1].text != "Xff" {
		t.Fatalf("0Xff: %v", ts)
	}
	for _, src := range []string{"9223372036854775808", "-9223372036854775809", "0x10000000000000000"} {
		_, err := scanAll(src)
		if err == nil || !strings.Contains(err.Error(), "This integer is too big") {
			t.Errorf("%q: err = %v", src, err)
		}
	}
}

func TestDoubles(t *testing.T) {
	cases := map[string]float64{
		"1.5":    1.5,
		"-1.5":   -1.5,
		"+1.5":   1.5,
		".5":     0.5,
		"1e3":    1000,
		"1.5e-3": 0.0015,
		"1E+2":   100,
		"-":      0, // a bare sign matches the double pattern with atof value 0
		"+":      0,
		"1e400":  math.Inf(1),
		"-1e400": math.Inf(-1),
	}
	for src, want := range cases {
		ts := mustScan(t, src)
		if ts[0].kind != token.DubConstant || ts[0].f != want {
			t.Errorf("%q: got %v %v, want double %v", src, ts[0].kind, ts[0].f, want)
		}
	}
	// "1." is the integer 1; the dot needs a digit after it to be part of
	// a double, and a lone "." matches no rule.
	_, err := scanAll("1.")
	if err == nil || !strings.Contains(err.Error(), `Unexpected token in input: "."`) {
		t.Fatalf("1.: err = %v", err)
	}
	// Tie-break: "1" matches int and double at the same length; the int
	// rule comes first in the flex file.
	if ts := mustScan(t, "1"); ts[0].kind != token.IntConstant {
		t.Fatalf("1 should be an integer, got %v", ts[0].kind)
	}
	// "-" followed by a digit is a negative integer, not a double.
	if ts := mustScan(t, "-1"); ts[0].kind != token.IntConstant || ts[0].i != -1 {
		t.Fatalf("-1: %v", ts[0])
	}
}

func TestLiterals(t *testing.T) {
	cases := map[string]string{
		`"abc"`:            "abc",
		`'abc'`:            "abc",
		`"a'b"`:            "a'b",
		`'a"b'`:            `a"b`,
		`""`:               "",
		`"\n\t\r"`:         "\n\t\r",
		`"\"\'\\"`:         `"'\`,
		`"tab	inside"`:     "tab\tinside",
		`"unicode ą"`:      "unicode ą",
		`"a//b /* c */ #"`: "a//b /* c */ #",
	}
	for src, want := range cases {
		ts := mustScan(t, src)
		if ts[0].kind != token.Literal || ts[0].text != want {
			t.Errorf("%s: got %v %q, want literal %q", src, ts[0].kind, ts[0].text, want)
		}
	}
	errs := map[string]string{
		`"abc`:       "End of file while reading string at 1",
		"\"a\nb\"":   "End of line while reading string at 1",
		`"a\qb"`:     `Invalid escape sequence '\q'`,
		`"a\`:        "End of file while reading string",
		"\"a\\\nb\"": "End of line while reading string",
	}
	for src, want := range errs {
		_, err := scanAll(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", src, err, want)
		}
	}
}

func TestComments(t *testing.T) {
	src := "a // line comment\nb # hash comment\nc /* block\ncomment */ d /***/ e /**/ f /*****/ g"
	ts := mustScan(t, src)
	var names []string
	for _, t := range ts[:len(ts)-1] {
		names = append(names, t.text)
	}
	if got := strings.Join(names, " "); got != "a b c d e f g" {
		t.Fatalf("tokens = %q", got)
	}
	if ts[3].line != 4 || ts[0].line != 1 || ts[1].line != 2 {
		t.Fatalf("lines = %d %d %d", ts[0].line, ts[1].line, ts[3].line)
	}
	for src, want := range map[string]string{
		"/* open":  "Unexpected end of file in multiline comment at 1",
		"/** open": "Unexpected end of file in doc-comment at 1",
	} {
		_, err := scanAll(src)
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%q: err = %v, want %q", src, err, want)
		}
	}
}

func TestDocComments(t *testing.T) {
	type doc struct {
		raw  string
		line int
	}
	var docs []doc
	ts, err := scanAll("/** one */ a\n/**\n * two\n */\nb /**/ c /***/ d", func(s *Scanner) {
		s.OnDoc = func(raw string, line int) { docs = append(docs, doc{raw, line}) }
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := kinds(ts); !equalKinds(got, []token.Kind{token.Identifier, token.Identifier, token.Identifier, token.Identifier, token.EOF}) {
		t.Fatalf("kinds = %v", got)
	}
	// The raw text excludes "/**" and has "*/" replaced by one space; the
	// line is the one the comment ends on.
	want := []doc{{" one  ", 1}, {"\n * two\n  ", 4}}
	if len(docs) != len(want) {
		t.Fatalf("docs = %v", docs)
	}
	for i := range want {
		if docs[i] != want[i] {
			t.Fatalf("doc %d = %v, want %v", i, docs[i], want[i])
		}
	}
}

func TestByteOrderMarkAndWhitespace(t *testing.T) {
	ts := mustScan(t, "\xEF\xBB\xBF  \t\r\n struct")
	if ts[0].kind != token.Struct || ts[0].line != 2 {
		t.Fatalf("got %v line %d", ts[0].kind, ts[0].line)
	}
	if ts := mustScan(t, ""); ts[0].kind != token.EOF || ts[0].line != 1 {
		t.Fatalf("empty input: %v", ts)
	}
}

func TestUnexpectedByte(t *testing.T) {
	for _, src := range []string{"@", "struct S {\n  1: i32 a ~\n}", "$x", "`"} {
		_, err := scanAll(src)
		if err == nil || !strings.Contains(err.Error(), "Unexpected token in input") {
			t.Errorf("%q: err = %v", src, err)
		}
	}
	_, err := scanAll("a\nb\n~")
	if e, ok := err.(*Error); !ok || e.Line != 3 {
		t.Fatalf("error line: %v", err)
	}
}

func TestLineNumbers(t *testing.T) {
	ts := mustScan(t, "a\n\nb\r\nc")
	if ts[0].line != 1 || ts[1].line != 3 || ts[2].line != 4 {
		t.Fatalf("lines = %d %d %d", ts[0].line, ts[1].line, ts[2].line)
	}
}

func TestPositions(t *testing.T) {
	src := "struct S {\n\t1: i32 a // c\n  2: string /* x */ b\n}"
	s := New([]byte(src))
	var got []token.Pos
	for {
		tok, err := s.Next()
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, tok.Pos)
		if tok.Kind == token.EOF {
			break
		}
	}
	want := []token.Pos{
		{1, 1}, {1, 8}, {1, 10}, // struct S {
		{2, 2}, {2, 3}, {2, 5}, {2, 9}, // 1 : i32 a
		{3, 3}, {3, 4}, {3, 6}, {3, 21}, // 2 : string b
		{4, 1}, {4, 2}, // } EOF
	}
	if len(got) != len(want) {
		t.Fatalf("got %d positions %v, want %d", len(got), got, len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("token %d at %v, want %v", i, got[i], want[i])
		}
	}
}

func TestErrorPosition(t *testing.T) {
	_, err := scanAll("struct S {\n  1: i32 a = \"unterminated")
	e, ok := err.(*Error)
	if !ok {
		t.Fatalf("got %v, want *Error", err)
	}
	if e.Line != 2 || e.Pos.Line != 2 || e.Pos.Col < 14 {
		t.Errorf("error at line %d pos %v, want line 2 and a column past the quote", e.Line, e.Pos)
	}
}
