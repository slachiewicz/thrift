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
// Only the Go and Java generators are available.
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/apache/thrift/compiler/go/audit"
	"github.com/apache/thrift/compiler/go/generate/golang"
	"github.com/apache/thrift/compiler/go/generate/java"
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
	fmt.Fprintln(os.Stderr, "  java (Java):")
	fmt.Fprintln(os.Stderr, "    beans:           Members will be private, and setter methods will return void.")
	fmt.Fprintln(os.Stderr, "    private_members: Members will be private, but setter methods will return 'this' like usual.")
	fmt.Fprintln(os.Stderr, "    private-members: Same as 'private_members' (deprecated).")
	fmt.Fprintln(os.Stderr, "    nocamel:         Do not use CamelCase field accessors with beans.")
	fmt.Fprintln(os.Stderr, "    fullcamel:       Convert underscored_accessor_or_service_names to camelCase.")
	fmt.Fprintln(os.Stderr, "    android:         Generated structures are Parcelable.")
	fmt.Fprintln(os.Stderr, "    android_legacy:  Do not use java.io.IOException(throwable) (available for Android 2.3 and above).")
	fmt.Fprintln(os.Stderr, "    option_type=[thrift|jdk8]:")
	fmt.Fprintln(os.Stderr, "                     thrift: wrap optional fields in thrift Option type.")
	fmt.Fprintln(os.Stderr, "                     jdk8: Wrap optional fields in JDK8+ Option type.")
	fmt.Fprintln(os.Stderr, "                     If the Option type is not specified, 'thrift' is used.")
	fmt.Fprintln(os.Stderr, "    rethrow_unhandled_exceptions:")
	fmt.Fprintln(os.Stderr, "                     Enable rethrow of unhandled exceptions and let them propagate further. (Default behavior is to catch and log it.)")
	fmt.Fprintln(os.Stderr, "    java5:           Generate Java 1.5 compliant code (includes android_legacy flag).")
	fmt.Fprintln(os.Stderr, "    future_iface:    Generate CompletableFuture based iface based on async client.")
	fmt.Fprintln(os.Stderr, "    reuse_objects:   Data objects will not be allocated, but existing instances will be used (read and write).")
	fmt.Fprintln(os.Stderr, "    reuse-objects:   Same as 'reuse_objects' (deprecated).")
	fmt.Fprintln(os.Stderr, "    sorted_containers:")
	fmt.Fprintln(os.Stderr, "                     Use TreeSet/TreeMap instead of HashSet/HashMap as a implementation of set/map.")
	fmt.Fprintln(os.Stderr, "    generated_annotations=[undated|suppress]:")
	fmt.Fprintln(os.Stderr, "                     undated: suppress the date at @Generated annotations")
	fmt.Fprintln(os.Stderr, "                     suppress: suppress @Generated annotations entirely")
	fmt.Fprintln(os.Stderr, "    unsafe_binaries: Do not copy ByteBuffers in constructors, getters, and setters.")
	fmt.Fprintln(os.Stderr, "    jakarta_annotations: generate jakarta annotations (javax by default)")
	fmt.Fprintln(os.Stderr, "    annotations_as_metadata:")
	fmt.Fprintln(os.Stderr, "                     Include Thrift field annotations as metadata in the generated code.")
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

	var generators []generatorFactory
	for _, spec := range generatorStrings {
		language, options := spec, ""
		if i := strings.IndexByte(spec, ':'); i >= 0 {
			language, options = spec[:i], spec[i+1:]
		}
		switch language {
		case "go":
			opts, err := golang.ParseOptions(options)
			if err != nil {
				failure("Error: %s", err.Error())
			}
			generators = append(generators, func(p *sema.Program) generator { return golang.New(p, opts) })
		case "java":
			opts, err := java.ParseOptions(options)
			if err != nil {
				failure("Error: %s", err.Error())
			}
			generators = append(generators, func(p *sema.Program) generator { return java.New(p, opts) })
		default:
			failure("Unable to get a generator for \"%s\": only the go and java generators are available in this compiler.", spec)
		}
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
	generate(program, generators, recurse)
}

// generator is what every language generator offers.
type generator interface {
	Generate() error
}

// generatorFactory builds a generator for one program; it is the
// t_generator_registry lookup with the options already parsed.
type generatorFactory func(*sema.Program) generator

// generate is generate() in main.cc: with -r the included programs come
// first, each inheriting the output path.
func generate(program *sema.Program, generators []generatorFactory, recurse bool) {
	if recurse {
		program.SetRecursive(true)
		for _, inc := range program.Includes() {
			inc.SetOutPath(program.OutPath(), program.IsOutPathAbsolute())
			generate(inc, generators, recurse)
		}
	}
	if err := generateProgram(program, generators); err != nil {
		failure("Error: %s", err.Error())
	}
}

func generateProgram(program *sema.Program, generators []generatorFactory) (err error) {
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
	for _, newGenerator := range generators {
		sema.ValidateInput(program)
		if err := newGenerator(program).Generate(); err != nil {
			return err
		}
	}
	return nil
}
