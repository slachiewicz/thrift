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

package delphi

import (
	"github.com/apache/thrift/compiler/go/sema"
)

// generateServiceServer is t_delphi_generator::generate_service_server.
func (g *Generator) generateServiceServer(tservice *sema.Service) {
	functions := tservice.Functions()

	extends := ""
	extendsProcessor := ""

	fullCls := g.normalizeClsnm(g.serviceName, "T") + ".TProcessorImpl"

	if tservice.Extends() != nil {
		extends = g.typeName(tservice.Extends(), true, true)
		extendsProcessor = extends + ".TProcessorImpl"
		g.ln(&g.sService, "TProcessorImpl = class("+extendsProcessor+", IProcessor)")
	} else {
		g.ln(&g.sService, "TProcessorImpl = class( TInterfacedObject, IProcessor)")
	}

	g.ln(&g.sService, "public")
	g.indentUp()
	g.ln(&g.sService, "constructor Create( iface_: Iface );")
	g.ln(&g.sService, "destructor Destroy; override;")
	g.indentDown()

	g.lnI(&g.sServiceImpl, "constructor "+fullCls+".Create( iface_: Iface );")
	g.lnI(&g.sServiceImpl, "begin")
	g.indentUpImpl()
	if tservice.Extends() != nil {
		g.lnI(&g.sServiceImpl, "inherited Create( iface_);")
	} else {
		g.lnI(&g.sServiceImpl, "inherited Create;")
	}
	g.lnI(&g.sServiceImpl, "Self.iface_ := iface_;")
	if tservice.Extends() != nil {
		g.lnI(&g.sServiceImpl, "ASSERT( processMap_ <> nil);  // inherited")
	} else {
		g.lnI(&g.sServiceImpl, "processMap_ := TThriftDictionaryImpl<string, TProcessFunction>.Create;")
	}

	for _, f := range functions {
		g.lnI(&g.sServiceImpl, "processMap_.AddOrSetValue( '"+f.Name()+"', "+f.Name()+"_Process);")
	}
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")
	g.raw(&g.sServiceImpl, "\n")

	g.lnI(&g.sServiceImpl, "destructor "+fullCls+".Destroy;")
	g.lnI(&g.sServiceImpl, "begin")
	g.indentUpImpl()
	g.lnI(&g.sServiceImpl, "inherited;")
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")
	g.raw(&g.sServiceImpl, "\n")

	g.ln(&g.sService, "private")
	g.indentUp()
	g.ln(&g.sService, "iface_: Iface;")
	g.indentDown()

	if tservice.Extends() == nil {
		g.ln(&g.sService, "protected")
		g.indentUp()
		g.ln(&g.sService, "type")
		g.indentUp()
		eventsArg := ""
		if g.opts.Events {
			eventsArg = "; const events : IRequestEvents"
		}
		g.ln(&g.sService, "TProcessFunction = reference to procedure( seqid: System.Integer; const iprot: "+
			"IProtocol; const oprot: IProtocol"+eventsArg+");")
		g.indentDown()
		g.indentDown()
		g.ln(&g.sService, "protected")
		g.indentUp()
		g.ln(&g.sService, "processMap_: IThriftDictionary<string, TProcessFunction>;")
		g.indentDown()
	}

	g.ln(&g.sService, "public")
	g.indentUp()
	if extends == "" {
		g.ln(&g.sService, "function Process( const iprot: IProtocol; const oprot: IProtocol; const "+
			"events : IProcessorEvents): System.Boolean;")
	} else {
		g.ln(&g.sService, "function Process( const iprot: IProtocol; const oprot: IProtocol; const "+
			"events : IProcessorEvents): System.Boolean; reintroduce;")
	}

	g.lnI(&g.sServiceImpl, "function "+fullCls+".Process( const iprot: IProtocol; "+
		"const oprot: IProtocol; const events "+
		": IProcessorEvents): System.Boolean;")
	g.lnI(&g.sServiceImpl, "var")
	g.indentUpImpl()
	g.lnI(&g.sServiceImpl, "msg : Thrift.Protocol.TThriftMessage;")
	g.lnI(&g.sServiceImpl, "fn : TProcessFunction;")
	g.lnI(&g.sServiceImpl, "x : TApplicationException;")
	if g.opts.Events {
		g.lnI(&g.sServiceImpl, "context : IRequestEvents;")
	}
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "begin")
	g.indentUpImpl()
	g.lnI(&g.sServiceImpl, "try")
	g.indentUpImpl()
	g.lnI(&g.sServiceImpl, "msg := iprot.ReadMessageBegin();")
	g.lnI(&g.sServiceImpl, "fn := nil;")
	g.lnI(&g.sServiceImpl, "if not processMap_.TryGetValue(msg.Name, fn)")
	g.lnI(&g.sServiceImpl, "or not Assigned(fn) then begin")
	g.indentUpImpl()
	g.lnI(&g.sServiceImpl, "TProtocolUtil.Skip(iprot, TType.Struct);")
	g.lnI(&g.sServiceImpl, "iprot.ReadMessageEnd();")
	g.lnI(&g.sServiceImpl, "x := "+
		"TApplicationExceptionUnknownMethod.Create("+
		"'Invalid method name: ''' + msg.Name + '''');")
	g.lnI(&g.sServiceImpl, "Thrift.Protocol.Init( msg, msg.Name, TMessageType.Exception, msg.SeqID);")
	g.lnI(&g.sServiceImpl, "oprot.WriteMessageBegin( msg);")
	g.lnI(&g.sServiceImpl, "x.Write(oprot);")
	g.lnI(&g.sServiceImpl, "oprot.WriteMessageEnd();")
	g.lnI(&g.sServiceImpl, "oprot.Transport.Flush();")
	g.lnI(&g.sServiceImpl, "Result := True;")
	g.lnI(&g.sServiceImpl, "Exit;")
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")
	if g.opts.Events {
		g.lnI(&g.sServiceImpl, "if events <> nil")
		g.lnI(&g.sServiceImpl, "then context := events.CreateRequestContext(msg.Name)")
		g.lnI(&g.sServiceImpl, "else context := nil;")
		g.lnI(&g.sServiceImpl, "try")
		g.indentUpImpl()
		g.lnI(&g.sServiceImpl, "fn(msg.SeqID, iprot, oprot, context);")
		g.indentDownImpl()
		g.lnI(&g.sServiceImpl, "finally")
		g.indentUpImpl()
		g.lnI(&g.sServiceImpl, "if context <> nil then begin")
		g.indentUpImpl()
		g.lnI(&g.sServiceImpl, "context.CleanupContext;")
		g.lnI(&g.sServiceImpl, "context := nil;")
		g.indentDownImpl()
		g.lnI(&g.sServiceImpl, "end;")
		g.indentDownImpl()
		g.lnI(&g.sServiceImpl, "end;")
	} else {
		g.lnI(&g.sServiceImpl, "fn(msg.SeqID, iprot, oprot);")
	}
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "except")
	g.indentUpImpl()
	g.lnI(&g.sServiceImpl, "on TTransportExceptionTimedOut do begin")
	g.indentUpImpl()
	g.lnI(&g.sServiceImpl, "Result := True;")
	g.lnI(&g.sServiceImpl, "Exit;")
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")
	g.lnI(&g.sServiceImpl, "else begin")
	g.indentUpImpl()
	g.lnI(&g.sServiceImpl, "Result := False;")
	g.lnI(&g.sServiceImpl, "Exit;")
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")
	g.lnI(&g.sServiceImpl, "Result := True;")
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")
	g.raw(&g.sServiceImpl, "\n")

	for _, f := range functions {
		g.generateProcessFunction(f)
	}

	g.indentDown()
	g.ln(&g.sService, "end;")
	g.raw(&g.sService, "\n")
}

