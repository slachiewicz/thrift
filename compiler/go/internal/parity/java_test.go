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
	"strings"
	"testing"

	"github.com/apache/thrift/compiler/go/generate/java"
	"github.com/apache/thrift/compiler/go/sema"
)

// javaRows is the "--gen java:..." matrix. The first seven are the
// invocations in lib/java/gradle/generateTestThrift.gradle; the rest
// cover every remaining option once. Rows without generated_annotations
// carry today's date in the @Generated annotation on both sides, so a
// run that straddles midnight can fail once.
var javaRows = []optionRow{
	{name: "jakarta", spec: "jakarta_annotations"},
	{name: "option_type_jdk8", spec: "option_type=jdk8,jakarta_annotations"},
	{name: "beans", spec: "beans,nocamel,future_iface,jakarta_annotations"},
	{name: "fullcamel", spec: "fullcamel,future_iface,jakarta_annotations"},
	{name: "reuse_objects", spec: "reuse_objects,jakarta_annotations"},
	{name: "unsafe_binaries", spec: "unsafe_binaries,jakarta_annotations"},
	{name: "annotations_as_metadata", spec: "annotations_as_metadata,jakarta_annotations"},
	{name: "none", spec: ""},
	{name: "none-r", spec: "", recurse: true},
	{name: "android", spec: "android"},
	{name: "private_members", spec: "private_members"},
	{name: "sorted_containers", spec: "sorted_containers"},
	{name: "java5", spec: "java5"},
	{name: "undated", spec: "generated_annotations=undated"},
	{name: "suppress", spec: "generated_annotations=suppress"},
	{name: "rethrow", spec: "rethrow_unhandled_exceptions"},
	{name: "option_type_thrift", spec: "option_type=thrift"},
	{name: "fullcamel_only", spec: "fullcamel"},
	{name: "nocamel_only", spec: "nocamel"},
}

// TestGenerateParityJava runs the C++ compiler and the Java generator
// over every corpus file for every option row and requires identical
// output trees.
func TestGenerateParityJava(t *testing.T) {
	root := RepoRoot(t)
	thrift := Compiler(t, root)
	files := Corpus(t, root)
	for _, row := range javaRows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			for _, file := range files {
				rel, _ := filepath.Rel(root, file)
				t.Run(rel, func(t *testing.T) {
					checkGenerateParityJava(t, thrift, file, row)
				})
			}
		})
	}
}

func checkGenerateParityJava(t *testing.T, thrift, file string, row optionRow) {
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
	args = append(args, "--gen", "java:"+row.spec, file)
	cmd := exec.Command(thrift, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cppErr := cmd.Run()

	opts, err := java.ParseOptions(row.spec)
	if err != nil {
		t.Fatal(err)
	}
	loader := &sema.Loader{}
	prog, goErr := loader.Load(file)
	if goErr == nil {
		prog.SetOutPath(goOut, true)
		goErr = java.Run(prog, opts, row.recurse)
	}

	if cppErr != nil {
		if goErr == nil {
			t.Fatalf("C++ compiler rejected the file but the Go generator accepted it.\ncpp: %s", stderr.String())
		}
		t.Logf("both reject: cpp=%q go=%q", strings.TrimSpace(stderr.String()), goErr)
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
