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

// Package generate is the generator registry: t_generator_registry.h. Each
// language package registers itself from init() with its name, its long
// name, its option table and a constructor, and the command looks the
// language of a --gen argument up here.
package generate

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/apache/thrift/compiler/go/sema"
)

// Option describes one generator option: the part after "lang:" of a
// --gen argument, as key or key=value.
type Option struct {
	// Name is the canonical key, in the C++ spelling.
	Name string
	// Aliases are other accepted spellings.
	Aliases []string
	// Deprecated names an alias that is documented as deprecated.
	Deprecated []string
	// Value is the placeholder shown for an option that takes a value,
	// such as "prefix" or "[undated|suppress]"; empty for a flag.
	Value string
	// Help is the description, one or more lines.
	Help string
}

// Runner generates one program, and with recurse every program it
// includes first.
type Runner interface {
	Run(program *sema.Program, recurse bool) error
}

// Info is what a language package registers: THRIFT_REGISTER_GENERATOR's
// arguments plus the option table the help text and the command line
// are built from.
type Info struct {
	// Name is the language as written in --gen, such as "go".
	Name string
	// LongName is the display name, such as "Go".
	LongName string
	// Options is the option table, in the order the help text lists it.
	Options []Option
	// Parse parses the option spec and returns the generator to run.
	Parse func(spec string) (Runner, error)
}

var (
	mu       sync.Mutex
	registry = map[string]Info{}
)

// Register adds a generator. Registering a name twice is a programming
// error.
func Register(info Info) {
	mu.Lock()
	defer mu.Unlock()
	if info.Name == "" || info.Parse == nil {
		panic("generate.Register: a generator needs a name and a Parse function")
	}
	if _, dup := registry[info.Name]; dup {
		panic("generate.Register: generator " + info.Name + " registered twice")
	}
	registry[info.Name] = info
}

// Lookup returns the generator registered under name.
func Lookup(name string) (Info, bool) {
	mu.Lock()
	defer mu.Unlock()
	info, ok := registry[name]
	return info, ok
}

// All returns every registered generator, sorted by name like the
// std::map the C++ registry keeps.
func All() []Info {
	mu.Lock()
	defer mu.Unlock()
	infos := make([]Info, 0, len(registry))
	for _, info := range registry {
		infos = append(infos, info)
	}
	sort.Slice(infos, func(i, j int) bool { return infos[i].Name < infos[j].Name })
	return infos
}

// New parses a --gen argument, "lang" or "lang:options", and returns the
// generator to run.
func New(spec string) (Runner, error) {
	language, options := spec, ""
	if i := strings.IndexByte(spec, ':'); i >= 0 {
		language, options = spec[:i], spec[i+1:]
	}
	info, ok := Lookup(language)
	if !ok {
		return nil, fmt.Errorf("Unable to get a generator for \"%s\"", spec)
	}
	return info.Parse(options)
}

// Documentation renders the option table the way the C++ help text lists
// a generator's options: four spaces, the key with its value placeholder
// and a colon, then the help aligned at column 21, on the next line when
// the key is too long for that.
func (info Info) Documentation() string {
	var sb strings.Builder
	for _, o := range info.Options {
		writeOption(&sb, o.Name, o.Value, o.Help)
		for _, alias := range o.Aliases {
			writeOption(&sb, alias, o.Value, "Same as '"+o.Name+"'.")
		}
		for _, alias := range o.Deprecated {
			writeOption(&sb, alias, o.Value, "Same as '"+o.Name+"' (deprecated).")
		}
	}
	return sb.String()
}

const (
	docIndent = "    "
	docColumn = 21
)

func writeOption(sb *strings.Builder, name, value, help string) {
	key := name
	if value != "" {
		key += "=" + value
	}
	key += ":"
	sb.WriteString(docIndent + key)
	lines := strings.Split(help, "\n")
	if len(docIndent)+len(key) < docColumn {
		sb.WriteString(strings.Repeat(" ", docColumn-len(docIndent)-len(key)))
	} else {
		sb.WriteString("\n" + strings.Repeat(" ", docColumn))
	}
	sb.WriteString(lines[0] + "\n")
	for _, line := range lines[1:] {
		sb.WriteString(strings.Repeat(" ", docColumn) + line + "\n")
	}
}
