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
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/apache/thrift/compiler/go/generate"
)

func runLanguages(args []string) int {
	fs := newFlagSet("languages", "[--json]", func(w io.Writer) {
		fmt.Fprintln(w, "List the generators this binary was built with and their options.")
	})
	asJSON := fs.Bool("json", false, "print the listing as JSON")
	if code := parseFlags(fs, args); code >= 0 {
		return code
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "languages takes no arguments")
		return exitError
	}
	infos := generate.All()
	if *asJSON {
		type option struct {
			Name       string   `json:"name"`
			Flag       string   `json:"flag"`
			Aliases    []string `json:"aliases,omitempty"`
			Deprecated []string `json:"deprecated,omitempty"`
			Value      string   `json:"value,omitempty"`
			Help       string   `json:"help"`
		}
		type language struct {
			Name     string   `json:"name"`
			LongName string   `json:"long_name"`
			Options  []option `json:"options"`
		}
		var out []language
		for _, info := range infos {
			l := language{Name: info.Name, LongName: info.LongName, Options: []option{}}
			for _, o := range info.Options {
				l.Options = append(l.Options, option{
					Name: o.Name, Flag: "--" + info.Name + "." + o.Name,
					Aliases: o.Aliases, Deprecated: o.Deprecated, Value: o.Value, Help: o.Help,
				})
			}
			out = append(out, l)
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintln(os.Stderr, "languages:", err)
			return exitError
		}
		return exitOK
	}
	for _, info := range infos {
		var names []string
		for _, o := range info.Options {
			names = append(names, o.Name)
		}
		fmt.Printf("%-8s %-12s options: %s\n", info.Name, info.LongName, strings.Join(names, ", "))
	}
	return exitOK
}
