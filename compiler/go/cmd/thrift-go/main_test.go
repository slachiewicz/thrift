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

package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The tests run the built binary, since every command exits the
// process. TestMain builds it once.
var (
	binary string
	root   string
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "thrift-go-test")
	if err != nil {
		panic(err)
	}
	binary = filepath.Join(dir, "thrift-go")
	build := exec.Command("go", "build", "-o", binary, ".")
	build.Stderr = os.Stderr
	if err := build.Run(); err != nil {
		panic(err)
	}
	wd, _ := os.Getwd()
	root = filepath.Clean(filepath.Join(wd, "..", "..", "..", ".."))
	code := m.Run()
	os.RemoveAll(dir)
	os.Exit(code)
}

type result struct {
	code   int
	stdout string
	stderr string
}

func run(t *testing.T, stdin []byte, args ...string) result {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = root
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	code := 0
	if err != nil {
		exitErr, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatal(err)
		}
		code = exitErr.ExitCode()
	}
	return result{code, out.String(), errOut.String()}
}

func readTree(t *testing.T, dir string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(dir, path)
		files[filepath.ToSlash(rel)] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

const goOpts = "thrift_import=github.com/apache/thrift/lib/go/thrift,package_prefix=x/"

func TestGenerateMatchesLegacyForm(t *testing.T) {
	newOut, oldOut := t.TempDir(), t.TempDir()
	r := run(t, nil, "generate", "--lang", "go", "--lang", "java", "--out", newOut, "--recurse",
		"--go.thrift-import", "github.com/apache/thrift/lib/go/thrift", "--go.package_prefix", "x/",
		"--java.jakarta-annotations", "--java.generated_annotations", "undated",
		"tutorial/tutorial.thrift")
	if r.code != 0 {
		t.Fatalf("generate: exit %d\n%s", r.code, r.stderr)
	}
	r = run(t, nil, "-out", oldOut, "-r", "--gen", "go:"+goOpts, "--gen", "java:jakarta_annotations,generated_annotations=undated", "tutorial/tutorial.thrift")
	if r.code != 0 {
		t.Fatalf("legacy: exit %d\n%s", r.code, r.stderr)
	}
	want, got := readTree(t, oldOut), readTree(t, newOut)
	if len(want) == 0 || len(want) != len(got) {
		t.Fatalf("legacy wrote %d files, generate wrote %d", len(want), len(got))
	}
	for name, content := range want {
		if got[name] != content {
			t.Errorf("%s differs between the two forms", name)
		}
	}
}

func TestGenerateSeveralFilesInOneRun(t *testing.T) {
	out := t.TempDir()
	r := run(t, nil, "gen", "--lang", "go", "--out", out, "--go.thrift_import", "github.com/apache/thrift/lib/go/thrift",
		"test/Recursive.thrift", "test/AnnotationTest.thrift")
	if r.code != 0 {
		t.Fatalf("exit %d\n%s", r.code, r.stderr)
	}
	files := readTree(t, out)
	for _, want := range []string{"recursive/Recursive.go", "annotationtest/AnnotationTest.go"} {
		if _, ok := files[want]; !ok {
			t.Errorf("%s not generated; got %v", want, keys(files))
		}
	}
}

func keys(m map[string]string) []string {
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestGenerateRejectsBadInvocations(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"no lang", []string{"generate", "tutorial/tutorial.thrift"}, "no --lang"},
		{"no files", []string{"generate", "--lang", "go"}, "no input files"},
		{"unknown lang", []string{"generate", "--lang", "cobol", "tutorial/tutorial.thrift"}, `unknown language "cobol"`},
		{"stray flag", []string{"generate", "--lang", "go", "--java.beans", "tutorial/tutorial.thrift"}, "--java.beans given but its language is not a target"},
		{"bad value", []string{"generate", "--lang", "java", "--java.option_type", "maybe", "tutorial/tutorial.thrift"}, "option_type must be"},
		{"unknown flag", []string{"generate", "--lang", "go", "--go.bogus", "tutorial/tutorial.thrift"}, "flag provided but not defined"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := run(t, nil, append(c.args, "--out", t.TempDir())...)
			if r.code != exitError {
				t.Errorf("exit %d, want %d", r.code, exitError)
			}
			if !strings.Contains(r.stderr, c.want) {
				t.Errorf("stderr %q does not contain %q", r.stderr, c.want)
			}
		})
	}
}

func TestCheck(t *testing.T) {
	if r := run(t, nil, "check", "test/ThriftTest.thrift", "tutorial/tutorial.thrift"); r.code != 0 {
		t.Errorf("valid files: exit %d\n%s", r.code, r.stderr)
	}
	bad := filepath.Join(t.TempDir(), "bad.thrift")
	os.WriteFile(bad, []byte("struct S {\n  1: i32 a\n  2 i32 b\n}\n"), 0o644)
	r := run(t, nil, "check", bad)
	if r.code != exitError || !strings.Contains(r.stderr, bad+":3:5: error: syntax error") {
		t.Errorf("syntax error: exit %d stderr %q", r.code, r.stderr)
	}
	// test/audit/test.thrift warns about byte; --strict turns that into
	// exit status 2, --no-warn silences it.
	r = run(t, nil, "check", "--strict", "test/audit/test.thrift")
	if r.code != exitPolicy || !strings.Contains(r.stderr, "test.thrift:68: warning:") {
		t.Errorf("strict: exit %d stderr %q", r.code, r.stderr)
	}
	r = run(t, nil, "check", "--no-warn", "test/audit/test.thrift")
	if r.code != 0 || r.stderr != "" {
		t.Errorf("no-warn: exit %d stderr %q", r.code, r.stderr)
	}
}

