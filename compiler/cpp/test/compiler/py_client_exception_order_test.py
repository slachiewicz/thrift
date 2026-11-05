#!/usr/bin/env python3
#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements. See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership. The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
#
"""THRIFT-1601: a generated Python client checks the declared exceptions of a
reply before its success field, so a server that fills in both (a primitive
success defaults to 0 on servers built without THRIFT-1474) raises."""

import atexit
import importlib
import os
import re
import shutil
import subprocess
import sys
import tempfile
import unittest

ROOT_DIR = os.path.abspath(os.path.join(os.path.dirname(__file__), *[os.pardir] * 4))

# The compiler CI jobs build no Python library, so use the repository's pure-Python
# sources, copied (not symlinked, for Windows) into a package named "thrift".
LIB_DIR = tempfile.mkdtemp()
atexit.register(shutil.rmtree, LIB_DIR, ignore_errors=True)
shutil.copytree(os.path.join(ROOT_DIR, "lib", "py", "src"), os.path.join(LIB_DIR, "thrift"))
sys.path.insert(0, LIB_DIR)

from thrift.Thrift import TMessageType  # noqa: E402
from thrift.protocol import TBinaryProtocol  # noqa: E402
from thrift.transport import TTransport  # noqa: E402

IDL = """
exception Oops { 1: string why }
service Svc {
  i32 get() throws (1: Oops oops)
  void ping() throws (1: Oops oops)
}
"""


class TestPyClientExceptionOrder(unittest.TestCase):

    THRIFT_COMPILER = None

    @classmethod
    def setUpClass(cls):
        cls.tmp_dir = tempfile.mkdtemp()
        with open(os.path.join(cls.tmp_dir, "Order.thrift"), "w") as f:
            f.write(IDL)

    @classmethod
    def tearDownClass(cls):
        shutil.rmtree(cls.tmp_dir, ignore_errors=True)

    def generate(self, flavor):
        out_dir = os.path.join(self.tmp_dir, "out-" + flavor.replace(":", "-"))
        os.mkdir(out_dir)
        result = subprocess.run(
            [self.THRIFT_COMPILER, "--gen", flavor, "-o", out_dir,
             os.path.join(self.tmp_dir, "Order.thrift")],
            capture_output=True, text=True, timeout=60)
        self.assertEqual(result.returncode, 0, result.stdout + result.stderr)
        return os.path.join(out_dir, "gen-" + flavor.replace(":", "."), "Order")

    def recv_body(self, flavor, name):
        with open(os.path.join(self.generate(flavor), "Svc.py")) as f:
            source = f.read()
        match = re.search(r"    def recv_%s\(.*?(?=\n    def |\nclass )" % name,
                          source, re.S)
        self.assertIsNotNone(match)
        return match.group(0)

    def assert_exception_checked_first(self, flavor):
        body = self.recv_body(flavor, "get")
        self.assertLess(body.index("result.oops is not None"),
                        body.index("result.success is not None"), body)

    def test_twisted_checks_exception_first(self):
        self.assert_exception_checked_first("py:twisted")

    def test_tornado_checks_exception_first(self):
        self.assert_exception_checked_first("py:tornado")

    def test_sync_client_raises_when_success_and_exception_are_set(self):
        gen_dir = self.generate("py")
        sys.path.insert(0, os.path.dirname(gen_dir))
        try:
            ttypes = importlib.import_module("Order.ttypes")
            svc = importlib.import_module("Order.Svc")
        finally:
            sys.path.pop(0)

        def reply(method, result):
            out = TTransport.TMemoryBuffer()
            oprot = TBinaryProtocol.TBinaryProtocol(out)
            oprot.writeMessageBegin(method, TMessageType.REPLY, 0)
            result.write(oprot)
            oprot.writeMessageEnd()
            oprot.trans.flush()
            return TBinaryProtocol.TBinaryProtocol(TTransport.TMemoryBuffer(out.getvalue()))

        oops = ttypes.Oops(why="boom")
        client = svc.Client(reply("get", svc.get_result(success=0, oops=oops)))
        with self.assertRaises(ttypes.Oops):
            client.recv_get()

        client = svc.Client(reply("get", svc.get_result(success=7)))
        self.assertEqual(client.recv_get(), 7)

        client = svc.Client(reply("ping", svc.ping_result(oops=oops)))
        with self.assertRaises(ttypes.Oops):
            client.recv_ping()


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print("Usage: %s <path-to-thrift-compiler>" % sys.argv[0], file=sys.stderr)
        sys.exit(1)
    TestPyClientExceptionOrder.THRIFT_COMPILER = sys.argv[1]
    sys.argv = [sys.argv[0]] + sys.argv[2:]
    unittest.main()
