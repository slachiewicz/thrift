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
	_ "github.com/apache/thrift/compiler/go/generate/cglib"    // registers c_glib
	_ "github.com/apache/thrift/compiler/go/generate/cl"       // registers cl
	_ "github.com/apache/thrift/compiler/go/generate/cpp"      // registers cpp
	_ "github.com/apache/thrift/compiler/go/generate/dart"     // registers dart
	_ "github.com/apache/thrift/compiler/go/generate/delphi"   // registers delphi
	_ "github.com/apache/thrift/compiler/go/generate/dlang"    // registers d
	_ "github.com/apache/thrift/compiler/go/generate/erl"      // registers erl
	_ "github.com/apache/thrift/compiler/go/generate/gv"       // registers gv
	_ "github.com/apache/thrift/compiler/go/generate/haxe"     // registers haxe
	_ "github.com/apache/thrift/compiler/go/generate/html"     // registers html
	_ "github.com/apache/thrift/compiler/go/generate/javame"   // registers javame
	_ "github.com/apache/thrift/compiler/go/generate/js"       // registers js
	_ "github.com/apache/thrift/compiler/go/generate/kotlin"   // registers kotlin
	_ "github.com/apache/thrift/compiler/go/generate/lua"      // registers lua
	_ "github.com/apache/thrift/compiler/go/generate/markdown" // registers markdown
	_ "github.com/apache/thrift/compiler/go/generate/mmd"      // registers mmd
	_ "github.com/apache/thrift/compiler/go/generate/netstd"   // registers netstd
	_ "github.com/apache/thrift/compiler/go/generate/ocaml"    // registers ocaml
	_ "github.com/apache/thrift/compiler/go/generate/perl"     // registers perl
	_ "github.com/apache/thrift/compiler/go/generate/php"      // registers php
	_ "github.com/apache/thrift/compiler/go/generate/py"       // registers py
	_ "github.com/apache/thrift/compiler/go/generate/rb"       // registers rb
	_ "github.com/apache/thrift/compiler/go/generate/rs"       // registers rs
	_ "github.com/apache/thrift/compiler/go/generate/st"       // registers st
	_ "github.com/apache/thrift/compiler/go/generate/xml"      // registers xml
	_ "github.com/apache/thrift/compiler/go/generate/xsd"      // registers xsd
	"github.com/apache/thrift/compiler/go/sema"
)

