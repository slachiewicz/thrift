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

// Command thrift-go compiles Thrift IDL to Go.
//
// It accepts the command line of the C++ thrift compiler, so that the
// existing build files can point their THRIFT variable at it:
//
//	thrift-go [options] --gen go[:option,...] file.thrift
//
// Only the Go generator is available.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/golang"
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] file\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "Use %s -help for a list of options\n", os.Args[0])
	os.Exit(1)
}

func help() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] file\n\n", os.Args[0])
	fmt.Fprintln(os.Stderr, "Options:")
	fmt.Fprintln(os.Stderr, "  -version    Print the compiler version")
	fmt.Fprintln(os.Stderr, "  -o dir      Set the output directory for gen-* packages")
	fmt.Fprintln(os.Stderr, "               (default: current directory)")
	fmt.Fprintln(os.Stderr, "  -out dir    Set the output location for generated files.")
	fmt.Fprintln(os.Stderr, "               (no gen-* folder will be created)")
	fmt.Fprintln(os.Stderr, "  -I dir      Add a directory to the list of directories")
	fmt.Fprintln(os.Stderr, "                searched for include directives")
	fmt.Fprintln(os.Stderr, "  -nowarn     Suppress all compiler warnings (BAD!)")
	fmt.Fprintln(os.Stderr, "  -strict     Strict compiler warnings on")
	fmt.Fprintln(os.Stderr, "  -v[erbose]  Verbose mode")
	fmt.Fprintln(os.Stderr, "  -r[ecurse]  Also generate included files")
	fmt.Fprintln(os.Stderr, "  -debug      Parse debug trace to stdout")
	fmt.Fprintln(os.Stderr, "  --allow-neg-keys  Allow negative field keys (Used to ")
	fmt.Fprintln(os.Stderr, "                     preserve protocol compatibility with")
	fmt.Fprintln(os.Stderr, "                     older .thrift files)")
	fmt.Fprintln(os.Stderr, "  --allow-64bit-consts  Do not print warnings about using 64-bit constants")
	fmt.Fprintln(os.Stderr, "  --gen STR   Generate code with a dynamically-registered generator.")
	fmt.Fprintln(os.Stderr, "               STR has the form language[:key1=val1[,key2[,key3=val3]]].")
	fmt.Fprintln(os.Stderr, "               Keys and values are options passed to the generator.")
	fmt.Fprintln(os.Stderr, "               Many options will not require values.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Available generators (and options):")
	fmt.Fprintln(os.Stderr, "  go (Go):")
	fmt.Fprintln(os.Stderr, "    package_prefix=  Package prefix for generated files.")
	fmt.Fprintln(os.Stderr, "    thrift_import=   Override thrift package import path (default:"+golang.DefaultThriftImport+")")
	fmt.Fprintln(os.Stderr, "    package=         Package name (default: inferred from thrift file name)")
	fmt.Fprintln(os.Stderr, "    ignore_initialisms")
	fmt.Fprintln(os.Stderr, "                     Disable automatic spelling correction of initialisms (e.g. \"URL\")")
	fmt.Fprintln(os.Stderr, "    read_write_private")
	fmt.Fprintln(os.Stderr, "                     Make read/write methods private, default is public Read/Write")
	fmt.Fprintln(os.Stderr, "    skip_remote")
	fmt.Fprintln(os.Stderr, "                     Skip the generating of -remote folders for the client binaries for services")
	fmt.Fprintln(os.Stderr, "    struct_key_entries")
	fmt.Fprintln(os.Stderr, "                     Generate maps keyed by a struct, union or exception as []thrift.MapEntry[*K, V] instead of map[*K]V")
	os.Exit(0)
}

