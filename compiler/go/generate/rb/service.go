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

package rb

import (
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService generates a thrift service.
func (g *Generator) generateService(tservice *sema.Service) {
	modules := rubyModules(tservice.Program())
	fServiceName := g.namespaceDir + underscore(g.serviceName) + ".rb"
	g.fService = &ofstream{}
	g.serviceNeedSeparator = false

	g.fService.write(rbAutogenComment() + "\n" + g.renderRequireThrift())

	if tservice.Extends() != nil {
		if g.opts.Namespaced {
			g.fService.write("require \"" +
				rbNamespaceToPathPrefix(tservice.Extends().Program().Namespace("rb")) +
				underscore(tservice.Extends().Name()) + "\"\n")
		} else {
			g.fService.write("require \"" + g.requirePrefix + underscore(tservice.Extends().Name()) + "\"\n")
		}
	}

	g.fService.write("require \"" + g.requirePrefix + underscore(g.programName) + "_types\"\n\n")

	beginNamespace(g.fService, modules)

	g.fService.indent()
	g.fService.write("module " + capitalize(tservice.Name()) + "\n")
	g.fService.indentUp()

	// Generate the three main parts of the service (well, two for now in
	// PHP).
	g.generateServiceClient(tservice)
	g.generateServiceServer(tservice)
	g.generateServiceHelpers(tservice)

	g.fService.indentDown()
	g.fService.indent()
	g.fService.write("end\n")

	endNamespace(g.fService, modules)

	// Close service file.
	emit.WriteFile(fServiceName, g.fService.String())
}

// generateServiceHelpers generates helper functions for a service.
func (g *Generator) generateServiceHelpers(tservice *sema.Service) {
	g.maybeSeparateTopLevel(g.fService)
	g.fService.indent()
	g.fService.write("# HELPER FUNCTIONS AND STRUCTURES\n")
	g.markTopLevelWritten(g.fService)

	for _, f := range tservice.Functions() {
		ts := f.Arglist()
		g.generateRbStruct(g.fService, ts, false)
		g.generateRbFunctionHelpers(f)
	}
}

// generateRbFunctionHelpers generates a struct and helpers for a function.
func (g *Generator) generateRbFunctionHelpers(tfunction *sema.Function) {
	result := sema.NewStruct(g.program)
	result.SetName(tfunction.Name() + "_result")
	success := sema.NewField(tfunction.ReturnType(), "success", 0)
	if !tfunction.ReturnType().IsVoid() {
		result.Append(success)
	}

	xs := tfunction.Xceptions()
	for _, f := range xs.Members() {
		result.Append(f)
	}
	g.generateRbStruct(g.fService, result, false)
}

// generateServiceClient generates a service client definition.
func (g *Generator) generateServiceClient(tservice *sema.Service) {
	g.maybeSeparateTopLevel(g.fService)
	extendsClient := ""
	extendsService := tservice.Extends()
	if extendsService != nil {
		extendsClient = " < " + fullTypeName(extendsService) + "::Client"
	}

	g.fService.indent()
	g.fService.write("class Client" + extendsClient + "\n")
	g.fService.indentUp()

	// Generate client method implementations.
	functions := tservice.Functions()
	g.fService.indent()
	g.fService.write("include ::Thrift::Client\n")
	if len(functions) != 0 {
		g.fService.write("\n")
	}

	for i, f := range functions {
		argStruct := f.Arglist()
		fields := argStruct.Members()
		funname := f.Name()

		// Open function.
		g.fService.indent()
		g.fService.write("def " + functionSignature(f) + "\n")
		g.fService.indentUp()
		g.fService.indent()
		g.fService.write("send_" + funname)
		if len(fields) != 0 {
			g.fService.write("(")
			for j, fld := range fields {
				if j != 0 {
					g.fService.write(", ")
				}
				g.fService.write(fld.Name())
			}
			g.fService.write(")")
		}
		g.fService.write("\n")

		if !f.IsOneway() {
			g.fService.indent()
			g.fService.write("recv_" + funname + "\n")
		}
		g.fService.indentDown()
		g.fService.indent()
		g.fService.write("end\n")
		g.fService.write("\n")

		g.fService.indent()
		g.fService.write("def send_" + functionSignature(f) + "\n")
		g.fService.indentUp()

		argsname := capitalize(f.Name() + "_args")
		messageSendProc := "send_message"
		if f.IsOneway() {
			messageSendProc = "send_oneway_message"
		}

		g.fService.indent()
		g.fService.write(messageSendProc + "(\"" + funname + "\", " + argsname)

		if len(fields) != 0 {
			g.fService.write(", {")
			for j, fld := range fields {
				if j != 0 {
					g.fService.write(", ")
				}
				g.fService.write(fld.Name() + ": " + fld.Name())
			}
			g.fService.write("}")
		}

		g.fService.write(")\n")

		g.fService.indentDown()
		g.fService.indent()
		g.fService.write("end\n")

		if !f.IsOneway() {
			resultname := capitalize(f.Name() + "_result")
			// t_struct noargs(program_); t_function recv_function(...):
			// the receive-side function has an empty argument list, so
			// its function_signature is always just its bare name.
			recvName := "recv_" + f.Name()

			// Open function.
			g.fService.write("\n")
			g.fService.indent()
			g.fService.write("def " + recvName + "\n")
			g.fService.indentUp()

			g.fService.indent()
			g.fService.write("fname, mtype, rseqid = receive_message_begin\n")
			g.fService.indent()
			g.fService.write("validate_message_begin(fname, mtype, rseqid, \"" + funname + "\")\n")

			xceptions := f.Xceptions().Members()

			g.fService.indent()
			if !f.ReturnType().IsVoid() || len(xceptions) != 0 {
				g.fService.write("result = ")
			}
			g.fService.write("receive_message(" + resultname + ")\n")

			// Careful, only return _result if not a void function.
			if !f.ReturnType().IsVoid() {
				g.fService.indent()
				g.fService.write("return result.success unless result.success.nil?\n")
			}

			for _, x := range xceptions {
				g.fService.indent()
				g.fService.write("raise result." + x.Name() + " unless result." + x.Name() + ".nil?\n")
			}

			// Careful, only return _result if not a void function.
			if f.ReturnType().IsVoid() {
				g.fService.indent()
				g.fService.write("nil\n")
			} else {
				g.fService.indent()
				g.fService.write("raise ::Thrift::ApplicationException.new(::Thrift::ApplicationException::" +
					"MISSING_RESULT, \"" + f.Name() + " failed: unknown result\")\n")
			}

			// Close function.
			g.fService.indentDown()
			g.fService.indent()
			g.fService.write("end\n")
		}

		if i+1 != len(functions) {
			g.fService.write("\n")
		}
	}

	g.fService.indentDown()
	g.fService.indent()
	g.fService.write("end\n")
	g.markTopLevelWritten(g.fService)
}

// generateServiceServer generates a service server definition.
func (g *Generator) generateServiceServer(tservice *sema.Service) {
	g.maybeSeparateTopLevel(g.fService)
	// Generate the dispatch methods.
	functions := tservice.Functions()

	extendsProcessor := ""
	extendsService := tservice.Extends()
	if extendsService != nil {
		extendsProcessor = " < " + fullTypeName(extendsService) + "::Processor"
	}

	// Generate the header portion.
	g.fService.indent()
	g.fService.write("class Processor" + extendsProcessor + "\n")
	g.fService.indentUp()

	g.fService.indent()
	g.fService.write("include ::Thrift::Processor\n")
	if len(functions) != 0 {
		g.fService.write("\n")
	}

	// Generate the process subfunctions.
	for i, f := range functions {
		g.generateProcessFunction(tservice, f)
		if i+1 != len(functions) {
			g.fService.write("\n")
		}
	}

	g.fService.indentDown()
	g.fService.indent()
	g.fService.write("end\n")
	g.markTopLevelWritten(g.fService)
}

// generateProcessFunction generates a process function definition.
func (g *Generator) generateProcessFunction(tservice *sema.Service, tfunction *sema.Function) {
	_ = tservice
	// Open function.
	g.fService.indent()
	g.fService.write("def process_" + tfunction.Name() + "(seqid, iprot, oprot)\n")
	g.fService.indentUp()

	argsname := capitalize(tfunction.Name()) + "_args"
	resultname := capitalize(tfunction.Name()) + "_result"

	argStruct := tfunction.Arglist()
	fields := argStruct.Members()

	g.fService.indent()
	if len(fields) != 0 {
		g.fService.write("args = ")
	}
	g.fService.write("read_args(iprot, " + argsname + ")\n")

	xs := tfunction.Xceptions()
	xceptions := xs.Members()

	// Declare result for non oneway function.
	if !tfunction.IsOneway() {
		g.fService.indent()
		g.fService.write("result = " + resultname + ".new\n")
	}

	// Try block for a function with exceptions.
	if len(xceptions) > 0 {
		g.fService.indent()
		g.fService.write("begin\n")
		g.fService.indentUp()
	}

	// Generate the function call.
	g.fService.indent()
	if !tfunction.IsOneway() && !tfunction.ReturnType().IsVoid() {
		g.fService.write("result.success = ")
	}
	g.fService.write("@handler." + tfunction.Name())
	if len(fields) != 0 {
		g.fService.write("(")
		for i, f := range fields {
			if i != 0 {
				g.fService.write(", ")
			}
			g.fService.write("args." + f.Name())
		}
		g.fService.write(")")
	}
	g.fService.write("\n")

	if !tfunction.IsOneway() && len(xceptions) > 0 {
		g.fService.indentDown()
		for _, x := range xceptions {
			g.fService.indent()
			g.fService.write("rescue " + fullTypeName(x.Type()) + " => " + x.Name() + "\n")
			if !tfunction.IsOneway() {
				g.fService.indentUp()
				g.fService.indent()
				g.fService.write("result." + x.Name() + " = " + x.Name() + "\n")
				g.fService.indentDown()
			}
		}
		g.fService.indent()
		g.fService.write("end\n")
	}

	// Shortcut out here for oneway functions.
	if tfunction.IsOneway() {
		g.fService.indent()
		g.fService.write("nil\n")
		g.fService.indentDown()
		g.fService.indent()
		g.fService.write("end\n")
		return
	}

	g.fService.indent()
	g.fService.write("write_result(result, oprot, \"" + tfunction.Name() + "\", seqid)\n")

	// Close function.
	g.fService.indentDown()
	g.fService.indent()
	g.fService.write("end\n")
}

// functionSignature renders a Ruby method signature, omitting parentheses
// when there are no arguments.
func functionSignature(tfunction *sema.Function) string {
	arguments := argumentList(tfunction.Arglist())
	if arguments == "" {
		return tfunction.Name()
	}
	return tfunction.Name() + "(" + arguments + ")"
}

// argumentList renders a field list.
func argumentList(tstruct *sema.Struct) string {
	result := ""
	first := true
	for _, f := range tstruct.Members() {
		if first {
			first = false
		} else {
			result += ", "
		}
		result += f.Name()
	}
	return result
}
