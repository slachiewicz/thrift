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

// Package scanner turns Thrift IDL source into tokens.
//
// It is a hand-written port of compiler/cpp/src/thrift/thriftl.ll. Where
// flex picks the longest match and breaks ties by rule order, this scanner
// computes the candidate lengths explicitly and applies the same tie-break,
// so that inputs such as "e10" (an identifier, THRIFT-3477) and "-" (a
// floating point constant with value zero) lex the way the C++ compiler
// lexes them.
package scanner

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/idl/token"
)

// Error is a lexical error with the line it was detected on.
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("line %d: %s", e.Line, e.Msg)
}

// Scanner produces tokens from a byte slice.
type Scanner struct {
	src  []byte
	off  int
	line int

	// OnDoc, when set, receives the raw text of every doc comment
	// ("/** ... */") together with the line on which the comment ends. The
	// text excludes the opening "/**" and has the closing "*/" replaced by a
	// single space, exactly as the C++ lexer hands it to clean_up_doctext.
	OnDoc func(raw string, line int)

	// OnWarning, when set, receives the warnings the C++ lexer prints:
	// the "byte" alias notice and the "async" deprecation notice.
	OnWarning func(line int, msg string)
}

// New returns a scanner positioned at the start of src. A UTF-8 byte-order
// mark at the start is skipped, as the C++ compiler does.
func New(src []byte) *Scanner {
	src = bytes.TrimPrefix(src, []byte{0xEF, 0xBB, 0xBF})
	return &Scanner{src: src, line: 1}
}

// Line returns the current line number.
func (s *Scanner) Line() int { return s.line }

func (s *Scanner) errorf(format string, args ...interface{}) error {
	return &Error{Line: s.line, Msg: fmt.Sprintf(format, args...)}
}

func (s *Scanner) peekByte(n int) (byte, bool) {
	if s.off+n < len(s.src) {
		return s.src[s.off+n], true
	}
	return 0, false
}

// advance consumes n bytes, counting newlines.
func (s *Scanner) advance(n int) {
	for i := 0; i < n && s.off < len(s.src); i++ {
		if s.src[s.off] == '\n' {
			s.line++
		}
		s.off++
	}
}

