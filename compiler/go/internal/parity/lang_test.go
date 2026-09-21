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

	"github.com/apache/thrift/compiler/go/generate"
	_ "github.com/apache/thrift/compiler/go/generate/markdown" // registers markdown
	_ "github.com/apache/thrift/compiler/go/generate/mmd"      // registers mmd
	_ "github.com/apache/thrift/compiler/go/generate/xml"      // registers xml
	_ "github.com/apache/thrift/compiler/go/generate/xsd"      // registers xsd
	"github.com/apache/thrift/compiler/go/sema"
)

// langRows is the option matrix of every generator that goes through the
// registry: the "--gen <lang>:<spec>" rows the oracle and golden tests run.
// A new generator adds its rows here and nothing else in this package.
// Go and Java keep their own tests, which predate the registry.
var langRows = map[string][]optionRow{
	"json": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		// merge mutates the C++ program in place, so merge under -r
		// depends on generation order and is not a parity row.
		{name: "merge", spec: "merge"},
	},
	"mmd": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "exceptions", spec: "exceptions"},
	},
	"xsd": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
	},
	"markdown": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "suffix", spec: "suffix=markdown"},
		{name: "noescape", spec: "noescape"},
	},
	"xml": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		// merge mutates the C++ program in place, so merge under -r
		// depends on generation order and is not a parity row.
		{name: "merge", spec: "merge"},
		{name: "no_default_ns", spec: "no_default_ns"},
		{name: "no_namespaces", spec: "no_namespaces"},
	},
}

// generateLang runs a registered generator on one corpus file in-process
// and returns the output tree, or the error the generator reported.
func generateLang(lang string) func(t *testing.T, file string, row optionRow) (map[string]string, error) {
	return func(t *testing.T, file string, row optionRow) (map[string]string, error) {
		t.Helper()
		out := t.TempDir()
		runner, err := generate.New(lang + ":" + row.spec)
		if err != nil {
			t.Fatal(err)
		}
		loader := &sema.Loader{}
		prog, err := loader.Load(file)
		if err != nil {
			return nil, err
		}
		prog.SetOutPath(out, true)
		if err := runner.Run(prog, row.recurse); err != nil {
			return nil, err
		}
		return readTree(t, out), nil
	}
}

// TestGenerateParityLang runs the C++ compiler and the registered Go
// generator over every corpus file for every row of langRows and requires
// identical output trees.
func TestGenerateParityLang(t *testing.T) {
	root := RepoRoot(t)
	thrift := Compiler(t, root)
	files := Corpus(t, root)
	for _, lang := range sortedLangs() {
		lang := lang
		if _, ok := generate.Lookup(lang); !ok {
			t.Fatalf("%s has rows but no registered generator", lang)
		}
		for _, row := range langRows[lang] {
			row := row
			t.Run(lang+"/"+row.name, func(t *testing.T) {
				for _, file := range files {
					rel, _ := filepath.Rel(root, file)
					t.Run(rel, func(t *testing.T) {
						checkLangParity(t, thrift, lang, file, row)
					})
				}
			})
		}
	}
}

func sortedLangs() []string {
	var langs []string
	for l := range langRows {
		langs = append(langs, l)
	}
	sortStrings(langs)
	return langs
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j-1] > s[j]; j-- {
			s[j-1], s[j] = s[j], s[j-1]
		}
	}
}

func checkLangParity(t *testing.T, thrift, lang, file string, row optionRow) {
	cppOut := filepath.Join(t.TempDir(), "cpp")
	if err := os.MkdirAll(cppOut, 0o755); err != nil {
		t.Fatal(err)
	}
	args := []string{"-out", cppOut}
	if row.recurse {
		args = append(args, "-r")
	}
	args = append(args, "--gen", lang+":"+row.spec, file)
	cmd := exec.Command(thrift, args...)
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	cppErr := cmd.Run()
	cppMsg := strings.TrimSpace(output.String())

	got, goErr := generateLang(lang)(t, file, row)

	if cppErr != nil {
		if goErr == nil {
			t.Fatalf("C++ compiler rejected the file but the Go generator accepted it.\ncpp: %s", cppMsg)
		}
		t.Logf("both reject: cpp=%q go=%q", cppMsg, goErr)
		return
	}
	if goErr != nil {
		t.Fatalf("C++ compiler accepted the file but the Go generator rejected it: %v", goErr)
	}
	want := readTree(t, cppOut)
	if strings.Join(sortedKeys(want), "\n") != strings.Join(sortedKeys(got), "\n") {
		t.Fatalf("file sets differ.\ncpp: %v\ngo:  %v", sortedKeys(want), sortedKeys(got))
	}
	for _, name := range sortedKeys(want) {
		if want[name] != got[name] {
			t.Fatalf("%s differs at %s", name, firstDiff(want[name], got[name]))
		}
	}
}

// TestGoldenLang is the golden-manifest test for every language in
// langRows; see golden_test.go.
func TestGoldenLang(t *testing.T) {
	for _, lang := range sortedLangs() {
		lang := lang
		t.Run(lang, func(t *testing.T) {
			runGolden(t, lang, langRows[lang], generateLang(lang))
		})
	}
}
