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
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// seedDirs are the repository directories, relative to its root, whose
// .thrift files seed the fuzzer. They are the positive parity corpus.
var seedDirs = []string{"lib/go/test", "test", "tutorial", "contrib", "compiler/cpp/tests/cpp", "compiler/go/testdata/accept", "compiler/go/testdata/reject", "lib/java/src/test/resources"}

// repoRoot walks up from the working directory to the checkout root, or
// returns "" when the package is built outside the repository.
func repoRoot() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "configure.ac")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

// FuzzParse checks that Parse never panics or hangs on any input; it
// returns either a program or an *Error. Run it for longer with
//
//	go test -fuzz=FuzzParse -fuzztime=1h ./compiler/go/idl/parser
//
// Without -fuzz it runs the seed corpus only, like every other test.
func FuzzParse(f *testing.F) {
	for _, s := range []string{
		"",
		"struct S { 1: i32 a }",
		"/** doc */ namespace go x /** s */ struct S { /** f */ 1: optional list<map<string, S>> f = [] (a = \"b\") }",
		"enum E { A = 1, B } const E c = E.A service X extends Y { oneway void f(1: E e) throws (1: X x) }",
		"typedef list cpp_type \"v\" <i32> L const L l = [1, 2.5, -, 0x10, \"s\", 'c', a.b]",
		"struct { \"unterminated",
		"/* open",
		"\xEF\xBB\xBF include \"a\" \n # comment\n",
	} {
		f.Add([]byte(s))
	}
	if root := repoRoot(); root != "" {
		for _, d := range seedDirs {
			_ = filepath.WalkDir(filepath.Join(root, d), func(path string, entry os.DirEntry, err error) error {
				if err != nil {
					return nil
				}
				if entry.IsDir() {
					if strings.HasPrefix(entry.Name(), "gen-") || entry.Name() == "gopath" {
						return filepath.SkipDir
					}
					return nil
				}
				if strings.HasSuffix(entry.Name(), ".thrift") {
					if data, err := os.ReadFile(path); err == nil {
						f.Add(data)
					}
				}
				return nil
			})
		}
	}
	f.Fuzz(func(t *testing.T, src []byte) {
		prog, err := Parse("fuzz.thrift", src, func(int, int, string) {})
		if (prog == nil) == (err == nil) {
			t.Fatalf("exactly one of program and error must be set: %v %v", prog, err)
		}
		if err != nil {
			if e, ok := err.(*Error); !ok || e.Line < 1 {
				t.Fatalf("error must be an *Error with a line: %#v", err)
			}
		}
	})
}
