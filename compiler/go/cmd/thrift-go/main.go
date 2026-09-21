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

// Command thrift-go compiles Thrift IDL to Go and to Java.
//
// It accepts the command line of the C++ thrift compiler, so that the
// existing build files can point their THRIFT variable at it:
//
//	thrift-go [options] --gen go[:option,...] file.thrift
//	thrift-go [options] --gen java[:option,...] file.thrift
//
// The generators come from the registry in compiler/go/generate; -help
// lists the ones this binary was built with.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/apache/thrift/compiler/go/audit"
	"github.com/apache/thrift/compiler/go/generate"
	_ "github.com/apache/thrift/compiler/go/generate/golang" // registers go
	_ "github.com/apache/thrift/compiler/go/generate/java"   // registers java
	"github.com/apache/thrift/compiler/go/internal/version"
	"github.com/apache/thrift/compiler/go/sema"
)

func usage() {
	fmt.Fprintf(os.Stderr, "Usage: %s [options] file\n", os.Args[0])
	fmt.Fprintf(os.Stderr, "       %s decode [flags] [file]\n", os.Args[0])
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
	fmt.Fprintln(os.Stderr, "Options related to audit operation")
	fmt.Fprintln(os.Stderr, "   --audit OldFile   Old Thrift file to be audited with 'file'")
	fmt.Fprintln(os.Stderr, "   --audit-allow-optional-field-removal")
	fmt.Fprintln(os.Stderr, "                Allow explicitly optional fields to be removed")
	fmt.Fprintln(os.Stderr, "   --audit-allow-required-field-to-default")
	fmt.Fprintln(os.Stderr, "                Allow required fields to use default requiredness")
	fmt.Fprintln(os.Stderr, "                Binding-dependent; includes service method arguments")
	fmt.Fprintln(os.Stderr, "  -Iold dir    Add a directory to the list of directories")
	fmt.Fprintln(os.Stderr, "                searched for include directives for old thrift file")
	fmt.Fprintln(os.Stderr, "  -Inew dir    Add a directory to the list of directories")
	fmt.Fprintln(os.Stderr, "                searched for include directives for new thrift file")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Available generators (and options):")
	for _, info := range generate.All() {
		fmt.Fprintf(os.Stderr, "  %s (%s):\n", info.Name, info.LongName)
		fmt.Fprint(os.Stderr, info.Documentation())
	}
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
	// Subcommands come first; everything else is the C++ compiler's
	// command line.
	switch os.Args[1] {
	case "decode":
		runDecode(os.Args[2:])
		return
	}

	loader := &sema.Loader{Diag: &sema.Diagnostics{Out: os.Stderr, WarnLevel: 1, Path: "arguments"}}
	var generatorStrings []string
	auditMode := false
	auditFatal := true
	auditOpts := audit.Options{WarnLevel: 1, Stdout: os.Stdout, Stderr: os.Stderr}
	oldInputFile, oldIncludePath, newIncludePath := "", "", ""
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
				auditOpts.WarnLevel = 0
			case "-strict":
				loader.Strict = 255
				loader.Diag.WarnLevel = 2
				auditOpts.WarnLevel = 2
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
			case "-audit":
				auditMode = true
				i++
				if i >= len(args) {
					fmt.Fprintln(os.Stderr, "Missing old thrift file name for audit operation")
					usage()
				}
				oldInputFile = args[i]
			case "-audit-nofatal":
				auditFatal = false
			case "-audit-allow-optional-field-removal":
				auditOpts.AllowOptionalFieldRemoval = true
			case "-audit-allow-required-field-to-default":
				auditOpts.AllowRequiredFieldToDefault = true
			case "-Iold":
				i++
				if i >= len(args) {
					fmt.Fprintln(os.Stderr, "Missing Include directory for old thrift file")
					usage()
				}
				oldIncludePath = args[i]
			case "-Inew":
				i++
				if i >= len(args) {
					fmt.Fprintln(os.Stderr, "Missing Include directory for new thrift file")
					usage()
				}
				newIncludePath = args[i]
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

	if auditMode {
		if oldInputFile == "" {
			fmt.Fprintln(os.Stderr, "Missing file name of old thrift file for audit")
			usage()
		}
		// The old file is parsed first, each file with its own include
		// directory added to the shared ones, like audit() in main.cc.
		shared := loader.IncludeDirs
		if oldIncludePath != "" {
			loader.IncludeDirs = append(append([]string{}, shared...), oldIncludePath)
		}
		oldProgram, err := loader.Load(oldInputFile)
		if err != nil {
			failure("%s", err.Error())
		}
		loader.IncludeDirs = shared
		if newIncludePath != "" {
			loader.IncludeDirs = append(append([]string{}, shared...), newIncludePath)
		}
		newProgram, err := loader.Load(last)
		if err != nil {
			failure("%s", err.Error())
		}
		if audit.Audit(newProgram, oldProgram, auditOpts) && auditFatal {
			os.Exit(2)
		}
		return
	}

	if len(generatorStrings) == 0 {
		fmt.Fprintln(os.Stderr, "No output language(s) specified")
		usage()
	}

	var generators []generate.Runner
	for _, spec := range generatorStrings {
		g, err := generate.New(spec)
		if err != nil {
			failure("%s", err.Error())
		}
		generators = append(generators, g)
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
	generateAll(program, generators, recurse)
}

// generateAll is generate() in main.cc: with -r the included programs come
// first, each inheriting the output path, and every generator runs on a
// program before the next program.
func generateAll(program *sema.Program, generators []generate.Runner, recurse bool) {
	if recurse {
		program.SetRecursive(true)
		for _, inc := range program.Includes() {
			inc.SetOutPath(program.OutPath(), program.IsOutPathAbsolute())
			generateAll(inc, generators, recurse)
		}
	}
	for _, g := range generators {
		if err := g.Run(program, false); err != nil {
			failure("Error: %s", err.Error())
		}
	}
}
