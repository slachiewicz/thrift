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
	out.WriteString(g.indent() + "package main\n\n")
	out.WriteString(g.indent() + "import (\n")
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
	out.WriteString(g.indent() + ")\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + unusedProtection)
	out.WriteString("\n")
	out.WriteString(g.indent() + "func Usage() {\n")
	g.indentUp()
	out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"Usage of \", os.Args[0], \" [-h host:port] [-u url] [-f[ramed]] function [arg1 [arg2...]]:\")\n")
	out.WriteString(g.indent() + "flag.PrintDefaults()\n")
	out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"\\nFunctions:\")\n")
	packageNameAliased := g.packageIdentifiers[g.realGoModule(g.program)]
	for _, f := range functions {
		out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"  " + f.ReturnType().Name() + " " + f.Name() + "(")
		args := f.Arglist().Members()
		for i, a := range args {
			if i > 0 {
				out.WriteString(", ")
			}
			out.WriteString(a.Type().Name() + " " + a.Name())
		}
		out.WriteString(")\")\n")
	}
	out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr)\n")
	out.WriteString(g.indent() + "os.Exit(0)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "type httpHeaders map[string]string\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "func (h httpHeaders) String() string {\n")
	g.indentUp()
	out.WriteString(g.indent() + "var m map[string]string = h\n")
	out.WriteString(g.indent() + "return fmt.Sprintf(\"%s\", m)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "func (h httpHeaders) Set(value string) error {\n")
	g.indentUp()
	out.WriteString(g.indent() + "parts := strings.Split(value, \": \")\n")
	out.WriteString(g.indent() + "if len(parts) != 2 {\n")
	g.indentUp()
	out.WriteString(g.indent() + "return fmt.Errorf(\"header should be of format 'Key: Value'\")\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "h[parts[0]] = parts[1]\n")
	out.WriteString(g.indent() + "return nil\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "func main() {\n")
	g.indentUp()
	out.WriteString(g.indent() + "flag.Usage = Usage\n")
	out.WriteString(g.indent() + "var host string\n")
	out.WriteString(g.indent() + "var port int\n")
	out.WriteString(g.indent() + "var protocol string\n")
	out.WriteString(g.indent() + "var urlString string\n")
	out.WriteString(g.indent() + "var framed bool\n")
	out.WriteString(g.indent() + "var useHttp bool\n")
	out.WriteString(g.indent() + "headers := make(httpHeaders)\n")
	out.WriteString(g.indent() + "var parsedUrl *url.URL\n")
	out.WriteString(g.indent() + "var trans thrift.TTransport\n")
	out.WriteString(g.indent() + "_ = strconv.Atoi\n")
	out.WriteString(g.indent() + "_ = math.Abs\n")
	out.WriteString(g.indent() + "flag.Usage = Usage\n")
	out.WriteString(g.indent() + "flag.StringVar(&host, \"h\", \"localhost\", \"Specify host and port\")\n")
	out.WriteString(g.indent() + "flag.IntVar(&port, \"p\", 9090, \"Specify port\")\n")
	out.WriteString(g.indent() + "flag.StringVar(&protocol, \"P\", \"binary\", \"Specify the protocol (binary, compact, simplejson, json)\")\n")
	out.WriteString(g.indent() + "flag.StringVar(&urlString, \"u\", \"\", \"Specify the url\")\n")
	out.WriteString(g.indent() + "flag.BoolVar(&framed, \"framed\", false, \"Use framed transport\")\n")
	out.WriteString(g.indent() + "flag.BoolVar(&useHttp, \"http\", false, \"Use http\")\n")
	out.WriteString(g.indent() + "flag.Var(headers, \"H\", \"Headers to set on the http(s) request (e.g. -H \\\"Key: Value\\\")\")\n")
	out.WriteString(g.indent() + "flag.Parse()\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "if len(urlString) > 0 {\n")
	g.indentUp()
	out.WriteString(g.indent() + "var err error\n")
	out.WriteString(g.indent() + "parsedUrl, err = url.Parse(urlString)\n")
	out.WriteString(g.indent() + "if err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"Error parsing URL: \", err)\n")
	out.WriteString(g.indent() + "flag.Usage()\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "host = parsedUrl.Host\n")
	out.WriteString(g.indent() + "useHttp = len(parsedUrl.Scheme) <= 0 || parsedUrl.Scheme == \"http\" || parsedUrl.Scheme == \"https\"\n")
	g.indentDown()
	out.WriteString(g.indent() + "} else if useHttp {\n")
	g.indentUp()
	out.WriteString(g.indent() + "_, err := url.Parse(fmt.Sprint(\"http://\", host, \":\", port))\n")
	out.WriteString(g.indent() + "if err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"Error parsing URL: \", err)\n")
	out.WriteString(g.indent() + "flag.Usage()\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "cmd := flag.Arg(0)\n")
	out.WriteString(g.indent() + "var err error\n")
	out.WriteString(g.indent() + "var cfg *thrift.TConfiguration = nil\n")
	out.WriteString(g.indent() + "if useHttp {\n")
	g.indentUp()
	out.WriteString(g.indent() + "trans, err = thrift.NewTHttpClient(parsedUrl.String())\n")
	out.WriteString(g.indent() + "if len(headers) > 0 {\n")
	g.indentUp()
	out.WriteString(g.indent() + "httptrans := trans.(*thrift.THttpClient)\n")
	out.WriteString(g.indent() + "for key, value := range headers {\n")
	g.indentUp()
	out.WriteString(g.indent() + "httptrans.SetHeader(key, value)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "} else {\n")
	g.indentUp()
	out.WriteString(g.indent() + "portStr := fmt.Sprint(port)\n")
	out.WriteString(g.indent() + "if strings.Contains(host, \":\") {\n")
	g.indentUp()
	out.WriteString(g.indent() + "host, portStr, err = net.SplitHostPort(host)\n")
	out.WriteString(g.indent() + "if err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"error with host:\", err)\n")
	out.WriteString(g.indent() + "os.Exit(1)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "trans = thrift.NewTSocketConf(net.JoinHostPort(host, portStr), cfg)\n")
	out.WriteString(g.indent() + "if err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"error resolving address:\", err)\n")
	out.WriteString(g.indent() + "os.Exit(1)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "if framed {\n")
	g.indentUp()
	out.WriteString(g.indent() + "trans = thrift.NewTFramedTransportConf(trans, cfg)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "if err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"Error creating transport\", err)\n")
	out.WriteString(g.indent() + "os.Exit(1)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "defer trans.Close()\n")
	out.WriteString(g.indent() + "var protocolFactory thrift.TProtocolFactory\n")
	out.WriteString(g.indent() + "switch protocol {\n")
	out.WriteString(g.indent() + "case \"compact\":\n")
	g.indentUp()
	out.WriteString(g.indent() + "protocolFactory = thrift.NewTCompactProtocolFactoryConf(cfg)\n")
	g.indentDown()
	out.WriteString(g.indent() + "case \"simplejson\":\n")
	g.indentUp()
	out.WriteString(g.indent() + "protocolFactory = thrift.NewTSimpleJSONProtocolFactoryConf(cfg)\n")
	g.indentDown()
	out.WriteString(g.indent() + "case \"json\":\n")
	g.indentUp()
	out.WriteString(g.indent() + "protocolFactory = thrift.NewTJSONProtocolFactory()\n")
	g.indentDown()
	out.WriteString(g.indent() + "case \"binary\", \"\":\n")
	g.indentUp()
	out.WriteString(g.indent() + "protocolFactory = thrift.NewTBinaryProtocolFactoryConf(cfg)\n")
	g.indentDown()
	out.WriteString(g.indent() + "default:\n")
	g.indentUp()
	out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"Invalid protocol specified: \", protocol)\n")
	out.WriteString(g.indent() + "Usage()\n")
	out.WriteString(g.indent() + "os.Exit(1)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString(g.indent() + "iprot := protocolFactory.GetProtocol(trans)\n")
	out.WriteString(g.indent() + "oprot := protocolFactory.GetProtocol(trans)\n")
	out.WriteString(g.indent() + "client := " + packageNameAliased + ".New" + g.publicize(g.serviceName) + "Client(thrift.NewTStandardClient(iprot, oprot))\n")
	out.WriteString(g.indent() + "if err := trans.Open(); err != nil {\n")
	g.indentUp()
	out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"Error opening socket to \", host, \":\", port, \" \", err)\n")
	out.WriteString(g.indent() + "os.Exit(1)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	out.WriteString("\n")
	out.WriteString(g.indent() + "switch cmd {\n")
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
		out.WriteString(g.indent() + "case \"" + escapeString(funcName) + "\":\n")
		g.indentUp()
		out.WriteString(g.indent() + "if flag.NArg()-1 != " + itoa(int64(numArgs)) + " {\n")
		g.indentUp()
		out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"" + escapeString(pubName) + " requires " + itoa(int64(numArgs)) + " args\")\n")
		out.WriteString(g.indent() + "flag.Usage()\n")
		g.indentDown()
		out.WriteString(g.indent() + "}\n")
		for i, a := range args {
			is := itoa(int64(i))
			flagArg := itoa(int64(i + 1))
			theType := a.Type()
			theType2 := sema.TrueType(theType)
			switch {
			case theType2.IsEnum():
				out.WriteString(g.indent() + "tmp" + is + ", err := (strconv.Atoi(flag.Arg(" + flagArg + ")))\n")
				out.WriteString(g.indent() + "if err != nil {\n")
				g.indentUp()
				out.WriteString(g.indent() + "Usage()\n")
				out.WriteString(g.indent() + "return\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
				// An enum declared in an included file lives in that file's
				// package, not in the one this service was generated into.
				enumModule := g.moduleName(theType)
				if enumModule == "" {
					enumModule = packageNameAliased
				}
				out.WriteString(g.indent() + "argvalue" + is + " := " + enumModule + "." + g.publicize(theType.Name()) + "(tmp" + is + ")\n")
			case theType2.IsBaseType():
				err := g.tmp("err")
				switch theType2.(*sema.BaseType).Base() {
				case sema.TypeVoid:
				case sema.TypeString:
					if theType2.IsBinary() {
						out.WriteString(g.indent() + "argvalue" + is + " := []byte(flag.Arg(" + flagArg + "))\n")
					} else {
						out.WriteString(g.indent() + "argvalue" + is + " := flag.Arg(" + flagArg + ")\n")
					}
				case sema.TypeBool:
					out.WriteString(g.indent() + "argvalue" + is + " := flag.Arg(" + flagArg + ") == \"true\"\n")
				case sema.TypeI8, sema.TypeI16, sema.TypeI32:
					goType := map[sema.BaseKind]string{sema.TypeI8: "int8", sema.TypeI16: "int16", sema.TypeI32: "int32"}[theType2.(*sema.BaseType).Base()]
					out.WriteString(g.indent() + "tmp" + is + ", " + err + " := (strconv.Atoi(flag.Arg(" + flagArg + ")))\n")
					out.WriteString(g.indent() + "if " + err + " != nil {\n")
					g.indentUp()
					out.WriteString(g.indent() + "Usage()\n")
					out.WriteString(g.indent() + "return\n")
					g.indentDown()
					out.WriteString(g.indent() + "}\n")
					out.WriteString(g.indent() + "argvalue" + is + " := " + goType + "(tmp" + is + ")\n")
				case sema.TypeI64:
					out.WriteString(g.indent() + "argvalue" + is + ", " + err + " := (strconv.ParseInt(flag.Arg(" + flagArg + "), 10, 64))\n")
					out.WriteString(g.indent() + "if " + err + " != nil {\n")
					g.indentUp()
					out.WriteString(g.indent() + "Usage()\n")
					out.WriteString(g.indent() + "return\n")
					g.indentDown()
					out.WriteString(g.indent() + "}\n")
				case sema.TypeDouble:
					out.WriteString(g.indent() + "argvalue" + is + ", " + err + " := (strconv.ParseFloat(flag.Arg(" + flagArg + "), 64))\n")
					out.WriteString(g.indent() + "if " + err + " != nil {\n")
					g.indentUp()
					out.WriteString(g.indent() + "Usage()\n")
					out.WriteString(g.indent() + "return\n")
					g.indentDown()
					out.WriteString(g.indent() + "}\n")
				case sema.TypeUUID:
					out.WriteString(g.indent() + "argvalue" + is + ", " + err + " := (thrift.ParseTuuid(flag.Arg(" + flagArg + ")))\n")
					out.WriteString(g.indent() + "if " + err + " != nil {\n")
					g.indentUp()
					out.WriteString(g.indent() + "Usage()\n")
					out.WriteString(g.indent() + "return\n")
					g.indentDown()
					out.WriteString(g.indent() + "}\n")
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
				structName := g.publicize(theType.Name())
				structModule := g.moduleName(theType)
				if structModule == "" {
					structModule = packageNameAliased
				}
				out.WriteString(g.indent() + arg + " := flag.Arg(" + flagArg + ")\n")
				out.WriteString(g.indent() + mbTrans + " := thrift.NewTMemoryBufferLen(len(" + arg + "))\n")
				out.WriteString(g.indent() + "defer " + mbTrans + ".Close()\n")
				out.WriteString(g.indent() + "_, " + err1 + " := " + mbTrans + ".WriteString(" + arg + ")\n")
				out.WriteString(g.indent() + "if " + err1 + " != nil {\n")
				g.indentUp()
				out.WriteString(g.indent() + "Usage()\n")
				out.WriteString(g.indent() + "return\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
				out.WriteString(g.indent() + factory + " := thrift.NewTJSONProtocolFactory()\n")
				out.WriteString(g.indent() + jsProt + " := " + factory + ".GetProtocol(" + mbTrans + ")\n")
				out.WriteString(g.indent() + "argvalue" + is + " := " + structModule + ".New" + structName + "()\n")
				out.WriteString(g.indent() + err2 + " := argvalue" + is + "." + g.readMethodName + "(context.Background(), " + jsProt + ")\n")
				out.WriteString(g.indent() + "if " + err2 + " != nil {\n")
				g.indentUp()
				out.WriteString(g.indent() + "Usage()\n")
				out.WriteString(g.indent() + "return\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
			case theType2.IsContainer() || theType2.IsXception():
				arg := g.tmp("arg")
				mbTrans := g.tmp("mbTrans")
				err1 := g.tmp("err")
				factory := g.tmp("factory")
				jsProt := g.tmp("jsProt")
				err2 := g.tmp("err")
				argName := g.publicize(a.Name())
				out.WriteString(g.indent() + arg + " := flag.Arg(" + flagArg + ")\n")
				out.WriteString(g.indent() + mbTrans + " := thrift.NewTMemoryBufferLen(len(" + arg + "))\n")
				out.WriteString(g.indent() + "defer " + mbTrans + ".Close()\n")
				out.WriteString(g.indent() + "_, " + err1 + " := " + mbTrans + ".WriteString(" + arg + ")\n")
				out.WriteString(g.indent() + "if " + err1 + " != nil {\n")
				g.indentUp()
				out.WriteString(g.indent() + "Usage()\n")
				out.WriteString(g.indent() + "return\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
				out.WriteString(g.indent() + factory + " := thrift.NewTJSONProtocolFactory()\n")
				out.WriteString(g.indent() + jsProt + " := " + factory + ".GetProtocol(" + mbTrans + ")\n")
				out.WriteString(g.indent() + "containerStruct" + is + " := " + argumentsModule + ".New" + argumentsName + "()\n")
				out.WriteString(g.indent() + err2 + " := containerStruct" + is + ".ReadField" + itoa(int64(i+1)) + "(context.Background(), " + jsProt + ")\n")
				out.WriteString(g.indent() + "if " + err2 + " != nil {\n")
				g.indentUp()
				out.WriteString(g.indent() + "Usage()\n")
				out.WriteString(g.indent() + "return\n")
				g.indentDown()
				out.WriteString(g.indent() + "}\n")
				out.WriteString(g.indent() + "argvalue" + is + " := containerStruct" + is + "." + argName + "\n")
			default:
				throw("Invalid argument type in generate_service_remote")
			}
			if theType.IsTypedef() {
				typedefModule := g.moduleName(theType)
				if typedefModule == "" {
					typedefModule = packageNameAliased
				}
				out.WriteString(g.indent() + "value" + is + " := " + typedefModule + "." + g.publicize(theType.Name()) + "(argvalue" + is + ")\n")
			} else {
				out.WriteString(g.indent() + "value" + is + " := argvalue" + is + "\n")
			}
		}
		out.WriteString(g.indent() + "fmt.Print(client." + pubName + "(")
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
		out.WriteString(g.indent() + "fmt.Print(\"\\n\")\n")
		out.WriteString(g.indent() + "break\n")
		g.indentDown()
	}
	out.WriteString(g.indent() + "case \"\":\n")
	g.indentUp()
	out.WriteString(g.indent() + "Usage()\n")
	g.indentDown()
	out.WriteString(g.indent() + "default:\n")
	g.indentUp()
	out.WriteString(g.indent() + "fmt.Fprintln(os.Stderr, \"Invalid function \", cmd)\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	g.indentDown()
	out.WriteString(g.indent() + "}\n")
	writeFile(remoteName, out.String())
	_ = os.Chmod(remoteName, 0o755)
}
