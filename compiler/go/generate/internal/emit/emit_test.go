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

package emit

import (
	"strings"
	"testing"
)

// TestDocstringComment pins DocstringComment to the C++ loop. The
// expected outputs are what t_generator::generate_docstring_comment,
// compiled on its own, prints with libc++ and with libstdc++ alike for the
// same arguments, with an indent of two spaces.
func TestDocstringComment(t *testing.T) {
	for _, c := range []struct {
		name, prefix, contents, want string
	}{
		{"empty", "", "", "  /**\n\n   */\n"},
		{"empty prefixed", " * ", "", "  /**\n   */\n"},
		{"one line", "", "a", "  /**\n  a\n   */\n"},
		{"one line prefixed", " * ", "a", "  /**\n   * a\n   */\n"},
		{"one line newline", "", "a\n", "  /**\n  a\n\n   */\n"},
		{"one line newline prefixed", " * ", "a\n", "  /**\n   * a\n   */\n"},
		{"two lines", "", "a\nb", "  /**\n  a\n  b\n   */\n"},
		{"two lines prefixed", " * ", "a\nb", "  /**\n   * a\n   * b\n   */\n"},
		{"two lines newline", "", "a\nb\n", "  /**\n  a\n  b\n\n   */\n"},
		{"two lines newline prefixed", " * ", "a\nb\n", "  /**\n   * a\n   * b\n   */\n"},
		{"blank middle", "", "a\n\nb\n", "  /**\n  a\n\n  b\n\n   */\n"},
		{"blank middle prefixed", " * ", "a\n\nb\n", "  /**\n   * a\n   * \n   * b\n   */\n"},
		{"blank only", "", "\n", "  /**\n\n\n   */\n"},
		{"blank only prefixed", " * ", "\n", "  /**\n   * \n   */\n"},
		{"two blanks", "", "\n\n", "  /**\n\n\n\n   */\n"},
		{"two blanks prefixed", " * ", "\n\n", "  /**\n   * \n   * \n   */\n"},
		{"line 1022", "", strings.Repeat("x", 1022) + "\nnext\n", "  /**\n  " + strings.Repeat("x", 1022) + "\n  next\n\n   */\n"},
		{"line 1022 prefixed", " * ", strings.Repeat("x", 1022) + "\nnext\n", "  /**\n   * " + strings.Repeat("x", 1022) + "\n   * next\n   */\n"},
		{"line 1023", "", strings.Repeat("x", 1023) + "\nnext\n", "  /**\n  " + strings.Repeat("x", 1023) + "\n  next\n\n   */\n"},
		{"line 1023 prefixed", " * ", strings.Repeat("x", 1023) + "\nnext\n", "  /**\n   * " + strings.Repeat("x", 1023) + "\n   * next\n   */\n"},
		{"line 1024", "", strings.Repeat("x", 1024) + "\nnext\n", "  /**\n  " + strings.Repeat("x", 1023) + "\n   */\n"},
		{"line 1024 prefixed", " * ", strings.Repeat("x", 1024) + "\nnext\n", "  /**\n   * " + strings.Repeat("x", 1023) + "\n   */\n"},
		{"line 1025", "", strings.Repeat("y", 1025) + "\nnext", "  /**\n  " + strings.Repeat("y", 1023) + "\n   */\n"},
		{"line 1025 prefixed", " * ", strings.Repeat("y", 1025) + "\nnext", "  /**\n   * " + strings.Repeat("y", 1023) + "\n   */\n"},
		{"eof 1023", "", strings.Repeat("z", 1023), "  /**\n  " + strings.Repeat("z", 1023) + "\n   */\n"},
		{"eof 1023 prefixed", " * ", strings.Repeat("z", 1023), "  /**\n   * " + strings.Repeat("z", 1023) + "\n   */\n"},
		{"eof 1024", "", strings.Repeat("z", 1024), "  /**\n  " + strings.Repeat("z", 1023) + "\n   */\n"},
		{"eof 1024 prefixed", " * ", strings.Repeat("z", 1024), "  /**\n   * " + strings.Repeat("z", 1023) + "\n   */\n"},
	} {
		var out strings.Builder
		DocstringComment(&out, "  ", "/**\n", c.prefix, c.contents, " */\n")
		if got := out.String(); got != c.want {
			t.Errorf("%s:\n got %q\nwant %q", c.name, got, c.want)
		}
	}
}
