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

package lua

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService is t_lua_generator::generate_service.
func (g *Generator) generateService(s *sema.Service) {
	outDir := g.outDir()
	curNs := getNamespace(g.program)
	fServiceName := outDir + curNs + s.Name() + ".lua"

	var fService strings.Builder
	fService.WriteString(g.autogenComment() + g.luaIncludes())
	if !g.opts.OmitRequires {
		fService.WriteString("\n" + "require '" + curNs + "ttypes'" + "\n")

		if s.Extends() != nil {
			fService.WriteString("require '" + getNamespace(s.Extends().Program()) + s.Extends().Name() + "'" + "\n")
		}
	}

	fService.WriteString("\n")

	g.generateServiceClient(&fService, s)
	g.generateServiceInterface(&fService, s)
	g.generateServiceProcessor(&fService, s)
	g.generateServiceHelpers(&fService, s)

	emit.WriteFile(fServiceName, fService.String())
}

// generateServiceInterface is t_lua_generator::generate_service_interface.
func (g *Generator) generateServiceInterface(out *strings.Builder, s *sema.Service) {
	classname := s.Name() + "Iface"
	extendsS := s.Extends()

	out.WriteString(classname + " = ")
	if extendsS != nil {
		out.WriteString(extendsS.Name() + "Iface:new{\n")
	} else {
		out.WriteString("__TObject:new{\n")
	}
	out.WriteString("  __type = '" + classname + "'" + "\n" + "}" + "\n" + "\n")
}

// generateServiceClient is t_lua_generator::generate_service_client.
func (g *Generator) generateServiceClient(out *strings.Builder, s *sema.Service) {
	classname := s.Name() + "Client"
	extendsS := s.Extends()

	out.WriteString(classname + " = __TObject.new(")
	if extendsS != nil {
		out.WriteString(extendsS.Name() + "Client")
	} else {
		out.WriteString("__TClient")
	}
	out.WriteString(", {\n  __type = '" + classname + "'\n})\n")

	for _, f := range s.Functions() {
		sig := g.functionSignature(f)
		funcname := f.Name()

		// Wrapper function
		out.WriteString(g.indent() + "\n" + "function " + classname + ":" + sig + "\n")
		g.indentUp()

		out.WriteString(g.indent() + "self:send_" + sig + "\n" + g.indent())
		if !f.IsOneway() {
			if !f.ReturnType().IsVoid() {
				out.WriteString("return ")
			}
			out.WriteString("self:recv_" + sig + "\n")
		}

		g.indentDown()
		g.line(out, "end")

		// Send function
		out.WriteString(g.indent() + "\n" + "function " + classname + ":send_" + sig + "\n")
		g.indentUp()

		messageType := "TMessageType.CALL"
		if f.IsOneway() {
			messageType = "TMessageType.ONEWAY"
		}
		g.line(out, "self.oprot:writeMessageBegin('"+funcname+"', "+messageType+", self._seqid)")
		g.line(out, "local args = "+funcname+"_args:new{}")

		// Set the args
		for _, arg := range f.Arglist().Members() {
			argname := arg.Name()
			if arg.Value() != nil {
				typ := sema.TrueType(arg.Type())
				g.line(out, "if "+argname+" ~= nil then")
				g.indentUp()
				g.line(out, "args."+argname+" = "+argname)
				g.indentDown()
				g.line(out, "else")
				g.indentUp()
				g.line(out, "args."+argname+" = "+g.renderConstValue(typ, arg.Value()))
				g.indentDown()
				g.line(out, "end")
			} else {
				g.line(out, "args."+argname+" = "+argname)
			}
		}

		g.line(out, "args:write(self.oprot)")
		g.line(out, "self.oprot:writeMessageEnd()")
		g.line(out, "self.oprot.trans:flush()")

		g.indentDown()
		g.line(out, "end")

		// Recv function
		if !f.IsOneway() {
			out.WriteString(g.indent() + "\n" + "function " + classname + ":recv_" + sig + "\n")
			g.indentUp()

			g.line(out, "local fname, mtype, rseqid = self.iprot:readMessageBegin()")
			g.line(out, "if mtype == TMessageType.EXCEPTION then")
			g.line(out, "  local x = TApplicationException:new{}")
			g.line(out, "  x:read(self.iprot)")
			g.line(out, "  self.iprot:readMessageEnd()")
			g.line(out, "  error(x)")
			g.line(out, "end")
			g.line(out, "local result = "+funcname+"_result:new{}")
			g.line(out, "result:read(self.iprot)")
			g.line(out, "self.iprot:readMessageEnd()")

			// Return the result if it's not a void function
			if !f.ReturnType().IsVoid() {
				g.line(out, "if result.success ~= nil then")
				g.line(out, "  return result.success")

				// Throw custom exceptions
				for _, x := range f.Xceptions().Members() {
					g.line(out, "elseif result."+x.Name()+" then")
					g.line(out, "  error(result."+x.Name()+")")
				}

				g.line(out, "end")
				g.line(out, "error(TApplicationException:new{errorCode = TApplicationException.MISSING_RESULT})")
			}

			g.indentDown()
			g.line(out, "end")
		}
	}
}

