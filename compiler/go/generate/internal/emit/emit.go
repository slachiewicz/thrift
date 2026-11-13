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
	// Direct is set for an error the C++ compiler reports by calling
	// failure() itself, as t_generator::validate_id does, rather than by
	// throwing to generate(), which prefixes "Error: ".
	Direct bool
}

func (e *Error) Error() string { return e.Msg }

// IsDirect reports Direct, for callers outside the generate tree.
func (e *Error) IsDirect() bool { return e.Direct }

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

// DocstringComment is t_generator::generate_docstring_comment, with the
// semantics of its loop over std::istream::getline(line, 1024):
//
//   - getline stores up to 1023 characters and extracts the newline that
//     ends them, even right after the 1023rd; a longer line sets failbit
//     and keeps its first 1023 characters, and the loop stops;
//   - running out of input sets eofbit, and failbit as well when nothing
//     was extracted, but the body still runs once on the empty buffer: so
//     contents that end in a newline, or are empty, print one more blank
//     line when linePrefix is empty;
//   - an empty line prints the prefix alone unless it came from running
//     out of input.
func DocstringComment(out *strings.Builder, indent, commentStart, linePrefix, contents, commentEnd string) {
	if commentStart != "" {
		out.WriteString(indent + commentStart)
	}
	for pos, done := 0, false; !done; {
		rest := contents[pos:]
		nl := strings.IndexByte(rest, '\n')
		var line string
		eof := false
		switch {
		case nl >= 0 && nl <= 1023:
			line, pos = rest[:nl], pos+nl+1
		case nl < 0 && len(rest) <= 1023:
			line, pos, eof, done = rest, len(contents), true, true
		default:
			line, done = rest[:1023], true
		}
		if len(line) > 0 {
			out.WriteString(indent + linePrefix + line + "\n")
		} else if linePrefix == "" {
			out.WriteString("\n")
		} else if !eof {
			out.WriteString(indent + linePrefix + "\n")
		}
	}
	if commentEnd != "" {
		out.WriteString(indent + commentEnd)
	}
}
