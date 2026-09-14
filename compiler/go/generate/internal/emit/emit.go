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

// Package emit holds the pieces of t_generator that every generator
// port shares: file writing, string escaping and the docstring loop.
package emit

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Error is a generator failure, the string a C++ generator throws.
type Error struct {
	Msg string
}

func (e *Error) Error() string { return e.Msg }

// Throw aborts generation with an Error; the generator's entry point
// recovers it.
func Throw(format string, args ...interface{}) {
	panic(&Error{Msg: fmt.Sprintf(format, args...)})
}

// WriteFile stores the content unless the file already holds it, like
// ofstream_with_content_based_conditional_update.
func WriteFile(path, content string) {
	if old, err := os.ReadFile(path); err == nil && string(old) == content {
		return
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		Throw("failed to write the output to the file '%s', details: '%s'", path, err.Error())
	}
}

// Mkdir is MKDIR: create the directory, ignoring failure.
func Mkdir(path string) {
	_ = os.MkdirAll(path, 0o755)
}

// EscapeString is t_generator::escape_string with the default escape
// table.
func EscapeString(in string) string {
	var sb strings.Builder
	for i := 0; i < len(in); i++ {
		switch c := in[i]; c {
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		case '"':
			sb.WriteString(`\"`)
		case '\\':
			sb.WriteString(`\\`)
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String()
}

// DoubleFixed16 is t_generator::emit_double_as_string: std::fixed with
// a precision of digits10 + 1.
func DoubleFixed16(v float64) string {
	return strconv.FormatFloat(v, 'f', 16, 64)
}

// DocstringComment is t_generator::generate_docstring_comment. It
// reproduces the getline loop: a line of 1024 characters or more makes
// the stream fail and drops the rest, an empty line prints the prefix
// alone unless it was produced by hitting the end of the input, and
// eofbit is only set when a read runs out of input rather than when it
// consumes the final newline.
func DocstringComment(out *strings.Builder, indent, commentStart, linePrefix, contents, commentEnd string) {
	if commentStart != "" {
		out.WriteString(indent + commentStart)
	}
	rest := contents
	for rest != "" {
		var line string
		nl := strings.IndexByte(rest, '\n')
		truncated := false
		switch {
		case nl >= 0 && nl < 1023:
			line, rest = rest[:nl], rest[nl+1:]
		case nl < 0 && len(rest) < 1024:
			line, rest = rest, ""
		default:
			line, rest = rest[:1023], ""
			truncated = true
		}
		eof := nl < 0
		if len(line) > 0 {
			out.WriteString(indent + linePrefix + line + "\n")
		} else if linePrefix == "" {
			out.WriteString("\n")
		} else if !eof {
			out.WriteString(indent + linePrefix + "\n")
		}
		if truncated {
			break
		}
	}
	if commentEnd != "" {
		out.WriteString(indent + commentEnd)
	}
}