func failure(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "[FAILURE] "+format+"\n", args...)
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	loader := &sema.Loader{Diag: &sema.Diagnostics{Out: os.Stderr, WarnLevel: 1, Path: "arguments"}}
	var generatorStrings []string
	outPath := ""
	outPathIsAbsolute := false
	recurse := false

	// The C++ compiler treats every argument but the last as options and
	// splits each of them on spaces.
	args := os.Args[1 : len(os.Args)-1]
	for i := 0; i < len(args); i++ {
		for _, arg := range strings.Split(args[i], " ") {
			if arg == "" {
				continue
			}
			if strings.HasPrefix(arg, "--") {
				arg = arg[1:]
			}
			switch arg {
			case "-help":
				help()
			case "-version":
				fmt.Printf("Thrift version %s\n", version.Version)
				os.Exit(0)
			case "-debug":
			case "-nowarn":
				loader.Diag.WarnLevel = 0
			case "-strict":
				loader.Strict = 255
				loader.Diag.WarnLevel = 2
			case "-v", "-verbose":
			case "-r", "-recurse":
				recurse = true
			case "-allow-neg-keys":
				loader.AllowNegFieldKeys = true
			case "-allow-64bit-consts":
				loader.Allow64BitConsts = true
			case "-gen":
				i++
				if i >= len(args) {
					fmt.Fprintln(os.Stderr, "Missing generator specification")
					usage()
				}
				generatorStrings = append(generatorStrings, args[i])
			case "-I":
				i++
				if i >= len(args) {
					fmt.Fprintln(os.Stderr, "Missing Include directory")
					usage()
				}
				loader.IncludeDirs = append(loader.IncludeDirs, args[i])
			case "-o", "-out":
				outPathIsAbsolute = arg == "-out"
				i++
				if i >= len(args) {
					fmt.Fprintln(os.Stderr, "-o: missing output directory")
					usage()
				}
				outPath = args[i]
				if st, err := os.Stat(outPath); err != nil || !st.IsDir() {
					fmt.Fprintf(os.Stderr, "Output directory %s is unusable: does not exist or is not a directory\n", outPath)
					os.Exit(255)
				}
			case "-audit", "-audit-nofatal", "-audit-allow-optional-field-removal",
				"-audit-allow-required-field-to-default", "-Iold", "-Inew":
				failure("The audit mode is not available in this compiler; use the C++ thrift compiler.")
			default:
				fmt.Fprintf(os.Stderr, "Unrecognized option: %s\n", arg)
				usage()
			}
		}
	}

	last := os.Args[len(os.Args)-1]
	switch last {
	case "-help", "--help":
		help()
	case "-version", "--version":
		fmt.Printf("Thrift version %s\n", version.Version)
		os.Exit(0)
	}

	if len(generatorStrings) == 0 {
		fmt.Fprintln(os.Stderr, "No output language(s) specified")
		usage()
	}

	var optionSets []golang.Options
	for _, spec := range generatorStrings {
		language, options := spec, ""
		if i := strings.IndexByte(spec, ':'); i >= 0 {
			language, options = spec[:i], spec[i+1:]
		}
		if language != "go" {
			failure("Unable to get a generator for \"%s\": only the go generator is available in this compiler.", spec)
		}
		opts, err := golang.ParseOptions(options)
		if err != nil {
			failure("Error: %s", err.Error())
		}
		optionSets = append(optionSets, opts)
	}

	program, err := loader.Load(last)
	if err != nil {
		failure("%s", err.Error())
	}
	if outPath != "" {
		program.SetOutPath(outPath, outPathIsAbsolute)
	}
	loader.Diag.Path = "generation"
	loader.Diag.Line = 1
	generate(program, optionSets, recurse)
}

// generate is generate() in main.cc: with -r the included programs come
// first, each inheriting the output path.
func generate(program *sema.Program, optionSets []golang.Options, recurse bool) {
	if recurse {
		program.SetRecursive(true)
		for _, inc := range program.Includes() {
			inc.SetOutPath(program.OutPath(), program.IsOutPathAbsolute())
			generate(inc, optionSets, recurse)
		}
	}
	if err := generateProgram(program, optionSets); err != nil {
		failure("Error: %s", err.Error())
	}
}

func generateProgram(program *sema.Program, optionSets []golang.Options) (err error) {
	defer func() {
		if r := recover(); r != nil {
			if e, ok := r.(*sema.Error); ok {
				err = e
				return
			}
			panic(r)
		}
	}()
	program.Scope.ResolveAllConsts()
	for _, opts := range optionSets {
		sema.ValidateInput(program)
		if err := golang.New(program, opts).Generate(); err != nil {
			return err
		}
	}
	return nil
}
