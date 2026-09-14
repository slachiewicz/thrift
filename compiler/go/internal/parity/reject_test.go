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

// rejectCorpus returns every file under compiler/go/testdata/reject, sorted.
func rejectCorpus(t *testing.T, root string) []string {
	t.Helper()
	dir := filepath.Join(root, "compiler", "go", "testdata", "reject")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".thrift") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	sort.Strings(files)
	if len(files) == 0 {
		t.Fatalf("no .thrift files in %s", dir)
	}
	return files
}

// goCompile runs a file through the same pipeline the command uses: load,
// then generate with the base option row. Load alone does not resolve
// constants or run the validators, so a reject test must go this far.
func goCompile(t *testing.T, file string) error {
	t.Helper()
	opts, err := golang.ParseOptions(baseSpec)
	if err != nil {
		t.Fatal(err)
	}
	loader := &sema.Loader{}
	prog, err := loader.Load(file)
	if err != nil {
		return err
	}
	prog.SetOutPath(t.TempDir(), true)
	return golang.Run(prog, opts, false)
}

// TestRejectGo checks that the Go tool rejects every file of the negative
// corpus. It needs no oracle, so it runs on every checkout.
func TestRejectGo(t *testing.T) {
	root := RepoRoot(t)
	for _, file := range rejectCorpus(t, root) {
		t.Run(filepath.Base(file), func(t *testing.T) {
			err := goCompile(t, file)
			if err == nil {
				t.Fatal("the Go tool accepted a file the negative corpus says is invalid")
			}
			t.Logf("go: %v", err)
		})
	}
}

// TestRejectParity is the drift guard for the negative corpus: the C++
// compiler must reject every file too, or the corpus is asserting the
// wrong thing.
func TestRejectParity(t *testing.T) {
	root := RepoRoot(t)
	thrift := Compiler(t, root)
	for _, file := range rejectCorpus(t, root) {
		t.Run(filepath.Base(file), func(t *testing.T) {
			cmd := exec.Command(thrift, "-out", t.TempDir(), "--gen", "go:"+baseSpec, file)
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			if err := cmd.Run(); err == nil {
				t.Fatalf("the C++ compiler accepted a file the negative corpus says is invalid.\ncpp: %s", stderr.String())
			}
			t.Logf("cpp: %s", strings.TrimSpace(stderr.String()))
		})
	}
}
