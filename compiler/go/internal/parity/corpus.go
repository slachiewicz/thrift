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

// Package parity compares the Go compiler with the C++ compiler over the
// repository's IDL corpus. The C++ binary is the oracle; nothing is
// committed as golden output.
package parity

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// RepoRoot returns the repository root, found by walking up from this
// package's directory.
func RepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "configure.ac")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("repository root not found")
		}
		dir = parent
	}
}

// Compiler returns the path of the C++ thrift compiler to use as the
// oracle, or skips the test. It reads THRIFT_COMPILER and falls back to
// compiler/cpp/thrift, which is where CI places the built binary.
func Compiler(t *testing.T, root string) string {
	t.Helper()
	if p := os.Getenv("THRIFT_COMPILER"); p != "" {
		return p
	}
	p := filepath.Join(root, "compiler", "cpp", "thrift")
	if _, err := os.Stat(p); err == nil {
		return p
	}
	t.Skip("THRIFT_COMPILER is not set and compiler/cpp/thrift does not exist")
	return ""
}

// corpusDirs are the directories, relative to the repository root, whose
// .thrift files form the positive corpus.
var corpusDirs = []string{
	"lib/go/test",
	"test",
	"tutorial",
	"contrib",
	"compiler/cpp/tests/cpp",
	"compiler/go/testdata/accept",
	"lib/java/src/test/resources",
}

// Corpus returns every .thrift file of the positive corpus, sorted.
// buildArtifacts are files under the corpus directories that a build
// writes and git ignores, and the files that include them. Their result
// depends on whether `make -C lib/go/test` has run, so they are not
// corpus: ThriftTest.thrift there is a filtered copy of test/ThriftTest.thrift,
// which is.
var buildArtifacts = map[string]bool{
	"lib/go/test/ThriftTest.thrift":     true,
	"lib/go/test/NamespacedTest.thrift": true,
	"lib/go/test/IncludesTest.thrift":   true,
}

func Corpus(t *testing.T, root string) []string {
	t.Helper()
	var files []string
	for _, d := range corpusDirs {
		dir := filepath.Join(root, d)
		err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				// Generated output directories are not corpus.
				if strings.HasPrefix(entry.Name(), "gen-") || entry.Name() == "gopath" {
					return filepath.SkipDir
				}
				return nil
			}
			if strings.HasSuffix(entry.Name(), ".thrift") {
				rel, _ := filepath.Rel(root, path)
				if buildArtifacts[filepath.ToSlash(rel)] {
					return nil
				}
				files = append(files, path)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(files)
	return files
}
