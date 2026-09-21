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
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/apache/thrift/compiler/go/generate/golang"
	"github.com/apache/thrift/compiler/go/generate/java"
	"github.com/apache/thrift/compiler/go/sema"
)

// The golden tests run on every checkout, without the C++ compiler. For
// every option row they generate the corpus and compare a digest of each
// output file against the manifest under testdata/golden, and for a few
// files whose output is worth reading they compare the files themselves.
//
//	go test ./compiler/go/internal/parity -run 'TestGolden' -update
//
// rewrites the manifests and the trees from the current generator. Run
// the oracle tests first: the manifests record what the C++ compiler
// produced, and -update only trusts a generator that has passed them.
var update = flag.Bool("update", false, "rewrite the golden manifests and trees")

// goldenDate is the @Generated date the Java generator writes in the
// golden tests, so that dated rows are reproducible.
var goldenDate = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

// treeFiles are the corpus files whose full output is stored per
// language, so that a change shows as a diff rather than a digest. They
// are small; the large files (ThriftTest, DocTest, IncludesTest) are
// covered by the manifests, and the oracle run shows their diffs.
var treeFiles = map[string][]string{
	"go": {
		"tutorial/tutorial.thrift",
		"test/AnnotationTest.thrift",
		"test/Recursive.thrift",
		"lib/go/test/ValidateTest.thrift",
		"compiler/go/testdata/accept/ConstEdgeCases.thrift",
		"compiler/go/testdata/accept/FieldEdgeCases.thrift",
		"compiler/go/testdata/accept/DocEdgeCases.thrift",
	},
	"java": {
		"tutorial/tutorial.thrift",
		"compiler/go/testdata/accept/FieldEdgeCases.thrift",
		"compiler/go/testdata/accept/DocEdgeCases.thrift",
	},
}

// treeRows are the rows whose trees are stored, one per language.
var treeRows = map[string]string{"go": "base-r", "java": "jakarta"}

func goldenDir(root string) string {
	return filepath.Join(root, "compiler", "go", "internal", "parity", "testdata", "golden")
}

// generateGo runs the Go generator on one corpus file in-process and
// returns the output tree, or the error the generator reported.
func generateGo(t *testing.T, file string, row optionRow) (map[string]string, error) {
	t.Helper()
	out := t.TempDir()
	opts, err := golang.ParseOptions(row.spec)
	if err != nil {
		t.Fatal(err)
	}
	loader := &sema.Loader{}
	prog, err := loader.Load(file)
	if err != nil {
		return nil, err
	}
	prog.SetOutPath(out, true)
	if err := golang.Run(prog, opts, row.recurse); err != nil {
		return nil, err
	}
	return readTree(t, out), nil
}

// generateJava is generateGo for the Java generator.
func generateJava(t *testing.T, file string, row optionRow) (map[string]string, error) {
	t.Helper()
	out := t.TempDir()
	opts, err := java.ParseOptions(row.spec)
	if err != nil {
		t.Fatal(err)
	}
	loader := &sema.Loader{}
	prog, err := loader.Load(file)
	if err != nil {
		return nil, err
	}
	prog.SetOutPath(out, true)
	if err := java.Run(prog, opts, row.recurse); err != nil {
		return nil, err
	}
	return readTree(t, out), nil
}

// A manifest maps "corpus file\toutput path" to the digest of the output,
// and "corpus file" alone to "reject\t<message>" for a file the generator
// rejects or to "empty" for one it accepts without writing anything.
type manifest map[string]string

const (
	rejectPrefix = "reject\t"
	emptyMark    = "empty"
)

func manifestPath(root, lang, row string) string {
	return filepath.Join(goldenDir(root), lang, row+".sum")
}

func readManifest(t *testing.T, path string) manifest {
	t.Helper()
	m := manifest{}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return m
	}
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		fields := strings.SplitN(line, "\t", 3)
		switch {
		case len(fields) == 3 && fields[0] == "reject":
			m[fields[1]] = rejectPrefix + fields[2]
		case len(fields) == 2 && fields[0] == emptyMark:
			m[fields[1]] = emptyMark
		case len(fields) == 3:
			m[fields[1]+"\t"+fields[2]] = fields[0]
		default:
			t.Fatalf("%s: cannot parse %q", path, line)
		}
	}
	return m
}

