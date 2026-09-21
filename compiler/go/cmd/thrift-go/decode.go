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

	"github.com/apache/thrift/compiler/go/decode"
	"github.com/apache/thrift/lib/go/thrift"
)

// runDecode is the decode subcommand: thrift-go decode [flags] [file].
// It reads Thrift-encoded bytes and prints them without a schema.
func runDecode(args []string) int {
	var lf loaderFlags
	fs := newFlagSet("decode", "[flags] [file]", func(w io.Writer) {
		fmt.Fprintln(w, "Decode Thrift-encoded bytes from file, or from standard input when file is - or absent,")
		fmt.Fprintln(w, "and print them as a tree of field ids, types and values. With --idl, the tree also")
		fmt.Fprintln(w, "carries the field names, IDL types, enum members and union or exception kinds the")
		fmt.Fprintln(w, "IDL declares; a bare struct needs --type, a message needs the service (--service when")
		fmt.Fprintln(w, "the IDL declares more than one). A field the IDL does not declare, or whose wire type")
		fmt.Fprintln(w, "differs, is marked, not rejected.")
	})
	idl := fs.String("idl", "", "IDL file to decode against")
	typeName := fs.String("type", "", "with --idl: the struct a bare value is, as Name or Included.Name")
	service := fs.String("service", "", "with --idl: the service a message belongs to")
	lf.register(fs)
	protocol := fs.String("protocol", "auto", "wire protocol: auto, binary, compact or json")
	framed := fs.String("framed", "auto", "whether the input is length-prefixed frames: auto, yes or no")
	message := fs.String("message", "auto", "whether the input starts with a message header: auto, yes or no")
	asJSON := fs.Bool("json", false, "print the decoded tree as JSON")
	maxMessage := fs.Int("max-message-size", thrift.DEFAULT_MAX_MESSAGE_SIZE, "largest message or string accepted, in bytes")
	maxFrame := fs.Int("max-frame-size", thrift.DEFAULT_MAX_FRAME_SIZE, "largest frame accepted, in bytes")
	if code := parseFlags(fs, args); code >= 0 {
		return code
	}
	if fs.NArg() > 1 {
		fmt.Fprintln(os.Stderr, "decode: at most one input file")
		return exitError
	}
	for name, value := range map[string]string{"protocol": *protocol, "framed": *framed, "message": *message} {
		if !validChoice(name, value) {
			fmt.Fprintf(os.Stderr, "decode: invalid value %q for -%s\n", value, name)
			return exitError
		}
	}

	var data []byte
	var err error
	if fs.NArg() == 0 || fs.Arg(0) == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(fs.Arg(0))
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		return exitError
	}

	res, err := decode.Decode(data, decode.Options{
		Protocol: decode.Protocol(*protocol),
		Framed:   decode.Tristate(*framed),
		Message:  decode.Tristate(*message),
		Config:   &thrift.TConfiguration{MaxMessageSize: int32(*maxMessage), MaxFrameSize: int32(*maxFrame)},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "decode: %v\n", err)
		return exitError
	}
	if *idl != "" {
		program, err := lf.loader().Load(*idl)
		if err != nil {
			report(err)
			return exitError
		}
		schema := &decode.Schema{Program: program, Type: *typeName, Service: *service}
		if err := schema.Apply(res); err != nil {
			fmt.Fprintf(os.Stderr, "decode: %v\n", err)
			return exitError
		}
		res.Schema = true
	} else if *typeName != "" || *service != "" {
		fmt.Fprintln(os.Stderr, "decode: --type and --service need --idl")
		return exitError
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(decode.ToJSON(res)); err != nil {
			fmt.Fprintf(os.Stderr, "decode: %v\n", err)
			return exitError
		}
		return exitOK
	}
	decode.Format(os.Stdout, res)
	return exitOK
}

func validChoice(name, value string) bool {
	switch name {
	case "protocol":
		return value == "auto" || value == "binary" || value == "compact" || value == "json"
	default:
		return value == "auto" || value == "yes" || value == "no"
	}
}
