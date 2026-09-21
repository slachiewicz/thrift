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

package java

import (
	"github.com/apache/thrift/compiler/go/generate"
	"github.com/apache/thrift/compiler/go/sema"
)

// The option table of THRIFT_REGISTER_GENERATOR(java, ...).
func init() {
	generate.Register(generate.Info{
		Name:     "java",
		LongName: "Java",
		Options: []generate.Option{
			{Name: "beans", Help: "Members will be private, and setter methods will return void."},
			{Name: "private_members", Deprecated: []string{"private-members"}, Help: "Members will be private, but setter methods will return 'this' like usual."},
			{Name: "nocamel", Help: "Do not use CamelCase field accessors with beans."},
			{Name: "fullcamel", Help: "Convert underscored_accessor_or_service_names to camelCase."},
			{Name: "android", Help: "Generated structures are Parcelable."},
			{Name: "android_legacy", Help: "Do not use java.io.IOException(throwable) (available for Android 2.3 and above)."},
			{Name: "option_type", Value: "[thrift|jdk8]", Help: "thrift: wrap optional fields in thrift Option type.\njdk8: Wrap optional fields in JDK8+ Option type.\nIf the Option type is not specified, 'thrift' is used."},
			{Name: "rethrow_unhandled_exceptions", Help: "Enable rethrow of unhandled exceptions and let them propagate further. (Default behavior is to catch and log it.)"},
			{Name: "java5", Help: "Generate Java 1.5 compliant code (includes android_legacy flag)."},
			{Name: "future_iface", Help: "Generate CompletableFuture based iface based on async client."},
			{Name: "reuse_objects", Deprecated: []string{"reuse-objects"}, Help: "Data objects will not be allocated, but existing instances will be used (read and write)."},
			{Name: "sorted_containers", Help: "Use TreeSet/TreeMap instead of HashSet/HashMap as a implementation of set/map."},
			{Name: "generated_annotations", Value: "[undated|suppress]", Help: "undated: suppress the date at @Generated annotations\nsuppress: suppress @Generated annotations entirely"},
			{Name: "unsafe_binaries", Help: "Do not copy ByteBuffers in constructors, getters, and setters."},
			{Name: "jakarta_annotations", Help: "generate jakarta annotations (javax by default)"},
			{Name: "annotations_as_metadata", Help: "Include Thrift field annotations as metadata in the generated code."},
		},
		Parse: func(spec string) (generate.Runner, error) {
			opts, err := ParseOptions(spec)
			if err != nil {
				return nil, err
			}
			return runner{opts}, nil
		},
	})
}

// runner is the registry's view of one parsed --gen java argument.
type runner struct{ opts Options }

func (r runner) Run(program *sema.Program, recurse bool) error {
	return Run(program, r.opts, recurse)
}

// Run generates the program and, when recurse is set, every program it
// includes first, each inheriting the output path. It is generate() in
// main.cc restricted to the Java generator.
func Run(program *sema.Program, opts Options, recurse bool) error {
	if recurse {
		program.SetRecursive(true)
		for _, inc := range program.Includes() {
			inc.SetOutPath(program.OutPath(), program.IsOutPathAbsolute())
			if err := Run(inc, opts, recurse); err != nil {
				return err
			}
		}
	}
	return generateOne(program, opts)
}

func generateOne(program *sema.Program, opts Options) (err error) {
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
	sema.ValidateInput(program)
	return New(program, opts).Generate()
}
