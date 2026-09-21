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

package golang

import (
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

func (g *Generator) generateService(s *sema.Service) {
	g.generateServiceInterface(s)
	g.generateServiceClient(s)
	g.generateServiceServer(s)
	g.generateServiceHelpers(s)
	if !g.opts.SkipRemote {
		g.generateServiceRemote(s)
	}
}

func (g *Generator) generateServiceHelpers(s *sema.Service) {
	g.beginTypesDeclaration()
	functions := s.Functions()
	g.fTypes.WriteString("// HELPER FUNCTIONS AND STRUCTURES\n")
	if len(functions) != 0 {
		g.fTypes.WriteString("\n")
	}
	for i, f := range functions {
		if i != 0 {
			g.fTypes.WriteString("\n")
		}
		g.generateGoStructDefinition(&g.fTypes, f.Arglist(), false, false, true)
		g.generateGoFunctionHelpers(f)
	}
}

func (g *Generator) generateGoFunctionHelpers(f *sema.Function) {
	if f.IsOneway() {
		return
	}
	result := sema.NewStruct(g.program)
	result.SetName(f.Name() + "_result")
	success := sema.NewField(f.ReturnType(), "success", 0)
	success.SetReq(sema.Optional)
	if !f.ReturnType().IsVoid() {
		result.Append(success)
	}
	for _, x := range f.Xceptions().Members() {
		x.SetReq(sema.Optional)
		result.Append(x)
	}
	g.fTypes.WriteString("\n")
	g.generateGoStructDefinition(&g.fTypes, result, false, true, false)
}

// extendsNames splits the type name of a base service into the qualified
// prefix and the publicized remainder, as the C++ code does inline.
func (g *Generator) extendsNames(s *sema.Service, suffix string) (extends, qualified, qualifiedNew string) {
	if s.Extends() == nil {
		return "", "", ""
	}
	extends = g.typeName(s.Extends())
	if index := strings.LastIndexByte(extends, '.'); index >= 0 {
		qualified = extends[:index+1] + g.publicize(extends[index+1:]) + suffix
		qualifiedNew = extends[:index+1] + "New" + g.publicize(extends[index+1:]) + suffix
	} else {
		qualified = g.publicize(extends) + suffix
		qualifiedNew = "New" + qualified
	}
	return
}

func (g *Generator) generateServiceInterface(s *sema.Service) {
	g.beginTypesDeclaration()
	out := &g.fTypes
	extendsIf := ""
	serviceName := g.publicize(s.Name())
	interfaceName := serviceName
	if s.Extends() != nil {
		extends := g.typeName(s.Extends())
		if index := strings.LastIndexByte(extends, '.'); index >= 0 {
			extendsIf = "\n" + extends[:index+1] + g.publicize(extends[index+1:]) + "\n"
		} else {
			extendsIf = "\n" + g.publicize(extends) + "\n"
		}
	}
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString("type " + interfaceName + " interface {" + extendsIf)
	if extendsIf == "" {
		out.WriteString("\n")
	}
	g.generateDocstring(out, s)
	functions := s.Functions()
	if len(functions) != 0 {
		if s.HasDoc() {
			out.WriteString("\n")
		}
		for _, f := range functions {
			g.generateFunctionDocstring(out, f)
			g.generateDeprecationComment(out, f.Annotations())
			out.WriteString(g.functionSignatureIf(f, "", true) + "\n")
		}
	}
	out.WriteString("}\n")
}

func (g *Generator) generateServiceClient(s *sema.Service) {
	g.beginTypesDeclaration()
	out := &g.fTypes
	serviceName := g.publicize(s.Name())
	functions := s.Functions()
	extends, extendsClient, extendsClientNew := g.extendsNames(s, "Client")
	extendsField := extendsClient
	if i := strings.IndexByte(extendsClient, '.'); i >= 0 {
		extendsField = extendsClient[i+1:]
	}
	g.generateDocstring(out, s)
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString("type " + serviceName + "Client struct {\n")
	if extendsClient != "" {
		out.WriteString("*" + extendsClient + "\n")
	} else {
		out.WriteString("c    thrift.TClient\n")
		out.WriteString("meta thrift.ResponseMeta\n")
	}
	out.WriteString("}\n\n")
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString("func New" + serviceName + "ClientFactory(t thrift.TTransport, f thrift.TProtocolFactory) *" + serviceName + "Client {\n")
	out.WriteString("return &" + serviceName + "Client")
	if extends != "" {
		out.WriteString("{" + extendsField + ": " + extendsClientNew + "Factory(t, f)}\n")
	} else {
		out.WriteString("{\n")
		out.WriteString("c: thrift.NewTStandardClient(f.GetProtocol(t), f.GetProtocol(t)),\n")
		out.WriteString("}\n")
	}
	out.WriteString("}\n\n")
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString("func New" + serviceName + "ClientProtocol(t thrift.TTransport, iprot thrift.TProtocol, oprot thrift.TProtocol) *" + serviceName + "Client {\n")
	out.WriteString("return &" + serviceName + "Client")
	if extends != "" {
		out.WriteString("{" + extendsField + ": " + extendsClientNew + "Protocol(t, iprot, oprot)}\n")
	} else {
		out.WriteString("{\n")
		out.WriteString("c: thrift.NewTStandardClient(iprot, oprot),\n")
		out.WriteString("}\n")
	}
	out.WriteString("}\n\n")
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString("func New" + serviceName + "Client(c thrift.TClient) *" + serviceName + "Client {\n")
	out.WriteString("return &" + serviceName + "Client{\n")
	if extends != "" {
		out.WriteString(extendsField + ": " + extendsClientNew + "(c),\n")
	} else {
		out.WriteString("c: c,\n")
	}
	out.WriteString("}\n")
	out.WriteString("}\n")
	if extends == "" || len(functions) != 0 {
		out.WriteString("\n")
	}
	if extends == "" {
		out.WriteString("func (p *" + serviceName + "Client) Client_() thrift.TClient {\n")
		out.WriteString("return p.c\n")
		out.WriteString("}\n\n")
		out.WriteString("func (p *" + serviceName + "Client) LastResponseMeta_() thrift.ResponseMeta {\n")
		out.WriteString("return p.meta\n")
		out.WriteString("}\n\n")
		out.WriteString("func (p *" + serviceName + "Client) SetLastResponseMeta_(meta thrift.ResponseMeta) {\n")
		out.WriteString("p.meta = meta\n")
		out.WriteString("}\n")
		if len(functions) != 0 {
			out.WriteString("\n")
		}
	}
	for i, f := range functions {
		fields := f.Arglist().Members()
		g.generateFunctionDocstring(out, f)
		g.generateDeprecationComment(out, f.Annotations())
		out.WriteString("func (p *" + serviceName + "Client) " + g.functionSignatureIf(f, "", true) + " {\n")
		method := f.Name()
		argsType := g.publicizeArgs(method + "_args")
		argsName := g.tmp("_args")
		out.WriteString("var " + argsName + " " + argsType + "\n")
		for _, fld := range fields {
			out.WriteString(argsName + "." + g.publicize(fld.Name()) + " = " + variableNameToGoName(fld.Name()) + "\n")
		}
		if !f.IsOneway() {
			metaName := g.tmp("_meta")
			resultName := g.tmp("_result")
			resultType := g.publicizeArgs(method + "_result")
			out.WriteString("var " + resultName + " " + resultType + "\n")
			out.WriteString("var " + metaName + " thrift.ResponseMeta\n")
			out.WriteString(metaName + ", _err = p.Client_().Call(ctx, \"" + method + "\", &" + argsName + ", &" + resultName + ")\n")
			out.WriteString("p.SetLastResponseMeta_(" + metaName + ")\n")
			out.WriteString("if _err != nil {\n")
			out.WriteString("return\n")
			out.WriteString("}\n")
			xceptions := f.Xceptions().Members()
			if len(xceptions) != 0 {
				out.WriteString("switch {\n")
				for _, x := range xceptions {
					pubname := g.publicize(x.Name())
					field := resultName + "." + pubname
					out.WriteString("case " + field + " != nil:\n")
					if !f.ReturnType().IsVoid() {
						out.WriteString("return _r, " + field + "\n")
					} else {
						out.WriteString("return " + field + "\n")
					}
				}
				out.WriteString("}\n\n")
			}
			if f.ReturnType().IsStruct() {
				retName := g.tmp("_ret")
				out.WriteString("if " + retName + " := " + resultName + ".GetSuccess(); " + retName + " != nil {\n")
				out.WriteString("return " + retName + ", nil\n")
				out.WriteString("}\n")
				out.WriteString("return nil, thrift.NewTApplicationException(thrift.MISSING_RESULT, \"" + method + " failed: unknown result\")\n")
			} else if !f.ReturnType().IsVoid() {
				out.WriteString("return " + resultName + ".GetSuccess(), nil\n")
			} else {
				out.WriteString("return nil\n")
			}
		} else {
			out.WriteString("p.SetLastResponseMeta_(thrift.ResponseMeta{})\n")
			out.WriteString("if _, err := p.Client_().Call(ctx, \"" + method + "\", &" + argsName + ", nil); err != nil {\n")
			out.WriteString("return err\n")
			out.WriteString("}\n")
			out.WriteString("return nil\n")
		}
		out.WriteString("}\n")
		if i+1 != len(functions) {
			out.WriteString("\n")
		}
	}
}

func (g *Generator) generateServiceServer(s *sema.Service) {
	g.beginTypesDeclaration()
	out := &g.fTypes
	functions := s.Functions()
	serviceName := g.publicize(s.Name())
	_, extendsProcessor, extendsProcessorNew := g.extendsNames(s, "Processor")
	pServiceName := g.privatize(s.Name())
	self := g.tmp("self")
	if extendsProcessor == "" {
		g.generateDeprecationComment(out, s.Annotations())
		out.WriteString("type " + serviceName + "Processor struct {\n")
		out.WriteString("processorMap map[string]thrift.TProcessorFunction\n")
		out.WriteString("handler      " + serviceName + "\n")
		out.WriteString("}\n\n")
		out.WriteString("func (p *" + serviceName + "Processor) AddToProcessorMap(key string, processor thrift.TProcessorFunction) {\n")
		out.WriteString("p.processorMap[key] = processor\n")
		out.WriteString("}\n\n")
		out.WriteString("func (p *" + serviceName + "Processor) GetProcessorFunction(key string) (processor thrift.TProcessorFunction, ok bool) {\n")
		out.WriteString("processor, ok = p.processorMap[key]\n")
		out.WriteString("return processor, ok\n")
		out.WriteString("}\n\n")
		out.WriteString("func (p *" + serviceName + "Processor) ProcessorMap() map[string]thrift.TProcessorFunction {\n")
		out.WriteString("return p.processorMap\n")
		out.WriteString("}\n\n")
		g.generateDeprecationComment(out, s.Annotations())
		out.WriteString("func New" + serviceName + "Processor(handler " + serviceName + ") *" + serviceName + "Processor {\n\n")
		out.WriteString(self + " := &" + serviceName + "Processor{handler: handler, processorMap: make(map[string]thrift.TProcessorFunction)}\n")
		for _, f := range functions {
			escapedFuncName := escapeString(f.Name())
			out.WriteString(self + ".processorMap[\"" + escapedFuncName + "\"] = &" + pServiceName + "Processor" + g.publicize(f.Name()) + "{handler: handler}\n")
		}
		x := g.tmp("x")
		out.WriteString("return " + self + "\n")
		out.WriteString("}\n\n")
		out.WriteString("func (p *" + serviceName + "Processor) Process(ctx context.Context, iprot, oprot thrift.TProtocol) (success bool, err thrift.TException) {\n")
		out.WriteString("name, _, seqId, err2 := iprot.ReadMessageBegin(ctx)\n")
		out.WriteString("if err2 != nil {\n")
		out.WriteString("return false, thrift.WrapTException(err2)\n")
		out.WriteString("}\n")
		out.WriteString("if processor, ok := p.GetProcessorFunction(name); ok {\n")
		out.WriteString("return processor.Process(ctx, seqId, iprot, oprot)\n")
		out.WriteString("}\n")
		out.WriteString("iprot.Skip(ctx, thrift.STRUCT)\n")
		out.WriteString("iprot.ReadMessageEnd(ctx)\n")
		out.WriteString(x + " := thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, \"Unknown function \"+name)\n")
		out.WriteString("oprot.WriteMessageBegin(ctx, name, thrift.EXCEPTION, seqId)\n")
		out.WriteString(x + ".Write(ctx, oprot)\n")
		out.WriteString("oprot.WriteMessageEnd(ctx)\n")
		out.WriteString("oprot.Flush(ctx)\n")
		out.WriteString("return false, " + x + "\n")
		out.WriteString("}\n")
	} else {
		out.WriteString("type " + serviceName + "Processor struct {\n")
		out.WriteString("*" + extendsProcessor + "\n")
		out.WriteString("}\n\n")
		out.WriteString("func New" + serviceName + "Processor(handler " + serviceName + ") *" + serviceName + "Processor {\n")
		out.WriteString(self + " := &" + serviceName + "Processor{" + extendsProcessorNew + "(handler)}\n")
		for _, f := range functions {
			escapedFuncName := escapeString(f.Name())
			out.WriteString(self + ".AddToProcessorMap(\"" + escapedFuncName + "\", &" + pServiceName + "Processor" + g.publicize(f.Name()) + "{handler: handler})\n")
		}
		out.WriteString("return " + self + "\n")
		out.WriteString("}\n")
	}
	for _, f := range functions {
		out.WriteString("\n")
		g.generateProcessFunction(s, f)
	}
}

func (g *Generator) generateProcessFunction(s *sema.Service, f *sema.Function) {
	out := &g.fTypes
	processorName := g.privatize(s.Name()) + "Processor" + g.publicize(f.Name())
	argsname := g.publicizeArgs(f.Name() + "_args")
	resultname := g.publicizeArgs(f.Name() + "_result")
	out.WriteString("type " + processorName + " struct {\n")
	out.WriteString("handler " + g.publicize(s.Name()) + "\n")
	out.WriteString("}\n\n")
	out.WriteString("func (p *" + processorName + ") Process(ctx context.Context, seqId int32, iprot, oprot thrift.TProtocol) (success bool, err thrift.TException) {\n")
	writeErr := ""
	if !f.IsOneway() {
		writeErr = g.tmp("_write_err")
		out.WriteString("var " + writeErr + " thrift.TException\n")
	}
	out.WriteString("args := " + argsname + "{}\n")
	out.WriteString("if err2 := args." + g.readMethodName + "(ctx, iprot); err2 != nil {\n")
	out.WriteString("iprot.ReadMessageEnd(ctx)\n")
	if !f.IsOneway() {
		out.WriteString("x := thrift.NewTApplicationException(thrift.PROTOCOL_ERROR, err2.Error())\n")
		out.WriteString("oprot.WriteMessageBegin(ctx, \"" + escapeString(f.Name()) + "\", thrift.EXCEPTION, seqId)\n")
		out.WriteString("x.Write(ctx, oprot)\n")
		out.WriteString("oprot.WriteMessageEnd(ctx)\n")
		out.WriteString("oprot.Flush(ctx)\n")
	}
	out.WriteString("return false, thrift.WrapTException(err2)\n")
	out.WriteString("}\n")
	out.WriteString("iprot.ReadMessageEnd(ctx)\n\n")
	out.WriteString("tickerCancel := func() {}\n")
	if !f.IsOneway() {
		out.WriteString("// Start a goroutine to do server side connectivity check.\n")
		out.WriteString("if thrift.ServerConnectivityCheckInterval > 0 {\n")
		out.WriteString("var cancel context.CancelCauseFunc\n")
		out.WriteString("ctx, cancel = context.WithCancelCause(ctx)\n")
		out.WriteString("defer cancel(nil)\n")
		out.WriteString("var tickerCtx context.Context\n")
		out.WriteString("tickerCtx, tickerCancel = context.WithCancel(context.Background())\n")
		out.WriteString("defer tickerCancel()\n")
		out.WriteString("go func(ctx context.Context, cancel context.CancelCauseFunc) {\n")
		out.WriteString("ticker := time.NewTicker(thrift.ServerConnectivityCheckInterval)\n")
		out.WriteString("defer ticker.Stop()\n")
		out.WriteString("for {\n")
		out.WriteString("select {\n")
		out.WriteString("case <-ctx.Done():\n")
		out.WriteString("return\n")
		out.WriteString("case <-ticker.C:\n")
		out.WriteString("if !iprot.Transport().IsOpen() {\n")
		out.WriteString("cancel(thrift.ErrAbandonRequest)\n")
		out.WriteString("return\n")
		out.WriteString("}\n")
		out.WriteString("}\n")
		out.WriteString("}\n")
		out.WriteString("}(tickerCtx, cancel)\n")
		out.WriteString("}\n\n")
	} else {
		out.WriteString("_ = tickerCancel\n\n")
	}
	if !f.IsOneway() {
		out.WriteString("result := " + resultname + "{}\n")
	}
	needReference := typeNeedReference(f.ReturnType())
	out.WriteString("if ")
	if !f.IsOneway() && !f.ReturnType().IsVoid() {
		out.WriteString("retval, ")
	}
	out.WriteString("err2 := p.handler." + g.publicize(f.Name()) + "(")
	out.WriteString("ctx")
	for _, a := range f.Arglist().Members() {
		out.WriteString(", args." + g.publicize(a.Name()))
	}
	out.WriteString("); err2 != nil {\n")
	out.WriteString("tickerCancel()\n")
	out.WriteString("err = thrift.WrapTException(err2)\n")
	xFields := f.Xceptions().Members()
	if len(xFields) != 0 {
		out.WriteString("switch v := err2.(type) {\n")
		for _, x := range xFields {
			out.WriteString("case " + g.typeToGoType(x.Type()) + ":\n")
			out.WriteString("result." + g.publicize(x.Name()) + " = v\n")
		}
		out.WriteString("default:\n")
	}
	if !f.IsOneway() {
		out.WriteString("if errors.Is(err2, thrift.ErrAbandonRequest) {\n")
		out.WriteString("return false, &thrift.ProcessorError{\n")
		out.WriteString("WriteError:    thrift.WrapTException(err2),\n")
		out.WriteString("EndpointError: err,\n")
		out.WriteString("}\n")
		out.WriteString("}\n")
		out.WriteString("if errors.Is(err2, context.Canceled) {\n")
		out.WriteString("if err3 := context.Cause(ctx); errors.Is(err3, thrift.ErrAbandonRequest) {\n")
		out.WriteString("return false, &thrift.ProcessorError{\n")
		out.WriteString("WriteError:    thrift.WrapTException(err3),\n")
		out.WriteString("EndpointError: err,\n")
		out.WriteString("}\n")
		out.WriteString("}\n")
		out.WriteString("}\n")
		exc := g.tmp("_exc")
		out.WriteString(exc + " := thrift.NewTApplicationException(thrift.INTERNAL_ERROR, \"Internal error processing " + escapeString(f.Name()) + ": \"+err2.Error())\n")
		out.WriteString("if err2 := oprot.WriteMessageBegin(ctx, \"" + escapeString(f.Name()) + "\", thrift.EXCEPTION, seqId); err2 != nil {\n")
		out.WriteString(writeErr + " = thrift.WrapTException(err2)\n")
		out.WriteString("}\n")
		out.WriteString("if err2 := " + exc + ".Write(ctx, oprot); " + writeErr + " == nil && err2 != nil {\n")
		out.WriteString(writeErr + " = thrift.WrapTException(err2)\n")
		out.WriteString("}\n")
		out.WriteString("if err2 := oprot.WriteMessageEnd(ctx); " + writeErr + " == nil && err2 != nil {\n")
		out.WriteString(writeErr + " = thrift.WrapTException(err2)\n")
		out.WriteString("}\n")
		out.WriteString("if err2 := oprot.Flush(ctx); " + writeErr + " == nil && err2 != nil {\n")
		out.WriteString(writeErr + " = thrift.WrapTException(err2)\n")
		out.WriteString("}\n")
		out.WriteString("if " + writeErr + " != nil {\n")
		out.WriteString("return false, &thrift.ProcessorError{\n")
		out.WriteString("WriteError:    " + writeErr + ",\n")
		out.WriteString("EndpointError: err,\n")
		out.WriteString("}\n")
		out.WriteString("}\n")
		out.WriteString("return true, err\n")
	}
	if len(xFields) != 0 {
		out.WriteString("}\n")
	}
	out.WriteString("}")
	if !f.IsOneway() {
		if !f.ReturnType().IsVoid() {
			out.WriteString(" else {\n")
			out.WriteString("result.Success = ")
			if needReference {
				out.WriteString("&")
			}
			out.WriteString("retval\n")
			out.WriteString("}\n")
		} else {
			out.WriteString("\n")
		}
		out.WriteString("tickerCancel()\n")
		out.WriteString("if err2 := oprot.WriteMessageBegin(ctx, \"" + escapeString(f.Name()) + "\", thrift.REPLY, seqId); err2 != nil {\n")
		out.WriteString(writeErr + " = thrift.WrapTException(err2)\n")
		out.WriteString("}\n")
		out.WriteString("if err2 := result." + g.writeMethodName + "(ctx, oprot); " + writeErr + " == nil && err2 != nil {\n")
		out.WriteString(writeErr + " = thrift.WrapTException(err2)\n")
		out.WriteString("}\n")
		out.WriteString("if err2 := oprot.WriteMessageEnd(ctx); " + writeErr + " == nil && err2 != nil {\n")
		out.WriteString(writeErr + " = thrift.WrapTException(err2)\n")
		out.WriteString("}\n")
		out.WriteString("if err2 := oprot.Flush(ctx); " + writeErr + " == nil && err2 != nil {\n")
		out.WriteString(writeErr + " = thrift.WrapTException(err2)\n")
		out.WriteString("}\n")
		out.WriteString("if " + writeErr + " != nil {\n")
		out.WriteString("return false, &thrift.ProcessorError{\n")
		out.WriteString("WriteError:    " + writeErr + ",\n")
		out.WriteString("EndpointError: err,\n")
		out.WriteString("}\n")
		out.WriteString("}\n")
		out.WriteString("return true, err\n")
	} else {
		out.WriteString("\n")
		out.WriteString("tickerCancel()\n")
		out.WriteString("return true, err\n")
	}
	out.WriteString("}\n")
}
