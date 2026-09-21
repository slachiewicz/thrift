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

package js

import (
	"strconv"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// itoa32 formats a 32-bit field key or enum value, as an int64_t would be
// inserted into an ostream.
func itoa32(i int32) string { return strconv.FormatInt(int64(i), 10) }

// generateService is generate_service: it generates a thrift service.
func (g *Generator) generateService(s *sema.Service) {
	g.fService.Reset()
	g.fServiceTS.Reset()

	ext := ".js"
	if g.opts.ESM {
		ext = ".mjs"
	}
	fServiceName := g.outDir() + g.serviceName + ext

	if g.opts.GenEpisodeFile {
		g.fEpisode.WriteString(g.serviceName + ":" + g.opts.ThriftPackageOutputDirectory + "/" + g.serviceName + "\n")
	}

	g.fService.WriteString(g.autogenComment())

	if (g.opts.Node || g.opts.ES6) && g.noNS {
		g.fService.WriteString("\"use strict\";\n\n")
	}

	g.fService.WriteString(g.jsIncludes() + "\n" + g.renderIncludes() + "\n")

	if g.opts.TS {
		if extends := s.Extends(); extends != nil {
			if _, ok := g.moduleName2ImportPath[extends.Name()]; !ok {
				g.fServiceTS.WriteString("/// <reference path=\"" + extends.Name() + ".d.ts\" />" + "\n")
			}
		}
		g.fServiceTS.WriteString(g.autogenComment() + "\n" + g.tsIncludes() + "\n" + g.renderTsIncludes() + "\n")
		if g.opts.Node {
			g.fServiceTS.WriteString("import ttypes = require('./" + g.program.Name() + "_types');" + "\n")
			for _, e := range g.program.Enums() {
				g.fServiceTS.WriteString("import " + e.Name() + " = ttypes." + g.jsNamespace(g.program) + e.Name() + "\n")
			}
			for _, c := range g.program.Consts() {
				g.fServiceTS.WriteString("import " + c.Name() + " = ttypes." + g.jsNamespace(g.program) + c.Name() + "\n")
			}
			for _, x := range g.program.Xceptions() {
				g.fServiceTS.WriteString("import " + x.Name() + " = ttypes." + g.jsNamespace(g.program) + x.Name() + "\n")
			}
			for _, st := range g.program.Structs() {
				g.fServiceTS.WriteString("import " + st.Name() + " = ttypes." + g.jsNamespace(g.program) + st.Name() + "\n")
			}
		} else {
			g.fServiceTS.WriteString("import { " + g.program.Name() + " } from \"./" + g.program.Name() + "_types\";" + "\n" + "\n")
		}
		if g.tsModule != "" {
			if g.opts.Node {
				g.fServiceTS.WriteString("declare module " + g.tsModule + " {")
			} else {
				g.fServiceTS.WriteString("declare module \"./" + g.program.Name() + "_types\" {" + "\n")
				g.indentUp()
				g.fServiceTS.WriteString(g.tsIndent() + "module " + g.program.Name() + " {" + "\n")
				g.indentUp()
			}
		}
	}

	if g.opts.Node {
		if extends := s.Extends(); extends != nil {
			g.fService.WriteString(g.jsConstType + extends.Name() + " = require('" + g.getImportPathService(extends) + "');" + "\n" +
				g.jsConstType + extends.Name() + "Client = " + extends.Name() + ".Client;" + "\n" +
				g.jsConstType + extends.Name() + "Processor = " + extends.Name() + ".Processor;" + "\n")
			if g.opts.TS {
				g.fServiceTS.WriteString("import " + extends.Name() + " = require('" + g.getImportPathService(extends) + "');" + "\n")
			}
		}
		if g.opts.ESM {
			g.fService.WriteString("import * as ttypes from './" + g.program.Name() + "_types.mjs';" + "\n")
		} else {
			g.fService.WriteString(g.jsConstType + "ttypes = require('./" + g.program.Name() + "_types');" + "\n")
		}
	}

	g.generateServiceHelpers(s)
	g.generateServiceInterface(s)
	g.generateServiceClient(s)

	if g.opts.Node {
		g.generateServiceProcessor(s)
	}

	emit.WriteFile(fServiceName, g.fService.String())
	if g.opts.TS {
		if g.tsModule != "" {
			if g.opts.Node {
				g.fServiceTS.WriteString("}" + "\n")
			} else {
				g.indentDown()
				g.fServiceTS.WriteString(g.tsIndent() + "}" + "\n")
				g.fServiceTS.WriteString("}" + "\n")
			}
		}
		emit.WriteFile(g.outDir()+g.serviceName+".d.ts", g.fServiceTS.String())
	}
}

// generateServiceProcessor is generate_service_processor: a service server
// definition.
func (g *Generator) generateServiceProcessor(s *sema.Service) {
	functions := s.Functions()

	var serviceVar string
	if !g.opts.Node || g.hasJsNamespace(s.Program()) {
		serviceVar = g.jsNamespace(s.Program()) + g.serviceName + "Processor"
		g.fService.WriteString(serviceVar)
	} else {
		serviceVar = g.serviceName + "Processor"
		g.fService.WriteString(g.jsConstType + serviceVar)
	}
	if g.opts.Node && g.opts.TS {
		g.fServiceTS.WriteString("\n" + "declare class Processor ")
		if extends := s.Extends(); extends != nil {
			g.fServiceTS.WriteString("extends " + extends.Name() + ".Processor ")
		}
		g.fServiceTS.WriteString("{" + "\n")
		g.indentUp()

		if s.Extends() == nil {
			g.fServiceTS.WriteString(g.tsIndent() + "private _handler: object;" + "\n" + "\n")
		}
		g.fServiceTS.WriteString(g.tsIndent() + "constructor(handler: object);" + "\n")
		g.fServiceTS.WriteString(g.tsIndent() + "process(input: thrift.TProtocol, output: thrift.TProtocol): void;" + "\n")
		g.indentDown()
	}

	isSubclassService := s.Extends() != nil

	// ES6 Constructor
	if g.opts.ES6 {
		if isSubclassService {
			g.fService.WriteString(" = class " + g.serviceName + "Processor extends " + s.Extends().Name() + "Processor {" + "\n")
		} else {
			g.fService.WriteString(" = class " + g.serviceName + "Processor {" + "\n")
		}
		g.indentUp()
		g.fService.WriteString(g.indent() + "constructor(handler) {" + "\n")
	} else {
		g.fService.WriteString(" = function(handler) {" + "\n")
	}

	g.indentUp()
	if g.opts.ES6 && isSubclassService {
		g.fService.WriteString(g.indent() + "super(handler);" + "\n")
	}
	g.fService.WriteString(g.indent() + "this._handler = handler;" + "\n")
	g.indentDown()

	// Done with constructor
	if g.opts.ES6 {
		g.fService.WriteString(g.indent() + "}" + "\n")
	} else {
		g.fService.WriteString(g.indent() + "};" + "\n")
	}

	// ES5 service inheritance
	if !g.opts.ES6 && isSubclassService {
		g.fService.WriteString(g.indent() + "Thrift.inherits(" + g.jsNamespace(s.Program()) + g.serviceName + "Processor, " + s.Extends().Name() + "Processor);" + "\n")
	}

	// Generate the server implementation
	if g.opts.ES6 {
		g.fService.WriteString(g.indent() + "process (input, output) {" + "\n")
	} else {
		g.fService.WriteString(g.indent() + g.jsNamespace(s.Program()) + g.serviceName + "Processor.prototype.process = function(input, output) {" + "\n")
	}

	g.indentUp()

	g.fService.WriteString(g.indent() + g.jsConstType + "r = input.readMessageBegin();" + "\n" +
		g.indent() + "if (this['process_' + r.fname]) {" + "\n" +
		g.indent() + "  return this['process_' + r.fname].call(this, r.rseqid, input, output);" + "\n" +
		g.indent() + "} else {" + "\n" +
		g.indent() + "  input.skip(Thrift.Type.STRUCT);" + "\n" +
		g.indent() + "  input.readMessageEnd();" + "\n" +
		g.indent() + "  " + g.jsConstType + "x = new Thrift.TApplicationException(Thrift.TApplicationExceptionType.UNKNOWN_METHOD, 'Unknown function ' + r.fname);" + "\n" +
		g.indent() + "  output.writeMessageBegin(r.fname, Thrift.MessageType.EXCEPTION, r.rseqid);" + "\n" +
		g.indent() + "  x[Symbol.for(\"write\")](output);" + "\n" +
		g.indent() + "  output.writeMessageEnd();" + "\n" +
		g.indent() + "  output.flush();" + "\n" +
		g.indent() + "}" + "\n")

	g.indentDown()
	if g.opts.ES6 {
		g.fService.WriteString(g.indent() + "}" + "\n")
	} else {
		g.fService.WriteString(g.indent() + "};" + "\n")
	}

	// Generate the process subfunctions
	for _, f := range functions {
		g.generateProcessFunction(s, f)
	}

	// Close off the processor class definition
	if g.opts.ES6 {
		g.indentDown()
		g.fService.WriteString(g.indent() + "};" + "\n")
	}
	if g.opts.Node && g.opts.TS {
		g.fServiceTS.WriteString("}" + "\n")
	}

	if g.opts.ESM {
		g.fService.WriteString("export { " + serviceVar + " as Processor };" + "\n")
	} else {
		g.fService.WriteString("exports.Processor = " + serviceVar + ";" + "\n")
	}
}

// generateProcessFunction is generate_process_function: a process function
// dispatcher.
func (g *Generator) generateProcessFunction(s *sema.Service, f *sema.Function) {
	if g.opts.ES6 {
		g.fService.WriteString(g.indent() + "process_" + f.Name() + " (seqid, input, output) {" + "\n")
	} else {
		g.fService.WriteString(g.indent() + g.jsNamespace(s.Program()) + g.serviceName + "Processor.prototype.process_" + f.Name() + " = function(seqid, input, output) {" + "\n")
	}
	if g.opts.TS {
		g.indentUp()
		g.fServiceTS.WriteString(g.tsIndent() + "process_" + f.Name() + "(seqid: number, input: thrift.TProtocol, output: thrift.TProtocol): void;" + "\n")
		g.indentDown()
	}

	g.indentUp()

	argsname := g.jsNamespace(g.program) + g.serviceName + "_" + f.Name() + "_args"
	resultname := g.jsNamespace(g.program) + g.serviceName + "_" + f.Name() + "_result"

	g.fService.WriteString(g.indent() + g.jsConstType + "args = new " + argsname + "();" + "\n" +
		g.indent() + "args[Symbol.for(\"read\")](input);" + "\n" +
		g.indent() + "input.readMessageEnd();" + "\n")

	fields := f.Arglist().Members()

	// Shortcut out here for oneway functions
	if f.IsOneway() {
		g.fService.WriteString(g.indent() + "this._handler." + f.Name() + "(")
		for i, fld := range fields {
			if i > 0 {
				g.fService.WriteString(", ")
			}
			g.fService.WriteString("args." + fld.Name())
		}
		g.fService.WriteString(");" + "\n")
		g.indentDown()

		if g.opts.ES6 {
			g.fService.WriteString(g.indent() + "}" + "\n")
		} else {
			g.fService.WriteString(g.indent() + "};" + "\n")
		}
		return
	}

	// Promise style invocation
	g.fService.WriteString(g.indent() + "if (this._handler." + f.Name() + ".length === " + strconv.Itoa(len(fields)) + ") {" + "\n")
	g.indentUp()

	switch {
	case g.opts.ES6:
		g.fService.WriteString(g.indent() + "new Promise((resolve) => resolve(this._handler." + f.Name() + ".bind(this._handler)(" + "\n")
	case g.opts.NativePromise:
		// Non-ES6 native Promise: use function expression with explicit
		// `this` binding so we don't rely on arrow-function lexical
		// `this`.
		g.fService.WriteString(g.indent() + "new Promise(function(resolve) { resolve(this._handler." + f.Name() + ".bind(this._handler)(" + "\n")
	default:
		maybeComma := ""
		if len(fields) > 0 {
			maybeComma = ","
		}
		g.fService.WriteString(g.indent() + "Q.fcall(this._handler." + f.Name() + ".bind(this._handler)" + maybeComma + "\n")
	}

	g.indentUp()
	for i, fld := range fields {
		maybeComma := ","
		if i == len(fields)-1 {
			maybeComma = ""
		}
		g.fService.WriteString(g.indent() + "args." + fld.Name() + maybeComma + "\n")
	}
	g.indentDown()

	switch {
	case g.opts.ES6:
		g.fService.WriteString(g.indent() + "))).then(result => {" + "\n")
	case g.opts.NativePromise:
		g.fService.WriteString(g.indent() + ")); }.bind(this)).then(function(result) {" + "\n")
	default:
		g.fService.WriteString(g.indent() + ").then(function(result) {" + "\n")
	}

	g.indentUp()
	g.fService.WriteString(g.indent() + g.jsConstType + "result_obj = new " + resultname + "({success: result});" + "\n" +
		g.indent() + "output.writeMessageBegin(\"" + f.Name() + "\", Thrift.MessageType.REPLY, seqid);" + "\n" +
		g.indent() + "result_obj[Symbol.for(\"write\")](output);" + "\n" +
		g.indent() + "output.writeMessageEnd();" + "\n" +
		g.indent() + "output.flush();" + "\n")
	g.indentDown()

	if g.opts.ES6 {
		g.fService.WriteString(g.indent() + "}).catch(err => {" + "\n")
	} else {
		g.fService.WriteString(g.indent() + "}).catch(function (err) {" + "\n")
	}
	g.indentUp()
	g.fService.WriteString(g.indent() + g.jsLetType + "result;" + "\n")

	hasException := false
	exceptions := f.Xceptions()
	for _, m := range exceptions.Members() {
		t := sema.TrueType(m.Type())
		if t.IsXception() {
			if !hasException {
				hasException = true
				g.fService.WriteString(g.indent() + "if (err instanceof " + g.jsTypeNamespace(t.Program()) + t.Name())
			} else {
				g.fService.WriteString(" || err instanceof " + g.jsTypeNamespace(t.Program()) + t.Name())
			}
		}
	}

	if hasException {
		g.fService.WriteString(") {" + "\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "result = new " + resultname + "(err);" + "\n" +
			g.indent() + "output.writeMessageBegin(\"" + f.Name() + "\", Thrift.MessageType.REPLY, seqid);" + "\n")
		g.indentDown()
		g.fService.WriteString(g.indent() + "} else {" + "\n")
		g.indentUp()
	}

	g.fService.WriteString(g.indent() + "result = new Thrift.TApplicationException(Thrift.TApplicationExceptionType.UNKNOWN, err.message);" + "\n" +
		g.indent() + "output.writeMessageBegin(\"" + f.Name() + "\", Thrift.MessageType.EXCEPTION, seqid);" + "\n")

	if hasException {
		g.indentDown()
		g.fService.WriteString(g.indent() + "}" + "\n")
	}

	g.fService.WriteString(g.indent() + "result[Symbol.for(\"write\")](output);" + "\n" +
		g.indent() + "output.writeMessageEnd();" + "\n" +
		g.indent() + "output.flush();" + "\n")
	g.indentDown()
	g.fService.WriteString(g.indent() + "});" + "\n")
	g.indentDown()
	// End promise style invocation

	// Callback style invocation
	g.fService.WriteString(g.indent() + "} else {" + "\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "this._handler." + f.Name() + "(")

	for _, fld := range fields {
		g.fService.WriteString("args." + fld.Name() + ", ")
	}

	if g.opts.ES6 {
		g.fService.WriteString("(err, result) => {" + "\n")
	} else {
		g.fService.WriteString("function (err, result) {" + "\n")
	}
	g.indentUp()
	g.fService.WriteString(g.indent() + g.jsLetType + "result_obj;" + "\n")

	g.fService.WriteString(g.indent() + "if ((err === null || typeof err === 'undefined')")
	if hasException {
		for _, m := range exceptions.Members() {
			t := sema.TrueType(m.Type())
			if t.IsXception() {
				g.fService.WriteString(" || err instanceof " + g.jsTypeNamespace(t.Program()) + t.Name())
			}
		}
	}
	g.fService.WriteString(") {" + "\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "result_obj = new " + resultname + "((err !== null || typeof err === 'undefined') ? err : {success: result});" + "\n" +
		g.indent() + "output.writeMessageBegin(\"" + f.Name() + "\", Thrift.MessageType.REPLY, seqid);" + "\n")
	g.indentDown()
	g.fService.WriteString(g.indent() + "} else {" + "\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "result_obj = new Thrift.TApplicationException(Thrift.TApplicationExceptionType.UNKNOWN, err.message);" + "\n" +
		g.indent() + "output.writeMessageBegin(\"" + f.Name() + "\", Thrift.MessageType.EXCEPTION, seqid);" + "\n")
	g.indentDown()
	g.fService.WriteString(g.indent() + "}" + "\n" + g.indent() + "result_obj[Symbol.for(\"write\")](output);" + "\n" +
		g.indent() + "output.writeMessageEnd();" + "\n" +
		g.indent() + "output.flush();" + "\n")

	g.indentDown()
	g.fService.WriteString(g.indent() + "});" + "\n")
	g.indentDown()
	g.fService.WriteString(g.indent() + "}" + "\n")
	// End callback style invocation

	g.indentDown()

	if g.opts.ES6 {
		g.fService.WriteString(g.indent() + "}" + "\n")
	} else {
		g.fService.WriteString(g.indent() + "};" + "\n")
	}
}

// generateServiceHelpers is generate_service_helpers: helper functions for
// a service.
func (g *Generator) generateServiceHelpers(s *sema.Service) {
	// Do not generate TS definitions for helper functions
	genTSTmp := g.opts.TS
	g.opts.TS = false

	functions := s.Functions()

	g.fService.WriteString("//HELPER FUNCTIONS AND STRUCTURES" + "\n" + "\n")

	for _, f := range functions {
		ts := f.Arglist()
		name := ts.Name()
		ts.SetName(g.serviceName + "_" + name)
		g.generateJsStructDefinition(&g.fService, ts, false, false)
		g.generateJsFunctionHelpers(f)
		ts.SetName(name)
	}

	g.opts.TS = genTSTmp
}

// generateJsFunctionHelpers is generate_js_function_helpers: a struct and
// helpers for a function.
func (g *Generator) generateJsFunctionHelpers(f *sema.Function) {
	result := sema.NewStruct(g.program)
	result.SetName(g.serviceName + "_" + f.Name() + "_result")
	if !f.ReturnType().IsVoid() {
		result.Append(sema.NewField(f.ReturnType(), "success", 0))
	}

	for _, fld := range f.Xceptions().Members() {
		result.Append(fld)
	}

	g.generateJsStructDefinition(&g.fService, result, false, false)
}

// generateServiceInterface is generate_service_interface: unused (JS has
// no interface concept).
func (g *Generator) generateServiceInterface(_ *sema.Service) {}

// generateServiceRest is generate_service_rest: unused (no REST
// interface).
func (g *Generator) generateServiceRest(_ *sema.Service) {}

// generateServiceClient is generate_service_client: a service client
// definition.
func (g *Generator) generateServiceClient(s *sema.Service) {
	isSubclassService := s.Extends() != nil

	clientVar := g.jsNamespace(s.Program()) + g.serviceName + "Client"
	if g.opts.Node {
		prefix := ""
		if !g.hasJsNamespace(s.Program()) {
			prefix = g.jsConstType
		}
		g.fService.WriteString(prefix + clientVar)
		if g.opts.TS {
			g.fServiceTS.WriteString(g.tsPrintDoc(s) + g.tsIndent() + g.tsDeclare() + "class " + "Client ")
			if extends := s.Extends(); extends != nil {
				g.fServiceTS.WriteString("extends " + extends.Name() + ".Client ")
			}
			g.fServiceTS.WriteString("{" + "\n")
		}
	} else {
		g.fService.WriteString(clientVar)
		if g.opts.TS {
			g.fServiceTS.WriteString(g.tsPrintDoc(s) + g.tsIndent() + g.tsDeclare() + "class " + g.serviceName + "Client ")
			if isSubclassService {
				g.fServiceTS.WriteString("extends " + s.Extends().Name() + "Client ")
			}
			g.fServiceTS.WriteString("{" + "\n")
		}
	}

	// ES6 Constructor
	if g.opts.ES6 {
		if isSubclassService {
			g.fService.WriteString(" = class " + g.serviceName + "Client extends " + g.jsNamespace(s.Extends().Program()) + s.Extends().Name() + "Client {" + "\n")
		} else {
			g.fService.WriteString(" = class " + g.serviceName + "Client {" + "\n")
		}
		g.indentUp()
		if g.opts.Node {
			g.fService.WriteString(g.indent() + "constructor(output, pClass) {" + "\n")
		} else {
			g.fService.WriteString(g.indent() + "constructor(input, output) {" + "\n")
		}
	} else {
		if g.opts.Node {
			g.fService.WriteString(" = function(output, pClass) {" + "\n")
		} else {
			g.fService.WriteString(" = function(input, output) {" + "\n")
		}
	}

	g.indentUp()

	if g.opts.Node {
		if g.opts.ES6 && isSubclassService {
			g.fService.WriteString(g.indent() + "super(output, pClass);" + "\n")
		}
		g.fService.WriteString(g.indent() + "this.output = output;" + "\n")
		g.fService.WriteString(g.indent() + "this.pClass = pClass;" + "\n")
		g.fService.WriteString(g.indent() + "this._seqid = 0;" + "\n")
		g.fService.WriteString(g.indent() + "this._reqs = {};" + "\n")
		if g.opts.TS {
			if !isSubclassService {
				g.fServiceTS.WriteString(g.tsIndent() + "private output: thrift.TTransport;" + "\n" +
					g.tsIndent() + "private pClass: thrift.TProtocol;" + "\n" +
					g.tsIndent() + "private _seqid: number;" + "\n" +
					"\n")
			}
			g.fServiceTS.WriteString(g.tsIndent() + "constructor(output: thrift.TTransport, pClass: { new(trans: thrift.TTransport): thrift.TProtocol });" + "\n")
		}
	} else {
		g.fService.WriteString(g.indent() + "this.input = input;" + "\n")
		g.fService.WriteString(g.indent() + "this.output = (!output) ? input : output;" + "\n")
		g.fService.WriteString(g.indent() + "this.seqid = 0;" + "\n")
		if g.opts.TS {
			g.fServiceTS.WriteString(g.tsIndent() + "input: Thrift.TJSONProtocol;" + "\n" +
				g.tsIndent() + "output: Thrift.TJSONProtocol;" + "\n" +
				g.tsIndent() + "seqid: number;" + "\n" + "\n" +
				g.tsIndent() + "constructor(input: Thrift.TJSONProtocol, output?: Thrift.TJSONProtocol);" + "\n")
		}
	}

	g.indentDown()

	if g.opts.ES6 {
		g.fService.WriteString(g.indent() + "}" + "\n")
	} else {
		g.fService.WriteString(g.indent() + "};" + "\n")
		if isSubclassService {
			g.fService.WriteString(g.indent() + "Thrift.inherits(" + g.jsNamespace(s.Program()) + g.serviceName + "Client, " +
				g.jsNamespace(s.Extends().Program()) + s.Extends().Name() + "Client);" + "\n")
		} else {
			// init prototype
			g.fService.WriteString(g.indent() + g.jsNamespace(s.Program()) + g.serviceName + "Client.prototype = {};" + "\n")
		}
	}

	// utils for multiplexed services
	if g.opts.Node {
		if g.opts.ES6 {
			g.fService.WriteString(g.indent() + "seqid () { return this._seqid; }" + "\n")
			g.fService.WriteString(g.indent() + "new_seqid () { return this._seqid += 1; }" + "\n")
		} else {
			g.fService.WriteString(g.indent() + g.jsNamespace(s.Program()) + g.serviceName + "Client.prototype.seqid = function() { return this._seqid; };" + "\n" +
				g.indent() + g.jsNamespace(s.Program()) + g.serviceName + "Client.prototype.new_seqid = function() { return this._seqid += 1; };" + "\n")
		}
	}

	// Generate client method implementations
	for _, f := range s.Functions() {
		g.generateClientFunction(s, f)
	}

	// Finish class definitions
	if g.opts.TS {
		g.fServiceTS.WriteString(g.tsIndent() + "}" + "\n")
	}
	if g.opts.ES6 {
		g.indentDown()
		g.fService.WriteString("};" + "\n")
	}

	if g.opts.ESM {
		g.fService.WriteString("export { " + clientVar + " as Client };" + "\n")
	} else if g.opts.Node {
		g.fService.WriteString("exports.Client = " + clientVar + ";" + "\n")
	}
}

// generateClientFunction is the body of the per-function loop inside
// generate_service_client.
func (g *Generator) generateClientFunction(s *sema.Service, f *sema.Function) {
	argStruct := f.Arglist()
	fields := argStruct.Members()
	funname := f.Name()
	arglist := g.argumentList(argStruct, false)

	// Open function
	g.fService.WriteString("\n")
	if g.opts.ES6 {
		g.fService.WriteString(g.indent() + funname + " (" + arglist + ") {" + "\n")
	} else {
		g.fService.WriteString(g.indent() + g.jsNamespace(s.Program()) + g.serviceName + "Client.prototype." + g.functionSignature(f, "", !g.opts.ES6) + " {" + "\n")
	}

	g.indentUp()

	if g.opts.TS {
		// function definition without callback
		g.fServiceTS.WriteString(g.tsPrintDoc(f) + g.tsIndent() + g.tsFunctionSignature(f, false) + "\n")
		// overload with callback
		g.fServiceTS.WriteString(g.tsPrintDoc(f) + g.tsIndent() + g.tsFunctionSignature(f, true) + "\n")
	}

	switch {
	case g.opts.ES6 && g.opts.Node:
		g.fService.WriteString(g.indent() + "this._seqid = this.new_seqid();" + "\n")
		g.fService.WriteString(g.indent() + g.jsConstType + "self = this;" + "\n" +
			g.indent() + "return new Promise((resolve, reject) => {" + "\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "self._reqs[self.seqid()] = (error, result) => {" + "\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "return error ? reject(error) : resolve(result);" + "\n")
		g.indentDown()
		g.fService.WriteString(g.indent() + "};" + "\n")
		g.fService.WriteString(g.indent() + "self.send_" + funname + "(" + arglist + ");" + "\n")
		g.indentDown()
		g.fService.WriteString(g.indent() + "});" + "\n")
	case g.opts.Node: // Node.js output      ./gen-nodejs
		g.fService.WriteString(g.indent() + "this._seqid = this.new_seqid();" + "\n" +
			g.indent() + "if (callback === undefined) {" + "\n")
		g.indentUp()
		if g.opts.NativePromise {
			g.fService.WriteString(g.indent() + g.jsConstType + "self = this;" + "\n" +
				g.indent() + "return new Promise(function(resolve, reject) {" + "\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "self._reqs[self.seqid()] = function(error, result) {" + "\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "if (error) {" + "\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "reject(error);" + "\n")
			g.indentDown()
			g.fService.WriteString(g.indent() + "} else {" + "\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "resolve(result);" + "\n")
			g.indentDown()
			g.fService.WriteString(g.indent() + "}" + "\n")
			g.indentDown()
			g.fService.WriteString(g.indent() + "};" + "\n")
			g.fService.WriteString(g.indent() + "self.send_" + funname + "(" + arglist + ");" + "\n")
			g.indentDown()
			g.fService.WriteString(g.indent() + "});" + "\n")
		} else {
			g.fService.WriteString(g.indent() + g.jsConstType + "_defer = Q.defer();" + "\n" +
				g.indent() + "this._reqs[this.seqid()] = function(error, result) {" + "\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "if (error) {" + "\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "_defer.reject(error);" + "\n")
			g.indentDown()
			g.fService.WriteString(g.indent() + "} else {" + "\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "_defer.resolve(result);" + "\n")
			g.indentDown()
			g.fService.WriteString(g.indent() + "}" + "\n")
			g.indentDown()
			g.fService.WriteString(g.indent() + "};" + "\n")
			g.fService.WriteString(g.indent() + "this.send_" + funname + "(" + arglist + ");" + "\n" +
				g.indent() + "return _defer.promise;" + "\n")
		}
		g.indentDown()
		g.fService.WriteString(g.indent() + "} else {" + "\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "this._reqs[this.seqid()] = callback;" + "\n" +
			g.indent() + "this.send_" + funname + "(" + arglist + ");" + "\n")
		g.indentDown()
		g.fService.WriteString(g.indent() + "}" + "\n")
	case g.opts.ES6:
		g.fService.WriteString(g.indent() + g.jsConstType + "self = this;" + "\n" +
			g.indent() + "return new Promise((resolve, reject) => {" + "\n")
		g.indentUp()
		comma := ""
		if arglist != "" {
			comma = ", "
		}
		g.fService.WriteString(g.indent() + "self.send_" + funname + "(" + arglist + comma + "(error, result) => {" + "\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "return error ? reject(error) : resolve(result);" + "\n")
		g.indentDown()
		g.fService.WriteString(g.indent() + "});" + "\n")
		g.indentDown()
		g.fService.WriteString(g.indent() + "});" + "\n")
	case g.opts.Jquery: // jQuery output       ./gen-js
		g.fService.WriteString(g.indent() + "if (callback === undefined) {" + "\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "this.send_" + funname + "(" + arglist + ");" + "\n")
		if !f.IsOneway() {
			g.fService.WriteString(g.indent())
			if !f.ReturnType().IsVoid() {
				g.fService.WriteString("return ")
			}
			g.fService.WriteString("this.recv_" + funname + "();" + "\n")
		}
		g.indentDown()
		g.fService.WriteString(g.indent() + "} else {" + "\n")
		g.indentUp()
		comma := ""
		if arglist != "" {
			comma = ", "
		}
		g.fService.WriteString(g.indent() + g.jsConstType + "postData = this.send_" + funname + "(" + arglist + comma + "true);" + "\n")
		g.fService.WriteString(g.indent() + "return this.output.getTransport()" + "\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + ".jqRequest(this, postData, arguments, this.recv_" + funname + ");" + "\n")
		g.indentDown()
		g.indentDown()
		g.fService.WriteString(g.indent() + "}" + "\n")
	default: // Standard JavaScript ./gen-js
		comma := ""
		if arglist != "" {
			comma = ", "
		}
		g.fService.WriteString(g.indent() + "this.send_" + funname + "(" + arglist + comma + "callback); " + "\n")
		if !f.IsOneway() {
			g.fService.WriteString(g.indent() + "if (!callback) {" + "\n")
			g.fService.WriteString(g.indent())
			if !f.ReturnType().IsVoid() {
				g.fService.WriteString("  return ")
			}
			g.fService.WriteString("this.recv_" + funname + "();" + "\n")
			g.fService.WriteString(g.indent() + "}" + "\n")
		}
	}

	g.indentDown()

	if g.opts.ES6 {
		g.fService.WriteString(g.indent() + "}" + "\n" + "\n")
	} else {
		g.fService.WriteString(g.indent() + "};" + "\n" + "\n")
	}

	g.generateSendFunction(s, f, argStruct, fields, funname)
	g.generateRecvFunction(s, f, funname)
}

// generateSendFunction is the "send_" half of the per-function loop
// inside generate_service_client.
func (g *Generator) generateSendFunction(s *sema.Service, f *sema.Function, argStruct *sema.Struct, fields []*sema.Field, funname string) {
	if g.opts.ES6 {
		if g.opts.Node {
			g.fService.WriteString(g.indent() + "send_" + funname + " (" + g.argumentList(argStruct, false) + ") {" + "\n")
		} else {
			// ES6 js still uses callbacks here. Should refactor this to
			// promise style later..
			g.fService.WriteString(g.indent() + "send_" + funname + " (" + g.argumentList(argStruct, true) + ") {" + "\n")
		}
	} else {
		g.fService.WriteString(g.indent() + g.jsNamespace(s.Program()) + g.serviceName + "Client.prototype.send_" + g.functionSignature(f, "", !g.opts.Node) + " {" + "\n")
	}

	g.indentUp()

	outputVar := "this.output"
	if g.opts.Node {
		g.fService.WriteString(g.indent() + g.jsConstType + "output = new this.pClass(this.output);" + "\n")
		outputVar = "output"
	}

	argsname := g.jsNamespace(g.program) + g.serviceName + "_" + f.Name() + "_args"

	messageType := "Thrift.MessageType.CALL"
	if f.IsOneway() {
		messageType = "Thrift.MessageType.ONEWAY"
	}

	// Build args
	if len(fields) > 0 {
		// It is possible that a method argument is named "params", we
		// need to ensure the locally generated identifier "params" is
		// uniquely named
		paramsIdentifier := g.nextIdentifierName(fields, "params")
		g.fService.WriteString(g.indent() + g.jsConstType + paramsIdentifier + " = {" + "\n")
		g.indentUp()
		for i, fld := range fields {
			g.fService.WriteString(g.indent() + fld.Name() + ": " + fld.Name())
			if i != len(fields)-1 {
				g.fService.WriteString("," + "\n")
			} else {
				g.fService.WriteString("\n")
			}
		}
		g.indentDown()
		g.fService.WriteString(g.indent() + "};" + "\n")

		// NOTE: "args" is a reserved keyword, so no need to generate a
		// unique identifier
		g.fService.WriteString(g.indent() + g.jsConstType + "args = new " + argsname + "(" + paramsIdentifier + ");" + "\n")
	} else {
		g.fService.WriteString(g.indent() + g.jsConstType + "args = new " + argsname + "();" + "\n")
	}

	// Serialize the request header within try/catch
	g.fService.WriteString(g.indent() + "try {" + "\n")
	g.indentUp()

	if g.opts.Node {
		g.fService.WriteString(g.indent() + outputVar + ".writeMessageBegin('" + f.Name() + "', " + messageType + ", this.seqid());" + "\n")
	} else {
		g.fService.WriteString(g.indent() + outputVar + ".writeMessageBegin('" + f.Name() + "', " + messageType + ", this.seqid);" + "\n")
	}

	// Write to the stream
	g.fService.WriteString(g.indent() + "args[Symbol.for(\"write\")](" + outputVar + ");" + "\n" + g.indent() + outputVar + ".writeMessageEnd();" + "\n")

	if g.opts.Node {
		if f.IsOneway() {
			g.fService.WriteString(g.indent() + "this.output.flush();" + "\n")
			g.fService.WriteString(g.indent() + g.jsConstType + "callback = this._reqs[this.seqid()] || function() {};" + "\n")
			g.fService.WriteString(g.indent() + "delete this._reqs[this.seqid()];" + "\n")
			g.fService.WriteString(g.indent() + "callback(null);" + "\n")
		} else {
			g.fService.WriteString(g.indent() + "return this.output.flush();" + "\n")
		}
	} else {
		switch {
		case g.opts.Jquery:
			g.fService.WriteString(g.indent() + "return this.output.getTransport().flush(callback);" + "\n")
		case g.opts.ES6:
			g.fService.WriteString(g.indent() + g.jsConstType + "self = this;" + "\n")
			if f.IsOneway() {
				g.fService.WriteString(g.indent() + "this.output.getTransport().flush(true, null);" + "\n")
				g.fService.WriteString(g.indent() + "callback();" + "\n")
			} else {
				g.fService.WriteString(g.indent() + "this.output.getTransport().flush(true, () => {" + "\n")
				g.indentUp()
				g.fService.WriteString(g.indent() + g.jsLetType + "error = null, result = null;" + "\n")
				g.fService.WriteString(g.indent() + "try {" + "\n")
				g.fService.WriteString(g.indent() + "  result = self.recv_" + funname + "();" + "\n")
				g.fService.WriteString(g.indent() + "} catch (e) {" + "\n")
				g.fService.WriteString(g.indent() + "  error = e;" + "\n")
				g.fService.WriteString(g.indent() + "}" + "\n")
				g.fService.WriteString(g.indent() + "callback(error, result);" + "\n")
				g.indentDown()
				g.fService.WriteString(g.indent() + "});" + "\n")
			}
		default:
			g.fService.WriteString(g.indent() + "if (callback) {" + "\n")
			g.indentUp()
			if f.IsOneway() {
				g.fService.WriteString(g.indent() + "this.output.getTransport().flush(true, null);" + "\n")
				g.fService.WriteString(g.indent() + "callback();" + "\n")
			} else {
				g.fService.WriteString(g.indent() + g.jsConstType + "self = this;" + "\n")
				g.fService.WriteString(g.indent() + "this.output.getTransport().flush(true, function() {" + "\n")
				g.indentUp()
				g.fService.WriteString(g.indent() + g.jsLetType + "result = null;" + "\n")
				g.fService.WriteString(g.indent() + "try {" + "\n")
				g.fService.WriteString(g.indent() + "  result = self.recv_" + funname + "();" + "\n")
				g.fService.WriteString(g.indent() + "} catch (e) {" + "\n")
				g.fService.WriteString(g.indent() + "  result = e;" + "\n")
				g.fService.WriteString(g.indent() + "}" + "\n")
				g.fService.WriteString(g.indent() + "callback(result);" + "\n")
				g.indentDown()
				g.fService.WriteString(g.indent() + "});" + "\n")
			}
			g.indentDown()
			g.fService.WriteString(g.indent() + "} else {" + "\n")
			g.fService.WriteString(g.indent() + "  return this.output.getTransport().flush();" + "\n")
			g.fService.WriteString(g.indent() + "}" + "\n")
		}
	}

	g.indentDown()
	g.fService.WriteString(g.indent() + "}" + "\n")

	// Reset the transport and delete registered callback if there was a
	// serialization error
	g.fService.WriteString(g.indent() + "catch (e) {" + "\n")
	g.indentUp()
	if g.opts.Node {
		g.fService.WriteString(g.indent() + "delete this._reqs[this.seqid()];" + "\n")
		g.fService.WriteString(g.indent() + "if (typeof " + outputVar + ".reset === 'function') {" + "\n")
		g.fService.WriteString(g.indent() + "  " + outputVar + ".reset();" + "\n")
		g.fService.WriteString(g.indent() + "}" + "\n")
	} else {
		g.fService.WriteString(g.indent() + "if (typeof " + outputVar + ".getTransport().reset === 'function') {" + "\n")
		g.fService.WriteString(g.indent() + "  " + outputVar + ".getTransport().reset();" + "\n")
		g.fService.WriteString(g.indent() + "}" + "\n")
	}
	g.fService.WriteString(g.indent() + "throw e;" + "\n")
	g.indentDown()
	g.fService.WriteString(g.indent() + "}" + "\n")

	g.indentDown()

	// Close send function
	if g.opts.ES6 {
		g.fService.WriteString(g.indent() + "}" + "\n")
	} else {
		g.fService.WriteString(g.indent() + "};" + "\n")
	}
}

// generateRecvFunction is the "recv_" half of the per-function loop
// inside generate_service_client.
func (g *Generator) generateRecvFunction(s *sema.Service, f *sema.Function, funname string) {
	if f.IsOneway() {
		return
	}
	resultname := g.jsNamespace(s.Program()) + g.serviceName + "_" + f.Name() + "_result"

	g.fService.WriteString("\n")
	// Open receive function
	switch {
	case g.opts.Node && g.opts.ES6:
		g.fService.WriteString(g.indent() + "recv_" + f.Name() + " (input, mtype, rseqid) {" + "\n")
	case g.opts.Node:
		g.fService.WriteString(g.indent() + g.jsNamespace(s.Program()) + g.serviceName + "Client.prototype.recv_" + f.Name() + " = function(input,mtype,rseqid) {" + "\n")
	case g.opts.ES6:
		g.fService.WriteString(g.indent() + "recv_" + f.Name() + " () {" + "\n")
	default:
		recvSig := "recv_" + f.Name() + " = function()"
		g.fService.WriteString(g.indent() + g.jsNamespace(s.Program()) + g.serviceName + "Client.prototype." + recvSig + " {" + "\n")
	}

	g.indentUp()

	inputVar := "this.input"
	if g.opts.Node {
		inputVar = "input"
	}

	if g.opts.Node {
		g.fService.WriteString(g.indent() + g.jsConstType + "callback = this._reqs[rseqid] || function() {};" + "\n" +
			g.indent() + "delete this._reqs[rseqid];" + "\n")
	} else {
		g.fService.WriteString(g.indent() + g.jsConstType + "ret = this.input.readMessageBegin();" + "\n" +
			g.indent() + g.jsConstType + "mtype = ret.mtype;" + "\n")
	}

	g.fService.WriteString(g.indent() + "if (mtype == Thrift.MessageType.EXCEPTION) {" + "\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + g.jsConstType + "x = new Thrift.TApplicationException();" + "\n" +
		g.indent() + "x[Symbol.for(\"read\")](" + inputVar + ");" + "\n" +
		g.indent() + inputVar + ".readMessageEnd();" + "\n" +
		g.indent() + g.renderRecvThrow("x") + "\n")
	g.scopeDown(&g.fService)

	g.fService.WriteString(g.indent() + g.jsConstType + "result = new " + resultname + "();" + "\n" +
		g.indent() + "result[Symbol.for(\"read\")](" + inputVar + ");" + "\n")

	g.fService.WriteString(g.indent() + inputVar + ".readMessageEnd();" + "\n" + "\n")

	xceptions := f.Xceptions().Members()
	for _, x := range xceptions {
		g.fService.WriteString(g.indent() + "if (null !== result." + x.Name() + ") {" + "\n" +
			g.indent() + "  " + g.renderRecvThrow("result."+x.Name()) + "\n" +
			g.indent() + "}" + "\n")
	}

	// Careful, only return result if not a void function
	if !f.ReturnType().IsVoid() {
		g.fService.WriteString(g.indent() + "if (null !== result.success) {" + "\n" + g.indent() + "  " + g.renderRecvReturn("result.success") + "\n" + g.indent() + "}" + "\n")
		g.fService.WriteString(g.indent() + g.renderRecvThrow("'"+f.Name()+" failed: unknown result'") + "\n")
	} else {
		if g.opts.Node {
			g.fService.WriteString(g.indent() + "callback(null);" + "\n")
		} else {
			g.fService.WriteString(g.indent() + "return;" + "\n")
		}
	}

	// Close receive function
	g.indentDown()
	if g.opts.ES6 {
		g.fService.WriteString(g.indent() + "}" + "\n")
	} else {
		g.fService.WriteString(g.indent() + "};" + "\n")
	}
}

// renderRecvThrow is render_recv_throw.
func (g *Generator) renderRecvThrow(v string) string {
	if g.opts.Node {
		return "return callback(" + v + ");"
	}
	return "throw " + v + ";"
}

// renderRecvReturn is render_recv_return.
func (g *Generator) renderRecvReturn(v string) string {
	if g.opts.Node {
		return "return callback(null, " + v + ");"
	}
	return "return " + v + ";"
}

// tsFunctionSignature is ts_function_signature: a TypeScript function
// signature of the form 'name(args: types): type;'.
func (g *Generator) tsFunctionSignature(f *sema.Function, includeCallback bool) string {
	fields := f.Arglist().Members()

	str := f.Name() + "("

	hasWrittenOptional := false

	for i, fld := range fields {
		// Ensure that non optional parameters do not follow optional
		// parameters. E.g. public foo(a: string, b?: string; c: string)
		// is invalid, c must be optional, or b non-optional.
		originalOptional := tsGetReq(fld)
		optional := originalOptional
		if hasWrittenOptional {
			optional = "?"
		}
		hasWrittenOptional = hasWrittenOptional || optional != ""

		str += fld.Name() + optional + ": " + g.tsGetType(fld.Type())

		if i+1 != len(fields) || (includeCallback && len(fields) > 0) {
			str += ", "
		}
	}

	if includeCallback {
		if g.opts.Node {
			exceptions := f.Xceptions()
			exceptionTypes := ""
			if exceptions != nil {
				for i, m := range exceptions.Members() {
					t := sema.TrueType(m.Type())
					if i == 0 {
						exceptionTypes = g.jsTypeNamespace(t.Program()) + t.Name()
					} else {
						exceptionTypes += " | " + g.jsTypeNamespace(t.Program()) + t.Name()
					}
				}
			}
			if exceptionTypes == "" {
				str += "callback: (error: void, response: " + g.tsGetType(f.ReturnType()) + ")=>void): "
			} else {
				str += "callback: (error: " + exceptionTypes + ", response: " + g.tsGetType(f.ReturnType()) + ")=>void): "
			}
		} else {
			str += "callback: (data: " + g.tsGetType(f.ReturnType()) + ")=>void): "
		}

		if g.opts.Jquery {
			str += "JQueryPromise<" + g.tsGetType(f.ReturnType()) + ">;"
		} else {
			str += "void;"
		}
	} else {
		if g.opts.ES6 {
			str += "): Promise<" + g.tsGetType(f.ReturnType()) + ">;"
		} else {
			str += "): " + g.tsGetType(f.ReturnType()) + ";"
		}
	}

	return str
}
