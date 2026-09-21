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
	"os"
	"strconv"
	"strings"

	"github.com/apache/thrift/compiler/go/generate/internal/emit"
	"github.com/apache/thrift/compiler/go/sema"
)

// generateServiceRemote generates a command line tool for making remote
// requests against the service.
func (g *Generator) generateServiceRemote(s *sema.Service) {
	functions := append([]*sema.Function{}, s.Functions()...)
	// Get all functions from parents.
	for parent := s.Extends(); parent != nil; parent = parent.Extends() {
		functions = append(functions, parent.Functions()...)
	}

	fRemoteName := g.packageDir + "/" + g.serviceName + "-remote"

	var out strings.Builder

	out.WriteString("#!/usr/bin/env python\n" +
		g.pyAutogenComment() + "\n" +
		"import sys\n" +
		"import pprint\n" +
		"if sys.version_info[0] > 2:\n" +
		indentStr + "from urllib.parse import urlparse\n" +
		"else:\n" +
		indentStr + "from urlparse import urlparse\n" +
		"from thrift.transport import TTransport, TSocket, TSSLSocket, THttpClient\n" +
		"from thrift.protocol.TBinaryProtocol import TBinaryProtocol\n" +
		"\n")

	out.WriteString("from " + g.module + " import " + maybeEscapeIdentifier(g.serviceName) + "\n" +
		"from " + g.module + ".ttypes import *\n" +
		"\n")

	out.WriteString("if len(sys.argv) <= 1 or sys.argv[1] == '--help':\n" +
		indentStr + "print('')\n" +
		indentStr + "print('Usage: ' + sys.argv[0] + ' [-h host[:port]] [-u url] [-f[ramed]] [-s[sl]] [-novalidate] [-ca_certs certs] [-keyfile keyfile] [-certfile certfile] function [arg1 [arg2...]]')\n" +
		indentStr + "print('')\n" +
		indentStr + "print('Functions:')\n")
	for _, f := range functions {
		out.WriteString(indentStr + "print('  " + f.ReturnType().Name() + " " + f.Name() + "(")
		args := f.Arglist().Members()
		first := true
		for _, a := range args {
			if first {
				first = false
			} else {
				out.WriteString(", ")
			}
			out.WriteString(a.Type().Name() + " " + a.Name())
		}
		out.WriteString(")')\n")
	}
	out.WriteString(indentStr + "print('')\n" + indentStr + "sys.exit(0)\n" + "\n")

	out.WriteString("pp = pprint.PrettyPrinter(indent=2)\n" +
		"host = 'localhost'\n" +
		"port = 9090\n" +
		"uri = ''\n" +
		"framed = False\n" +
		"ssl = False\n" +
		"validate = True\n" +
		"ca_certs = None\n" +
		"keyfile = None\n" +
		"certfile = None\n" +
		"http = False\n" +
		"argi = 1\n" +
		"\n" +
		"if sys.argv[argi] == '-h':\n" +
		indentStr + "parts = sys.argv[argi + 1].split(':')\n" +
		indentStr + "host = parts[0]\n" +
		indentStr + "if len(parts) > 1:\n" +
		indentStr + indentStr + "port = int(parts[1])\n" +
		indentStr + "argi += 2\n" +
		"\n" +
		"if sys.argv[argi] == '-u':\n" +
		indentStr + "url = urlparse(sys.argv[argi + 1])\n" +
		indentStr + "parts = url[1].split(':')\n" +
		indentStr + "host = parts[0]\n" +
		indentStr + "if len(parts) > 1:\n" +
		indentStr + indentStr + "port = int(parts[1])\n" +
		indentStr + "else:\n" +
		indentStr + indentStr + "port = 80\n" +
		indentStr + "uri = url[2]\n" +
		indentStr + "if url[4]:\n" +
		indentStr + indentStr + "uri += '?%s' % url[4]\n" +
		indentStr + "http = True\n" +
		indentStr + "argi += 2\n" +
		"\n" +
		"if sys.argv[argi] == '-f' or sys.argv[argi] == '-framed':\n" +
		indentStr + "framed = True\n" +
		indentStr + "argi += 1\n" +
		"\n" +
		"if sys.argv[argi] == '-s' or sys.argv[argi] == '-ssl':\n" +
		indentStr + "ssl = True\n" +
		indentStr + "argi += 1\n" +
		"\n" +
		"if sys.argv[argi] == '-novalidate':\n" +
		indentStr + "validate = False\n" +
		indentStr + "argi += 1\n" +
		"\n" +
		"if sys.argv[argi] == '-ca_certs':\n" +
		indentStr + "ca_certs = sys.argv[argi+1]\n" +
		indentStr + "argi += 2\n" +
		"\n" +
		"if sys.argv[argi] == '-keyfile':\n" +
		indentStr + "keyfile = sys.argv[argi+1]\n" +
		indentStr + "argi += 2\n" +
		"\n" +
		"if sys.argv[argi] == '-certfile':\n" +
		indentStr + "certfile = sys.argv[argi+1]\n" +
		indentStr + "argi += 2\n" +
		"\n" +
		"cmd = sys.argv[argi]\n" +
		"args = sys.argv[argi + 1:]\n" +
		"\n" +
		"if http:\n" +
		indentStr + "transport = THttpClient.THttpClient(host, port, uri)\n" +
		"else:\n" +
		indentStr + "if ssl:\n" +
		indentStr + indentStr + "socket = TSSLSocket.TSSLSocket(host, port, validate=validate, ca_certs=ca_certs, keyfile=keyfile, certfile=certfile)\n" +
		indentStr + "else:\n" +
		indentStr + indentStr + "socket = TSocket.TSocket(host, port)\n" +
		indentStr + "if framed:\n" +
		indentStr + indentStr + "transport = TTransport.TFramedTransport(socket)\n" +
		indentStr + "else:\n" +
		indentStr + indentStr + "transport = TTransport.TBufferedTransport(socket)\n" +
		"protocol = TBinaryProtocol(transport)\n" +
		"client = " + maybeEscapeIdentifier(g.serviceName) + ".Client(protocol)\n" +
		"transport.open()\n" +
		"\n")

	// Generate the dispatch methods.
	first := true
	for _, f := range functions {
		if first {
			first = false
		} else {
			out.WriteString("el")
		}

		args := f.Arglist().Members()
		numArgs := len(args)

		out.WriteString("if cmd == '" + f.Name() + "':\n")
		g.indentUp()
		out.WriteString(g.indent() + "if len(args) != " + strconv.Itoa(numArgs) + ":\n" +
			g.indent() + indentStr + "print('" + f.Name() + " requires " + strconv.Itoa(numArgs) + " args')\n" +
			g.indent() + indentStr + "sys.exit(1)\n" +
			g.indent() + "pp.pprint(client." + maybeEscapeIdentifier(f.Name()) + "(")
		g.indentDown()
		firstArg := true
		for i, a := range args {
			if firstArg {
				firstArg = false
			} else {
				out.WriteString(" ")
			}
			switch {
			case a.Type().IsBinary():
				out.WriteString("args[" + strconv.Itoa(i) + "].encode('utf-8'),")
			case a.Type().IsString():
				out.WriteString("args[" + strconv.Itoa(i) + "],")
			default:
				out.WriteString("eval(args[" + strconv.Itoa(i) + "]),")
			}
		}
		out.WriteString("))\n")

		out.WriteString("\n")
	}

	if len(functions) > 0 {
		out.WriteString("else:\n")
		out.WriteString(indentStr + "print('Unrecognized method %s' % cmd)\n")
		out.WriteString(indentStr + "sys.exit(1)\n")
		out.WriteString("\n")
	}

	out.WriteString("transport.close()\n")

	emit.WriteFile(fRemoteName, out.String())
	_ = os.Chmod(fRemoteName, 0o755)
}