// generateProcessFunction is t_delphi_generator::generate_process_function.
func (g *Generator) generateProcessFunction(tfunction *sema.Function) {
	funcname := tfunction.Name()
	fullCls := g.normalizeClsnm(g.serviceName, "T") + ".TProcessorImpl"

	orgArgsname := funcname + "_args"
	argsClsnm := g.normalizeClsnm(orgArgsname, "T")
	argsIntfnm := g.normalizeClsnm(orgArgsname, "I")

	orgResultname := funcname + "_result"
	resultClsnm := g.normalizeClsnm(orgResultname, "T")
	resultIntfnm := g.normalizeClsnm(orgResultname, "I")

	eventsArg := ""
	if g.opts.Events {
		eventsArg = "; const events : IRequestEvents"
	}

	g.ln(&g.sService, "procedure "+funcname+
		"_Process( seqid: System.Integer; const iprot: IProtocol; const oprot: IProtocol"+eventsArg+");")

	if tfunction.IsOneway() {
		g.lnI(&g.sServiceImpl, "// one way processor")
	} else {
		g.lnI(&g.sServiceImpl, "// both way processor")
	}

	g.lnI(&g.sServiceImpl, "procedure "+fullCls+"."+funcname+
		"_Process( seqid: System.Integer; const iprot: IProtocol; const oprot: IProtocol"+eventsArg+");")
	g.lnI(&g.sServiceImpl, "var")
	g.indentUpImpl()
	g.lnI(&g.sServiceImpl, "args: "+argsIntfnm+";")
	if !tfunction.IsOneway() {
		g.lnI(&g.sServiceImpl, "msg: Thrift.Protocol.TThriftMessage;")
		g.lnI(&g.sServiceImpl, "ret: "+resultIntfnm+";")
		g.lnI(&g.sServiceImpl, "appx : TApplicationException;")
	}

	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "begin")
	g.indentUpImpl()

	if !tfunction.IsOneway() {
		g.lnI(&g.sServiceImpl, "ret := "+resultClsnm+"Impl.Create;")
	}

	g.lnI(&g.sServiceImpl, "try")
	g.indentUpImpl()

	if g.opts.Events {
		g.lnI(&g.sServiceImpl, "if events <> nil then events.PreRead;")
	}
	g.lnI(&g.sServiceImpl, "args := "+argsClsnm+"Impl.Create;")
	g.lnI(&g.sServiceImpl, "args.Read(iprot);")
	g.lnI(&g.sServiceImpl, "iprot.ReadMessageEnd();")
	if g.opts.Events {
		g.lnI(&g.sServiceImpl, "if events <> nil then events.PostRead;")
	}

	xceptions := tfunction.Xceptions().Members()

	argStruct := tfunction.Arglist()
	fields := argStruct.Members()

	g.wrI(&g.sServiceImpl, "")
	if !tfunction.IsOneway() && !tfunction.ReturnType().IsVoid() {
		g.raw(&g.sServiceImpl, "ret.Success := ")
	}
	g.raw(&g.sServiceImpl, "iface_."+g.normalizeName(tfunction.Name(), true, false, false)+"(")
	first := true
	for _, f := range fields {
		if first {
			first = false
		} else {
			g.raw(&g.sServiceImpl, ", ")
		}
		g.raw(&g.sServiceImpl, "args."+g.propNameF(f, false, ""))
	}
	g.raw(&g.sServiceImpl, ");\n")

	for _, f := range fields {
		g.lnI(&g.sServiceImpl, "args."+g.propNameF(f, false, "")+" := "+g.emptyValue(f.Type())+";")
	}

	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "except")
	g.indentUpImpl()

	for _, x := range xceptions {
		g.lnI(&g.sServiceImpl, "on E: "+g.typeName(x.Type(), true, true)+" do begin")
		g.indentUpImpl()
		if !tfunction.IsOneway() {
			g.lnI(&g.sServiceImpl, "ret."+g.propNameF(x, false, "")+" := E.ExceptionData;")
		}
		g.indentDownImpl()
		g.lnI(&g.sServiceImpl, "end;")
	}

	g.lnI(&g.sServiceImpl, "on E: Exception do begin")
	g.indentUpImpl()
	if g.opts.Events {
		g.lnI(&g.sServiceImpl, "if events <> nil then events.UnhandledError(E);")
	}
	if !tfunction.IsOneway() {
		g.lnI(&g.sServiceImpl, "appx := TApplicationExceptionInternalError.Create(E.Message);")
		g.lnI(&g.sServiceImpl, "try")
		g.indentUpImpl()
		if g.opts.Events {
			g.lnI(&g.sServiceImpl, "if events <> nil then events.PreWrite;")
		}
		g.lnI(&g.sServiceImpl, "Thrift.Protocol.Init( msg, '"+
			funcname+"', TMessageType.Exception, seqid);")
		g.lnI(&g.sServiceImpl, "oprot.WriteMessageBegin( msg);")
		g.lnI(&g.sServiceImpl, "appx.Write(oprot);")
		g.lnI(&g.sServiceImpl, "oprot.WriteMessageEnd();")
		g.lnI(&g.sServiceImpl, "oprot.Transport.Flush();")
		if g.opts.Events {
			g.lnI(&g.sServiceImpl, "if events <> nil then events.PostWrite;")
		}
		g.lnI(&g.sServiceImpl, "Exit;")
		g.indentDownImpl()
		g.lnI(&g.sServiceImpl, "finally")
		g.indentUpImpl()
		g.lnI(&g.sServiceImpl, "appx.Free;")
		g.indentDownImpl()
		g.lnI(&g.sServiceImpl, "end;")
	}
	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")

	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")

	if !tfunction.IsOneway() {
		if g.opts.Events {
			g.lnI(&g.sServiceImpl, "if events <> nil then events.PreWrite;")
		}
		g.lnI(&g.sServiceImpl, "Thrift.Protocol.Init( msg, '"+
			funcname+"', TMessageType.Reply, seqid); ")
		g.lnI(&g.sServiceImpl, "oprot.WriteMessageBegin( msg); ")
		g.lnI(&g.sServiceImpl, "ret.Write(oprot);")
		g.lnI(&g.sServiceImpl, "oprot.WriteMessageEnd();")
		g.lnI(&g.sServiceImpl, "oprot.Transport.Flush();")
		if g.opts.Events {
			g.lnI(&g.sServiceImpl, "if events <> nil then events.PostWrite;")
		}
	} else if g.opts.Events {
		g.lnI(&g.sServiceImpl, "if events <> nil then events.OnewayComplete;")
	}

	g.indentDownImpl()
	g.lnI(&g.sServiceImpl, "end;")
	g.raw(&g.sServiceImpl, "\n")
}
