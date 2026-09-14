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

// Package parser builds the syntax tree of one Thrift IDL file.
//
// It is a recursive-descent port of compiler/cpp/src/thrift/thrifty.yy, one
// method per grammar rule. The grammar is conflict-free LALR(1), so one
// token of lookahead is always enough. The parser fetches its lookahead at
// the same points where bison would, because doc comments attach to the
// token that follows them and the C++ compiler's program-doc heuristics
// depend on when that token is read.
package parser

import (
	"fmt"

	"github.com/apache/thrift/compiler/go/idl/ast"
	"github.com/apache/thrift/compiler/go/idl/scanner"
	"github.com/apache/thrift/compiler/go/idl/token"
)

// Error is a syntax error with the line it was detected on.
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

// Parser holds the state of one parse.
type Parser struct {
	sc     *scanner.Scanner
	tok    token.Token
	hasTok bool
	doc    docState
	// Warn receives the warnings that the C++ lexer prints.
	Warn func(line int, msg string)
}

// Parse parses one file's source. The path is recorded in the result and
// is not opened.
func Parse(path string, src []byte, warn func(line int, msg string)) (prog *ast.Program, err error) {
	p := &Parser{sc: scanner.New(src), Warn: warn}
	p.sc.OnDoc = p.doc.onDoc
	p.sc.OnWarning = warn
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*Error); ok {
				prog, err = nil, e
				return
			}
			if e, ok := r.(*scanner.Error); ok {
				prog, err = nil, &Error{Line: e.Line, Msg: e.Msg}
				return
			}
			panic(r)
		}
	}()
	prog = &ast.Program{Path: path}
	p.parseProgram(prog)
	return prog, nil
}

func (p *Parser) fail(line int, format string, args ...interface{}) {
	panic(&Error{Line: line, Msg: fmt.Sprintf(format, args...)})
}

// peek returns the lookahead token, reading it if necessary.
func (p *Parser) peek() token.Token {
	if !p.hasTok {
		t, err := p.sc.Next()
		if err != nil {
			panic(err)
		}
		p.tok, p.hasTok = t, true
	}
	return p.tok
}

func (p *Parser) next() token.Token {
	t := p.peek()
	p.hasTok = false
	return t
}

func (p *Parser) expect(k token.Kind) token.Token {
	t := p.peek()
	if t.Kind != k {
		p.fail(t.Line, "syntax error: unexpected %s, expecting %q", t, k.String())
	}
	return p.next()
}

func (p *Parser) accept(k token.Kind) bool {
	if p.peek().Kind == k {
		p.next()
		return true
	}
	return false
}

// Program: HeaderList DefinitionList
func (p *Parser) parseProgram(prog *ast.Program) {
	for {
		t := p.peek()
		if t.Kind != token.Include && t.Kind != token.Namespace && t.Kind != token.CppInclude {
			break
		}
		p.doc.destroy()
		prog.Headers = append(prog.Headers, p.parseHeader())
	}
	for {
		t := p.peek()
		if !startsDefinition(t.Kind) {
			break
		}
		doc, hasDoc := p.doc.capture()
		def := p.parseDefinition()
		if hasDoc {
			setDefinitionDoc(def, doc)
			p.doc.setDoc()
		}
		prog.Definitions = append(prog.Definitions, def)
	}
	if t := p.peek(); t.Kind != token.EOF {
		p.fail(t.Line, "syntax error: unexpected %s", t)
	}
	if doc, ok := p.doc.programDoc(); ok {
		prog.Doc = doc
		p.doc.setDoc()
	}
	p.doc.destroy()
}

func startsDefinition(k token.Kind) bool {
	switch k {
	case token.Const, token.Typedef, token.Enum, token.Struct, token.Union, token.Xception, token.Service:
		return true
	}
	return false
}

func setDefinitionDoc(def ast.Definition, doc string) {
	switch d := def.(type) {
	case *ast.Const:
		d.Doc = doc
	case *ast.Typedef:
		d.Doc = doc
	case *ast.Enum:
		d.Doc = doc
	case *ast.Struct:
		d.Doc = doc
	case *ast.Service:
		d.Doc = doc
	}
}

