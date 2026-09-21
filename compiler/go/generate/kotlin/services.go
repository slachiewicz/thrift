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

package kotlin

import (
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateService is t_kotlin_generator::generate_service.
func (g *Generator) generateService(svc *sema.Service) {
	g.generateServiceInterface(svc)
	g.generateServiceClient(svc)
	g.generateServiceProcessor(svc)
	g.generateServiceArgsHelpers(svc)
	g.generateServiceResultHelpers(svc)
}

// functionSignature is t_kotlin_generator::function_signature.
func (g *Generator) functionSignature(f *sema.Function, prefix string) string {
	result := "suspend fun " + prefix + f.Name() + "("
	first := true
	for _, field := range f.Arglist().Members() {
		if first {
			first = false
		} else {
			result += ", "
		}
		result += field.Name() + ": " + g.typeName(field.Type(), false, false, false)
	}
	result += "): "
	result += g.typeName(f.ReturnType(), false, false, false)
	return result
}

// generateServiceInterface is t_kotlin_generator::generate_service_interface.
func (g *Generator) generateServiceInterface(svc *sema.Service) {
	fServiceName := g.packageDir + "/" + svc.Name() + ".kt"
	var out strings.Builder
	out.WriteString(autogenComment() + g.kotlinPackage())
	out.WriteString("interface " + svc.Name() + " {\n")
	g.indentUp()
	for _, f := range svc.Functions() {
		g.generateKdocComment(&out, f)
		out.WriteString(g.indent() + g.functionSignature(f, "") + "\n")
	}
	g.scopeDown(&out)
	out.WriteString("\n\n")
	emit.WriteFile(fServiceName, out.String())
}

// generateServiceClient is t_kotlin_generator::generate_service_client.
func (g *Generator) generateServiceClient(svc *sema.Service) {
	fServiceName := g.packageDir + "/" + svc.Name() + "Client.kt"
	var out strings.Builder
	out.WriteString(autogenComment() + warningSuppressions() + g.kotlinPackage())
	emit.DocstringComment(&out, g.indent(), "/**\n", " * ", "client implementation for ["+svc.Name()+"]", " */\n")
	out.WriteString(g.indent() + "class " + svc.Name() + "Client(\n")
	g.indentUp()
	out.WriteString(g.indent() + "protocolFactory: org.apache.thrift.protocol.TProtocolFactory,\n")
	out.WriteString(g.indent() + "clientManager: org.apache.thrift.async.TAsyncClientManager,\n")
	out.WriteString(g.indent() + "transport: org.apache.thrift.transport.TNonblockingTransport\n")
	g.indentDown()
	out.WriteString("): org.apache.thrift.async.TAsyncClient(protocolFactory, clientManager, transport), " + svc.Name() + " {\n\n")

	g.indentUp()
	{
		out.WriteString(g.indent() + "private val seqId = java.util.concurrent.atomic.AtomicInteger()\n\n")
		for _, f := range svc.Functions() {
			out.WriteString(g.indent() + "override " + g.functionSignature(f, "") + " {\n")
			g.indentUp()
			{
				argsName := svc.Name() + "FunctionArgs." + f.Name() + "_args"
				out.WriteString(g.indent() + "val args = " + argsName + "(")
				first := true
				for _, field := range f.Arglist().Members() {
					if !first {
						out.WriteString(", ")
					}
					first = false
					out.WriteString(field.Name())
				}
				out.WriteString(")\n")
				out.WriteString(g.indent() + "return transformCallback {\n")
				g.indentUp()
				{
					out.WriteString(g.indent() + "checkReady()\n")
					out.WriteString(g.indent() + "___currentMethod = ProcessCall." + f.Name() + "Call(args, seqId.getAndIncrement(), this, ___protocolFactory, ___transport, it)\n")
					out.WriteString(g.indent() + "___manager.call(___currentMethod)\n")
				}
				g.scopeDown(&out)
			}
			g.scopeDown(&out)
		}

		out.WriteString(g.indent() + "private suspend fun <R> org.apache.thrift.async.TAsyncClient.transformCallback(action: (org.apache.thrift.async.AsyncMethodCallback<R>) -> Unit): R {\n")
		g.indentUp()
		out.WriteString(g.indent() + "val deferred = kotlinx.coroutines.CompletableDeferred<R>()\n")
		out.WriteString(g.indent() + "val callback = object : org.apache.thrift.async.AsyncMethodCallback<R> {\n")
		g.indentUp()
		out.WriteString(g.indent() + "override fun onComplete(response: R) { deferred.complete(response) }\n")
		out.WriteString(g.indent() + "override fun onError(exception: java.lang.Exception) { deferred.completeExceptionally(exception) }\n")
		g.scopeDown(&out)
		out.WriteString(g.indent() + "action(callback)\n")
		out.WriteString(g.indent() + "return deferred.await()\n")
		g.scopeDown(&out)

		out.WriteString(g.indent() + "sealed interface ProcessCall {\n")
		g.indentUp()
		for _, f := range svc.Functions() {
			g.generateClientCall(&out, svc, f)
		}
		g.scopeDown(&out)
	}
	g.scopeDown(&out)
	out.WriteString("\n\n")
	emit.WriteFile(fServiceName, out.String())
}

// generateClientCall is t_kotlin_generator::generate_client_call.
func (g *Generator) generateClientCall(out *strings.Builder, svc *sema.Service, f *sema.Function) {
	funname := f.Name()
	funclassname := funname + "Call"
	rtype := g.typeName(f.ReturnType(), true, false, false)

	out.WriteString(g.indent() + "class " + funclassname + "(\n")
	g.indentUp()
	argsName := svc.Name() + "FunctionArgs." + f.Name() + "_args"
	out.WriteString(g.indent() + "val args: " + argsName + ",\n")
	out.WriteString(g.indent() + "val seqId: kotlin.Int,\n")
	out.WriteString(g.indent() + "client: org.apache.thrift.async.TAsyncClient,\n")
	out.WriteString(g.indent() + "protocolFactory: org.apache.thrift.protocol.TProtocolFactory,\n")
	out.WriteString(g.indent() + "transport: org.apache.thrift.transport.TNonblockingTransport,\n")
	out.WriteString(g.indent() + "resultHandler: org.apache.thrift.async.AsyncMethodCallback<" + rtype + ">,\n")
	g.indentDown()
	oneway := "false"
	if f.IsOneway() {
		oneway = "true"
	}
	out.WriteString(g.indent() + ") : org.apache.thrift.async.TAsyncMethodCall<" + rtype + ">(client, protocolFactory, transport, resultHandler, " + oneway + "), ProcessCall {\n")

	g.indentUp()
	out.WriteString(g.indent() + "override fun write_args(protocol: org.apache.thrift.protocol.TProtocol) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "val marker = org.apache.thrift.protocol.TMessage(\"" + f.Name() + "\", org.apache.thrift.protocol.TMessageType.CALL, seqId)\n")
	out.WriteString(g.indent() + "protocol.writeMessage(marker) { args.write(protocol) }\n")
	g.scopeDown(out)

	out.WriteString(g.indent() + "override fun getResult(): " + rtype + " {\n")
	g.indentUp()
	out.WriteString(g.indent() + "check(state == org.apache.thrift.async.TAsyncMethodCall.State.RESPONSE_READ) { \"Method call not finished!\" }\n")
	out.WriteString(g.indent() + "val memoryTransport = org.apache.thrift.transport.TMemoryInputTransport(frameBuffer.array())\n")
	out.WriteString(g.indent() + "val protocol = client.protocolFactory.getProtocol(memoryTransport)\n")

	if f.IsOneway() {
		out.WriteString(g.indent() + "// one way function, nothing to read\n")
	} else {
		out.WriteString(g.indent() + "return protocol.readMessage {\n")
		g.indentUp()
		{
			out.WriteString(g.indent() + "if (it.type == org.apache.thrift.protocol.TMessageType.EXCEPTION) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "val ex = org.apache.thrift.TApplicationException().apply { read(protocol) }\n")
			out.WriteString(g.indent() + "throw ex\n")
			g.scopeDown(out)
			out.WriteString(g.indent() + "if (it.seqid != seqId) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "throw org.apache.thrift.TApplicationException(\n")
			g.indentUp()
			out.WriteString(g.indent() + "org.apache.thrift.TApplicationException.BAD_SEQUENCE_ID,\n")
			out.WriteString(g.indent() + "\"" + funname + " failed: out of sequence response: expected $seqId but got ${it.seqid}\"\n")
			g.indentDown()
			out.WriteString(g.indent() + ")\n")
			g.scopeDown(out)
			resultName := svc.Name() + "FunctionResult." + f.Name() + "_result"
			out.WriteString(g.indent() + "val result = " + resultName + "().apply { read(protocol) }\n")
			for _, x := range f.Xceptions().Members() {
				out.WriteString(g.indent() + "result." + x.Name() + "?.let { throw it }\n")
			}
			if !f.ReturnType().IsVoid() {
				out.WriteString(g.indent() + "result.success ?: throw org.apache.thrift.TApplicationException(org.apache.thrift.TApplicationException.MISSING_RESULT, \"returnString failed: unknown result\")\n")
			}
		}
		g.scopeDown(out)
	}
	g.scopeDown(out)
	g.scopeDown(out)
}

// generateServiceProcessor is t_kotlin_generator::generate_service_processor.
func (g *Generator) generateServiceProcessor(svc *sema.Service) {
	fServiceName := g.packageDir + "/" + svc.Name() + "Processor.kt"
	var out strings.Builder
	out.WriteString(autogenComment() + warningSuppressions() + g.kotlinPackage())
	out.WriteString("import kotlinx.coroutines.future.future\n")
	out.WriteString("\n")

	emit.DocstringComment(&out, g.indent(), "/**\n", " * ", "server implementation for ["+svc.Name()+"]", " */\n")
	out.WriteString(g.indent() + "class " + svc.Name() + "Processor(\n")
	g.indentUp()
	out.WriteString(g.indent() + "handler: " + svc.Name() + ",\n")
	out.WriteString(g.indent() + "private val scope: kotlinx.coroutines.CoroutineScope,\n")
	out.WriteString(g.indent() + "private val processMap: kotlin.collections.Map<kotlin.String, org.apache.thrift.AsyncProcessFunction<" + svc.Name() + ", out org.apache.thrift.TBase<*, *>, out kotlin.Any, out org.apache.thrift.TBase<*, *>>> = mapOf(\n")
	g.indentUp()
	for _, f := range svc.Functions() {
		out.WriteString(g.indent() + "\"" + f.Name() + "\" to ProcessFunction." + f.Name() + "(scope),\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + ")\n")
	g.indentDown()
	out.WriteString("): org.apache.thrift.TBaseAsyncProcessor<" + svc.Name() + ">(handler, processMap) {\n")
	g.indentUp()
	out.WriteString(g.indent() + "companion object {\n")
	g.indentUp()
	out.WriteString(g.indent() + "internal val logger: org.slf4j.Logger = org.slf4j.LoggerFactory.getLogger(" + svc.Name() + "Processor::class.java)\n")
	g.scopeDown(&out)

	out.WriteString(g.indent() + "sealed interface ProcessFunction {\n")
	g.indentUp()

	for _, f := range svc.Functions() {
		g.generateServiceProcessFunction(&out, svc, f)
	}
	g.scopeDown(&out)
	g.scopeDown(&out)
	out.WriteString("\n\n")
	emit.WriteFile(fServiceName, out.String())
}

// generateServiceProcessFunction is
// t_kotlin_generator::generate_service_process_function.
func (g *Generator) generateServiceProcessFunction(out *strings.Builder, svc *sema.Service, f *sema.Function) {
	argsName := svc.Name() + "FunctionArgs." + f.Name() + "_args"
	rtype := g.typeName(f.ReturnType(), true, false, false)
	resultname := svc.Name() + "FunctionResult." + f.Name() + "_result"

	onewayResult := resultname
	if f.IsOneway() {
		onewayResult = "org.apache.thrift.TBase<*, *>"
	}
	out.WriteString(g.indent() + "class " + f.Name() + "<I : " + svc.Name() + ">(private val scope: kotlinx.coroutines.CoroutineScope) : org.apache.thrift.AsyncProcessFunction<I, " + argsName + ", " + rtype + ", " + onewayResult + ">(\"" + f.Name() + "\"), ProcessFunction {\n")
	g.indentUp()
	{
		onewayBool := "false"
		if f.IsOneway() {
			onewayBool = "true"
		}
		out.WriteString(g.indent() + "override fun isOneway() = " + onewayBool + "\n")
		out.WriteString(g.indent() + "override fun getEmptyArgsInstance() = " + argsName + "()\n")
		out.WriteString(g.indent() + "override fun getEmptyResultInstance() = ")
		if f.IsOneway() {
			out.WriteString("null\n")
		} else {
			out.WriteString(resultname + "()\n")
		}
		out.WriteString(g.indent() + "\n")
		out.WriteString(g.indent() + "override fun start(iface: I, args: " + argsName + ", resultHandler: org.apache.thrift.async.AsyncMethodCallback<" + rtype + ">) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "scope.future {\n")
		g.indentUp()
		out.WriteString(g.indent() + "iface." + f.Name() + "(")
		{
			first := true
			for _, field := range f.Arglist().Members() {
				if first {
					first = false
				} else {
					out.WriteString(", ")
				}
				out.WriteString("args." + field.Name() + "!!")
			}
		}
		out.WriteString(")\n")
		g.indentDown()
		out.WriteString(g.indent() + "}.whenComplete { r, t ->\n")
		{
			g.indentUp()
			out.WriteString(g.indent() + "if (t != null) {\n")
			g.indentUp()
			out.WriteString(g.indent() + "resultHandler.onError(t as java.lang.Exception)\n")
			g.indentDown()
			out.WriteString(g.indent() + "} else {\n")
			g.indentUp()
			out.WriteString(g.indent() + "resultHandler.onComplete(r)\n")
		}
		g.scopeDown(out)
		g.scopeDown(out)
		g.scopeDown(out)

		out.WriteString(g.indent() + "override fun getResultHandler(fb: org.apache.thrift.server.AbstractNonblockingServer.AsyncFrameBuffer, seqid: Int) =\n")
		g.indentUp()
		{
			out.WriteString(g.indent() + "object : org.apache.thrift.async.AsyncMethodCallback<" + rtype + ">{\n")
			g.indentUp()
			{
				out.WriteString(g.indent() + "override fun onComplete(response: " + rtype + ") {\n")
				g.indentUp()
				if f.IsOneway() {
					out.WriteString(g.indent() + "// one way function, no result handling\n")
				} else {
					out.WriteString(g.indent() + "val result = " + resultname + "()\n")
					if !f.ReturnType().IsVoid() {
						out.WriteString(g.indent() + "result.success = response\n")
					}
					out.WriteString(g.indent() + "try {\n")
					g.indentUp()
					out.WriteString(g.indent() + "sendResponse(fb, result, org.apache.thrift.protocol.TMessageType.REPLY, seqid)\n")
					g.indentDown()
					out.WriteString(g.indent() + "} catch (e: org.apache.thrift.transport.TTransportException) {\n")
					g.indentUp()
					out.WriteString(g.indent() + "logger.error(\"TTransportException writing to internal frame buffer\", e)\n")
					out.WriteString(g.indent() + "fb.close()\n")
					g.indentDown()
					out.WriteString(g.indent() + "} catch (e: Exception) {\n")
					g.indentUp()
					out.WriteString(g.indent() + "logger.error(\"Exception writing to internal frame buffer\", e)\n")
					out.WriteString(g.indent() + "onError(e)\n")
					g.scopeDown(out)
				}
				g.scopeDown(out)
			}
			{
				out.WriteString(g.indent() + "override fun onError(exception: kotlin.Exception) {\n")
				g.indentUp()
				if f.IsOneway() {
					out.WriteString(g.indent() + "if (exception is org.apache.thrift.transport.TTransportException) {\n")
					g.indentUp()
					out.WriteString(g.indent() + "logger.error(\"TTransportException inside handler\", exception)\n")
					out.WriteString(g.indent() + "fb.close()\n")
					g.indentDown()
					out.WriteString(g.indent() + "} else {\n")
					g.indentUp()
					out.WriteString(g.indent() + "logger.error(\"Exception inside oneway handler\", exception)\n")
					g.scopeDown(out)
				} else {
					out.WriteString(g.indent() + "val (msgType, msg) = when (exception) {\n")
					g.indentUp()

					for _, x := range f.Xceptions().Members() {
						out.WriteString(g.indent() + "is " + g.typeName(x.Type(), false, false, false) + " -> {\n")
						g.indentUp()
						out.WriteString(g.indent() + "val result = " + resultname + "()\n")
						out.WriteString(g.indent() + "result." + x.Name() + " = exception\n")
						out.WriteString(g.indent() + "org.apache.thrift.protocol.TMessageType.REPLY to result\n")
						g.scopeDown(out)
					}

					out.WriteString(g.indent() + "is org.apache.thrift.transport.TTransportException -> {\n")
					g.indentUp()
					out.WriteString(g.indent() + "logger.error(\"TTransportException inside handler\", exception)\n")
					out.WriteString(g.indent() + "fb.close()\n")
					out.WriteString(g.indent() + "return\n")
					g.scopeDown(out)

					out.WriteString(g.indent() + "is org.apache.thrift.TApplicationException -> {\n")
					g.indentUp()
					out.WriteString(g.indent() + "logger.error(\"TApplicationException inside handler\", exception)\n")
					out.WriteString(g.indent() + "org.apache.thrift.protocol.TMessageType.EXCEPTION to exception\n")
					g.scopeDown(out)

					out.WriteString(g.indent() + "else -> {\n")
					g.indentUp()
					out.WriteString(g.indent() + "logger.error(\"Exception inside handler\", exception)\n")
					out.WriteString(g.indent() + "org.apache.thrift.protocol.TMessageType.EXCEPTION to org.apache.thrift.TApplicationException(org.apache.thrift.TApplicationException.INTERNAL_ERROR, exception.message)\n")
					g.scopeDown(out)
					g.scopeDown(out)

					out.WriteString(g.indent() + "try {\n")
					g.indentUp()
					out.WriteString(g.indent() + "sendResponse(fb, msg, msgType, seqid)\n")
					g.indentDown()
					out.WriteString(g.indent() + "} catch (ex: java.lang.Exception) {\n")
					g.indentUp()
					out.WriteString(g.indent() + "logger.error(\"Exception writing to internal frame buffer\", ex)\n")
					out.WriteString(g.indent() + "fb.close()\n")
					g.scopeDown(out)
				}

				g.scopeDown(out)
			}
			g.scopeDown(out)
		}
		g.indentDown()
	}
	g.scopeDown(out)
}

// generateServiceResultHelpers is
// t_kotlin_generator::generate_service_result_helpers. It builds a
// throwaway "<func>_result" struct per non-oneway function, the same
// trick every Thrift generator's RPC layer uses, and hands it to
// generateStructDefinition.
func (g *Generator) generateServiceResultHelpers(svc *sema.Service) {
	fServiceResultName := g.packageDir + "/" + svc.Name() + "FunctionResult.kt"
	var out strings.Builder
	out.WriteString(autogenComment() + warningSuppressions() + g.kotlinPackage())

	emit.DocstringComment(&out, g.indent(), "/**\n", " * ", "function result for ["+svc.Name()+"]", " */\n")
	out.WriteString(g.indent() + "sealed interface " + svc.Name() + "FunctionResult {\n")
	g.indentUp()
	for _, f := range svc.Functions() {
		if f.IsOneway() {
			continue
		}
		result := sema.NewStruct(g.program)
		result.SetName(f.Name() + "_result")
		if !f.ReturnType().IsVoid() {
			success := sema.NewField(f.ReturnType(), "success", 0)
			result.Append(success)
		}
		for _, x := range f.Xceptions().Members() {
			result.Append(x)
		}
		g.generateStructDefinition(&out, result, false, svc.Name()+"FunctionResult")
	}
	g.scopeDown(&out)
	emit.WriteFile(fServiceResultName, out.String())
}

// generateServiceArgsHelpers is
// t_kotlin_generator::generate_service_args_helpers.
func (g *Generator) generateServiceArgsHelpers(svc *sema.Service) {
	fServiceArgsName := g.packageDir + "/" + svc.Name() + "FunctionArgs.kt"
	var out strings.Builder
	out.WriteString(autogenComment() + warningSuppressions() + g.kotlinPackage())
	emit.DocstringComment(&out, g.indent(), "/**\n", " * ", "function arguments for ["+svc.Name()+"]", " */\n")
	out.WriteString(g.indent() + "sealed interface " + svc.Name() + "FunctionArgs {\n")
	g.indentUp()
	for _, f := range svc.Functions() {
		ts := f.Arglist()
		g.generateStructDefinition(&out, ts, false, svc.Name()+"FunctionArgs")
		out.WriteString("\n")
	}
	g.scopeDown(&out)
	emit.WriteFile(fServiceArgsName, out.String())
}
