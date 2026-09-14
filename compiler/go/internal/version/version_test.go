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

package version

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestMatchesConfigureAC fails when the version constant drifts from the
// AC_INIT line of configure.ac, which is what the C++ compiler embeds in
// every generated file.
func TestMatchesConfigureAC(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var src []byte
	for {
		src, err = os.ReadFile(filepath.Join(dir, "configure.ac"))
		if err == nil {
			break
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("configure.ac not found; not running inside the repository")
		}
		dir = parent
	}
	m := regexp.MustCompile(`AC_INIT\(\[thrift\], \[([^\]]+)\]\)`).FindSubmatch(src)
	if m == nil {
		t.Fatal("AC_INIT line not found in configure.ac")
	}
	if got := string(m[1]); got != Version {
		t.Fatalf("configure.ac says %q, version.Version is %q", got, Version)
	}
}
