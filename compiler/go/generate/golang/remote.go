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
	"os"
	"sort"
	"strings"

	"github.com/apache/thrift/compiler/go/sema"
)

// generateServiceRemote writes the <service>-remote command line client.
func (g *Generator) generateServiceRemote(s *sema.Service) {
	var functions []*sema.Function
	// Functions may come from a parent service, and when that service lives
	// in an included file its args struct is in another package, so the map
	// records the declaring service rather than only its name.
	funcToService := map[string]*sema.Service{}
	for parent := s; parent != nil; parent = parent.Extends() {
		pFunctions := parent.Functions()
		functions = append(functions, pFunctions...)
		for _, f := range pFunctions {
			if _, ok := funcToService[f.Name()]; !ok {
				funcToService[f.Name()] = parent
			}
		}
	}
	if len(functions) == 0 {
		return
	}
	remoteDir := g.packageDir + "/" + underscore(g.serviceName) + "-remote"
	mkdir(remoteDir)
	remoteName := remoteDir + "/" + underscore(g.serviceName) + "-remote.go"
	var out strings.Builder
	var unusedProtection string
	systemPackages := []string{"context", "flag", "fmt", "math", "net", "net/url", "os", "strconv", "strings"}
	thriftPackage := []string{"thrift \"" + g.opts.ThriftImport + "\""}
	systemImports := g.renderSystemPackages(systemPackages)
	thriftImport := g.renderSystemPackages(thriftPackage)
	out.WriteString(g.autogenComment())
	out.WriteString("package main\n\n")
	out.WriteString("import (\n")
	out.WriteString(systemImports)
	out.WriteString("\n" + thriftImport)
	localPrograms := map[string]*sema.Program{}
	for _, inc := range g.program.Includes() {
		if _, ok := localPrograms[g.realGoModule(inc)]; !ok {
			localPrograms[g.realGoModule(inc)] = inc
		}
	}
	// A function inherited from a service declared in a file that this
	// program does not include directly still needs that file's package for
	// its args struct.
	for ancestor := s.Extends(); ancestor != nil; ancestor = ancestor.Extends() {
		if _, ok := localPrograms[g.realGoModule(ancestor.Program())]; !ok {
			localPrograms[g.realGoModule(ancestor.Program())] = ancestor.Program()
		}
	}
	if _, ok := localPrograms[g.realGoModule(g.program)]; !ok {
		localPrograms[g.realGoModule(g.program)] = g.program
	}
	if len(localPrograms) != 0 {
		out.WriteString("\n")
		keys := make([]string, 0, len(localPrograms))
		for k := range localPrograms {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			out.WriteString(g.renderProgramImport(localPrograms[k], &unusedProtection))
		}
	}
	out.WriteString(")\n")
	out.WriteString("\n")
	out.WriteString(unusedProtection)
	out.WriteString("\n")
	out.WriteString("func Usage() {\n")
	out.WriteString("fmt.Fprintln(os.Stderr, \"Usage of \", os.Args[0], \" [-h host:port] [-u url] [-f[ramed]] function [arg1 [arg2...]]:\")\n")
	out.WriteString("flag.PrintDefaults()\n")
	out.WriteString("fmt.Fprintln(os.Stderr, \"\\nFunctions:\")\n")
	packageNameAliased := g.packageIdentifiers[g.realGoModule(g.program)]
	for _, f := range functions {
		out.WriteString("fmt.Fprintln(os.Stderr, \"  " + f.ReturnType().Name() + " " + f.Name() + "(")
		args := f.Arglist().Members()
		for i, a := range args {
			if i > 0 {
				out.WriteString(", ")
			}
			out.WriteString(a.Type().Name() + " " + a.Name())
		}
		out.WriteString(")\")\n")
	}
	out.WriteString("fmt.Fprintln(os.Stderr)\n")
	out.WriteString("os.Exit(0)\n")
	out.WriteString("}\n")
	out.WriteString("\n")
	out.WriteString("type httpHeaders map[string]string\n")
	out.WriteString("\n")
	out.WriteString("func (h httpHeaders) String() string {\n")
	out.WriteString("var m map[string]string = h\n")
	out.WriteString("return fmt.Sprintf(\"%s\", m)\n")
	out.WriteString("}\n")
	out.WriteString("\n")
	out.WriteString("func (h httpHeaders) Set(value string) error {\n")
	out.WriteString("parts := strings.Split(value, \": \")\n")
	out.WriteString("if len(parts) != 2 {\n")
	out.WriteString("return fmt.Errorf(\"header should be of format 'Key: Value'\")\n")
	out.WriteString("}\n")
	out.WriteString("h[parts[0]] = parts[1]\n")
	out.WriteString("return nil\n")
	out.WriteString("}\n")
	out.WriteString("\n")
	out.WriteString("func main() {\n")
	out.WriteString("flag.Usage = Usage\n")
	out.WriteString("var host string\n")
	out.WriteString("var port int\n")
	out.WriteString("var protocol string\n")
	out.WriteString("var urlString string\n")
	out.WriteString("var framed bool\n")
	out.WriteString("var useHttp bool\n")
	out.WriteString("headers := make(httpHeaders)\n")
	out.WriteString("var parsedUrl *url.URL\n")
	out.WriteString("var trans thrift.TTransport\n")
	out.WriteString("_ = strconv.Atoi\n")
	out.WriteString("_ = math.Abs\n")
	out.WriteString("flag.Usage = Usage\n")
	out.WriteString("flag.StringVar(&host, \"h\", \"localhost\", \"Specify host and port\")\n")
	out.WriteString("flag.IntVar(&port, \"p\", 9090, \"Specify port\")\n")
	out.WriteString("flag.StringVar(&protocol, \"P\", \"binary\", \"Specify the protocol (binary, compact, simplejson, json)\")\n")
	out.WriteString("flag.StringVar(&urlString, \"u\", \"\", \"Specify the url\")\n")
	out.WriteString("flag.BoolVar(&framed, \"framed\", false, \"Use framed transport\")\n")
	out.WriteString("flag.BoolVar(&useHttp, \"http\", false, \"Use http\")\n")
	out.WriteString("flag.Var(headers, \"H\", \"Headers to set on the http(s) request (e.g. -H \\\"Key: Value\\\")\")\n")
	out.WriteString("flag.Parse()\n")
	out.WriteString("\n")
	out.WriteString("if len(urlString) > 0 {\n")
	out.WriteString("var err error\n")
	out.WriteString("parsedUrl, err = url.Parse(urlString)\n")
	out.WriteString("if err != nil {\n")
	out.WriteString("fmt.Fprintln(os.Stderr, \"Error parsing URL: \", err)\n")
	out.WriteString("flag.Usage()\n")
	out.WriteString("}\n")
	out.WriteString("host = parsedUrl.Host\n")
	out.WriteString("useHttp = len(parsedUrl.Scheme) <= 0 || parsedUrl.Scheme == \"http\" || parsedUrl.Scheme == \"https\"\n")
	out.WriteString("} else if useHttp {\n")
	out.WriteString("_, err := url.Parse(fmt.Sprint(\"http://\", host, \":\", port))\n")
	out.WriteString("if err != nil {\n")
	out.WriteString("fmt.Fprintln(os.Stderr, \"Error parsing URL: \", err)\n")
	out.WriteString("flag.Usage()\n")
	out.WriteString("}\n")
	out.WriteString("}\n")
	out.WriteString("\n")
	out.WriteString("cmd := flag.Arg(0)\n")
	out.WriteString("var err error\n")
	out.WriteString("var cfg *thrift.TConfiguration = nil\n")
	out.WriteString("if useHttp {\n")
	out.WriteString("trans, err = thrift.NewTHttpClient(parsedUrl.String())\n")
	out.WriteString("if len(headers) > 0 {\n")
	out.WriteString("httptrans := trans.(*thrift.THttpClient)\n")
	out.WriteString("for key, value := range headers {\n")
	out.WriteString("httptrans.SetHeader(key, value)\n")
	out.WriteString("}\n")
	out.WriteString("}\n")
	out.WriteString("} else {\n")
	out.WriteString("portStr := fmt.Sprint(port)\n")
	out.WriteString("if strings.Contains(host, \":\") {\n")
	out.WriteString("host, portStr, err = net.SplitHostPort(host)\n")
	out.WriteString("if err != nil {\n")
	out.WriteString("fmt.Fprintln(os.Stderr, \"error with host:\", err)\n")
	out.WriteString("os.Exit(1)\n")
	out.WriteString("}\n")
	out.WriteString("}\n")
	out.WriteString("trans = thrift.NewTSocketConf(net.JoinHostPort(host, portStr), cfg)\n")
	out.WriteString("if err != nil {\n")
	out.WriteString("fmt.Fprintln(os.Stderr, \"error resolving address:\", err)\n")
	out.WriteString("os.Exit(1)\n")
	out.WriteString("}\n")
	out.WriteString("if framed {\n")
	out.WriteString("trans = thrift.NewTFramedTransportConf(trans, cfg)\n")
	out.WriteString("}\n")
	out.WriteString("}\n")
	out.WriteString("if err != nil {\n")
	out.WriteString("fmt.Fprintln(os.Stderr, \"Error creating transport\", err)\n")
	out.WriteString("os.Exit(1)\n")
	out.WriteString("}\n")
	out.WriteString("defer trans.Close()\n")
	out.WriteString("var protocolFactory thrift.TProtocolFactory\n")
	out.WriteString("switch protocol {\n")
	out.WriteString("case \"compact\":\n")
	out.WriteString("protocolFactory = thrift.NewTCompactProtocolFactoryConf(cfg)\n")
	out.WriteString("case \"simplejson\":\n")
	out.WriteString("protocolFactory = thrift.NewTSimpleJSONProtocolFactoryConf(cfg)\n")
	out.WriteString("case \"json\":\n")
	out.WriteString("protocolFactory = thrift.NewTJSONProtocolFactory()\n")
	out.WriteString("case \"binary\", \"\":\n")
	out.WriteString("protocolFactory = thrift.NewTBinaryProtocolFactoryConf(cfg)\n")
	out.WriteString("default:\n")
	out.WriteString("fmt.Fprintln(os.Stderr, \"Invalid protocol specified: \", protocol)\n")
	out.WriteString("Usage()\n")
	out.WriteString("os.Exit(1)\n")
	out.WriteString("}\n")
	out.WriteString("iprot := protocolFactory.GetProtocol(trans)\n")
	out.WriteString("oprot := protocolFactory.GetProtocol(trans)\n")
	out.WriteString("client := " + packageNameAliased + ".New" + g.publicize(g.serviceName) + "Client(thrift.NewTStandardClient(iprot, oprot))\n")
	out.WriteString("if err := trans.Open(); err != nil {\n")
	out.WriteString("fmt.Fprintln(os.Stderr, \"Error opening socket to \", host, \":\", port, \" \", err)\n")
	out.WriteString("os.Exit(1)\n")
	out.WriteString("}\n")
	out.WriteString("\n")
	out.WriteString("switch cmd {\n")
	for _, f := range functions {
		args := f.Arglist().Members()
		numArgs := len(args)
		funcName := f.Name()
		pubName := g.publicize(funcName)
		declaringService := funcToService[funcName]
		argumentsName := g.publicizeIn(funcName+"_args", true, declaringService.Name())
		// The args struct is generated next to the service that declares the
		// function, which is another package when that service comes from an
		// included file.
		argumentsModule := g.moduleName(declaringService)
		if argumentsModule == "" {
			argumentsModule = packageNameAliased
		}
		out.WriteString("case \"" + escapeString(funcName) + "\":\n")
		out.WriteString("if flag.NArg()-1 != " + itoa(int64(numArgs)) + " {\n")
		out.WriteString("fmt.Fprintln(os.Stderr, \"" + escapeString(pubName) + " requires " + itoa(int64(numArgs)) + " args\")\n")
		out.WriteString("flag.Usage()\n")
		out.WriteString("}\n")
		for i, a := range args {
			is := itoa(int64(i))
			flagArg := itoa(int64(i + 1))
			theType := a.Type()
			theType2 := sema.TrueType(theType)
			switch {
			case theType2.IsEnum():
				out.WriteString("tmp" + is + ", err := (strconv.Atoi(flag.Arg(" + flagArg + ")))\n")
				out.WriteString("if err != nil {\n")
				out.WriteString("Usage()\n")
				out.WriteString("return\n")
				out.WriteString("}\n")
				// An enum declared in an included file lives in that file's
				// package, not in the one this service was generated into.
				enumModule := g.moduleName(theType)
				if enumModule == "" {
					enumModule = packageNameAliased
				}
				out.WriteString("argvalue" + is + " := " + enumModule + "." + g.publicize(theType.Name()) + "(tmp" + is + ")\n")
			case theType2.IsBaseType():
				err := g.tmp("err")
				switch theType2.(*sema.BaseType).Base() {
				case sema.TypeVoid:
				case sema.TypeString:
					if theType2.IsBinary() {
						out.WriteString("argvalue" + is + " := []byte(flag.Arg(" + flagArg + "))\n")
					} else {
						out.WriteString("argvalue" + is + " := flag.Arg(" + flagArg + ")\n")
					}
				case sema.TypeBool:
					out.WriteString("argvalue" + is + " := flag.Arg(" + flagArg + ") == \"true\"\n")
				case sema.TypeI8, sema.TypeI16, sema.TypeI32:
					goType := map[sema.BaseKind]string{sema.TypeI8: "int8", sema.TypeI16: "int16", sema.TypeI32: "int32"}[theType2.(*sema.BaseType).Base()]
					out.WriteString("tmp" + is + ", " + err + " := (strconv.Atoi(flag.Arg(" + flagArg + ")))\n")
					out.WriteString("if " + err + " != nil {\n")
					out.WriteString("Usage()\n")
					out.WriteString("return\n")
					out.WriteString("}\n")
					out.WriteString("argvalue" + is + " := " + goType + "(tmp" + is + ")\n")
				case sema.TypeI64:
					out.WriteString("argvalue" + is + ", " + err + " := (strconv.ParseInt(flag.Arg(" + flagArg + "), 10, 64))\n")
					out.WriteString("if " + err + " != nil {\n")
					out.WriteString("Usage()\n")
					out.WriteString("return\n")
					out.WriteString("}\n")
				case sema.TypeDouble:
					out.WriteString("argvalue" + is + ", " + err + " := (strconv.ParseFloat(flag.Arg(" + flagArg + "), 64))\n")
					out.WriteString("if " + err + " != nil {\n")
					out.WriteString("Usage()\n")
					out.WriteString("return\n")
					out.WriteString("}\n")
				case sema.TypeUUID:
					out.WriteString("argvalue" + is + ", " + err + " := (thrift.ParseTuuid(flag.Arg(" + flagArg + ")))\n")
					out.WriteString("if " + err + " != nil {\n")
					out.WriteString("Usage()\n")
					out.WriteString("return\n")
					out.WriteString("}\n")
				default:
					throw("Invalid base type in generate_service_remote")
				}
			case theType2.IsStruct():
				arg := g.tmp("arg")
				mbTrans := g.tmp("mbTrans")
				err1 := g.tmp("err")
				factory := g.tmp("factory")
				jsProt := g.tmp("jsProt")
				err2 := g.tmp("err")
				// A typedef of a struct is an alias for it, so the constructor to
				// call is the struct's own, in the struct's package.
				structName := g.publicize(theType2.Name())
				structModule := g.moduleName(theType2)
				if structModule == "" {
					structModule = packageNameAliased
				}
				out.WriteString(arg + " := flag.Arg(" + flagArg + ")\n")
				out.WriteString(mbTrans + " := thrift.NewTMemoryBufferLen(len(" + arg + "))\n")
				out.WriteString("defer " + mbTrans + ".Close()\n")
				out.WriteString("_, " + err1 + " := " + mbTrans + ".WriteString(" + arg + ")\n")
				out.WriteString("if " + err1 + " != nil {\n")
				out.WriteString("Usage()\n")
				out.WriteString("return\n")
				out.WriteString("}\n")
				out.WriteString(factory + " := thrift.NewTJSONProtocolFactory()\n")
				out.WriteString(jsProt + " := " + factory + ".GetProtocol(" + mbTrans + ")\n")
				out.WriteString("argvalue" + is + " := " + structModule + ".New" + structName + "()\n")
				out.WriteString(err2 + " := argvalue" + is + "." + g.readMethodName + "(context.Background(), " + jsProt + ")\n")
				out.WriteString("if " + err2 + " != nil {\n")
				out.WriteString("Usage()\n")
				out.WriteString("return\n")
				out.WriteString("}\n")
			case theType2.IsContainer() || theType2.IsXception():
				arg := g.tmp("arg")
				mbTrans := g.tmp("mbTrans")
				err1 := g.tmp("err")
				factory := g.tmp("factory")
				jsProt := g.tmp("jsProt")
				err2 := g.tmp("err")
				argName := g.publicize(a.Name())
				out.WriteString(arg + " := flag.Arg(" + flagArg + ")\n")
				out.WriteString(mbTrans + " := thrift.NewTMemoryBufferLen(len(" + arg + "))\n")
				out.WriteString("defer " + mbTrans + ".Close()\n")
				out.WriteString("_, " + err1 + " := " + mbTrans + ".WriteString(" + arg + ")\n")
				out.WriteString("if " + err1 + " != nil {\n")
				out.WriteString("Usage()\n")
				out.WriteString("return\n")
				out.WriteString("}\n")
				out.WriteString(factory + " := thrift.NewTJSONProtocolFactory()\n")
				out.WriteString(jsProt + " := " + factory + ".GetProtocol(" + mbTrans + ")\n")
				out.WriteString("containerStruct" + is + " := " + argumentsModule + ".New" + argumentsName + "()\n")
				out.WriteString(err2 + " := containerStruct" + is + ".ReadField" + itoa(int64(i+1)) + "(context.Background(), " + jsProt + ")\n")
				out.WriteString("if " + err2 + " != nil {\n")
				out.WriteString("Usage()\n")
				out.WriteString("return\n")
				out.WriteString("}\n")
				out.WriteString("argvalue" + is + " := containerStruct" + is + "." + argName + "\n")
			default:
				throw("Invalid argument type in generate_service_remote")
			}
			if theType.IsTypedef() && !theType2.IsStruct() {
				// A typedef of a struct needs no conversion: it is generated as a Go
				// alias, so the value already has the right type.
				typedefModule := g.moduleName(theType)
				if typedefModule == "" {
					typedefModule = packageNameAliased
				}
				out.WriteString("value" + is + " := " + typedefModule + "." + g.publicize(theType.Name()) + "(argvalue" + is + ")\n")
			} else {
				out.WriteString("value" + is + " := argvalue" + is + "\n")
			}
		}
		out.WriteString("fmt.Print(client." + pubName + "(")
		out.WriteString("context.Background()")
		for i, a := range args {
			out.WriteString(", ")
			is := itoa(int64(i))
			if a.Type().IsBaseType() && a.Type().(*sema.BaseType).Base() == sema.TypeVoid {
				continue
			}
			out.WriteString("value" + is)
		}
		out.WriteString("))\n")
		out.WriteString("fmt.Print(\"\\n\")\n")
		out.WriteString("break\n")
	}
	out.WriteString("case \"\":\n")
	out.WriteString("Usage()\n")
	out.WriteString("default:\n")
	out.WriteString("fmt.Fprintln(os.Stderr, \"Invalid function \", cmd)\n")
	out.WriteString("}\n")
	out.WriteString("}\n")
	writeFile(remoteName, out.String())
	_ = os.Chmod(remoteName, 0o755)
}