func writeManifest(t *testing.T, path string, m manifest) {
	t.Helper()
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var sb strings.Builder
	for _, k := range keys {
		v := m[k]
		switch {
		case strings.HasPrefix(v, rejectPrefix):
			sb.WriteString("reject\t" + k + "\t" + strings.TrimPrefix(v, rejectPrefix) + "\n")
		case v == emptyMark:
			sb.WriteString(emptyMark + "\t" + k + "\n")
		default:
			sb.WriteString(v + "\t" + k + "\n")
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
}

// digest is the first 64 bits of the sha256, enough to tell a change
// apart and short enough to keep the manifests readable.
func digest(content string) string {
	sum := sha256.Sum256([]byte(content))
	return hex.EncodeToString(sum[:8])
}

// entriesFor renders one corpus file's result as manifest entries. A
// rejection message names the file by its absolute path; the manifest
// stores it relative to the repository.
func entriesFor(root, rel string, files map[string]string, err error) manifest {
	m := manifest{}
	if err != nil {
		msg := strings.TrimSpace(err.Error())
		msg = strings.ReplaceAll(msg, filepath.ToSlash(root)+"/", "")
		m[rel] = rejectPrefix + msg
		return m
	}
	if len(files) == 0 {
		m[rel] = emptyMark
		return m
	}
	for name, content := range files {
		m[rel+"\t"+name] = digest(content)
	}
	return m
}

// checkManifest compares the entries of one corpus file against the
// manifest and reports every difference.
func checkManifest(t *testing.T, want manifest, rel string, got manifest) {
	t.Helper()
	var wantKeys []string
	for k := range want {
		if k == rel || strings.HasPrefix(k, rel+"\t") {
			wantKeys = append(wantKeys, k)
		}
	}
	sort.Strings(wantKeys)
	if len(wantKeys) == 0 {
		t.Fatalf("%s is not in the manifest; run with -update", rel)
	}
	for _, k := range wantKeys {
		g, ok := got[k]
		switch {
		case !ok:
			t.Errorf("%s: missing from the output", strings.ReplaceAll(k, "\t", " "))
		case g != want[k]:
			t.Errorf("%s: changed (manifest %s, output %s)", strings.ReplaceAll(k, "\t", " "), short(want[k]), short(g))
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("%s: not in the manifest", strings.ReplaceAll(k, "\t", " "))
		}
	}
}

func short(v string) string {
	if strings.HasPrefix(v, rejectPrefix) {
		return v
	}
	if len(v) > 12 {
		return v[:12]
	}
	return v
}

// runGolden is the body of the golden tests for one language.
func runGolden(t *testing.T, lang string, rows []optionRow, generate func(*testing.T, string, optionRow) (map[string]string, error)) {
	root := RepoRoot(t)
	files := Corpus(t, root)
	for _, row := range rows {
		row := row
		t.Run(row.name, func(t *testing.T) {
			path := manifestPath(root, lang, row.name)
			want := readManifest(t, path)
			got := manifest{}
			for _, file := range files {
				rel, _ := filepath.Rel(root, file)
				rel = filepath.ToSlash(rel)
				t.Run(rel, func(t *testing.T) {
					tree, err := generate(t, file, row)
					entries := entriesFor(root, rel, tree, err)
					for k, v := range entries {
						got[k] = v
					}
					if *update {
						return
					}
					checkManifest(t, want, rel, entries)
				})
			}
			if *update {
				writeManifest(t, path, got)
			}
		})
	}
}

func TestGoldenGo(t *testing.T) {
	runGolden(t, "go", optionRows, generateGo)
}

func TestGoldenJava(t *testing.T) {
	java.Now = func() time.Time { return goldenDate }
	defer func() { java.Now = time.Now }()
	runGolden(t, "java", javaRows, generateJava)
}

// treeDir is where the stored output of one corpus file lives.
func treeDir(root, lang, row, rel string) string {
	slug := strings.NewReplacer("/", "__", ".thrift", "").Replace(rel)
	return filepath.Join(goldenDir(root), "trees", lang, row, slug)
}

func findRow(rows []optionRow, name string) optionRow {
	for _, r := range rows {
		if r.name == name {
			return r
		}
	}
	panic("no row " + name)
}

// TestGoldenTrees compares the full output of the tree files against the
// stored trees, so that a change reads as a diff.
func TestGoldenTrees(t *testing.T) {
	root := RepoRoot(t)
	java.Now = func() time.Time { return goldenDate }
	defer func() { java.Now = time.Now }()
	for _, lang := range []string{"go", "java"} {
		var row optionRow
		var generate func(*testing.T, string, optionRow) (map[string]string, error)
		switch lang {
		case "go":
			row, generate = findRow(optionRows, treeRows[lang]), generateGo
		case "java":
			row, generate = findRow(javaRows, treeRows[lang]), generateJava
		}
		for _, rel := range treeFiles[lang] {
			t.Run(lang+"/"+rel, func(t *testing.T) {
				got, err := generate(t, filepath.Join(root, filepath.FromSlash(rel)), row)
				if err != nil {
					t.Fatal(err)
				}
				dir := treeDir(root, lang, row.name, rel)
				if *update {
					if err := os.RemoveAll(dir); err != nil {
						t.Fatal(err)
					}
					for name, content := range got {
						path := filepath.Join(dir, filepath.FromSlash(name))
						if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
							t.Fatal(err)
						}
					}
					return
				}
				if _, err := os.Stat(dir); err != nil {
					t.Fatalf("%s is missing; run with -update", dir)
				}
				want := readTree(t, dir)
				if strings.Join(sortedKeys(want), "\n") != strings.Join(sortedKeys(got), "\n") {
					t.Fatalf("file sets differ.\nstored: %v\noutput: %v", sortedKeys(want), sortedKeys(got))
				}
				for _, name := range sortedKeys(want) {
					if want[name] != got[name] {
						t.Errorf("%s differs at %s", name, firstDiff(want[name], got[name]))
					}
				}
			})
		}
	}
}
