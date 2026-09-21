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
	"fmt"
	"io"
	"os"

	"github.com/apache/thrift/compiler/go/sema"
)

func runCheck(args []string) int {
	var lf loaderFlags
	fs := newFlagSet("check", "[flags] file.thrift...", func(w io.Writer) {
		fmt.Fprintln(w, "Parse and validate every file, including constant resolution and the checks the")
		fmt.Fprintf(w, "generators run first, without generating. Exit status %d on an error; with --strict,\n", exitError)
		fmt.Fprintf(w, "%d when a warning was printed.\n", exitPolicy)
	})
	lf.register(fs)
	if code := parseFlags(fs, args); code >= 0 {
		return code
	}
	if fs.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "check: no input files")
		return exitError
	}
	loader := lf.loader()
	status := exitOK
	for _, file := range fs.Args() {
		program, err := loader.Load(file)
		if err != nil {
			report(err)
			status = exitError
			continue
		}
		if err := validate(program); err != nil {
			report(err)
			status = exitError
		}
	}
	if status == exitOK && lf.strict && loader.Diag.Count > 0 {
		return exitPolicy
	}
	return status
}

// validate runs what a generator runs before emitting.
func validate(program *sema.Program) (err error) {
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
	return nil
}
