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

package sema_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apache/thrift/compiler/go/sema"
)

// seedDirs are the repository directories whose .thrift files seed the
// fuzzer: the positive corpus and the reject corpus.
var seedDirs = []string{"lib/go/test", "test", "tutorial", "contrib", "compiler/cpp/tests/cpp", "compiler/go/testdata/accept", "compiler/go/testdata/reject", "lib/java/src/test/resources"}

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

// FuzzLoad runs the whole front end on arbitrary input: parse, scope
// resolution, constant resolution and the validators, which is where a
// fail() that a parse alone never reaches would hide. Every input must
// end in a program or a *sema.Error; any other panic is a bug. Run it for
// longer with
//
//	go test -fuzz=FuzzLoad -fuzztime=30m ./compiler/go/sema
func FuzzLoad(f *testing.F) {
	for _, s := range []string{
		"",
		"struct S { 1: i32 a }",
		"enum E { A = 1, B } const E c = E.A const list<E> l = [E.B, 1]",
		"typedef Later T struct Later { 1: T self, 2: optional list<Later> more = [] }",
		"struct S { 1: required i32 a = 1, 1: i32 dup }",
		"const map<string, i32> m = {\"a\": 1, \"a\": 2}",
		"service X extends X { void f() throws (1: S e) }",
		"include \"missing.thrift\" struct S { 1: missing.T t }",
		"namespace go a.b const uuid u = \"not-a-uuid\"",
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
	// The file lives in an empty directory, so a relative include finds
	// nothing and takes the warning path.
	path := filepath.Join(f.TempDir(), "fuzz.thrift")
	f.Fuzz(func(t *testing.T, src []byte) {
		loader := &sema.Loader{}
		prog, err := loader.LoadSource(path, src)
		if (prog == nil) == (err == nil) {
			t.Fatalf("exactly one of program and error must be set: %v %v", prog, err)
		}
		if err != nil {
			if _, ok := err.(*sema.Error); !ok {
				t.Fatalf("error must be a *sema.Error: %#v", err)
			}
			return
		}
		resolveAndValidate(t, prog)
	})
}

// resolveAndValidate runs the steps the generators run before emitting,
// accepting a *sema.Error and nothing else.
func resolveAndValidate(t *testing.T, prog *sema.Program) {
	defer func() {
		if r := recover(); r != nil {
			if _, ok := r.(*sema.Error); !ok {
				panic(r)
			}
		}
	}()
	prog.Scope.ResolveAllConsts()
	sema.ValidateInput(prog)
}

func TestLoadSource(t *testing.T) {
	loader := &sema.Loader{}
	prog, err := loader.LoadSource(filepath.Join(t.TempDir(), "mem.thrift"), []byte("struct S { 1: i32 a }"))
	if err != nil {
		t.Fatal(err)
	}
	if prog.Name() != "mem" || len(prog.Structs()) != 1 {
		t.Fatalf("got program %q with %d structs", prog.Name(), len(prog.Structs()))
	}
	if _, err := loader.LoadSource(filepath.Join(t.TempDir(), "bad.thrift"), []byte("struct {")); err == nil {
		t.Fatal("a syntax error was accepted")
	}
}