// Header: Include | namespace id id TypeAnnotations | namespace * id | cpp_include literal
func (p *Parser) parseHeader() ast.Header {
	t := p.next()
	switch t.Kind {
	case token.Include:
		lit := p.expect(token.Literal)
		p.doc.declareValid()
		return &ast.Include{Path: lit.Text, Line: lit.Line}
	case token.CppInclude:
		lit := p.expect(token.Literal)
		p.doc.declareValid()
		return &ast.CppInclude{Path: lit.Text, Line: lit.Line}
	case token.Namespace:
		if p.accept(token.Star) {
			name := p.expect(token.Identifier)
			p.doc.declareValid()
			return &ast.Namespace{Scope: "*", Name: name.Text, Line: name.Line}
		}
		scope := p.expect(token.Identifier)
		name := p.expect(token.Identifier)
		ann := p.parseTypeAnnotations()
		p.doc.declareValid()
		return &ast.Namespace{Scope: scope.Text, Name: name.Text, Annotations: ann, Line: name.Line}
	}
	p.fail(t.Line, "syntax error: unexpected %s", t)
	return nil
}

func (p *Parser) parseDefinition() ast.Definition {
	t := p.peek()
	switch t.Kind {
	case token.Const:
		return p.parseConst()
	case token.Typedef:
		return p.parseTypedef()
	case token.Enum:
		return p.parseEnum()
	case token.Struct, token.Union:
		return p.parseStruct()
	case token.Xception:
		return p.parseXception()
	case token.Service:
		return p.parseService()
	}
	p.fail(t.Line, "syntax error: unexpected %s", t)
	return nil
}

// CommaOrSemicolonOptional
func (p *Parser) separator() {
	if k := p.peek().Kind; k == token.Comma || k == token.Semicolon {
		p.next()
	}
}

// Typedef: typedef FieldType identifier TypeAnnotations CommaOrSemicolonOptional
func (p *Parser) parseTypedef() *ast.Typedef {
	p.expect(token.Typedef)
	typ := p.parseFieldType()
	name := p.expect(token.Identifier)
	ann := p.parseTypeAnnotations()
	p.separator()
	return &ast.Typedef{Type: typ, Name: name.Text, Annotations: ann, Line: name.Line}
}

// Enum: enum identifier { EnumDefList } TypeAnnotations
func (p *Parser) parseEnum() *ast.Enum {
	p.expect(token.Enum)
	name := p.expect(token.Identifier)
	p.expect(token.LBrace)
	e := &ast.Enum{Name: name.Text, Line: name.Line}
	for p.peek().Kind != token.RBrace {
		// EnumDef: CaptureDocText EnumValue TypeAnnotations CommaOrSemicolonOptional
		doc, hasDoc := p.doc.capture()
		id := p.expect(token.Identifier)
		v := &ast.EnumValue{Name: id.Text, Line: id.Line}
		if p.accept(token.Equals) {
			iv := p.expect(token.IntConstant)
			v.Value, v.HasValue = iv.Int, true
		}
		v.Annotations = p.parseTypeAnnotations()
		p.separator()
		if hasDoc {
			v.Doc = doc
			p.doc.setDoc()
		}
		e.Values = append(e.Values, v)
	}
	p.expect(token.RBrace)
	e.Annotations = p.parseTypeAnnotations()
	return e
}

// Const: const FieldType identifier = ConstValue CommaOrSemicolonOptional
func (p *Parser) parseConst() *ast.Const {
	p.expect(token.Const)
	typ := p.parseFieldType()
	name := p.expect(token.Identifier)
	p.expect(token.Equals)
	val := p.parseConstValue()
	p.separator()
	return &ast.Const{Type: typ, Name: name.Text, Value: val, Line: name.Line}
}

// ConstValue: int | double | literal | identifier | ConstList | ConstMap
func (p *Parser) parseConstValue() *ast.ConstValue {
	t := p.peek()
	switch t.Kind {
	case token.IntConstant:
		p.next()
		return &ast.ConstValue{Kind: ast.ConstInt, Int: t.Int, Line: t.Line}
	case token.DubConstant:
		p.next()
		return &ast.ConstValue{Kind: ast.ConstDouble, Double: t.Float, Line: t.Line}
	case token.Literal:
		p.next()
		return &ast.ConstValue{Kind: ast.ConstString, Str: t.Text, Line: t.Line}
	case token.Identifier:
		p.next()
		return &ast.ConstValue{Kind: ast.ConstIdentifier, Ident: t.Text, Line: t.Line}
	case token.LBracket:
		p.next()
		v := &ast.ConstValue{Kind: ast.ConstList, Line: t.Line}
		for p.peek().Kind != token.RBracket {
			v.List = append(v.List, p.parseConstValue())
			p.separator()
		}
		p.expect(token.RBracket)
		return v
	case token.LBrace:
		p.next()
		v := &ast.ConstValue{Kind: ast.ConstMap, Line: t.Line}
		for p.peek().Kind != token.RBrace {
			k := p.parseConstValue()
			p.expect(token.Colon)
			val := p.parseConstValue()
			p.separator()
			v.Map = append(v.Map, ast.ConstMapEntry{Key: k, Value: val})
		}
		p.expect(token.RBrace)
		return v
	}
	p.fail(t.Line, "syntax error: unexpected %s", t)
	return nil
}