// generateServiceProcessor is t_lua_generator::generate_service_processor.
func (g *Generator) generateServiceProcessor(out *strings.Builder, s *sema.Service) {
	classname := s.Name() + "Processor"
	extendsS := s.Extends()

	out.WriteString("\n" + classname + " = __TObject.new(")
	if extendsS != nil {
		out.WriteString(extendsS.Name() + "Processor" + "\n")
	} else {
		out.WriteString("__TProcessor" + "\n")
	}
	out.WriteString(", {\n __type = '" + classname + "'\n})\n")

	// Process function
	out.WriteString(g.indent() + "\n" + "function " + classname + ":process(iprot, oprot, server_ctx)" + "\n")
	g.indentUp()

	g.line(out, "local name, mtype, seqid = iprot:readMessageBegin()")
	g.line(out, "local func_name = 'process_' .. name")
	g.line(out, "if not self[func_name] or ttype(self[func_name]) ~= 'function' then")
	g.indentUp()
	g.indentRaw(out, "if oprot ~= nil then")
	g.indentUp()
	out.WriteString("\n")
	g.line(out, "iprot:skip(TType.STRUCT)")
	g.line(out, "iprot:readMessageEnd()")
	g.line(out, "x = TApplicationException:new{")
	g.line(out, "  errorCode = TApplicationException.UNKNOWN_METHOD")
	g.line(out, "}")
	g.line(out, "oprot:writeMessageBegin(name, TMessageType.EXCEPTION, seqid)")
	g.line(out, "x:write(oprot)")
	g.line(out, "oprot:writeMessageEnd()")
	g.line(out, "oprot.trans:flush()")
	g.indentDown()
	g.line(out, "end")
	g.line(out, "return false, 'Unknown function '..name")
	g.indentDown()
	g.line(out, "else")
	g.line(out, "  return self[func_name](self, seqid, iprot, oprot, server_ctx)")
	g.line(out, "end")

	g.indentDown()
	g.line(out, "end")

	// Generate the process subfunctions
	for _, f := range s.Functions() {
		g.generateProcessFunction(out, s, f)
	}
}

// generateProcessFunction is t_lua_generator::generate_process_function.
func (g *Generator) generateProcessFunction(out *strings.Builder, s *sema.Service, f *sema.Function) {
	classname := s.Name() + "Processor"
	argsname := f.Name() + "_args"
	resultname := f.Name() + "_result"
	fnName := f.Name()

	out.WriteString(g.indent() + "\n" + "function " + classname + ":process_" + fnName + "(seqid, iprot, oprot, server_ctx)" + "\n")
	g.indentUp()

	// Read the request
	g.line(out, "local args = "+argsname+":new{}")
	g.line(out, "local reply_type = TMessageType.REPLY")
	g.line(out, "args:read(iprot)")
	g.line(out, "iprot:readMessageEnd()")

	if !f.IsOneway() {
		g.line(out, "local result = "+resultname+":new{}")
	} else {
		g.line(out, "oprot.trans:flushOneway()")
	}

	call := "local status, res = pcall(self.handler." + fnName + ", self.handler"
	args := f.Arglist()
	if len(args.Members()) > 0 {
		call += ", " + g.argumentList(args, "args.")
	}
	call += ")"
	g.line(out, call)

	if !f.IsOneway() {
		g.line(out, "if not status then")
		g.line(out, "  reply_type = TMessageType.EXCEPTION")
		g.line(out, "  result = TApplicationException:new{message = res}")

		xf := f.Xceptions().Members()
		for _, x := range xf {
			g.line(out, "elseif ttype(res) == '"+x.Type().Name()+"' then")
			g.line(out, "  result."+x.Name()+" = res")
		}

		g.line(out, "else")
		g.line(out, "  result.success = res")
		g.line(out, "end")
		g.line(out, "oprot:writeMessageBegin('"+fnName+"', reply_type, seqid)")
		g.line(out, "result:write(oprot)")
		g.line(out, "oprot:writeMessageEnd()")
		g.line(out, "oprot.trans:flush()")
	}
	g.line(out, "return status, res")
	g.indentDown()
	g.line(out, "end")
}

// generateServiceHelpers is t_lua_generator::generate_service_helpers.
func (g *Generator) generateServiceHelpers(out *strings.Builder, s *sema.Service) {
	functions := s.Functions()

	out.WriteString("\n" + "-- HELPER FUNCTIONS AND STRUCTURES")
	for _, f := range functions {
		g.generateLuaStructDefinition(out, f.Arglist(), false)
		g.generateFunctionHelpers(out, f)
	}
}

// generateFunctionHelpers is t_lua_generator::generate_function_helpers.
func (g *Generator) generateFunctionHelpers(out *strings.Builder, f *sema.Function) {
	if f.IsOneway() {
		return
	}
	result := sema.NewStruct(g.program)
	result.SetName(f.Name() + "_result")
	success := sema.NewField(f.ReturnType(), "success", 0)
	if !f.ReturnType().IsVoid() {
		result.Append(success)
	}

	for _, x := range f.Xceptions().Members() {
		result.Append(x)
	}
	g.generateLuaStructDefinition(out, result, false)
}
