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
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/sema"
)

// languageFlags holds the --<lang>.<option> flags built from the
// registry: one per documented option, in the canonical spelling and
// with '-' for '_' as an alias.
type languageFlags struct {
	// values maps "lang\toption" to the flag's value; a flag without a
	// value is a *bool, one with a value a *string.
	values map[string]interface{}
	// set records which canonical keys were given on the command line.
	set map[string]bool
}

func registerLanguageFlags(fs *flag.FlagSet) *languageFlags {
	lf := &languageFlags{values: map[string]interface{}{}, set: map[string]bool{}}
	for _, info := range generate.All() {
		for _, o := range info.Options {
			key := info.Name + "\t" + o.Name
			help := strings.Split(o.Help, "\n")[0]
			names := []string{o.Name}
			if alt := strings.ReplaceAll(o.Name, "_", "-"); alt != o.Name {
				names = append(names, alt)
			}
			for i, name := range names {
				flagName := info.Name + "." + name
				h := help
				if i > 0 {
					h = "same as --" + info.Name + "." + o.Name
				}
				if o.Value != "" {
					var v string
					if i == 0 {
						lf.values[key] = &v
						fs.StringVar(&v, flagName, "", h+" ("+o.Value+")")
					} else {
						fs.StringVar(lf.values[key].(*string), flagName, "", h)
					}
				} else {
					var v bool
					if i == 0 {
						lf.values[key] = &v
						fs.BoolVar(&v, flagName, false, h)
					} else {
						fs.BoolVar(lf.values[key].(*bool), flagName, false, h)
					}
				}
			}
		}
	}
	return lf
}

// spec builds the "lang:key=value,key" option string for one language
// from the flags that were given, and reports the flags given for
// languages that are not targets.
func (lf *languageFlags) spec(lang string, targets map[string]bool) (string, error) {
	var parts []string
	var strays []string
	keys := make([]string, 0, len(lf.values))
	for k := range lf.values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		l, option, _ := strings.Cut(key, "\t")
		given := false
		part := option
		switch v := lf.values[key].(type) {
		case *bool:
			given = *v
		case *string:
			given = *v != ""
			part = option + "=" + *v
		}
		if !given {
			continue
		}
		if !targets[l] {
			strays = append(strays, "--"+l+"."+option)
			continue
		}
		if l == lang {
			parts = append(parts, part)
		}
	}
	if len(strays) > 0 && lang == "" {
		return "", fmt.Errorf("%s given but %s not a target; add --lang", strings.Join(strays, ", "), strayVerb(strays))
	}
	return strings.Join(parts, ","), nil
}

func strayVerb(strays []string) string {
	if len(strays) == 1 {
		return "its language is"
	}
	return "their languages are"
}

func runGenerate(args []string) int {
	var langs stringList
	var lf loaderFlags
	fs := newFlagSet("generate", "--lang <language> [flags] file.thrift...", func(w io.Writer) {
		fmt.Fprintln(w, "Generate code for every file, in one run. Languages:", strings.Join(languageNames(), ", ")+".")
		fmt.Fprintln(w, "A generator's options are flags named --<language>.<option>; 'thrift-go languages' lists them.")
	})
	fs.Var(&langs, "lang", "target language; repeatable")
	out := fs.String("out", ".", "directory the files are written to, created if missing")
	recurse := fs.Bool("recurse", false, "also generate the included files")
	lf.register(fs)
	langFlags := registerLanguageFlags(fs)
	if code := parseFlags(fs, args); code >= 0 {
		return code
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "generate: no input files")
		return exitError
	}
	if len(langs) == 0 {
		fmt.Fprintln(os.Stderr, "generate: no --lang given; one of "+strings.Join(languageNames(), ", "))
		return exitError
	}
	targets := map[string]bool{}
	for _, l := range langs {
		if _, ok := generate.Lookup(l); !ok {
			fmt.Fprintf(os.Stderr, "generate: unknown language %q; one of %s\n", l, strings.Join(languageNames(), ", "))
			return exitError
		}
		targets[l] = true
	}
	if _, err := langFlags.spec("", targets); err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		return exitError
	}
	var runners []generate.Runner
	for _, l := range langs {
		spec, _ := langFlags.spec(l, targets)
		r, err := generate.New(l + ":" + spec)
		if err != nil {
			fmt.Fprintln(os.Stderr, "generate:", err)
			return exitError
		}
		runners = append(runners, r)
	}
	outDir, err := filepath.Abs(*out)
	if err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		return exitError
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintln(os.Stderr, "generate:", err)
		return exitError
	}

	loader := lf.loader()
	for _, file := range fs.Args() {
		program, err := loader.Load(file)
		if err != nil {
			report(err)
			return exitError
		}
		program.SetOutPath(outDir, true)
		if code := generateTree(program, runners, *recurse); code != exitOK {
			return code
		}
	}
	return exitOK
}

// generateTree runs every generator on the program and, with recurse, on
// its includes first.
func generateTree(program *sema.Program, runners []generate.Runner, recurse bool) int {
	if recurse {
		program.SetRecursive(true)
		for _, inc := range program.Includes() {
			inc.SetOutPath(program.OutPath(), program.IsOutPathAbsolute())
			if code := generateTree(inc, runners, recurse); code != exitOK {
				return code
			}
		}
	}
	for _, r := range runners {
		if err := r.Run(program, false); err != nil {
			report(err)
			return exitGenerator
		}
	}
	return exitOK
}
