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

package ocaml

import (
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// generateService is t_ocaml_generator::generate_service. It does not write
// the files itself: f_service_/f_service_i_ in the C++ source are two
// ofstream_with_content_based_conditional_update members that live for the
// whole generator instance, and generate_service() reopens them for every
// service in the program. That class's open() discards whatever content is
// currently buffered instead of flushing it first
// (compiler/cpp/src/thrift/generate/t_generator.h,
// template_ofstream_with_content_based_conditional_update::open), and nothing
// ever calls close() between services, so only the *last* service processed
// for a program actually reaches disk - every earlier one is silently
// built and then discarded. Generate reproduces that by calling this for
// every service (so tmp_/indent_ still advance exactly as the C++ source's
// wasted work does) but only writing the result of the final call.
func (g *Generator) generateService(s *sema.Service) (fServiceName, fServiceIName, fService, fServiceI string) {
	outDir := g.outDir()
	fServiceName = outDir + capitalize(g.serviceName) + ".ml"
	fServiceIName = outDir + capitalize(g.serviceName) + ".mli"

	var svc, svcI strings.Builder

	svc.WriteString(g.ocamlAutogenComment() + "\n" + g.ocamlImports() + "\n")
	svcI.WriteString(g.ocamlAutogenComment() + "\n" + g.ocamlImports() + "\n")

	// The extends-namespace import the C++ source comments out is left
	// out here too.

	svc.WriteString("open " + capitalize(g.programName) + "_types" + "\n" + "\n")
	svcI.WriteString("open " + capitalize(g.programName) + "_types" + "\n" + "\n")

	// Generate the three main parts of the service
	g.generateServiceHelpers(&svc, s)
	g.generateServiceInterface(&svc, &svcI, s)
	g.generateServiceClient(&svc, &svcI, s)
	g.generateServiceServer(&svc, &svcI, s)

	return fServiceName, fServiceIName, svc.String(), svcI.String()
}

// generateServiceHelpers is t_ocaml_generator::generate_service_helpers:
// generates helper functions for a service.
func (g *Generator) generateServiceHelpers(fService *strings.Builder, s *sema.Service) {
	functions := s.Functions()

	fService.WriteString(g.indent() + "(* HELPER FUNCTIONS AND STRUCTURES *)" + "\n" + "\n")

	for _, f := range functions {
		ts := f.Arglist()
		g.generateOcamlStructDefinition(fService, ts, false)
		g.generateOcamlFunctionHelpers(fService, f)
	}
}

// generateOcamlFunctionHelpers is
// t_ocaml_generator::generate_ocaml_function_helpers: generates a struct
// and helpers for a function.
func (g *Generator) generateOcamlFunctionHelpers(fService *strings.Builder, f *sema.Function) {
	result := sema.NewStruct(g.program)
	result.SetName(decapitalize(f.Name()) + "_result")
	if !f.ReturnType().IsVoid() {
		success := sema.NewField(f.ReturnType(), "success", 0)
		result.Append(success)
	}

	for _, x := range f.Xceptions().Members() {
		result.Append(x)
	}
	g.generateOcamlStructDefinition(fService, result, false)
}

// generateServiceInterface is t_ocaml_generator::generate_service_interface.
func (g *Generator) generateServiceInterface(fService, fServiceI *strings.Builder, s *sema.Service) {
	fService.WriteString(g.indent() + "class virtual iface =" + "\n" + "object (self)" + "\n")
	fServiceI.WriteString(g.indent() + "class virtual iface :" + "\n" + "object" + "\n")

	g.indentUp()

	if s.Extends() != nil {
		extends := g.typeName(s.Extends())
		fService.WriteString(g.indent() + "inherit " + extends + ".iface" + "\n")
		fServiceI.WriteString(g.indent() + "inherit " + extends + ".iface" + "\n")
	}

	for _, f := range s.Functions() {
		ft := g.functionType(f.Arglist(), f.ReturnType(), true, true)
		fService.WriteString(g.indent() + "method virtual " + decapitalize(f.Name()) + " : " + ft + "\n")
		fServiceI.WriteString(g.indent() + "method virtual " + decapitalize(f.Name()) + " : " + ft + "\n")
	}
	g.indentDown()
	fService.WriteString(g.indent() + "end" + "\n" + "\n")
	fServiceI.WriteString(g.indent() + "end" + "\n" + "\n")
}

// generateServiceClient is t_ocaml_generator::generate_service_client:
// generates a service client definition. Note that in OCaml, the client
// doesn't implement iface. This is because the client does not (and should
// not have to) deal with arguments being None.
func (g *Generator) generateServiceClient(fService, fServiceI *strings.Builder, s *sema.Service) {
	fService.WriteString(g.indent() + "class client (iprot : Protocol.t) (oprot : Protocol.t) =" + "\n" +
		"object (self)" + "\n")
	fServiceI.WriteString(g.indent() + "class client : Protocol.t -> Protocol.t -> " + "\n" + "object" + "\n")
	g.indentUp()

	if s.Extends() != nil {
		extends := g.typeName(s.Extends())
		fService.WriteString(g.indent() + "inherit " + extends + ".client iprot oprot as super" + "\n")
		fServiceI.WriteString(g.indent() + "inherit " + extends + ".client" + "\n")
	}
	fService.WriteString(g.indent() + "val mutable seqid = 0" + "\n")

	// Generate client method implementations
	for _, f := range s.Functions() {
		argStruct := f.Arglist()
		fields := argStruct.Members()
		funname := f.Name()

		// Open function
		fService.WriteString(g.indent() + "method " + g.functionSignature(f.Name(), f.Arglist(), "") + " = " + "\n")
		fServiceI.WriteString(g.indent() + "method " + decapitalize(f.Name()) + " : " +
			g.functionType(f.Arglist(), f.ReturnType(), true, false) + "\n")
		g.indentUp()
		fService.WriteString(g.indent() + "self#send_" + funname)

		for _, fld := range fields {
			fService.WriteString(" " + decapitalize(fld.Name()))
		}
		fService.WriteString(";" + "\n")

		if !f.IsOneway() {
			fService.WriteString(g.indent())
			fService.WriteString("self#recv_" + funname + "\n")
		}
		g.indentDown()

		fService.WriteString(g.indent() + "method private send_" + g.functionSignature(f.Name(), f.Arglist(), "") + " = " + "\n")
		g.indentUp()

		argsname := decapitalize(f.Name() + "_args")

		// Serialize the request header
		oneway := "Protocol.CALL"
		if f.IsOneway() {
			oneway = "Protocol.ONEWAY"
		}
		fService.WriteString(g.indent() + "oprot#writeMessageBegin (\"" + f.Name() + "\", " + oneway + ", seqid);" + "\n")

		fService.WriteString(g.indent() + "let args = new " + argsname + " in" + "\n")
		g.indentUp()

		for _, fld := range fields {
			fService.WriteString(g.indent() + "args#set_" + fld.Name() + " " + fld.Name() + ";" + "\n")
		}

		// Write to the stream
		fService.WriteString(g.indent() + "args#write oprot;" + "\n" + g.indent() + "oprot#writeMessageEnd;" +
			"\n" + g.indent() + "oprot#getTransport#flush" + "\n")

		g.indentDown()
		g.indentDown()

		if !f.IsOneway() {
			resultname := decapitalize(f.Name() + "_result")
			noargs := sema.NewStruct(g.program)

			// Open function
			fService.WriteString(g.indent() + "method private " + g.functionSignature("recv_"+f.Name(), noargs, "") + " =" + "\n")
			g.indentUp()

			// TODO(mcslee): Validate message reply here, seq ids etc.

			fService.WriteString(g.indent() + "let (fname, mtype, rseqid) = iprot#readMessageBegin in" + "\n")
			g.indentUp()
			fService.WriteString(g.indent() + "(if mtype = Protocol.EXCEPTION then" + "\n" + g.indent() +
				"  let x = Application_Exn.read iprot in" + "\n")
			g.indentUp()
			fService.WriteString(g.indent() + "  (iprot#readMessageEnd;" + g.indent() +
				"   raise (Application_Exn.E x))" + "\n")
			g.indentDown()
			fService.WriteString(g.indent() + "else ());" + "\n")
			res := "_"

			xceptions := f.Xceptions().Members()

			if !f.ReturnType().IsVoid() || len(xceptions) > 0 {
				res = "result"
			}
			fService.WriteString(g.indent() + "let " + res + " = read_" + resultname + " iprot in" + "\n")
			g.indentUp()
			fService.WriteString(g.indent() + "iprot#readMessageEnd;" + "\n")

			// Careful, only return _result if not a void function
			if !f.ReturnType().IsVoid() {
				fService.WriteString(g.indent() + "match result#get_success with Some v -> v | None -> (" + "\n")
				g.indentUp()
			}

			for _, x := range xceptions {
				fService.WriteString(g.indent() + "(match result#get_" + x.Name() +
					" with None -> () | Some _v ->" + "\n")
				fService.WriteString(g.indent() + "  raise (" + capitalize(g.exceptionCtor(x.Type())) +
					" _v));" + "\n")
			}

			// Careful, only return _result if not a void function
			if f.ReturnType().IsVoid() {
				fService.WriteString(g.indent() + "()" + "\n")
			} else {
				fService.WriteString(g.indent() +
					"raise (Application_Exn.E (Application_Exn.create Application_Exn.MISSING_RESULT \"" +
					f.Name() + " failed: unknown result\")))" + "\n")
				g.indentDown()
			}

			// Close function
			g.indentDown()
			g.indentDown()
			g.indentDown()
		}
	}

	g.indentDown()
	fService.WriteString(g.indent() + "end" + "\n" + "\n")
	fServiceI.WriteString(g.indent() + "end" + "\n" + "\n")
}

// generateServiceServer is t_ocaml_generator::generate_service_server.
func (g *Generator) generateServiceServer(fService, fServiceI *strings.Builder, s *sema.Service) {
	// Generate the dispatch methods
	functions := s.Functions()

	// Generate the header portion
	fService.WriteString(g.indent() + "class processor (handler : iface) =" + "\n" + g.indent() + "object (self)" + "\n")
	fServiceI.WriteString(g.indent() + "class processor : iface ->" + "\n" + g.indent() + "object" + "\n")
	g.indentUp()

	fService.WriteString(g.indent() + "inherit Processor.t" + "\n" + "\n")
	fServiceI.WriteString(g.indent() + "inherit Processor.t" + "\n" + "\n")
	extends := ""

	if s.Extends() != nil {
		extends = g.typeName(s.Extends())
		fService.WriteString(g.indent() + "inherit " + extends + ".processor (handler :> " + extends + ".iface)" + "\n")
		fServiceI.WriteString(g.indent() + "inherit " + extends + ".processor" + "\n")
	}

	if extends == "" {
		fService.WriteString(g.indent() + "val processMap = Hashtbl.create " + strconv.Itoa(len(functions)) + "\n")
	}
	fServiceI.WriteString(g.indent() + "val processMap : (string, int * Protocol.t * Protocol.t -> unit) Hashtbl.t" + "\n")

	// Generate the server implementation
	fService.WriteString(g.indent() + "method process iprot oprot =" + "\n")
	fServiceI.WriteString(g.indent() + "method process : Protocol.t -> Protocol.t -> bool" + "\n")
	g.indentUp()

	fService.WriteString(g.indent() + "let (name, typ, seqid)  = iprot#readMessageBegin in" + "\n")
	g.indentUp()
	// TODO(mcslee): validate message

	// HOT: dictionary function lookup
	fService.WriteString(g.indent() + "if Hashtbl.mem processMap name then" + "\n" + g.indent() +
		"  (Hashtbl.find processMap name) (seqid, iprot, oprot)" + "\n" + g.indent() +
		"else (" + "\n" + g.indent() + "  iprot#skip(Protocol.T_STRUCT);" + "\n" +
		g.indent() + "  iprot#readMessageEnd;" + "\n" + g.indent() +
		"  let x = Application_Exn.create Application_Exn.UNKNOWN_METHOD (\"Unknown function \"^name) in" + "\n" +
		g.indent() + "    oprot#writeMessageBegin(name, Protocol.EXCEPTION, seqid);" + "\n" +
		g.indent() + "    x#write oprot;" + "\n" + g.indent() + "    oprot#writeMessageEnd;" + "\n" +
		g.indent() + "    oprot#getTransport#flush" + "\n" + g.indent() + ");" + "\n")

	// Read end of args field, the T_STOP, and the struct close
	fService.WriteString(g.indent() + "true" + "\n")
	g.indentDown()
	g.indentDown()
	// Generate the process subfunctions
	for _, f := range functions {
		g.generateProcessFunction(fService, s, f)
	}

	fService.WriteString(g.indent() + "initializer" + "\n")
	g.indentUp()
	for _, f := range functions {
		fService.WriteString(g.indent() + "Hashtbl.add processMap \"" + f.Name() + "\" self#process_" + f.Name() + ";" + "\n")
	}
	g.indentDown()

	g.indentDown()
	fService.WriteString(g.indent() + "end" + "\n" + "\n")
	fServiceI.WriteString(g.indent() + "end" + "\n" + "\n")
}

// generateProcessFunction is t_ocaml_generator::generate_process_function:
// generates a process function definition.
func (g *Generator) generateProcessFunction(fService *strings.Builder, s *sema.Service, f *sema.Function) {
	_ = s
	// Open function
	fService.WriteString(g.indent() + "method private process_" + f.Name() + " (seqid, iprot, oprot) =" + "\n")
	g.indentUp()

	argsname := decapitalize(f.Name()) + "_args"
	resultname := decapitalize(f.Name()) + "_result"

	// Generate the function call
	argStruct := f.Arglist()
	fields := argStruct.Members()

	args := "args"
	if len(fields) == 0 {
		args = "_"
	}

	fService.WriteString(g.indent() + "let " + args + " = read_" + argsname + " iprot in" + "\n")
	g.indentUp()
	fService.WriteString(g.indent() + "iprot#readMessageEnd;" + "\n")

	xceptions := f.Xceptions().Members()

	// Declare result for non oneway function
	if !f.IsOneway() {
		fService.WriteString(g.indent() + "let result = new " + resultname + " in" + "\n")
		g.indentUp()
	}

	// Try block for a function with exceptions
	if len(xceptions) > 0 {
		fService.WriteString(g.indent() + "(try" + "\n")
		g.indentUp()
	}

	fService.WriteString(g.indent())
	if !f.IsOneway() && !f.ReturnType().IsVoid() {
		fService.WriteString("result#set_success ")
	}
	fService.WriteString("(handler#" + f.Name())
	for _, fld := range fields {
		fService.WriteString(" args#get_" + fld.Name())
	}
	fService.WriteString(");" + "\n")

	if len(xceptions) > 0 {
		g.indentDown()
		fService.WriteString(g.indent() + "with" + "\n")
		g.indentUp()
		for _, x := range xceptions {
			fService.WriteString(g.indent() + "| " + capitalize(g.exceptionCtor(x.Type())) + " " + x.Name() + " -> " + "\n")
			g.indentUp()
			g.indentUp()
			if !f.IsOneway() {
				fService.WriteString(g.indent() + "result#set_" + x.Name() + " " + x.Name() + "\n")
			} else {
				fService.WriteString(g.indent() + "()")
			}
			g.indentDown()
			g.indentDown()
		}
		g.indentDown()
		fService.WriteString(g.indent() + ");" + "\n")
	}

	// Shortcut out here for oneway functions
	if f.IsOneway() {
		fService.WriteString(g.indent() + "()" + "\n")
		g.indentDown()
		g.indentDown()
		return
	}

	fService.WriteString(g.indent() + "oprot#writeMessageBegin (\"" + f.Name() +
		"\", Protocol.REPLY, seqid);" + "\n" + g.indent() + "result#write oprot;" + "\n" +
		g.indent() + "oprot#writeMessageEnd;" + "\n" + g.indent() +
		"oprot#getTransport#flush" + "\n")

	// Close function
	g.indentDown()
	g.indentDown()
	g.indentDown()
}
