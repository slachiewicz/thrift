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
		g.indentUp()
		if index := strings.LastIndexByte(extends, '.'); index >= 0 {
			extendsIf = "\n" + g.indent() + extends[:index+1] + g.publicize(extends[index+1:]) + "\n"
		} else {
			extendsIf = "\n" + g.indent() + g.publicize(extends) + "\n"
		}
		g.indentDown()
	}
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString(g.indent() + "type " + interfaceName + " interface {" + extendsIf)
	if extendsIf == "" {
		out.WriteString("\n")
	}
	g.indentUp()
	g.generateDocstring(out, s)
	functions := s.Functions()
	if len(functions) != 0 {
		if s.HasDoc() {
			out.WriteString("\n")
		}
		for _, f := range functions {
			g.generateFunctionDocstring(out, f)
			g.generateDeprecationComment(out, f.Annotations())
			out.WriteString(g.indent() + g.functionSignatureIf(f, "", true) + "\n")
		}
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
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
	out.WriteString(g.indent() + "type " + serviceName + "Client struct {\n")
	g.indentUp()
	if extendsClient != "" {
		out.WriteString(g.indent() + "*" + extendsClient + "\n")
	} else {
		out.WriteString(g.indent() + "c    thrift.TClient\n")
		out.WriteString(g.indent() + "meta thrift.ResponseMeta\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString(g.indent() + "func New" + serviceName + "ClientFactory(t thrift.TTransport, f thrift.TProtocolFactory) *" + serviceName + "Client {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return &" + serviceName + "Client")
	if extends != "" {
		out.WriteString("{" + extendsField + ": " + extendsClientNew + "Factory(t, f)}\n")
	} else {
		g.indentUp()
		out.WriteString("{\n")
		out.WriteString(g.indent() + "c: thrift.NewTStandardClient(f.GetProtocol(t), f.GetProtocol(t)),\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString(g.indent() + "func New" + serviceName + "ClientProtocol(t thrift.TTransport, iprot thrift.TProtocol, oprot thrift.TProtocol) *" + serviceName + "Client {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return &" + serviceName + "Client")
	if extends != "" {
		out.WriteString("{" + extendsField + ": " + extendsClientNew + "Protocol(t, iprot, oprot)}\n")
	} else {
		g.indentUp()
		out.WriteString("{\n")
		out.WriteString(g.indent() + "c: thrift.NewTStandardClient(iprot, oprot),\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	g.generateDeprecationComment(out, s.Annotations())
	out.WriteString(g.indent() + "func New" + serviceName + "Client(c thrift.TClient) *" + serviceName + "Client {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return &" + serviceName + "Client{\n")
	g.indentUp()
	if extends != "" {
		out.WriteString(g.indent() + extendsField + ": " + extendsClientNew + "(c),\n")
	} else {
		out.WriteString(g.indent() + "c: c,\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	if extends == "" || len(functions) != 0 {
		out.WriteString("\n")
	}
	if extends == "" {
		out.WriteString(g.indent() + "func (p *" + serviceName + "Client) Client_() thrift.TClient {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return p.c\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
		out.WriteString(g.indent() + "func (p *" + serviceName + "Client) LastResponseMeta_() thrift.ResponseMeta {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return p.meta\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
		out.WriteString(g.indent() + "func (p *" + serviceName + "Client) SetLastResponseMeta_(meta thrift.ResponseMeta) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "p.meta = meta\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		if len(functions) != 0 {
			out.WriteString("\n")
		}
	}
	for i, f := range functions {
		fields := f.Arglist().Members()
		g.generateFunctionDocstring(out, f)
		g.generateDeprecationComment(out, f.Annotations())
		out.WriteString(g.indent() + "func (p *" + serviceName + "Client) " + g.functionSignatureIf(f, "", true) + " {\n")
		g.indentUp()
		method := f.Name()
		argsType := g.publicizeArgs(method + "_args")
		argsName := g.tmp("_args")
		out.WriteString(g.indent() + "var " + argsName + " " + argsType + "\n")
		for _, fld := range fields {
			out.WriteString(g.indent() + argsName + "." + g.publicize(fld.Name()) + " = " + variableNameToGoName(fld.Name()) + "\n")
		}
		if !f.IsOneway() {
			metaName := g.tmp("_meta")
			resultName := g.tmp("_result")
			resultType := g.publicizeArgs(method + "_result")
			out.WriteString(g.indent() + "var " + resultName + " " + resultType + "\n")
			out.WriteString(g.indent() + "var " + metaName + " thrift.ResponseMeta\n")
			out.WriteString(g.indent() + metaName + ", _err = p.Client_().Call(ctx, \"" + method + "\", &" + argsName + ", &" + resultName + ")\n")
			out.WriteString(g.indent() + "p.SetLastResponseMeta_(" + metaName + ")\n")
			out.WriteString(g.indent() + "if _err != nil {\n")
			g.indentUp()
			out.WriteString(g.indent() + "return\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
			xceptions := f.Xceptions().Members()
			if len(xceptions) != 0 {
				out.WriteString(g.indent() + "switch {\n")
				for _, x := range xceptions {
					pubname := g.publicize(x.Name())
					field := resultName + "." + pubname
					out.WriteString(g.indent() + "case " + field + " != nil:\n")
					g.indentUp()
					if !f.ReturnType().IsVoid() {
						out.WriteString(g.indent() + "return _r, " + field + "\n")
					} else {
						out.WriteString(g.indent() + "return " + field + "\n")
					}
					g.indentDown()
				}
				out.WriteString(g.indent() + "}\n\n")
			}
			if f.ReturnType().IsStruct() {
				retName := g.tmp("_ret")
				out.WriteString(g.indent() + "if " + retName + " := " + resultName + ".GetSuccess(); " + retName + " != nil {\n")
				g.indentUp()
				out.WriteString(g.indent() + "return " + retName + ", nil\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
				out.WriteString(g.indent() + "return nil, thrift.NewTApplicationException(thrift.MISSING_RESULT, \"" + method + " failed: unknown result\")\n")
			} else if !f.ReturnType().IsVoid() {
				out.WriteString(g.indent() + "return " + resultName + ".GetSuccess(), nil\n")
			} else {
				out.WriteString(g.indent() + "return nil\n")
			}
		} else {
			out.WriteString(g.indent() + "p.SetLastResponseMeta_(thrift.ResponseMeta{})\n")
			out.WriteString(g.indent() + "if _, err := p.Client_().Call(ctx, \"" + method + "\", &" + argsName + ", nil); err != nil {\n")
			g.indentUp()
			out.WriteString(g.indent() + "return err\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
			out.WriteString(g.indent() + "return nil\n")
		}
		g.indentDown()
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
		out.WriteString(g.indent() + "type " + serviceName + "Processor struct {\n")
		g.indentUp()
		out.WriteString(g.indent() + "processorMap map[string]thrift.TProcessorFunction\n")
		out.WriteString(g.indent() + "handler      " + serviceName + "\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
		out.WriteString(g.indent() + "func (p *" + serviceName + "Processor) AddToProcessorMap(key string, processor thrift.TProcessorFunction) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "p.processorMap[key] = processor\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
		out.WriteString(g.indent() + "func (p *" + serviceName + "Processor) GetProcessorFunction(key string) (processor thrift.TProcessorFunction, ok bool) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "processor, ok = p.processorMap[key]\n")
		out.WriteString(g.indent() + "return processor, ok\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
		out.WriteString(g.indent() + "func (p *" + serviceName + "Processor) ProcessorMap() map[string]thrift.TProcessorFunction {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return p.processorMap\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
		g.generateDeprecationComment(out, s.Annotations())
		out.WriteString(g.indent() + "func New" + serviceName + "Processor(handler " + serviceName + ") *" + serviceName + "Processor {\n\n")
		g.indentUp()
		out.WriteString(g.indent() + self + " := &" + serviceName + "Processor{handler: handler, processorMap: make(map[string]thrift.TProcessorFunction)}\n")
		for _, f := range functions {
			escapedFuncName := escapeString(f.Name())
			out.WriteString(g.indent() + self + ".processorMap[\"" + escapedFuncName + "\"] = &" + pServiceName + "Processor" + g.publicize(f.Name()) + "{handler: handler}\n")
		}
		x := g.tmp("x")
		out.WriteString(g.indent() + "return " + self + "\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
		out.WriteString(g.indent() + "func (p *" + serviceName + "Processor) Process(ctx context.Context, iprot, oprot thrift.TProtocol) (success bool, err thrift.TException) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "name, _, seqId, err2 := iprot.ReadMessageBegin(ctx)\n")
		out.WriteString(g.indent() + "if err2 != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return false, thrift.WrapTException(err2)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "if processor, ok := p.GetProcessorFunction(name); ok {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return processor.Process(ctx, seqId, iprot, oprot)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "iprot.Skip(ctx, thrift.STRUCT)\n")
		out.WriteString(g.indent() + "iprot.ReadMessageEnd(ctx)\n")
		out.WriteString(g.indent() + x + " := thrift.NewTApplicationException(thrift.UNKNOWN_METHOD, \"Unknown function \"+name)\n")
		out.WriteString(g.indent() + "oprot.WriteMessageBegin(ctx, name, thrift.EXCEPTION, seqId)\n")
		out.WriteString(g.indent() + x + ".Write(ctx, oprot)\n")
		out.WriteString(g.indent() + "oprot.WriteMessageEnd(ctx)\n")
		out.WriteString(g.indent() + "oprot.Flush(ctx)\n")
		out.WriteString(g.indent() + "return false, " + x + "\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	} else {
		out.WriteString(g.indent() + "type " + serviceName + "Processor struct {\n")
		g.indentUp()
		out.WriteString(g.indent() + "*" + extendsProcessor + "\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
		out.WriteString(g.indent() + "func New" + serviceName + "Processor(handler " + serviceName + ") *" + serviceName + "Processor {\n")
		g.indentUp()
		out.WriteString(g.indent() + self + " := &" + serviceName + "Processor{" + extendsProcessorNew + "(handler)}\n")
		for _, f := range functions {
			escapedFuncName := escapeString(f.Name())
			out.WriteString(g.indent() + self + ".AddToProcessorMap(\"" + escapedFuncName + "\", &" + pServiceName + "Processor" + g.publicize(f.Name()) + "{handler: handler})\n")
		}
		out.WriteString(g.indent() + "return " + self + "\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
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
	out.WriteString(g.indent() + "type " + processorName + " struct {\n")
	g.indentUp()
	out.WriteString(g.indent() + "handler " + g.publicize(s.Name()) + "\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n\n")
	out.WriteString(g.indent() + "func (p *" + processorName + ") Process(ctx context.Context, seqId int32, iprot, oprot thrift.TProtocol) (success bool, err thrift.TException) {\n")
	g.indentUp()
	writeErr := ""
	if !f.IsOneway() {
		writeErr = g.tmp("_write_err")
		out.WriteString(g.indent() + "var " + writeErr + " thrift.TException\n")
	}
	out.WriteString(g.indent() + "args := " + argsname + "{}\n")
	out.WriteString(g.indent() + "if err2 := args." + g.readMethodName + "(ctx, iprot); err2 != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "iprot.ReadMessageEnd(ctx)\n")
	if !f.IsOneway() {
		out.WriteString(g.indent() + "x := thrift.NewTApplicationException(thrift.PROTOCOL_ERROR, err2.Error())\n")
		out.WriteString(g.indent() + "oprot.WriteMessageBegin(ctx, \"" + escapeString(f.Name()) + "\", thrift.EXCEPTION, seqId)\n")
		out.WriteString(g.indent() + "x.Write(ctx, oprot)\n")
		out.WriteString(g.indent() + "oprot.WriteMessageEnd(ctx)\n")
		out.WriteString(g.indent() + "oprot.Flush(ctx)\n")
	}
	out.WriteString(g.indent() + "return false, thrift.WrapTException(err2)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "iprot.ReadMessageEnd(ctx)\n\n")
	out.WriteString(g.indent() + "tickerCancel := func() {}\n")
	if !f.IsOneway() {
		out.WriteString(g.indent() + "// Start a goroutine to do server side connectivity check.\n")
		out.WriteString(g.indent() + "if thrift.ServerConnectivityCheckInterval > 0 {\n")
		g.indentUp()
		out.WriteString(g.indent() + "var cancel context.CancelCauseFunc\n")
		out.WriteString(g.indent() + "ctx, cancel = context.WithCancelCause(ctx)\n")
		out.WriteString(g.indent() + "defer cancel(nil)\n")
		out.WriteString(g.indent() + "var tickerCtx context.Context\n")
		out.WriteString(g.indent() + "tickerCtx, tickerCancel = context.WithCancel(context.Background())\n")
		out.WriteString(g.indent() + "defer tickerCancel()\n")
		out.WriteString(g.indent() + "go func(ctx context.Context, cancel context.CancelCauseFunc) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "ticker := time.NewTicker(thrift.ServerConnectivityCheckInterval)\n")
		out.WriteString(g.indent() + "defer ticker.Stop()\n")
		out.WriteString(g.indent() + "for {\n")
		g.indentUp()
		out.WriteString(g.indent() + "select {\n")
		out.WriteString(g.indent() + "case <-ctx.Done():\n")
		g.indentUp()
		out.WriteString(g.indent() + "return\n")
		g.indentDown()
		out.WriteString(g.indent() + "case <-ticker.C:\n")
		g.indentUp()
		out.WriteString(g.indent() + "if !iprot.Transport().IsOpen() {\n")
		g.indentUp()
		out.WriteString(g.indent() + "cancel(thrift.ErrAbandonRequest)\n")
		out.WriteString(g.indent() + "return\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}(tickerCtx, cancel)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n\n")
	} else {
		out.WriteString(g.indent() + "_ = tickerCancel\n\n")
	}
	if !f.IsOneway() {
		out.WriteString(g.indent() + "result := " + resultname + "{}\n")
	}
	needReference := typeNeedReference(f.ReturnType())
	out.WriteString(g.indent() + "if ")
	if !f.IsOneway() && !f.ReturnType().IsVoid() {
		out.WriteString("retval, ")
	}
	out.WriteString("err2 := p.handler." + g.publicize(f.Name()) + "(")
	out.WriteString("ctx")
	for _, a := range f.Arglist().Members() {
		out.WriteString(", args." + g.publicize(a.Name()))
	}
	out.WriteString("); err2 != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "tickerCancel()\n")
	out.WriteString(g.indent() + "err = thrift.WrapTException(err2)\n")
	xFields := f.Xceptions().Members()
	if len(xFields) != 0 {
		out.WriteString(g.indent() + "switch v := err2.(type) {\n")
		for _, x := range xFields {
			out.WriteString(g.indent() + "case " + g.typeToGoType(x.Type()) + ":\n")
			g.indentUp()
			out.WriteString(g.indent() + "result." + g.publicize(x.Name()) + " = v\n")
			g.indentDown()
		}
		out.WriteString(g.indent() + "default:\n")
		g.indentUp()
	}
	if !f.IsOneway() {
		out.WriteString(g.indent() + "if errors.Is(err2, thrift.ErrAbandonRequest) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return false, &thrift.ProcessorError{\n")
		g.indentUp()
		out.WriteString(g.indent() + "WriteError:    thrift.WrapTException(err2),\n")
		out.WriteString(g.indent() + "EndpointError: err,\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "if errors.Is(err2, context.Canceled) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "if err3 := context.Cause(ctx); errors.Is(err3, thrift.ErrAbandonRequest) {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return false, &thrift.ProcessorError{\n")
		g.indentUp()
		out.WriteString(g.indent() + "WriteError:    thrift.WrapTException(err3),\n")
		out.WriteString(g.indent() + "EndpointError: err,\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		exc := g.tmp("_exc")
		out.WriteString(g.indent() + exc + " := thrift.NewTApplicationException(thrift.INTERNAL_ERROR, \"Internal error processing " + escapeString(f.Name()) + ": \"+err2.Error())\n")
		out.WriteString(g.indent() + "if err2 := oprot.WriteMessageBegin(ctx, \"" + escapeString(f.Name()) + "\", thrift.EXCEPTION, seqId); err2 != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + writeErr + " = thrift.WrapTException(err2)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "if err2 := " + exc + ".Write(ctx, oprot); " + writeErr + " == nil && err2 != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + writeErr + " = thrift.WrapTException(err2)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "if err2 := oprot.WriteMessageEnd(ctx); " + writeErr + " == nil && err2 != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + writeErr + " = thrift.WrapTException(err2)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "if err2 := oprot.Flush(ctx); " + writeErr + " == nil && err2 != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + writeErr + " = thrift.WrapTException(err2)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "if " + writeErr + " != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return false, &thrift.ProcessorError{\n")
		g.indentUp()
		out.WriteString(g.indent() + "WriteError:    " + writeErr + ",\n")
		out.WriteString(g.indent() + "EndpointError: err,\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "return true, err\n")
	}
	if len(xFields) != 0 {
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + "}")
	if !f.IsOneway() {
		if !f.ReturnType().IsVoid() {
			out.WriteString(" else {\n")
			g.indentUp()
			out.WriteString(g.indent() + "result.Success = ")
			if needReference {
				out.WriteString("&")
			}
			out.WriteString("retval\n")
			g.indentDown()
			out.WriteString(g.indent() + "}\n")
		} else {
			out.WriteString("\n")
		}
		out.WriteString(g.indent() + "tickerCancel()\n")
		out.WriteString(g.indent() + "if err2 := oprot.WriteMessageBegin(ctx, \"" + escapeString(f.Name()) + "\", thrift.REPLY, seqId); err2 != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + writeErr + " = thrift.WrapTException(err2)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "if err2 := result." + g.writeMethodName + "(ctx, oprot); " + writeErr + " == nil && err2 != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + writeErr + " = thrift.WrapTException(err2)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "if err2 := oprot.WriteMessageEnd(ctx); " + writeErr + " == nil && err2 != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + writeErr + " = thrift.WrapTException(err2)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "if err2 := oprot.Flush(ctx); " + writeErr + " == nil && err2 != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + writeErr + " = thrift.WrapTException(err2)\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "if " + writeErr + " != nil {\n")
		g.indentUp()
		out.WriteString(g.indent() + "return false, &thrift.ProcessorError{\n")
		g.indentUp()
		out.WriteString(g.indent() + "WriteError:    " + writeErr + ",\n")
		out.WriteString(g.indent() + "EndpointError: err,\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		out.WriteString(g.indent() + "return true, err\n")
	} else {
		out.WriteString("\n")
		out.WriteString(g.indent() + "tickerCancel()\n")
		out.WriteString(g.indent() + "return true, err\n")
	}
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
}
