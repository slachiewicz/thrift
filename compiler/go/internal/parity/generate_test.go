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

package parity

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/apache/thrift/compiler/go/generate/golang"
	"github.com/apache/thrift/compiler/go/sema"
)

// optionRow is one "--gen go:..." configuration of the output matrix.
type optionRow struct {
	name    string
	spec    string
	recurse bool
}

const baseSpec = "thrift_import=github.com/apache/thrift/lib/go/thrift,package_prefix=github.com/apache/thrift/lib/go/test/gopath/src/"

var optionRows = []optionRow{
	{name: "base", spec: baseSpec},
	{name: "base-r", spec: baseSpec, recurse: true},
	{name: "none", spec: ""},
	{name: "skip_remote", spec: baseSpec + ",skip_remote"},
	{name: "struct_key_entries", spec: baseSpec + ",struct_key_entries"},
	{name: "read_write_private", spec: baseSpec + ",read_write_private"},
	{name: "ignore_initialisms", spec: baseSpec + ",ignore_initialisms"},
	{name: "package", spec: baseSpec + ",package=parity"},
}

// readTree returns every file under root keyed by its relative path.
func readTree(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// TestGenerateParity runs the C++ compiler and the Go generator over every
// corpus file for every option row and requires identical output trees.
func TestGenerateParity(t *testing.T) {
	root := RepoRoot(t)
	thrift := Compiler(t, root)
	files := Corpus(t, root)
	for _, row := range optionRows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			for _, file := range files {
				rel, _ := filepath.Rel(root, file)
				t.Run(rel, func(t *testing.T) {
					checkGenerateParity(t, thrift, file, row)
				})
			}
		})
	}
}

func checkGenerateParity(t *testing.T, thrift, file string, row optionRow) {
	tmp := t.TempDir()
	cppOut := filepath.Join(tmp, "cpp")
	goOut := filepath.Join(tmp, "go")
	if err := os.MkdirAll(cppOut, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(goOut, 0o755); err != nil {
		t.Fatal(err)
	}

	args := []string{"-out", cppOut}
	if row.recurse {
		args = append(args, "-r")
	}
	args = append(args, "--gen", "go:"+row.spec, file)
	cmd := exec.Command(thrift, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cppErr := cmd.Run()

	opts, err := golang.ParseOptions(row.spec)
	if err != nil {
		t.Fatal(err)
	}
	loader := &sema.Loader{}
	prog, goErr := loader.Load(file)
	if goErr == nil {
		prog.SetOutPath(goOut, true)
		goErr = golang.Run(prog, opts, row.recurse)
	}

	if cppErr != nil {
		if goErr == nil {
			t.Fatalf("C++ compiler rejected the file but the Go generator accepted it.\ncpp: %s", stderr.String())
		}
		return
	}
	if goErr != nil {
		t.Fatalf("C++ compiler accepted the file but the Go generator rejected it: %v", goErr)
	}

	want := readTree(t, cppOut)
	got := readTree(t, goOut)
	if strings.Join(sortedKeys(want), "\n") != strings.Join(sortedKeys(got), "\n") {
		t.Fatalf("file sets differ.\ncpp: %v\ngo:  %v", sortedKeys(want), sortedKeys(got))
	}
	for _, name := range sortedKeys(want) {
		if want[name] != got[name] {
			t.Fatalf("%s differs at %s", name, firstDiff(want[name], got[name]))
		}
	}
}
