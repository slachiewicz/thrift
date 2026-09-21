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

package generate_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/apache/thrift/compiler/go/generate"
	_ "github.com/apache/thrift/compiler/go/generate/golang"
	_ "github.com/apache/thrift/compiler/go/generate/java"
	"github.com/apache/thrift/compiler/go/sema"
)

type nopRunner struct{}

func (nopRunner) Run(*sema.Program, bool) error { return nil }

func TestRegisterRejectsDuplicatesAndBlanks(t *testing.T) {
	generate.Register(generate.Info{Name: "test_lang", LongName: "Test", Parse: func(string) (generate.Runner, error) { return nopRunner{}, nil }})
	for name, info := range map[string]generate.Info{
		"twice":    {Name: "test_lang", Parse: func(string) (generate.Runner, error) { return nopRunner{}, nil }},
		"no name":  {Parse: func(string) (generate.Runner, error) { return nopRunner{}, nil }},
		"no parse": {Name: "test_lang2"},
	} {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("Register did not panic")
				}
			}()
			generate.Register(info)
		})
	}
}

func TestAllIsSortedAndHoldsBothGenerators(t *testing.T) {
	var names []string
	for _, info := range generate.All() {
		names = append(names, info.Name)
	}
	joined := strings.Join(names, ",")
	if !strings.Contains(joined, "go,java") {
		t.Fatalf("All() = %v, want go before java", names)
	}
	for i := 1; i < len(names); i++ {
		if names[i-1] >= names[i] {
			t.Fatalf("All() is not sorted: %v", names)
		}
	}
}

func TestNew(t *testing.T) {
	for _, spec := range []string{"go", "go:", "go:skip_remote", "java", "java:beans,option_type=jdk8"} {
		if _, err := generate.New(spec); err != nil {
			t.Errorf("New(%q): %v", spec, err)
		}
	}
	for _, spec := range []string{"rb", "go:bogus", "java:option_type=maybe", ""} {
		if _, err := generate.New(spec); err == nil {
			t.Errorf("New(%q) accepted", spec)
		}
	}
	if _, err := generate.New("rb:x"); err == nil || !strings.Contains(err.Error(), `Unable to get a generator for "rb:x"`) {
		t.Errorf("New(rb:x) = %v, want the C++ wording", err)
	}
}

// sampleValue picks a value the help placeholder admits: the first
// alternative of "[a|b]", or "x" for a free placeholder.
func sampleValue(placeholder string) string {
	if m := regexp.MustCompile(`^\[([^|\]]+)`).FindStringSubmatch(placeholder); m != nil {
		return m[1]
	}
	return "x"
}

// TestEveryDocumentedOptionParses keeps the option tables honest: each
// key the help text lists, and each alias, must be accepted by the
// generator's own parser.
func TestEveryDocumentedOptionParses(t *testing.T) {
	for _, info := range generate.All() {
		if info.Name == "test_lang" {
			continue
		}
		for _, o := range info.Options {
			keys := append([]string{o.Name}, o.Aliases...)
			keys = append(keys, o.Deprecated...)
			for _, key := range keys {
				spec := info.Name + ":" + key
				if o.Value != "" {
					spec += "=" + sampleValue(o.Value)
				}
				if _, err := generate.New(spec); err != nil {
					t.Errorf("documented option %s is rejected: %v", spec, err)
				}
			}
		}
	}
}

func TestDocumentation(t *testing.T) {
	info := generate.Info{Options: []generate.Option{
		{Name: "short", Help: "Fits on the line."},
		{Name: "a_rather_long_key", Deprecated: []string{"a-rather-long-key"}, Help: "Wraps.\nSecond line."},
		{Name: "valued", Value: "[one|two]", Help: "Takes a value."},
	}}
	want := "" +
		"    short:           Fits on the line.\n" +
		"    a_rather_long_key:\n" +
		"                     Wraps.\n" +
		"                     Second line.\n" +
		"    a-rather-long-key:\n" +
		"                     Same as 'a_rather_long_key' (deprecated).\n" +
		"    valued=[one|two]:\n" +
		"                     Takes a value.\n"
	if got := info.Documentation(); got != want {
		t.Fatalf("Documentation() =\n%s\nwant\n%s", got, want)
	}
}
