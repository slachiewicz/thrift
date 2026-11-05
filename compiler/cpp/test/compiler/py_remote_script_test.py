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
"""Tests for the <Service>-remote script of the Python generator."""

import os
import shutil
import subprocess
import sys
import tempfile
import unittest


class TestPyRemoteScript(unittest.TestCase):

    THRIFT_COMPILER = None
    SCHEMA = """
namespace py remotetest

service Svc {
  i32 add(1: i32 a, 2: i32 b)
}
"""

    def setUp(self):
        self.tmp_dir = tempfile.mkdtemp()
        self.fixture = os.path.join(self.tmp_dir, "RemoteTest.thrift")
        with open(self.fixture, "w") as f:
            f.write(self.SCHEMA)

    def tearDown(self):
        shutil.rmtree(self.tmp_dir, ignore_errors=True)

    def generate(self, options):
        out = os.path.join(self.tmp_dir, "out_" + options.replace(":", "_"))
        os.mkdir(out)
        subprocess.run(
            [self.THRIFT_COMPILER, "--gen", options, "-out", out, self.fixture],
            check=True,
            capture_output=True,
            text=True,
        )
        return os.path.join(out, "remotetest")

    def test_blocking_client_gets_remote_script(self):
        for options in ("py", "py:new_style", "py:dynamic"):
            with self.subTest(options=options):
                path = os.path.join(self.generate(options), "Svc-remote")
                self.assertTrue(os.path.isfile(path))
                with open(path) as f:
                    source = f.read()
                self.assertIn("client = Svc.Client(protocol)", source)
                compile(source, path, "exec")

    def test_twisted_and_tornado_have_no_remote_script(self):
        # Their Client takes transport and protocol factories, so the generated
        # script (Client(protocol)) can never work.
        for options in ("py:twisted", "py:tornado"):
            with self.subTest(options=options):
                package = self.generate(options)
                self.assertTrue(os.path.isfile(os.path.join(package, "Svc.py")))
                self.assertFalse(os.path.exists(os.path.join(package, "Svc-remote")))


if __name__ == "__main__":
    if len(sys.argv) < 2:
        print(f"Usage: {sys.argv[0]} <path-to-thrift-compiler>", file=sys.stderr)
        sys.exit(1)
    TestPyRemoteScript.THRIFT_COMPILER = sys.argv[1]
    sys.argv = [sys.argv[0]] + sys.argv[2:]
    unittest.main()