// Struct: StructHead identifier XsdAll { FieldList } TypeAnnotations
func (p *Parser) parseStruct() *ast.Struct {
	head := p.next()
	s := &ast.Struct{Kind: ast.StructStruct}
	if head.Kind == token.Union {
		s.Kind = ast.StructUnion
	}
	name := p.expect(token.Identifier)
	s.Name, s.Line = name.Text, name.Line
	s.XsdAll = p.accept(token.XsdAll)
	p.expect(token.LBrace)
	s.Fields = p.parseFieldList(token.RBrace)
	p.expect(token.RBrace)
	s.Annotations = p.parseTypeAnnotations()
	return s
}

// Xception: exception identifier { FieldList } TypeAnnotations
func (p *Parser) parseXception() *ast.Struct {
	p.expect(token.Xception)
	name := p.expect(token.Identifier)
	s := &ast.Struct{Kind: ast.StructException, Name: name.Text, Line: name.Line}
	p.expect(token.LBrace)
	s.Fields = p.parseFieldList(token.RBrace)
	p.expect(token.RBrace)
	s.Annotations = p.parseTypeAnnotations()
	return s
}

// Service: service identifier Extends { FunctionList } TypeAnnotations
func (p *Parser) parseService() *ast.Service {
	p.expect(token.Service)
	name := p.expect(token.Identifier)
	s := &ast.Service{Name: name.Text, Line: name.Line}
	if p.accept(token.Extends) {
		base := p.expect(token.Identifier)
		s.Extends, s.HasExtends = base.Text, true
	}
	p.expect(token.LBrace)
	for p.peek().Kind != token.RBrace {
		s.Functions = append(s.Functions, p.parseFunction())
	}
	p.expect(token.RBrace)
	s.Annotations = p.parseTypeAnnotations()
	return s
}

// Function: CaptureDocText Oneway FunctionType identifier ( FieldList ) Throws TypeAnnotations CommaOrSemicolonOptional
func (p *Parser) parseFunction() *ast.Function {
	doc, hasDoc := p.doc.capture()
	f := &ast.Function{}
	if k := p.peek().Kind; k == token.Oneway || k == token.Async {
		p.next()
		f.Oneway = true
	}
	if p.peek().Kind == token.Void {
		p.next()
	} else {
		f.ReturnType = p.parseFieldType()
	}
	name := p.expect(token.Identifier)
	f.Name, f.Line = name.Text, name.Line
	p.expect(token.LParen)
	f.Args = p.parseFieldList(token.RParen)
	p.expect(token.RParen)
	if p.accept(token.Throws) {
		p.expect(token.LParen)
		f.Throws = p.parseFieldList(token.RParen)
		f.HasThrows = true
		p.expect(token.RParen)
	}
	f.Annotations = p.parseTypeAnnotations()
	p.separator()
	if hasDoc {
		f.Doc = doc
		p.doc.setDoc()
	}
	return f
}

// FieldList: fields until the closing token. The closer is not consumed.
func (p *Parser) parseFieldList(closer token.Kind) []*ast.Field {
	var fields []*ast.Field
	for {
		t := p.peek()
		if t.Kind == closer || t.Kind == token.EOF {
			return fields
		}
		fields = append(fields, p.parseField())
	}
}

// Field: CaptureDocText FieldIdentifier FieldRequiredness FieldType FieldReference FieldName FieldValue XsdOptional XsdNillable XsdAttributes TypeAnnotations CommaOrSemicolonOptional
func (p *Parser) parseField() *ast.Field {
	doc, hasDoc := p.doc.capture()
	f := &ast.Field{}
	if t := p.peek(); t.Kind == token.IntConstant {
		p.next()
		p.expect(token.Colon)
		f.ID, f.HasID = t.Int, true
	}
	switch p.peek().Kind {
	case token.Required:
		p.next()
		f.Req = ast.ReqRequired
	case token.Optional:
		p.next()
		f.Req = ast.ReqOptional
	}
	f.Type = p.parseFieldType()
	f.Reference = p.accept(token.Reference)
	name := p.parseFieldName()
	f.Name, f.Line = name.Text, name.Line
	if p.accept(token.Equals) {
		f.Default = p.parseConstValue()
	}
	f.XsdOptional = p.accept(token.XsdOptional)
	f.XsdNillable = p.accept(token.XsdNillable)
	if p.accept(token.XsdAttrs) {
		p.expect(token.LBrace)
		f.XsdAttrs = p.parseFieldList(token.RBrace)
		f.HasXsdAttrs = true
		p.expect(token.RBrace)
	}
	f.Annotations = p.parseTypeAnnotations()
	p.separator()
	if hasDoc {
		f.Doc = doc
		p.doc.setDoc()
	}
	return f
}

