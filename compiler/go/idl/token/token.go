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

// Package token defines the lexical tokens of the Thrift IDL.
//
// The token set mirrors the %token declarations in
// compiler/cpp/src/thrift/thrifty.yy one for one, including the legacy
// tokens that no generator uses any more, so that the Go front end accepts
// and rejects exactly the same inputs as the C++ front end.
package token

import "strconv"

// Kind identifies a token class.
type Kind int

// Token kinds. The order groups symbols, then literals, then keywords; the
// numeric values carry no meaning.
const (
	EOF Kind = iota
	Illegal

	// Literals and identifiers.
	Identifier  // [a-zA-Z_](\.[a-zA-Z_0-9]|[a-zA-Z_0-9])*
	Literal     // a quoted string, escapes already decoded
	IntConstant // decimal or 0x hexadecimal, or the keywords true and false
	DubConstant // a floating point literal

	// Single-character symbols.
	Colon     // :
	Semicolon // ;
	Comma     // ,
	LBrace    // {
	RBrace    // }
	LParen    // (
	RParen    // )
	Equals    // =
	Less      // <
	Greater   // >
	LBracket  // [
	RBracket  // ]
	Star      // *

	// Header keywords.
	Include
	Namespace
	CppInclude
	CppType
	XsdAll
	XsdOptional
	XsdNillable
	XsdAttrs

	// Base type keywords.
	Void
	Bool
	String
	Binary
	UUID
	Byte
	I8
	I16
	I32
	I64
	Double

	// Container keywords.
	Map
	List
	Set

	// Function modifiers.
	Oneway
	Async

	// Declaration keywords.
	Typedef
	Struct
	Xception
	Throws
	Extends
	Service
	Enum
	Const
	Required
	Optional
	Union
	Reference // &
)

var kindNames = map[Kind]string{
	EOF:         "end of file",
	Illegal:     "illegal token",
	Identifier:  "identifier",
	Literal:     "string literal",
	IntConstant: "integer constant",
	DubConstant: "floating point constant",
	Colon:       ":",
	Semicolon:   ";",
	Comma:       ",",
	LBrace:      "{",
	RBrace:      "}",
	LParen:      "(",
	RParen:      ")",
	Equals:      "=",
	Less:        "<",
	Greater:     ">",
	LBracket:    "[",
	RBracket:    "]",
	Star:        "*",
	Include:     "include",
	Namespace:   "namespace",
	CppInclude:  "cpp_include",
	CppType:     "cpp_type",
	XsdAll:      "xsd_all",
	XsdOptional: "xsd_optional",
	XsdNillable: "xsd_nillable",
	XsdAttrs:    "xsd_attrs",
	Void:        "void",
	Bool:        "bool",
	String:      "string",
	Binary:      "binary",
	UUID:        "uuid",
	Byte:        "byte",
	I8:          "i8",
	I16:         "i16",
	I32:         "i32",
	I64:         "i64",
	Double:      "double",
	Map:         "map",
	List:        "list",
	Set:         "set",
	Oneway:      "oneway",
	Async:       "async",
	Typedef:     "typedef",
	Struct:      "struct",
	Xception:    "exception",
	Throws:      "throws",
	Extends:     "extends",
	Service:     "service",
	Enum:        "enum",
	Const:       "const",
	Required:    "required",
	Optional:    "optional",
	Union:       "union",
	Reference:   "&",
}

// String returns the keyword or symbol spelling of the kind, or a
// description for the token classes that have no fixed spelling.
func (k Kind) String() string {
	if s, ok := kindNames[k]; ok {
		return s
	}
	return "token(" + strconv.Itoa(int(k)) + ")"
}

// Keywords maps every reserved word to its kind. The lexer consults it after
// scanning an identifier, which reproduces flex's longest-match rule: a
// keyword only wins when the identifier is exactly the keyword.
var Keywords = map[string]Kind{
	"namespace":    Namespace,
	"cpp_include":  CppInclude,
	"cpp_type":     CppType,
	"xsd_all":      XsdAll,
	"xsd_optional": XsdOptional,
	"xsd_nillable": XsdNillable,
	"xsd_attrs":    XsdAttrs,
	"include":      Include,
	"void":         Void,
	"bool":         Bool,
	"byte":         Byte,
	"i8":           I8,
	"i16":          I16,
	"i32":          I32,
	"i64":          I64,
	"double":       Double,
	"string":       String,
	"binary":       Binary,
	"uuid":         UUID,
	"map":          Map,
	"list":         List,
	"set":          Set,
	"oneway":       Oneway,
	"typedef":      Typedef,
	"struct":       Struct,
	"union":        Union,
	"exception":    Xception,
	"extends":      Extends,
	"throws":       Throws,
	"service":      Service,
	"enum":         Enum,
	"const":        Const,
	"required":     Required,
	"optional":     Optional,
	"async":        Async,
}

// IsKeyword reports whether the kind is a reserved word.
func (k Kind) IsKeyword() bool {
	return k >= Include && k <= Union
}

// Token is one lexical unit with its position.
type Token struct {
	Kind Kind
	// Line is the 1-based line on which the token ends, which is what the
	// C++ compiler's yylineno reports when the token is returned.
	Line int
	// Text is the identifier or the decoded string literal.
	Text string
	// Int is the value of an IntConstant.
	Int int64
	// Float is the value of a DubConstant.
	Float float64
}

// String renders the token for error messages.
func (t Token) String() string {
	switch t.Kind {
	case Identifier:
		return "identifier " + strconv.Quote(t.Text)
	case Literal:
		return "string " + strconv.Quote(t.Text)
	case IntConstant:
		return "integer " + strconv.FormatInt(t.Int, 10)
	case DubConstant:
		return "number " + strconv.FormatFloat(t.Float, 'g', -1, 64)
	}
	return strconv.Quote(t.Kind.String())
}
