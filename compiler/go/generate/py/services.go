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

package py

import (
	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService is generate_service.
func (g *Generator) generateService(s *sema.Service) {
	g.fService.Reset()
	fServiceName := g.packageDir + "/" + maybeEscapeIdentifier(g.serviceName) + ".py"

	g.fService.WriteString(g.pyAutogenComment() + "\n" + g.pyImports() + "\n")

	if s.Extends() != nil {
		g.fService.WriteString("import " + g.realPyModule(s.Extends().Program(), g.opts.PackagePrefix) + "." +
			maybeEscapeIdentifier(s.Extends().Name()) + "\n")
	}

	g.fService.WriteString("import logging" + "\n" +
		"from .ttypes import *" + "\n" +
		"from thrift.Thrift import TProcessor" + "\n" +
		"from thrift.transport import TTransport" + "\n" +
		g.opts.ImportDynbase)
	if g.opts.ZopeInterface {
		g.fService.WriteString("from zope.interface import Interface, implementer\n")
	}

	if g.opts.Twisted {
		g.fService.WriteString("from twisted.internet import defer\n" +
			"from thrift.transport import TTwisted\n")
	} else if g.opts.Tornado {
		g.fService.WriteString("from tornado import gen\n")
		g.fService.WriteString("from tornado import concurrent\n")
	}

	g.fService.WriteString("all_structs = []\n")

	// Generate the three main parts of the service.
	g.generateServiceInterface(s)
	g.generateServiceClient(s)
	g.generateServiceServer(s)
	g.generateServiceHelpers(s)
	g.generateServiceRemote(s)

	g.fService.WriteString("fix_spec(all_structs)\n" + "del all_structs\n")

	emit.WriteFile(fServiceName, g.fService.String())
}

// generateServiceHelpers generates helper functions for a service.
func (g *Generator) generateServiceHelpers(s *sema.Service) {
	g.fService.WriteString("\n# HELPER FUNCTIONS AND STRUCTURES\n")

	for _, f := range s.Functions() {
		ts := f.Arglist()
		g.generatePyStructDefinition(&g.fService, ts, false)
		g.generatePyThriftSpec(&g.fService, ts, false)
		g.generatePyFunctionHelpers(f)
	}
}

// generatePyFunctionHelpers generates a struct and helpers for a function.
func (g *Generator) generatePyFunctionHelpers(f *sema.Function) {
	if f.IsOneway() {
		return
	}
	result := sema.NewStruct(g.program)
	result.SetName(f.Name() + "_result")
	if !f.ReturnType().IsVoid() {
		success := sema.NewField(f.ReturnType(), "success", 0)
		result.Append(success)
	}

	for _, field := range f.Xceptions().Members() {
		result.Append(field)
	}
	g.generatePyStructDefinition(&g.fService, result, false)
	g.generatePyThriftSpec(&g.fService, result, false)
}

// generateServiceInterface generates a service interface definition.
func (g *Generator) generateServiceInterface(s *sema.Service) {
	extends := ""
	extendsIf := ""
	if s.Extends() != nil {
		extends = g.typeName(s.Extends())
		extendsIf = "(" + extends + ".Iface)"
	} else {
		if g.opts.ZopeInterface {
			extendsIf = "(Interface)"
		} else if g.opts.NewStyle || g.opts.Dynamic || g.opts.Tornado {
			extendsIf = "(object)"
		}
	}

	g.fService.WriteString("\n\nclass Iface" + extendsIf + ":\n")
	g.indentUp()
	g.generatePythonDocstring(&g.fService, s)
	functions := s.Functions()
	if len(functions) == 0 {
		g.fService.WriteString(g.indent() + "pass\n")
	} else {
		first := true
		for _, f := range functions {
			if first {
				first = false
			} else {
				g.fService.WriteString("\n")
			}
			g.fService.WriteString(g.indent() + "def " + g.functionSignature(f.Name(), f.Arglist(), f.ReturnType(), true, false) + ":\n")
			g.indentUp()
			g.generatePythonDocstringFunction(&g.fService, f)
			g.fService.WriteString(g.indent() + "pass\n")
			g.indentDown()
		}
	}
	g.indentDown()
}

// generateServiceClient generates a service client definition.
func (g *Generator) generateServiceClient(s *sema.Service) {
	extends := ""
	extendsClient := ""
	if s.Extends() != nil {
		extends = g.typeName(s.Extends())
		if g.opts.ZopeInterface {
			extendsClient = "(" + extends + ".Client)"
		} else {
			extendsClient = extends + ".Client, "
		}
	} else {
		if g.opts.ZopeInterface && (g.opts.NewStyle || g.opts.Dynamic) {
			extendsClient = "(object)"
		}
	}

	g.fService.WriteString("\n\n")

	if g.opts.ZopeInterface {
		g.fService.WriteString("@implementer(Iface)\n" +
			"class Client" + extendsClient + ":\n" +
			"\n")
	} else {
		g.fService.WriteString("class Client(" + extendsClient + "Iface):\n")
	}
	g.indentUp()
	g.generatePythonDocstring(&g.fService, s)

	// Constructor function.
	if g.opts.Twisted {
		g.fService.WriteString(g.indent() + "def __init__(self, transport, oprot_factory):\n")
	} else if g.opts.Tornado {
		g.fService.WriteString(g.indent() + "def __init__(self, transport, iprot_factory, oprot_factory=None):\n")
	} else {
		g.fService.WriteString(g.indent() + "def __init__(self, iprot, oprot=None):\n")
	}
	g.indentUp()
	if extends == "" {
		if g.opts.Twisted {
			g.fService.WriteString(g.indent() + "self._transport = transport\n" +
				g.indent() + "self._oprot_factory = oprot_factory\n" +
				g.indent() + "self._seqid = 0\n" +
				g.indent() + "self._reqs = {}\n")
		} else if g.opts.Tornado {
			g.fService.WriteString(g.indent() + "self._transport = transport\n" +
				g.indent() + "self._iprot_factory = iprot_factory\n" +
				g.indent() + "self._oprot_factory = (oprot_factory if oprot_factory is not None\n" +
				g.indent() + "                       else iprot_factory)\n" +
				g.indent() + "self._seqid = 0\n" +
				g.indent() + "self._reqs = {}\n" +
				g.indent() + "self._transport.io_loop.spawn_callback(self._start_receiving)\n")
		} else {
			g.fService.WriteString(g.indent() + "self._iprot = self._oprot = iprot\n" +
				g.indent() + "if oprot is not None:\n" +
				g.indent() + indentStr + "self._oprot = oprot\n" +
				g.indent() + "self._seqid = 0\n")
		}
	} else {
		if g.opts.Twisted {
			g.fService.WriteString(g.indent() + extends + ".Client.__init__(self, transport, oprot_factory)\n")
		} else if g.opts.Tornado {
			g.fService.WriteString(g.indent() + extends + ".Client.__init__(self, transport, iprot_factory, oprot_factory)\n")
		} else {
			g.fService.WriteString(g.indent() + extends + ".Client.__init__(self, iprot, oprot)\n")
		}
	}
	g.indentDown()

	if g.opts.Tornado && extends == "" {
		g.fService.WriteString("\n" +
			g.indent() + "@gen.coroutine\n" +
			g.indent() + "def _start_receiving(self):\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "while True:\n")
		g.indentUp()
		g.fService.WriteString(g.indent() + "try:\n" +
			g.indent() + indentStr + "frame = yield self._transport.readFrame()\n" +
			g.indent() + "except TTransport.TTransportException as e:\n" +
			g.indent() + indentStr + "for future in self._reqs.values():\n" +
			g.indent() + indentStr + indentStr + "future.set_exception(e)\n" +
			g.indent() + indentStr + "self._reqs = {}\n" +
			g.indent() + indentStr + "return\n" +
			g.indent() + "tr = TTransport.TMemoryBuffer(frame)\n" +
			g.indent() + "iprot = self._iprot_factory.getProtocol(tr)\n" +
			g.indent() + "(fname, mtype, rseqid) = iprot.readMessageBegin()\n" +
			g.indent() + "method = getattr(self, 'recv_' + fname)\n" +
			g.indent() + "future = self._reqs.pop(rseqid, None)\n" +
			g.indent() + "if not future:\n" +
			g.indent() + indentStr + "# future has already been discarded\n" +
			g.indent() + indentStr + "continue\n" +
			g.indent() + "try:\n" +
			g.indent() + indentStr + "result = method(iprot, mtype, rseqid)\n" +
			g.indent() + "except Exception as e:\n" +
			g.indent() + indentStr + "future.set_exception(e)\n" +
			g.indent() + "else:\n" +
			g.indent() + indentStr + "future.set_result(result)\n")
		g.indentDown()
		g.indentDown()
	}

	// Generate client method implementations.
	for _, f := range s.Functions() {
		fields := f.Arglist().Members()
		funname := f.Name()

		g.fService.WriteString("\n")
		// Open function.
		g.fService.WriteString(g.indent() + "def " + g.functionSignature(f.Name(), f.Arglist(), f.ReturnType(), false, false) + ":\n")
		g.indentUp()
		g.generatePythonDocstringFunction(&g.fService, f)
		switch {
		case g.opts.Twisted:
			g.fService.WriteString(g.indent() + "seqid = self._seqid = self._seqid + 1\n")
			g.fService.WriteString(g.indent() + "self._reqs[seqid] = defer.Deferred()\n\n")
			g.fService.WriteString(g.indent() + "d = defer.maybeDeferred(self.send_" + funname)
		case g.opts.Tornado:
			g.fService.WriteString(g.indent() + "self._seqid += 1\n")
			if !f.IsOneway() {
				g.fService.WriteString(g.indent() + "future = self._reqs[self._seqid] = concurrent.Future()\n")
			}
			g.fService.WriteString(g.indent() + "self.send_" + funname + "(")
		default:
			g.fService.WriteString(g.indent() + "self.send_" + funname + "(")
		}

		// A leading comma is needed if there are args for the twisted
		// case, since it is called as maybeDeferred(funcname, arg).
		first := !g.opts.Twisted
		for _, fld := range fields {
			if first {
				first = false
			} else {
				g.fService.WriteString(", ")
			}
			g.fService.WriteString(maybeEscapeIdentifier(fld.Name()))
		}

		g.fService.WriteString(")\n")

		if !f.IsOneway() {
			switch {
			case g.opts.Twisted:
				// nothing; see the next block.
			case g.opts.Tornado:
				g.fService.WriteString(g.indent() + "return future\n")
			default:
				g.fService.WriteString(g.indent())
				if !f.ReturnType().IsVoid() {
					g.fService.WriteString("return ")
				}
				g.fService.WriteString("self.recv_" + funname + "()\n")
			}
		}
		g.indentDown()

		if g.opts.Twisted {
			// This block injects the body of the send_<> method for
			// twisted (and a cb/eb pair).
			g.indentUp()
			g.fService.WriteString(g.indent() + "d.addCallbacks(\n")

			g.indentUp()
			g.fService.WriteString(g.indent() + "callback=self.cb_send_" + funname + ",\n" +
				g.indent() + "callbackArgs=(seqid,),\n" +
				g.indent() + "errback=self.eb_send_" + funname + ",\n" +
				g.indent() + "errbackArgs=(seqid,))\n")
			g.indentDown()

			g.fService.WriteString(g.indent() + "return d\n")
			g.indentDown()
			g.fService.WriteString("\n")

			g.fService.WriteString(g.indent() + "def cb_send_" + funname + "(self, _, seqid):\n")
			g.indentUp()
			if f.IsOneway() {
				// If one-way, fire the deferred and remove it from _reqs.
				g.fService.WriteString(g.indent() + "d = self._reqs.pop(seqid)\n" +
					g.indent() + "d.callback(None)\n" +
					g.indent() + "return d\n")
			} else {
				g.fService.WriteString(g.indent() + "return self._reqs[seqid]\n")
			}
			g.indentDown()
			g.fService.WriteString("\n")

			// Add an errback to fail the request if the call to send_<>
			// raised an exception.
			g.fService.WriteString(g.indent() + "def eb_send_" + funname + "(self, f, seqid):\n")
			g.indentUp()
			g.fService.WriteString(g.indent() + "d = self._reqs.pop(seqid)\n" +
				g.indent() + "d.errback(f)\n" +
				g.indent() + "return d\n")
			g.indentDown()
		}

		g.fService.WriteString("\n")
		g.fService.WriteString(g.indent() + "def send_" + g.functionSignature(f.Name(), f.Arglist(), f.ReturnType(), false, true) + ":\n")
		g.indentUp()

		argsname := maybeEscapeIdentifier(f.Name()) + "_args"
		messageType := "TMessageType.CALL"
		if f.IsOneway() {
			messageType = "TMessageType.ONEWAY"
		}

		// Serialize the request header.
		if g.opts.Twisted || g.opts.Tornado {
			g.fService.WriteString(g.indent() + "oprot = self._oprot_factory.getProtocol(self._transport)\n" +
				g.indent() + "oprot.writeMessageBegin('" + f.Name() + "', " + messageType + ", self._seqid)\n")
		} else {
			g.fService.WriteString(g.indent() + "self._oprot.writeMessageBegin('" + f.Name() + "', " + messageType + ", self._seqid)\n")
		}

		g.fService.WriteString(g.indent() + "args = " + argsname + "()\n")

		for _, fld := range fields {
			g.fService.WriteString(g.indent() + "args." + maybeEscapeIdentifier(fld.Name()) + " = " + maybeEscapeIdentifier(fld.Name()) + "\n")
		}

		// Write to the stream.
		if g.opts.Twisted || g.opts.Tornado {
			g.fService.WriteString(g.indent() + "args.write(oprot)\n" +
				g.indent() + "oprot.writeMessageEnd()\n" +
				g.indent() + "oprot.trans.flush()\n")
		} else {
			g.fService.WriteString(g.indent() + "args.write(self._oprot)\n" +
				g.indent() + "self._oprot.writeMessageEnd()\n" +
				g.indent() + "self._oprot.trans.flush()\n")
		}

		g.indentDown()

		if !f.IsOneway() {
			resultname := maybeEscapeIdentifier(f.Name()) + "_result"
			// Open function.
			g.fService.WriteString("\n")
			if g.opts.Twisted || g.opts.Tornado {
				g.fService.WriteString(g.indent() + "def recv_" + maybeEscapeIdentifier(f.Name()) + "(self, iprot, mtype, rseqid):\n")
			} else {
				noargs := sema.NewStruct(g.program)
				g.fService.WriteString(g.indent() + "def " + g.functionSignature("recv_"+maybeEscapeIdentifier(f.Name()), noargs, f.ReturnType(), false, false) + ":\n")
			}
			g.indentUp()

			switch {
			case g.opts.Twisted:
				g.fService.WriteString(g.indent() + "d = self._reqs.pop(rseqid)\n")
			case g.opts.Tornado:
			default:
				g.fService.WriteString(g.indent() + "iprot = self._iprot\n" +
					g.indent() + "(fname, mtype, rseqid) = iprot.readMessageBegin()\n")
			}

			g.fService.WriteString(g.indent() + "if mtype == TMessageType.EXCEPTION:\n" +
				g.indent() + indentStr + "x = TApplicationException()\n")

			if g.opts.Twisted {
				g.fService.WriteString(g.indent() + indentStr + "x.read(iprot)\n" +
					g.indent() + indentStr + "iprot.readMessageEnd()\n" +
					g.indent() + indentStr + "return d.errback(x)\n" +
					g.indent() + "result = " + resultname + "()\n" +
					g.indent() + "result.read(iprot)\n" +
					g.indent() + "iprot.readMessageEnd()\n")
			} else {
				g.fService.WriteString(g.indent() + indentStr + "x.read(iprot)\n" +
					g.indent() + indentStr + "iprot.readMessageEnd()\n" +
					g.indent() + indentStr + "raise x\n" +
					g.indent() + "result = " + resultname + "()\n" +
					g.indent() + "result.read(iprot)\n" +
					g.indent() + "iprot.readMessageEnd()\n")
			}

			// Only return _result if not a void function.
			if !f.ReturnType().IsVoid() {
				g.fService.WriteString(g.indent() + "if result.success is not None:\n")
				if g.opts.Twisted {
					g.fService.WriteString(g.indent() + indentStr + "return d.callback(result.success)\n")
				} else {
					g.fService.WriteString(g.indent() + indentStr + "return result.success\n")
				}
			}

			for _, x := range f.Xceptions().Members() {
				xname := x.Name()
				g.fService.WriteString(g.indent() + "if result." + maybeEscapeIdentifier(xname) + " is not None:\n")
				if g.opts.Twisted {
					g.fService.WriteString(g.indent() + indentStr + "return d.errback(result." + maybeEscapeIdentifier(xname) + ")\n")
				} else {
					g.fService.WriteString(g.indent() + indentStr + "raise result." + maybeEscapeIdentifier(xname) + "\n")
				}
			}

			// Only return _result if not a void function.
			if f.ReturnType().IsVoid() {
				if g.opts.Twisted {
					g.fService.WriteString(g.indent() + "return d.callback(None)\n")
				} else {
					g.fService.WriteString(g.indent() + "return\n")
				}
			} else {
				if g.opts.Twisted {
					g.fService.WriteString(g.indent() +
						"return d.errback(TApplicationException(TApplicationException.MISSING_RESULT, \"" +
						f.Name() + " failed: unknown result\"))\n")
				} else {
					g.fService.WriteString(g.indent() +
						"raise TApplicationException(TApplicationException.MISSING_RESULT, \"" +
						f.Name() + " failed: unknown result\")\n")
				}
			}

			// Close function.
			g.indentDown()
		}
	}

	g.indentDown()
}

// generateServiceServer generates a service server definition.
func (g *Generator) generateServiceServer(s *sema.Service) {
	extends := ""
	extendsProcessor := ""
	if s.Extends() != nil {
		extends = g.typeName(s.Extends())
		extendsProcessor = extends + ".Processor, "
	}

	g.fService.WriteString("\n\n")

	// Generate the header portion.
	if g.opts.ZopeInterface {
		g.fService.WriteString("@implementer(Iface)\n" +
			"class Processor(" + extendsProcessor + "TProcessor):\n")
	} else {
		g.fService.WriteString("class Processor(" + extendsProcessor + "Iface, TProcessor):\n")
	}

	g.indentUp()

	g.fService.WriteString(g.indent() + "def __init__(self, handler):\n")
	g.indentUp()
	if extends == "" {
		if g.opts.ZopeInterface {
			g.fService.WriteString(g.indent() + "self._handler = Iface(handler)\n")
		} else {
			g.fService.WriteString(g.indent() + "self._handler = handler\n")
		}

		g.fService.WriteString(g.indent() + "self._processMap = {}\n")
	} else {
		if g.opts.ZopeInterface {
			g.fService.WriteString(g.indent() + extends + ".Processor.__init__(self, Iface(handler))\n")
		} else {
			g.fService.WriteString(g.indent() + extends + ".Processor.__init__(self, handler)\n")
		}
	}
	for _, f := range s.Functions() {
		g.fService.WriteString(g.indent() + "self._processMap[\"" + f.Name() +
			"\"] = Processor.process_" + maybeEscapeIdentifier(f.Name()) + "\n")
	}
	g.fService.WriteString(g.indent() + "self._on_message_begin = None\n")
	g.indentDown()
	g.fService.WriteString("\n")

	g.fService.WriteString(g.indent() + "def on_message_begin(self, func):\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "self._on_message_begin = func\n")
	g.indentDown()
	g.fService.WriteString("\n")

	// Generate the server implementation.
	g.fService.WriteString(g.indent() + "def process(self, iprot, oprot):\n")
	g.indentUp()

	g.fService.WriteString(g.indent() + "(name, type, seqid) = iprot.readMessageBegin()\n")
	g.fService.WriteString(g.indent() + "if self._on_message_begin:\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "self._on_message_begin(name, type, seqid)\n")
	g.indentDown()

	// HOT: dictionary function lookup.
	g.fService.WriteString(g.indent() + "if name not in self._processMap:\n")
	g.indentUp()
	g.fService.WriteString(g.indent() + "iprot.skip(TType.STRUCT)\n" +
		g.indent() + "iprot.readMessageEnd()\n" +
		g.indent() + "x = TApplicationException(TApplicationException.UNKNOWN_METHOD, 'Unknown function %s' % (name))\n" +
		g.indent() + "oprot.writeMessageBegin(name, TMessageType.EXCEPTION, seqid)\n" +
		g.indent() + "x.write(oprot)\n" +
		g.indent() + "oprot.writeMessageEnd()\n" +
		g.indent() + "oprot.trans.flush()\n")

	if g.opts.Twisted {
		g.fService.WriteString(g.indent() + "return defer.succeed(None)\n")
	} else {
		g.fService.WriteString(g.indent() + "return\n")
	}
	g.indentDown()

	g.fService.WriteString(g.indent() + "else:\n")

	if g.opts.Twisted || g.opts.Tornado {
		g.fService.WriteString(g.indent() + indentStr + "return self._processMap[name](self, seqid, iprot, oprot)\n")
	} else {
		g.fService.WriteString(g.indent() + indentStr + "self._processMap[name](self, seqid, iprot, oprot)\n")

		// Read end of args field, the T_STOP, and the struct close.
		g.fService.WriteString(g.indent() + "return True\n")
	}

	g.indentDown()

	// Generate the process subfunctions.
	for _, f := range s.Functions() {
		g.fService.WriteString("\n")
		g.generateProcessFunction(s, f)
	}

	g.indentDown()
}

// generateProcessFunction generates a process function definition.
func (g *Generator) generateProcessFunction(_ *sema.Service, f *sema.Function) {
	// Open function.
	if g.opts.Tornado {
		g.fService.WriteString(g.indent() + "@gen.coroutine\n" + g.indent() + "def process_" +
			maybeEscapeIdentifier(f.Name()) + "(self, seqid, iprot, oprot):\n")
	} else {
		g.fService.WriteString(g.indent() + "def process_" + maybeEscapeIdentifier(f.Name()) +
			"(self, seqid, iprot, oprot):\n")
	}

	g.indentUp()

	argsname := maybeEscapeIdentifier(f.Name()) + "_args"
	resultname := maybeEscapeIdentifier(f.Name()) + "_result"

	g.fService.WriteString(g.indent() + "args = " + argsname + "()\n" +
		g.indent() + "args.read(iprot)\n" +
		g.indent() + "iprot.readMessageEnd()\n")

	xceptions := f.Xceptions().Members()

	// Declare result for non-oneway functions.
	if !f.IsOneway() {
		g.fService.WriteString(g.indent() + "result = " + resultname + "()\n")
	}

	switch {
	case g.opts.Twisted:
		// Generate the function call.
		fields := f.Arglist().Members()

		g.fService.WriteString(g.indent() + "d = defer.maybeDeferred(self._handler." + maybeEscapeIdentifier(f.Name()) + ", ")
		first := true
		for _, fld := range fields {
			if first {
				first = false
			} else {
				g.fService.WriteString(", ")
			}
			g.fService.WriteString("args." + maybeEscapeIdentifier(fld.Name()))
		}
		g.fService.WriteString(")\n")

		if f.IsOneway() {
			g.fService.WriteString(g.indent() + "d.addErrback(self.handle_exception_" + f.Name() + ", seqid)\n")
		} else {
			g.fService.WriteString(g.indent() + "d.addCallback(self.write_results_success_" + f.Name() + ", result, seqid, oprot)\n" +
				g.indent() + "d.addErrback(self.write_results_exception_" + f.Name() + ", result, seqid, oprot)\n")
		}
		g.fService.WriteString(g.indent() + "return d\n\n")

		g.indentDown()

		if f.IsOneway() {
			g.fService.WriteString(g.indent() + "def handle_exception_" + f.Name() + "(self, error, seqid):\n")
		} else {
			g.fService.WriteString(g.indent() + "def write_results_success_" + f.Name() + "(self, success, result, seqid, oprot):\n")
			g.indentUp()
			if !f.ReturnType().IsVoid() {
				g.fService.WriteString(g.indent() + "result.success = success\n")
			}
			g.fService.WriteString(g.indent() + "oprot.writeMessageBegin(\"" + f.Name() + "\", TMessageType.REPLY, seqid)\n" +
				g.indent() + "result.write(oprot)\n" +
				g.indent() + "oprot.writeMessageEnd()\n" +
				g.indent() + "oprot.trans.flush()\n" +
				"\n")
			g.indentDown()

			g.fService.WriteString(g.indent() + "def write_results_exception_" + f.Name() + "(self, error, result, seqid, oprot):\n")
		}
		g.indentUp()
		if !f.IsOneway() {
			g.fService.WriteString(g.indent() + "msg_type = TMessageType.REPLY\n")
		}
		g.fService.WriteString(g.indent() + "try:\n")

		// Kinda absurd.
		g.fService.WriteString(g.indent() + indentStr + "error.raiseException()\n")
		if !f.IsOneway() {
			for _, x := range xceptions {
				xname := x.Name()
				g.fService.WriteString(g.indent() + "except " + g.typeName(x.Type()) + " as " + maybeEscapeIdentifier(xname) + ":\n")
				g.indentUp()
				g.fService.WriteString(g.indent() + "result." + maybeEscapeIdentifier(xname) + " = " + maybeEscapeIdentifier(xname) + "\n")
				g.indentDown()
			}
		}
		g.fService.WriteString(g.indent() + "except TTransport.TTransportException:\n" +
			g.indent() + indentStr + "raise\n")
		if !f.IsOneway() {
			g.fService.WriteString(g.indent() + "except TApplicationException as ex:\n" +
				g.indent() + indentStr + "logging.exception('TApplication exception in handler')\n" +
				g.indent() + indentStr + "msg_type = TMessageType.EXCEPTION\n" +
				g.indent() + indentStr + "result = ex\n" +
				g.indent() + "except Exception:\n" +
				g.indent() + indentStr + "logging.exception('Unexpected exception in handler')\n" +
				g.indent() + indentStr + "msg_type = TMessageType.EXCEPTION\n" +
				g.indent() + indentStr + "result = TApplicationException(TApplicationException.INTERNAL_ERROR, 'Internal error')\n" +
				g.indent() + "oprot.writeMessageBegin(\"" + f.Name() + "\", msg_type, seqid)\n" +
				g.indent() + "result.write(oprot)\n" +
				g.indent() + "oprot.writeMessageEnd()\n" +
				g.indent() + "oprot.trans.flush()\n")
		} else {
			g.fService.WriteString(g.indent() + "except Exception:\n" +
				g.indent() + indentStr + "logging.exception('Exception in oneway handler')\n")
		}
		g.indentDown()

	case g.opts.Tornado:
		// Generate the function call.
		fields := f.Arglist().Members()

		if !f.IsOneway() {
			g.fService.WriteString(g.indent() + "msg_type = TMessageType.REPLY\n")
		}
		g.fService.WriteString(g.indent() + "try:\n")
		g.indentUp()
		g.fService.WriteString(g.indent())
		if !f.IsOneway() && !f.ReturnType().IsVoid() {
			g.fService.WriteString("result.success = ")
		}
		g.fService.WriteString("yield gen.maybe_future(self._handler." + maybeEscapeIdentifier(f.Name()) + "(")
		first := true
		for _, fld := range fields {
			if first {
				first = false
			} else {
				g.fService.WriteString(", ")
			}
			g.fService.WriteString("args." + maybeEscapeIdentifier(fld.Name()))
		}
		g.fService.WriteString("))\n")

		g.indentDown()
		if !f.IsOneway() {
			for _, x := range xceptions {
				xname := x.Name()
				g.fService.WriteString(g.indent() + "except " + g.typeName(x.Type()) + " as " + maybeEscapeIdentifier(xname) + ":\n" +
					g.indent() + indentStr + "result." + maybeEscapeIdentifier(xname) + " = " + maybeEscapeIdentifier(xname) + "\n")
			}
		}
		g.fService.WriteString(g.indent() + "except TTransport.TTransportException:\n" +
			g.indent() + indentStr + "raise\n")
		if !f.IsOneway() {
			g.fService.WriteString(g.indent() + "except TApplicationException as ex:\n" +
				g.indent() + indentStr + "logging.exception('TApplication exception in handler')\n" +
				g.indent() + indentStr + "msg_type = TMessageType.EXCEPTION\n" +
				g.indent() + indentStr + "result = ex\n" +
				g.indent() + "except Exception:\n" +
				g.indent() + indentStr + "logging.exception('Unexpected exception in handler')\n" +
				g.indent() + indentStr + "msg_type = TMessageType.EXCEPTION\n" +
				g.indent() + indentStr + "result = TApplicationException(TApplicationException.INTERNAL_ERROR, 'Internal error')\n")
		} else {
			g.fService.WriteString(g.indent() + "except Exception:\n" +
				g.indent() + indentStr + "logging.exception('Exception in oneway handler')\n")
		}

		if !f.IsOneway() {
			g.fService.WriteString(g.indent() + "oprot.writeMessageBegin(\"" + f.Name() + "\", msg_type, seqid)\n" +
				g.indent() + "result.write(oprot)\n" +
				g.indent() + "oprot.writeMessageEnd()\n" +
				g.indent() + "oprot.trans.flush()\n")
		}

		// Close function.
		g.indentDown()

	default: // py
		// Try block for a function with exceptions. It also catches
		// arbitrary exceptions raised by the handler method to propagate
		// them to the client.
		g.fService.WriteString(g.indent() + "try:\n")
		g.indentUp()

		// Generate the function call.
		fields := f.Arglist().Members()

		g.fService.WriteString(g.indent())
		if !f.IsOneway() && !f.ReturnType().IsVoid() {
			g.fService.WriteString("result.success = ")
		}
		g.fService.WriteString("self._handler." + maybeEscapeIdentifier(f.Name()) + "(")
		first := true
		for _, fld := range fields {
			if first {
				first = false
			} else {
				g.fService.WriteString(", ")
			}
			g.fService.WriteString("args." + maybeEscapeIdentifier(fld.Name()))
		}
		g.fService.WriteString(")\n")
		if !f.IsOneway() {
			g.fService.WriteString(g.indent() + "msg_type = TMessageType.REPLY\n")
		}

		g.indentDown()
		g.fService.WriteString(g.indent() + "except TTransport.TTransportException:\n" +
			g.indent() + indentStr + "raise\n")

		if !f.IsOneway() {
			for _, x := range xceptions {
				xname := x.Name()
				g.fService.WriteString(g.indent() + "except " + g.typeName(x.Type()) + " as " + maybeEscapeIdentifier(xname) + ":\n")
				g.indentUp()
				g.fService.WriteString(g.indent() + "msg_type = TMessageType.REPLY\n")
				g.fService.WriteString(g.indent() + "result." + maybeEscapeIdentifier(xname) + " = " + maybeEscapeIdentifier(xname) + "\n")
				g.indentDown()
			}

			g.fService.WriteString(g.indent() + "except TApplicationException as ex:\n" +
				g.indent() + indentStr + "logging.exception('TApplication exception in handler')\n" +
				g.indent() + indentStr + "msg_type = TMessageType.EXCEPTION\n" +
				g.indent() + indentStr + "result = ex\n" +
				g.indent() + "except Exception:\n" +
				g.indent() + indentStr + "logging.exception('Unexpected exception in handler')\n" +
				g.indent() + indentStr + "msg_type = TMessageType.EXCEPTION\n" +
				g.indent() + indentStr + "result = TApplicationException(TApplicationException.INTERNAL_ERROR, 'Internal error')\n" +
				g.indent() + "oprot.writeMessageBegin(\"" + f.Name() + "\", msg_type, seqid)\n" +
				g.indent() + "result.write(oprot)\n" +
				g.indent() + "oprot.writeMessageEnd()\n" +
				g.indent() + "oprot.trans.flush()\n")
		} else {
			g.fService.WriteString(g.indent() + "except Exception:\n" +
				g.indent() + indentStr + "logging.exception('Exception in oneway handler')\n")
		}

		// Close function.
		g.indentDown()
	}
}
