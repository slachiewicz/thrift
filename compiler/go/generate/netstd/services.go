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

package netstd

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService is generate_service: one file per service.
func (g *Generator) generateService(s *sema.Service) {
	fServiceName := g.namespaceDir + "/" + g.serviceName + ".cs"
	var f strings.Builder

	g.resetIndent()
	f.WriteString(g.autogenComment() + g.netstdTypeUsings() + g.netstdThriftUsings() + "\n\n")

	g.pragmasAndDirectives(&f)
	g.startNetstdNamespace(&f)

	f.WriteString(g.indent() + "public partial class " + g.normalizeName(g.serviceName, false) + "\n")
	f.WriteString(g.indent() + "{\n")
	g.indentUp()

	g.generateServiceInterface(&f, s)
	g.generateServiceClient(&f, s)
	g.generateServiceServer(&f, s)
	g.generateServiceHelpers(&f, s)

	g.indentDown()
	f.WriteString(g.indent() + "}\n")

	g.endNetstdNamespace(&f)

	emit.WriteFile(fServiceName, f.String())
}

// generateServiceInterface is generate_service_interface.
func (g *Generator) generateServiceInterface(out *strings.Builder, s *sema.Service) {
	extendsIface := ""
	if s.Extends() != nil {
		extendsIface = " : " + g.tn(s.Extends()) + ".IAsync"
	}

	g.netstdDoc(out, s)

	if g.isWCFEnabled() {
		out.WriteString(g.indent() + "[ServiceContract(Namespace=\"" + g.opts.WCFNamespace + "\")]\n")
	}

	g.generateDeprecationAttribute(out, s.Annotations())
	g.prepareMemberNameMappingService(s)
	out.WriteString(g.indent() + "public interface IAsync" + extendsIface + "\n")
	out.WriteString(g.indent() + "{\n")

	g.indentUp()
	for _, f := range s.Functions() {
		g.netstdDocFunction(out, f)

		// if we're using WCF, add the corresponding attributes
		if g.isWCFEnabled() {
			out.WriteString(g.indent() + "[OperationContract]\n")

			for _, x := range f.Xceptions().Members() {
				out.WriteString(g.indent() + "[FaultContract(typeof(" + g.tn(x.Type()) + "Fault))]\n")
			}
		}

		g.generateDeprecationAttribute(out, f.Annotations())
		out.WriteString(g.indent() + g.functionSignatureAsync(f, "", modeFullDecl) + ";\n\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	g.cleanupMemberNameMapping()
}

// generateServiceHelpers is generate_service_helpers.
func (g *Generator) generateServiceHelpers(out *strings.Builder, s *sema.Service) {
	g.prepareMemberNameMappingService(s)
	out.WriteString(g.indent() + "public class InternalStructs\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	for _, f := range s.Functions() {
		ts := f.Arglist()
		g.collectExtensionsTypesStruct(ts)
		g.generateNetstdStructDefinition(out, ts, false, true, false)
		g.generateFunctionHelpers(out, f)
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	g.cleanupMemberNameMapping()
}

// generateServiceClient is generate_service_client.
func (g *Generator) generateServiceClient(out *strings.Builder, s *sema.Service) {
	extendsClient := ""
	if s.Extends() != nil {
		extendsClient = g.tn(s.Extends()) + ".Client, "
	} else {
		extendsClient = "TBaseClient, IDisposable, "
	}

	out.WriteString("\n")

	g.netstdDoc(out, s)
	g.generateDeprecationAttribute(out, s.Annotations())
	g.prepareMemberNameMappingService(s)
	out.WriteString(g.indent() + "public class Client : " + extendsClient + "IAsync\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	out.WriteString(g.indent() + "public Client(TProtocol protocol) : this(protocol, protocol)\n")
	out.WriteString(g.indent() + "{\n")
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "public Client(TProtocol inputProtocol, TProtocol outputProtocol) : base(inputProtocol, outputProtocol)\n")
	out.WriteString(g.indent() + "{\n")
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")

	for _, fn := range s.Functions() {
		rawFuncName := fn.Name()
		functionName := rawFuncName
		if g.opts.AsyncPostfix {
			functionName += "Async"
		}

		// async
		g.generateDeprecationAttribute(out, fn.Annotations())
		out.WriteString(g.indent() + "public async " + g.functionSignatureAsync(fn, "", modeFullDecl) + "\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		out.WriteString(g.indent() + "await send_" + functionName + "(")
		callArgs := g.argumentList(fn.Arglist(), false)
		if callArgs != "" {
			out.WriteString(callArgs + ", ")
		}
		out.WriteString(cancellationTokenName + ");\n")
		if !fn.IsOneway() {
			ret := "return "
			if fn.ReturnType().IsVoid() {
				ret = ""
			}
			out.WriteString(g.indent() + ret + "await recv_" + functionName + "(" + cancellationTokenName + ");\n")
		}
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		// async send
		g.generateDeprecationAttribute(out, fn.Annotations())
		out.WriteString(g.indent() + "public async " + g.functionSignatureAsync(fn, "send_", modeNoReturn) + "\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()

		tmpvar := g.tmp("tmp")
		argsname := fn.Name() + "_args"

		msgType := "Call"
		if fn.IsOneway() {
			msgType = "Oneway"
		}
		out.WriteString(g.indent() + "await OutputProtocol.WriteMessageBeginAsync(new TMessage(\"" + rawFuncName +
			"\", TMessageType." + msgType + ", SeqId), " + cancellationTokenName + ");\n")
		out.WriteString(g.indent() + "\n")
		out.WriteString(g.indent() + "var " + tmpvar + " = new InternalStructs." + argsname + "() {\n")
		g.indentUp()

		argStruct := fn.Arglist()
		g.collectExtensionsTypesStruct(argStruct)
		g.prepareMemberNameMappingStruct(argStruct)
		fields := argStruct.Members()

		for _, fld := range fields {
			out.WriteString(g.indent() + g.propName(fld, false) + " = " + g.normalizeName(fld.Name(), true) + ",\n")
		}

		g.indentDown()
		out.WriteString(g.indent() + "};\n")

		out.WriteString(g.indent() + "\n")
		out.WriteString(g.indent() + "await " + tmpvar + ".WriteAsync(OutputProtocol, " + cancellationTokenName + ");\n")
		out.WriteString(g.indent() + "await OutputProtocol.WriteMessageEndAsync(" + cancellationTokenName + ");\n")
		out.WriteString(g.indent() + "await OutputProtocol.Transport.FlushAsync(" + cancellationTokenName + ");\n")

		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		if !fn.IsOneway() {
			// async recv
			g.generateDeprecationAttribute(out, fn.Annotations())
			out.WriteString(g.indent() + "public async " + g.functionSignatureAsync(fn, "recv_", modeNoArgs) + "\n")
			out.WriteString(g.indent() + "{\n")
			g.indentUp()

			resultname := fn.Name() + "_result"
			xs := fn.Xceptions()
			g.collectExtensionsTypesStruct(xs)
			g.prepareMemberNameMappingFields(xs.Members(), resultname)

			tmpvar = g.tmp("tmp")
			out.WriteString(g.indent() + "\n")
			out.WriteString(g.indent() + "var " + tmpvar + " = await InputProtocol.ReadMessageBeginAsync(" + cancellationTokenName + ");\n")
			out.WriteString(g.indent() + "if (" + tmpvar + ".Type == TMessageType.Exception)\n")
			out.WriteString(g.indent() + "{\n")
			g.indentUp()

			tmpvar = g.tmp("tmp")
			out.WriteString(g.indent() + "var " + tmpvar + " = await TApplicationException.ReadAsync(InputProtocol, " + cancellationTokenName + ");\n")
			out.WriteString(g.indent() + "await InputProtocol.ReadMessageEndAsync(" + cancellationTokenName + ");\n")
			out.WriteString(g.indent() + "throw " + tmpvar + ";\n")
			g.indentDown()

			tmpvar = g.tmp("tmp")
			out.WriteString(g.indent() + "}\n")
			out.WriteString("\n")
			out.WriteString(g.indent() + "var " + tmpvar + " = new InternalStructs." + resultname + "();\n")
			out.WriteString(g.indent() + "await " + tmpvar + ".ReadAsync(InputProtocol, " + cancellationTokenName + ");\n")
			out.WriteString(g.indent() + "await InputProtocol.ReadMessageEndAsync(" + cancellationTokenName + ");\n")

			if !fn.ReturnType().IsVoid() {
				out.WriteString(g.indent() + "if (" + tmpvar + ".__isset.success)\n")
				out.WriteString(g.indent() + "{\n")
				g.indentUp()
				nullableValue := g.nullableValueAccess(fn.ReturnType())
				out.WriteString(g.indent() + "return " + tmpvar + ".Success" + nullableValue + ";\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
			}

			for _, x := range xs.Members() {
				out.WriteString(g.indent() + "if (" + tmpvar + ".__isset." + getIssetName(g.normalizeName(x.Name(), false)) + ")\n")
				out.WriteString(g.indent() + "{\n")
				g.indentUp()
				out.WriteString(g.indent() + "throw " + tmpvar + "." + g.propName(x, false) + g.nullableValueAccess(x.Type()) + ";\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
			}

			if !fn.ReturnType().IsVoid() {
				out.WriteString(g.indent() + "throw new TApplicationException(TApplicationException.ExceptionType.MissingResult, \"" +
					functionName + " failed: unknown result\");\n")
			}

			g.cleanupMemberNameMapping()
			g.indentDown()
			out.WriteString(g.indent() + "}\n\n")
		}

		g.cleanupMemberNameMapping()
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	g.cleanupMemberNameMapping()
}

// generateServiceServer is generate_service_server.
func (g *Generator) generateServiceServer(out *strings.Builder, s *sema.Service) {
	functions := s.Functions()

	extends := ""
	extendsProcessor := ""
	if s.Extends() != nil {
		extends = g.tn(s.Extends())
		extendsProcessor = extends + ".AsyncProcessor, "
	}

	g.prepareMemberNameMappingService(s)
	out.WriteString(g.indent() + "public class AsyncProcessor : " + extendsProcessor + "ITAsyncProcessor\n")
	out.WriteString(g.indent() + "{\n")

	g.indentUp()

	out.WriteString(g.indent() + "private readonly IAsync _iAsync;\n")
	out.WriteString(g.indent() + "private readonly ILogger<AsyncProcessor>" + g.nullableSuffix() + " _logger;\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "public AsyncProcessor(IAsync iAsync, ILogger<AsyncProcessor>" + g.nullableSuffix() + " logger = default)")

	if extends != "" {
		out.WriteString(" : base(iAsync)")
	}

	out.WriteString("\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	out.WriteString(g.indent() + "_iAsync = iAsync ?? throw new ArgumentNullException(nameof(iAsync));\n")
	out.WriteString(g.indent() + "_logger = logger;\n")
	for _, f := range functions {
		rawFuncName := f.Name()
		out.WriteString(g.indent() + "processMap_[\"" + rawFuncName + "\"] = " + rawFuncName + "_ProcessAsync;\n")
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")

	if extends == "" {
		out.WriteString(g.indent() + "protected delegate global::System.Threading.Tasks.Task ProcessFunction(int seqid, TProtocol iprot, TProtocol oprot, CancellationToken " + cancellationTokenName + ");\n")
	}

	if extends == "" {
		out.WriteString(g.indent() + "protected Dictionary<string, ProcessFunction> processMap_ = ")
		switch {
		case g.opts.TargetNetVersion >= 8:
			out.WriteString("[];\n")
		case g.opts.TargetNetVersion >= 6:
			out.WriteString("new();\n")
		default:
			out.WriteString("new Dictionary<string, ProcessFunction>();\n")
		}
	}

	out.WriteString("\n")

	if extends == "" {
		out.WriteString(g.indent() + "public async Task<bool> ProcessAsync(TProtocol iprot, TProtocol oprot)\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		out.WriteString(g.indent() + "return await ProcessAsync(iprot, oprot, CancellationToken.None);\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		out.WriteString(g.indent() + "public async Task<bool> ProcessAsync(TProtocol iprot, TProtocol oprot, CancellationToken " + cancellationTokenName + ")\n")
	} else {
		out.WriteString(g.indent() + "public new async Task<bool> ProcessAsync(TProtocol iprot, TProtocol oprot)\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
		out.WriteString(g.indent() + "return await ProcessAsync(iprot, oprot, CancellationToken.None);\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")

		out.WriteString(g.indent() + "public new async Task<bool> ProcessAsync(TProtocol iprot, TProtocol oprot, CancellationToken " + cancellationTokenName + ")\n")
	}

	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "try\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "var msg = await iprot.ReadMessageBeginAsync(" + cancellationTokenName + ");\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "processMap_.TryGetValue(msg.Name, out var fn);\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "if (fn == null)\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "await TProtocolUtil.SkipAsync(iprot, TType.Struct, " + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "await iprot.ReadMessageEndAsync(" + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "var x = new TApplicationException (TApplicationException.ExceptionType.UnknownMethod, \"Invalid method name: '\" + msg.Name + \"'\");\n")
	out.WriteString(g.indent() + "await oprot.WriteMessageBeginAsync(new TMessage(msg.Name, TMessageType.Exception, msg.SeqID), " + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "await x.WriteAsync(oprot, " + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "await oprot.WriteMessageEndAsync(" + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "await oprot.Transport.FlushAsync(" + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "return true;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "await fn(msg.SeqID, iprot, oprot, " + cancellationTokenName + ");\n")
	out.WriteString("\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "catch (IOException)\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()
	out.WriteString(g.indent() + "return false;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "return true;\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")

	for _, f := range functions {
		g.generateProcessFunctionAsync(out, s, f)
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	g.cleanupMemberNameMapping()
}

// generateFunctionHelpers is generate_function_helpers.
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

	g.collectExtensionsTypesStruct(result)
	g.generateNetstdStructDefinition(out, result, false, true, true)
}

// generateProcessFunctionAsync is generate_process_function_async.
func (g *Generator) generateProcessFunctionAsync(out *strings.Builder, s *sema.Service, f *sema.Function) {
	_ = s
	out.WriteString(g.indent() + "public async global::System.Threading.Tasks.Task " + f.Name() +
		"_ProcessAsync(int seqid, TProtocol iprot, TProtocol oprot, CancellationToken " + cancellationTokenName + ")\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	argsname := f.Name() + "_args"
	resultname := f.Name() + "_result"

	args := g.tmp("tmp")
	out.WriteString(g.indent() + "var " + args + " = new InternalStructs." + argsname + "();\n")
	out.WriteString(g.indent() + "await " + args + ".ReadAsync(iprot, " + cancellationTokenName + ");\n")
	out.WriteString(g.indent() + "await iprot.ReadMessageEndAsync(" + cancellationTokenName + ");\n")

	tmpResult := g.tmp("tmp")
	if !f.IsOneway() {
		out.WriteString(g.indent() + "var " + tmpResult + " = new InternalStructs." + resultname + "();\n")
	}

	out.WriteString(g.indent() + "try\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	xs := f.Xceptions()
	xceptions := xs.Members()

	if len(xceptions) > 0 {
		out.WriteString(g.indent() + "try\n")
		out.WriteString(g.indent() + "{\n")
		g.indentUp()
	}

	argStruct := f.Arglist()
	fields := argStruct.Members()

	if isDeprecated(f.Annotations()) {
		out.WriteString(g.indent() + "#pragma warning disable CS0618,CS0612\n")
	}

	out.WriteString(g.indent())
	if !f.IsOneway() && !f.ReturnType().IsVoid() {
		out.WriteString(tmpResult + ".Success = ")
	}

	methodName := g.normalizeName(f.Name(), false)
	if g.opts.AsyncPostfix {
		methodName += "Async"
	}
	out.WriteString("await _iAsync." + g.funcName(methodName, false) + "(")

	first := true
	g.collectExtensionsTypesStruct(argStruct)
	g.prepareMemberNameMappingStruct(argStruct)
	for _, fl := range fields {
		if first {
			first = false
		} else {
			out.WriteString(", ")
		}

		out.WriteString(args + "." + g.propName(fl, false))
	}

	g.cleanupMemberNameMapping()

	if !first {
		out.WriteString(", ")
	}

	out.WriteString("" + cancellationTokenName + ");\n")

	if isDeprecated(f.Annotations()) {
		out.WriteString(g.indent() + "#pragma warning restore CS0618,CS0612\n")
	}

	g.collectExtensionsTypesStruct(xs)
	g.prepareMemberNameMappingFields(xs.Members(), resultname)
	if len(xceptions) > 0 {
		g.indentDown()
		out.WriteString(g.indent() + "}\n")

		for _, x := range xceptions {
			tmpex := g.tmp("tmp")
			out.WriteString(g.indent() + "catch (" + g.tn(x.Type()) + " " + tmpex + ")\n")
			out.WriteString(g.indent() + "{\n")

			if !f.IsOneway() {
				g.indentUp()
				out.WriteString(g.indent() + tmpResult + "." + g.propName(x, false) + " = " + tmpex + ";\n")
				g.indentDown()
			}
			out.WriteString(g.indent() + "}\n")
		}
	}

	if !f.IsOneway() {
		out.WriteString(g.indent() + "await oprot.WriteMessageBeginAsync(new TMessage(\"" +
			f.Name() + "\", TMessageType.Reply, seqid), " + cancellationTokenName + "); \n")
		out.WriteString(g.indent() + "await " + tmpResult + ".WriteAsync(oprot, " + cancellationTokenName + ");\n")
	}
	g.indentDown()

	g.cleanupMemberNameMapping()

	tmpex := g.tmp("tmp")
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "catch (TTransportException)\n")
	out.WriteString(g.indent() + "{\n")
	out.WriteString(g.indent() + "  throw;\n")
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "catch (Exception " + tmpex + ")\n")
	out.WriteString(g.indent() + "{\n")
	g.indentUp()

	tmpvar := g.tmp("tmp")
	out.WriteString(g.indent() + "var " + tmpvar + " = $\"Error occurred in {GetType().FullName}: {" + tmpex + ".Message}\";\n")
	out.WriteString(g.indent() + "if ((_logger != null) && _logger.IsEnabled(LogLevel.Error))\n")
	g.indentUp()
	out.WriteString(g.indent() + "_logger.LogError(\"{Exception}, {Message}\", " + tmpex + ", " + tmpvar + ");\n")
	g.indentDown()
	out.WriteString(g.indent() + "else\n")
	g.indentUp()
	out.WriteString(g.indent() + "Console.Error.WriteLine(" + tmpvar + ");\n")
	g.indentDown()

	if f.IsOneway() {
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	} else {
		tmpvar = g.tmp("tmp")
		out.WriteString(g.indent() + "var " + tmpvar + " = new TApplicationException(TApplicationException.ExceptionType.InternalError,\" Internal error.\");\n")
		out.WriteString(g.indent() + "await oprot.WriteMessageBeginAsync(new TMessage(\"" + f.Name() +
			"\", TMessageType.Exception, seqid), " + cancellationTokenName + ");\n")
		out.WriteString(g.indent() + "await " + tmpvar + ".WriteAsync(oprot, " + cancellationTokenName + ");\n")
		g.indentDown()

		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "await oprot.WriteMessageEndAsync(" + cancellationTokenName + ");\n")
		out.WriteString(g.indent() + "await oprot.Transport.FlushAsync(" + cancellationTokenName + ");\n")
	}

	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
}
