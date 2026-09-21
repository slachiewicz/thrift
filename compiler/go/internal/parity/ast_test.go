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
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/apache/thrift/compiler/go/internal/jsondump"
	"github.com/apache/thrift/compiler/go/sema"
)

// firstDiff describes the first line on which two texts differ.
func firstDiff(want, got string) string {
	wl := strings.Split(want, "\n")
	gl := strings.Split(got, "\n")
	for i := 0; i < len(wl) || i < len(gl); i++ {
		var w, g string
		if i < len(wl) {
			w = wl[i]
		}
		if i < len(gl) {
			g = gl[i]
		}
		if w != g {
			return fmt.Sprintf("line %d:\n  cpp: %q\n  go:  %q", i+1, w, g)
		}
	}
	return "no difference"
}

// TestASTParity checks the front end alone: the Go loader's view of every
// corpus file, rendered like --gen json, must equal the C++ compiler's
// --gen json output. When the C++ compiler rejects a file, the Go loader
// must reject it too.
func TestASTParity(t *testing.T) {
	root := RepoRoot(t)
	thrift := Compiler(t, root)
	for _, file := range Corpus(t, root) {
		rel, _ := filepath.Rel(root, file)
		t.Run(rel, func(t *testing.T) {
			out := t.TempDir()
			cmd := exec.Command(thrift, "-out", out, "--gen", "json", file)
			// The C++ compiler prints warnings and some failures to
			// stdout, so keep both streams.
			var output bytes.Buffer
			cmd.Stdout = &output
			cmd.Stderr = &output
			cppErr := cmd.Run()
			cppOut := strings.TrimSpace(output.String())

			loader := &sema.Loader{}
			prog, goErr := loader.Load(file)

			if cppErr != nil {
				if strings.Contains(cppOut, "[FAILURE:generation:") {
					// The JSON generator failed on a program the front end
					// accepted; the output parity test covers such files.
					t.Skipf("the JSON generator cannot render this program: %s", cppOut)
				}
				if strings.Contains(cppOut, "\" not defined") {
					// A typedef the front end could not resolve, which the
					// C++ compiler reports only once a generator asks for
					// the type. The Go loader is as lazy, so the output
					// parity test covers such files. lib/go/test's
					// NamespacedTest.thrift takes this path in a clean
					// checkout: its ThriftTest.thrift include is made by
					// `make -C lib/go/test`.
					t.Skipf("the C++ compiler failed past the front end: %s", cppOut)
				}
				if goErr == nil {
					t.Fatalf("C++ compiler rejected the file but the Go loader accepted it.\ncpp: %s", cppOut)
				}
				t.Logf("both reject: cpp=%q go=%q", cppOut, goErr)
				return
			}
			if goErr != nil {
				t.Fatalf("C++ compiler accepted the file but the Go loader rejected it: %v", goErr)
			}

			want, err := os.ReadFile(filepath.Join(out, prog.Name()+".json"))
			if err != nil {
				t.Fatal(err)
			}
			got := jsondump.Dump(prog)
			if string(want) != got {
				t.Fatalf("JSON differs at %s", firstDiff(string(want), got))
			}
		})
	}
}
