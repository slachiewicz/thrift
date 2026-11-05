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
# THttpClient keeps the HTTP connection open between flush() calls, so a
# server that supports keep-alive sees one TCP connection for many calls.
#

import os
import sys
import threading
import unittest
from http.server import BaseHTTPRequestHandler, HTTPServer

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import _import_local_thrift  # noqa
from thrift.transport import THttpClient  # noqa


class Handler(BaseHTTPRequestHandler):
    protocol_version = 'HTTP/1.1'

    def do_POST(self):
        self.rfile.read(int(self.headers['Content-Length']))
        body = b'reply'
        close = self.server.close_after_reply
        self.send_response(200)
        self.send_header('Content-Length', str(len(body)))
        if close:
            self.send_header('Connection', 'close')
        self.end_headers()
        self.wfile.write(body)

    def log_message(self, *args):
        pass


class CountingServer(HTTPServer):
    close_after_reply = False
    connections = 0

    def get_request(self):
        request = HTTPServer.get_request(self)
        self.connections += 1
        return request


class THttpClientKeepAliveTest(unittest.TestCase):
    def setUp(self):
        self.server = CountingServer(('127.0.0.1', 0), Handler)
        self.thread = threading.Thread(target=self.server.serve_forever)
        self.thread.daemon = True
        self.thread.start()
        self.client = THttpClient.THttpClient(
            'http://127.0.0.1:%d/' % self.server.server_port)
        self.client.open()

    def tearDown(self):
        self.client.close()
        self.server.shutdown()
        self.server.server_close()
        self.thread.join()

    def call(self, payload=b'ping', read=True):
        self.client.write(payload)
        self.client.flush()
        return self.client.read(5) if read else None

    def test_connection_is_reused(self):
        for _ in range(3):
            self.assertEqual(b'reply', self.call())
        self.assertEqual(1, self.server.connections)

    def test_unread_response_is_drained(self):
        # The reply of the first call is never read; the second call must
        # still work on the same connection.
        self.call(read=False)
        self.assertEqual(b'reply', self.call())
        self.assertEqual(1, self.server.connections)

    def test_reconnects_after_connection_close(self):
        self.server.close_after_reply = True
        self.assertEqual(b'reply', self.call())
        self.assertEqual(b'reply', self.call())
        self.assertEqual(2, self.server.connections)

    def test_flush_opens_a_closed_client(self):
        self.client.close()
        self.assertEqual(b'reply', self.call())
        self.assertTrue(self.client.isOpen())


if __name__ == '__main__':
    unittest.main()