func TestAudit(t *testing.T) {
	r := run(t, nil, "audit", "test/audit/test.thrift", "test/audit/break7.thrift")
	if r.code != exitPolicy || !strings.Contains(r.stderr, "Requiredness Changed for Id = 2 in test_struct6") {
		t.Errorf("breaking: exit %d stderr %q", r.code, r.stderr)
	}
	if r := run(t, nil, "audit", "--no-fatal", "test/audit/test.thrift", "test/audit/break7.thrift"); r.code != 0 {
		t.Errorf("no-fatal: exit %d", r.code)
	}
	if r := run(t, nil, "audit", "test/audit/test.thrift", "test/audit/warning.thrift"); r.code != 0 || !strings.Contains(r.stdout, "[Thrift Audit Warning:") {
		t.Errorf("warnings only: exit %d stdout %q", r.code, r.stdout)
	}
	r = run(t, nil, "audit", "--allow-required-field-to-default", "test/audit/required_to_default_old.thrift", "test/audit/required_to_default_new.thrift")
	if r.code != 0 {
		t.Errorf("allowed change: exit %d stderr %q", r.code, r.stderr)
	}
	if r := run(t, nil, "audit", "test/audit/test.thrift"); r.code != exitError {
		t.Errorf("one file: exit %d", r.code)
	}
}

func TestLanguagesVersionHelp(t *testing.T) {
	r := run(t, nil, "languages")
	if r.code != 0 || !strings.Contains(r.stdout, "go       Go") || !strings.Contains(r.stdout, "java     Java") {
		t.Errorf("languages: exit %d stdout %q", r.code, r.stdout)
	}
	r = run(t, nil, "languages", "--json")
	if r.code != 0 || !strings.Contains(r.stdout, `"flag": "--go.thrift_import"`) {
		t.Errorf("languages --json: exit %d stdout %q", r.code, r.stdout)
	}
	r = run(t, nil, "version")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "Thrift version 0.") {
		t.Errorf("version: exit %d stdout %q", r.code, r.stdout)
	}
	for _, args := range [][]string{{"help"}, {"help", "generate"}, {"help", "exit-codes"}, {"generate", "-h"}, {"decode", "--help"}} {
		r := run(t, nil, args...)
		if r.code != 0 || r.stdout == "" {
			t.Errorf("%v: exit %d stdout %q stderr %q", args, r.code, r.stdout, r.stderr)
		}
	}
	if r := run(t, nil, "help", "bogus"); r.code != exitError {
		t.Errorf("help bogus: exit %d", r.code)
	}
	if r := run(t, nil, "frobnicate"); r.code != exitError || !strings.Contains(r.stderr, `unknown command "frobnicate"`) {
		t.Errorf("unknown command: exit %d stderr %q", r.code, r.stderr)
	}
}

func TestLegacyFormStillWorks(t *testing.T) {
	r := run(t, nil, "-version")
	if r.code != 0 || !strings.HasPrefix(r.stdout, "Thrift version") {
		t.Errorf("-version: exit %d stdout %q", r.code, r.stdout)
	}
	r = run(t, nil, "--help")
	if r.code != 0 || !strings.Contains(r.stderr, "Available generators") {
		t.Errorf("--help: exit %d", r.code)
	}
	r = run(t, nil, "-audit", "test/audit/test.thrift", "test/audit/break7.thrift")
	if r.code != exitPolicy {
		t.Errorf("-audit: exit %d", r.code)
	}
	r = run(t, nil, "--gen", "cobol", "tutorial/tutorial.thrift")
	if r.code != exitError || !strings.Contains(r.stderr, `Unable to get a generator for "cobol"`) {
		t.Errorf("--gen erl: exit %d stderr %q", r.code, r.stderr)
	}
}

func TestDecodeCommand(t *testing.T) {
	// A binary struct {1: i32 7, 2: string "hi"}.
	data := []byte{0x08, 0x00, 0x01, 0x00, 0x00, 0x00, 0x07, 0x0b, 0x00, 0x02, 0x00, 0x00, 0x00, 0x02, 'h', 'i', 0x00}
	r := run(t, data, "decode")
	want := "struct {\n  1: i32 7\n  2: string \"hi\"\n}\n"
	if r.code != 0 || r.stdout != want {
		t.Errorf("stdin: exit %d stdout %q", r.code, r.stdout)
	}
	r = run(t, data, "decode", "--json", "-")
	if r.code != 0 || !strings.Contains(r.stdout, `"protocol": "binary"`) {
		t.Errorf("--json: exit %d stdout %q", r.code, r.stdout)
	}
	if r := run(t, nil, "decode", "--protocol", "xml"); r.code != exitError {
		t.Errorf("bad protocol: exit %d", r.code)
	}
	// With the tutorial IDL the same bytes are a Work struct, named.
	r = run(t, data, "decode", "--idl", "tutorial/tutorial.thrift", "--type", "Work")
	if r.code != 0 || !strings.Contains(r.stdout, "struct Work {\n  1: num1 i32 7\n  2: num2 string \"hi\"  (the IDL says i32)\n}") {
		t.Errorf("--idl: exit %d stdout %q stderr %q", r.code, r.stdout, r.stderr)
	}
	if r := run(t, data, "decode", "--type", "Work"); r.code != exitError {
		t.Errorf("--type without --idl: exit %d", r.code)
	}
	if r := run(t, data, "decode", "--idl", "tutorial/tutorial.thrift"); r.code != exitError || !strings.Contains(r.stderr, "--type") {
		t.Errorf("--idl without --type on a bare struct: exit %d stderr %q", r.code, r.stderr)
	}
}