// FieldName: an identifier or any keyword the grammar allows as a field
// name. The C++ grammar lists them explicitly; the excluded ones are the
// xsd_* keywords, cpp_type, and the symbols.
func (p *Parser) parseFieldName() token.Token {
	t := p.peek()
	switch t.Kind {
	case token.Identifier:
		p.next()
		return t
	case token.Namespace, token.CppInclude, token.Include, token.Void, token.Bool,
		token.Byte, token.I8, token.I16, token.I32, token.I64, token.Double,
		token.String, token.Binary, token.UUID, token.Map, token.List, token.Set,
		token.Oneway, token.Async, token.Typedef, token.Struct, token.Union,
		token.Xception, token.Extends, token.Throws, token.Service, token.Enum,
		token.Const, token.Required, token.Optional:
		p.next()
		t.Text = t.Kind.String()
		return t
	}
	p.fail(t.Line, "syntax error: unexpected %s, expecting a field name", t)
	return t
}

var baseKinds = map[token.Kind]ast.BaseKind{
	token.String: ast.BaseString,
	token.Binary: ast.BaseBinary,
	token.UUID:   ast.BaseUUID,
	token.Bool:   ast.BaseBool,
	token.Byte:   ast.BaseI8,
	token.I8:     ast.BaseI8,
	token.I16:    ast.BaseI16,
	token.I32:    ast.BaseI32,
	token.I64:    ast.BaseI64,
	token.Double: ast.BaseDouble,
}

// FieldType: identifier | BaseType | ContainerType
func (p *Parser) parseFieldType() ast.TypeRef {
	t := p.peek()
	if t.Kind == token.Identifier {
		p.next()
		return &ast.NamedTypeRef{Name: t.Text, Line: t.Line}
	}
	if bk, ok := baseKinds[t.Kind]; ok {
		p.next()
		return &ast.BaseTypeRef{Kind: bk, Annotations: p.parseTypeAnnotations()}
	}
	switch t.Kind {
	case token.Map:
		p.next()
		m := &ast.MapTypeRef{}
		m.CppType, m.HasCppType = p.parseCppType()
		p.expect(token.Less)
		m.Key = p.parseFieldType()
		p.expect(token.Comma)
		m.Value = p.parseFieldType()
		p.expect(token.Greater)
		m.Annotations = p.parseTypeAnnotations()
		return m
	case token.Set:
		p.next()
		s := &ast.SetTypeRef{}
		s.CppType, s.HasCppType = p.parseCppType()
		p.expect(token.Less)
		s.Elem = p.parseFieldType()
		p.expect(token.Greater)
		s.Annotations = p.parseTypeAnnotations()
		return s
	case token.List:
		p.next()
		l := &ast.ListTypeRef{Line: t.Line}
		l.CppType, l.HasCppType = p.parseCppType()
		p.expect(token.Less)
		l.Elem = p.parseFieldType()
		p.expect(token.Greater)
		l.TrailingCppType, l.HasTrailingCpp = p.parseCppType()
		l.Annotations = p.parseTypeAnnotations()
		return l
	}
	p.fail(t.Line, "syntax error: unexpected %s, expecting a type", t)
	return nil
}

// CppType: cpp_type literal | empty
func (p *Parser) parseCppType() (string, bool) {
	if p.accept(token.CppType) {
		lit := p.expect(token.Literal)
		return lit.Text, true
	}
	return "", false
}

// TypeAnnotations: ( TypeAnnotationList ) | empty
func (p *Parser) parseTypeAnnotations() []ast.Annotation {
	if !p.accept(token.LParen) {
		return nil
	}
	var anns []ast.Annotation
	for p.peek().Kind != token.RParen {
		key := p.expect(token.Identifier)
		value := "1"
		if p.accept(token.Equals) {
			value = p.expect(token.Literal).Text
		}
		p.separator()
		anns = append(anns, ast.Annotation{Key: key.Text, Value: value})
	}
	p.expect(token.RParen)
	return anns
}