// langRows is the option matrix of every generator that goes through the
// registry: the "--gen <lang>:<spec>" rows the oracle and golden tests run.
// A new generator adds its rows here and nothing else in this package.
// Go and Java keep their own tests, which predate the registry.
var langRows = map[string][]optionRow{
	"delphi": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "com_types", spec: "com_types"},
		{name: "com_types_rtti", spec: "com_types,rtti"},
		{name: "register_types", spec: "register_types"},
		{name: "rtti", spec: "rtti"},
	},
	"c_glib": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
	},
	"perl": {},
	"d": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
	},
	"dart": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "library_name", spec: "library_name=mylib"},
		{name: "library_prefix", spec: "library_prefix=my_parent_lib.src.gen"},
		{name: "pubspec_lib", spec: "pubspec_lib=thrift: 0.22.0"},
	},
	"cl": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "no_asd", spec: "no_asd"},
		{name: "sys_pref", spec: "sys_pref=my-thrift-gen-"},
	},
	"erl": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "legacynames", spec: "legacynames"},
		{name: "maps", spec: "maps"},
		{name: "app_prefix", spec: "app_prefix=test_"},
		{name: "delimiter", spec: "delimiter=_"},
		{name: "string", spec: "string=binary"},
		{name: "set", spec: "set=v2"},
		{name: "type", spec: "type=nominal"},
	},
	"cpp": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "cob_style", spec: "cob_style"},
		{name: "no_client_completion", spec: "no_client_completion"},
		{name: "no_default_operators", spec: "no_default_operators"},
		{name: "templates", spec: "templates"},
		{name: "pure_enums", spec: "pure_enums"},
		{name: "pure_enums_enum_class", spec: "pure_enums=enum_class"},
		{name: "include_prefix", spec: "include_prefix"},
		{name: "moveable_types", spec: "moveable_types"},
		{name: "moveable_types_forward_setter", spec: "moveable_types=forward_setter"},
		{name: "no_ostream_operators", spec: "no_ostream_operators"},
		{name: "no_skeleton", spec: "no_skeleton"},
		{name: "template_streamop", spec: "template_streamop"},
		{name: "no_constructors", spec: "no_constructors"},
		{name: "private_optional", spec: "private_optional"},
		// Every distinct "--gen cpp:..." spec the repository's own build
		// files use.
		{name: "private_optional_template_streamop", spec: "private_optional,template_streamop"},
		{name: "templates_cob_style", spec: "templates,cob_style"},
	},
	"gv": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "exceptions", spec: "exceptions"},
	},
	"haxe": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "callbacks", spec: "callbacks"},
		{name: "rtti", spec: "rtti"},
		{name: "buildmacro", spec: "buildmacro=my.macros.Class.method(args)"},
	},
	"json": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		// merge mutates the C++ program in place, so merge under -r
		// depends on generation order and is not a parity row.
		{name: "merge", spec: "merge"},
	},
	"lua": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "omit_requires", spec: "omit_requires"},
	},
	// t_kotlin_generator registers no options at all (THRIFT_REGISTER_GENERATOR's
	// doc string is empty), and no build file in the repository passes
	// --gen kotlin:... with any options either.
	"kotlin": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
	},
	"mmd": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "exceptions", spec: "exceptions"},
	},
	"ocaml": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
	},
	"py": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "zope.interface", spec: "zope.interface"},
		{name: "twisted", spec: "twisted"},
		{name: "tornado", spec: "tornado"},
		{name: "no_utf8strings", spec: "no_utf8strings"},
		{name: "coding", spec: "coding=utf-8"},
		{name: "slots", spec: "slots"},
		{name: "dynamic", spec: "dynamic"},
		{name: "dynamic-slots", spec: "dynamic,slots"},
		{name: "dynbase", spec: "dynbase=MyBase"},
		{name: "dynfrozen", spec: "dynfrozen=MyFrozenBase"},
		{name: "dynexc", spec: "dynexc=MyExc"},
		{name: "dynfrozenexc", spec: "dynfrozenexc=MyFrozenExc"},
		{name: "dynimport", spec: "dynimport=from foo.bar import CLS"},
		{name: "package_prefix", spec: "package_prefix=top.pkg."},
		{name: "old_style", spec: "old_style"},
		{name: "enum", spec: "enum"},
		{name: "enum-slots", spec: "enum,slots"},
		{name: "type_hints", spec: "type_hints,enum"},
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
	"rb": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "rubygems", spec: "rubygems"},
		{name: "namespaced", spec: "namespaced"},
		{name: "both", spec: "rubygems,namespaced"},
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
	"html": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "standalone", spec: "standalone"},
		{name: "noescape", spec: "noescape"},
	},
	"rs": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "crate_prefix", spec: "crate_prefix=super"},
	},
	"javame": {},
	"st": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
	},
	"js": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "jquery", spec: "jquery"},
		{name: "node", spec: "node"},
		{name: "ts", spec: "ts"},
		{name: "es6", spec: "es6"},
		// with_ns and esm are only valid combined with node; a spec that
		// is rejected outright (independent of any input file) is not a
		// parity row this harness can run, since generate.New fails
		// before any file is read, so only the valid combination is a row.
		{name: "node_with_ns", spec: "node,with_ns"},
		{name: "node_ts", spec: "node,ts"},
		{name: "es6_ts", spec: "es6,ts"},
		{name: "node_bigint", spec: "node,bigint"},
		{name: "node_es6", spec: "node,es6"},
		{name: "node_es6_bigint", spec: "node,es6,bigint"},
		{name: "node_es6_esm", spec: "node,es6,esm"},
		{name: "node_native_promise_false", spec: "node,native_promise=false"},
		// thrift_package_output_directory is exercised with a path that
		// exists nowhere but in the episode file it writes; imports= is
		// left out because every build-file use of it reads an episode
		// file from a prior generation step that a fresh checkout does
		// not have.
		{name: "node_ts_episode", spec: "node,ts,thrift_package_output_directory=first-episode"},
	},
	"php": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "inlined", spec: "inlined"},
		{name: "server", spec: "server"},
		{name: "oop", spec: "oop"},
		{name: "rest", spec: "rest"},
		{name: "nsglobal", spec: "nsglobal="},
		{name: "validate", spec: "validate"},
		{name: "json", spec: "json"},
		{name: "getters_setters", spec: "getters_setters"},
		{name: "classmap", spec: "classmap"},
		// The repository's own build files' distinct --gen php:... specs.
		{name: "classmap_server_rest", spec: "classmap,server,rest"},
		{name: "classmap_server_rest_nsglobal", spec: "classmap,server,rest,nsglobal="},
		{name: "inlined_nsglobal", spec: "inlined,nsglobal="},
		{name: "json_nsglobal", spec: "json,nsglobal="},
		{name: "oop_nsglobal", spec: "oop,nsglobal="},
		{name: "validate_nsglobal", spec: "validate,nsglobal="},
		{name: "validate_oop_nsglobal", spec: "validate,oop,nsglobal="},
	},
	"netstd": {
		{name: "none", spec: ""},
		{name: "none-r", spec: "", recurse: true},
		{name: "wcf", spec: "wcf"},
		{name: "serial", spec: "serial"},
		{name: "union", spec: "union"},
		{name: "pascal", spec: "pascal"},
		{name: "net8", spec: "net8"},
		{name: "net9", spec: "net9"},
		{name: "net10", spec: "net10"},
		{name: "no_deepcopy", spec: "no_deepcopy"},
		{name: "async_postfix", spec: "async_postfix"},
		{name: "union_serial", spec: "union,serial"},
		{name: "wcf_union_serial_net8", spec: "wcf,union,serial,net8"},
		{name: "wcf_union_serial_net9", spec: "wcf,union,serial,net9"},
		{name: "wcf_union_serial_net10", spec: "wcf,union,serial,net10"},
	},
}

// undefinedInCpp lists, per language, the corpus files on which the C++
// generator's behaviour is undefined, so that its output is not an
// oracle. The oracle test skips them, and the golden manifests pin the
// Go result. (The gv generator's entries left with THRIFT-6332.)
var undefinedInCpp = map[string]map[string]string{
	"js": {
		// t_js_generator::render_const_value's TYPE_UUID case writes
		// `out << "'" << value << "'"` where value is the t_const_value*
		// itself, not get_escaped_string(value): it prints the pointer's
		// address, which differs between runs. ConstantsDemo.thrift
		// declares `const uuid` values, so this path is reachable.
		"test/ConstantsDemo.thrift": "render_const_value prints a raw t_const_value* pointer for a uuid constant",
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
						if why, ok := undefinedInCpp[lang][filepath.ToSlash(rel)]; ok {
							t.Skipf("the C++ generator's behaviour is undefined here (%s); the golden manifest pins the Go result", why)
						}
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