func isIdentStart(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isIdentChar(c byte) bool {
	return isIdentStart(c) || (c >= '0' && c <= '9')
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isHexDigit(c byte) bool {
	return isDigit(c) || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func isSymbol(c byte) bool {
	return strings.IndexByte(":;,{}()=<>[]", c) >= 0
}

var symbolKinds = map[byte]token.Kind{
	':': token.Colon,
	';': token.Semicolon,
	',': token.Comma,
	'{': token.LBrace,
	'}': token.RBrace,
	'(': token.LParen,
	')': token.RParen,
	'=': token.Equals,
	'<': token.Less,
	'>': token.Greater,
	'[': token.LBracket,
	']': token.RBracket,
	'*': token.Star,
}

// identLen returns the length of the identifier at off, per the pattern
// [a-zA-Z_](\.[a-zA-Z_0-9]|[a-zA-Z_0-9])*. A dot is only consumed when an
// identifier character follows it.
func (s *Scanner) identLen() int {
	c, ok := s.peekByte(0)
	if !ok || !isIdentStart(c) {
		return 0
	}
	n := 1
	for {
		c, ok = s.peekByte(n)
		if !ok {
			return n
		}
		switch {
		case isIdentChar(c):
			n++
		case c == '.':
			d, ok := s.peekByte(n + 1)
			if ok && isIdentChar(d) {
				n += 2
			} else {
				return n
			}
		default:
			return n
		}
	}
}

// intLen matches [+-]?[0-9]+.
func (s *Scanner) intLen() int {
	n := 0
	if c, ok := s.peekByte(0); ok && (c == '+' || c == '-') {
		n = 1
	}
	start := n
	for {
		c, ok := s.peekByte(n)
		if !ok || !isDigit(c) {
			break
		}
		n++
	}
	if n == start {
		return 0
	}
	return n
}

// hexLen matches [+-]?"0x"[0-9A-Fa-f]+.
func (s *Scanner) hexLen() int {
	n := 0
	if c, ok := s.peekByte(0); ok && (c == '+' || c == '-') {
		n = 1
	}
	if c, ok := s.peekByte(n); !ok || c != '0' {
		return 0
	}
	if c, ok := s.peekByte(n + 1); !ok || c != 'x' {
		return 0
	}
	n += 2
	start := n
	for {
		c, ok := s.peekByte(n)
		if !ok || !isHexDigit(c) {
			break
		}
		n++
	}
	if n == start {
		return 0
	}
	return n
}

// dubLen matches [+-]?[0-9]*(\.[0-9]+)?([eE][+-]?[0-9]+)?. Every part is
// optional, so the match can be a bare sign; flex then returns a floating
// point constant whose atof value is zero, and so does this scanner.
func (s *Scanner) dubLen() int {
	n := 0
	if c, ok := s.peekByte(0); ok && (c == '+' || c == '-') {
		n = 1
	}
	for {
		c, ok := s.peekByte(n)
		if !ok || !isDigit(c) {
			break
		}
		n++
	}
	if c, ok := s.peekByte(n); ok && c == '.' {
		m := n + 1
		for {
			c, ok := s.peekByte(m)
			if !ok || !isDigit(c) {
				break
			}
			m++
		}
		if m > n+1 {
			n = m
		}
	}
	if c, ok := s.peekByte(n); ok && (c == 'e' || c == 'E') {
		m := n + 1
		if c, ok := s.peekByte(m); ok && (c == '+' || c == '-') {
			m++
		}
		digits := m
		for {
			c, ok := s.peekByte(m)
			if !ok || !isDigit(c) {
				break
			}
			m++
		}
		if m > digits {
			n = m
		}
	}
	return n
}

// Next returns the next token. At end of input it returns a token of kind
// token.EOF and no error, repeatedly.
func (s *Scanner) Next() (token.Token, error) {
	for {
		c, ok := s.peekByte(0)
		if !ok {
			return token.Token{Kind: token.EOF, Line: s.line}, nil
		}
		switch {
		case c == ' ' || c == '\t' || c == '\r' || c == '\n':
			s.advance(1)
			continue
		case c == '/':
			d, _ := s.peekByte(1)
			if d == '/' {
				s.skipLine()
				continue
			}
			if d == '*' {
				if err := s.blockComment(); err != nil {
					return token.Token{}, err
				}
				continue
			}
		case c == '#':
			s.skipLine()
			continue
		}
		break
	}

	c, _ := s.peekByte(0)
	line := s.line

	if k, ok := symbolKinds[c]; ok {
		s.advance(1)
		return token.Token{Kind: k, Line: line}, nil
	}
	if c == '&' {
		s.advance(1)
		return token.Token{Kind: token.Reference, Line: line}, nil
	}
	if c == '"' || c == '\'' {
		return s.literal()
	}

	if n := s.identLen(); n > 0 {
		text := string(s.src[s.off : s.off+n])
		s.advance(n)
		switch text {
		case "true":
			return token.Token{Kind: token.IntConstant, Line: s.line, Int: 1}, nil
		case "false":
			return token.Token{Kind: token.IntConstant, Line: s.line, Int: 0}, nil
		}
		if k, ok := token.Keywords[text]; ok {
			switch k {
			case token.Byte:
				s.warn(`The "byte" type is a compatibility alias for "i8". Use "i8" to emphasize the signedness of this type.`)
			case token.Async:
				s.warn(`"async" is deprecated.  It is called "oneway" now.`)
			}
			return token.Token{Kind: k, Line: s.line, Text: text}, nil
		}
		return token.Token{Kind: token.Identifier, Line: s.line, Text: text}, nil
	}

	// Numbers: longest match wins, ties go to the earlier flex rule, which
	// is intconstant, then hexconstant, then dubconstant.
	il, hl, dl := s.intLen(), s.hexLen(), s.dubLen()
	best := il
	if hl > best {
		best = hl
	}
	if dl > best {
		best = dl
	}
	if best > 0 {
		text := string(s.src[s.off : s.off+best])
		s.advance(best)
		switch {
		case il == best:
			v, err := strconv.ParseInt(text, 10, 64)
			if err != nil {
				return token.Token{}, s.errorf("This integer is too big: %q", text)
			}
			return token.Token{Kind: token.IntConstant, Line: s.line, Int: v, Text: text}, nil
		case hl == best:
			neg := strings.HasPrefix(text, "-")
			digits := strings.TrimLeft(text, "+-")[2:]
			v, err := strconv.ParseInt(digits, 16, 64)
			if err != nil {
				return token.Token{}, s.errorf("This integer is too big: %q", text)
			}
			if neg {
				v = -v
			}
			return token.Token{Kind: token.IntConstant, Line: s.line, Int: v, Text: text}, nil
		default:
			// atof semantics: an unparsable prefix such as "-" is zero.
			v, err := strconv.ParseFloat(text, 64)
			if err != nil {
				if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
					// atof saturates to +/-Inf on overflow; ParseFloat
					// returns the same value with ErrRange.
				} else {
					v = 0
				}
			}
			return token.Token{Kind: token.DubConstant, Line: s.line, Float: v, Text: text}, nil
		}
	}

	return token.Token{}, s.errorf("Unexpected token in input: %q", string(c))
}

func (s *Scanner) warn(msg string) {
	if s.OnWarning != nil {
		s.OnWarning(s.line, msg)
	}
}

func (s *Scanner) skipLine() {
	for {
		c, ok := s.peekByte(0)
		if !ok || c == '\n' {
			return
		}
		s.advance(1)
	}
}

// blockComment consumes a comment starting with "/*". A run of stars
// closed directly by "/" is a "silly comment" and is discarded; "/**"
// followed by anything else is a doc comment; "/*" is a plain comment.
func (s *Scanner) blockComment() error {
	// Silly comment: "/*" "*"* "*/".
	n := 2
	for {
		c, ok := s.peekByte(n)
		if !ok {
			break
		}
		if c == '*' {
			n++
			continue
		}
		if c == '/' && n > 2 {
			s.advance(n + 1)
			return nil
		}
		break
	}

	isDoc := false
	if c, _ := s.peekByte(2); c == '*' {
		isDoc = true
		s.advance(3)
	} else {
		s.advance(2)
	}
	start := s.off
	state := 0
	for state < 2 {
		c, ok := s.peekByte(0)
		if !ok {
			if isDoc {
				return s.errorf("Unexpected end of file in doc-comment at %d", s.line)
			}
			return s.errorf("Unexpected end of file in multiline comment at %d", s.line)
		}
		s.advance(1)
		switch c {
		case '*':
			state = 1
		case '/':
			if state == 1 {
				state = 2
			} else {
				state = 0
			}
		default:
			state = 0
		}
	}
	if isDoc && s.OnDoc != nil {
		// The C++ lexer overwrites the "*/" with " \0", so the raw text
		// ends in a single space.
		raw := string(s.src[start:s.off-2]) + " "
		s.OnDoc(raw, s.line)
	}
	return nil
}

func (s *Scanner) literal() (token.Token, error) {
	mark, _ := s.peekByte(0)
	s.advance(1)
	var result []byte
	for {
		c, ok := s.peekByte(0)
		if !ok {
			return token.Token{}, s.errorf("End of file while reading string at %d", s.line)
		}
		s.advance(1)
		switch c {
		case '\n':
			return token.Token{}, s.errorf("End of line while reading string at %d", s.line-1)
		case '\\':
			e, ok := s.peekByte(0)
			if !ok {
				return token.Token{}, s.errorf("End of file while reading string at %d", s.line)
			}
			s.advance(1)
			switch e {
			case '\n':
				return token.Token{}, s.errorf("End of line while reading string at %d", s.line-1)
			case 'r':
				result = append(result, '\r')
			case 'n':
				result = append(result, '\n')
			case 't':
				result = append(result, '\t')
			case '"':
				result = append(result, '"')
			case '\'':
				result = append(result, '\'')
			case '\\':
				result = append(result, '\\')
			default:
				if e >= 0x20 && e <= 0x7e {
					return token.Token{}, s.errorf("Invalid escape sequence '\\%c'. Use \\\\ for a literal backslash.", e)
				}
				return token.Token{}, s.errorf("Invalid escape byte 0x%02X. Use \\\\ for a literal backslash.", e)
			}
		default:
			if c == mark {
				return token.Token{Kind: token.Literal, Line: s.line, Text: string(result)}, nil
			}
			result = append(result, c)
		}
	}
}
