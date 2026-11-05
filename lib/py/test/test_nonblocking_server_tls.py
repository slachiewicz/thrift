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
#
# TNonblockingServer over TSSLServerSocket (THRIFT-4866): a non-blocking
# SSLSocket raises SSLWantReadError instead of returning a short read, and it
# holds decrypted records that select() on the descriptor cannot see. A
# request of more than one TLS record (16 KB) used to close the connection.
#

import os
import socket
import ssl
import struct
import sys
import threading
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import _import_local_thrift  # noqa
from thrift.server import TNonblockingServer  # noqa
from thrift.transport.TSSLSocket import TSSLServerSocket  # noqa

KEYS = os.path.join(os.path.dirname(os.path.dirname(os.path.dirname(
    os.path.dirname(os.path.abspath(__file__))))), 'test', 'keys')
SERVER_CERT = os.path.join(KEYS, 'server.crt')
SERVER_KEY = os.path.join(KEYS, 'server.key')


class LengthProcessor(object):
    """Answers with the number of request bytes it was given."""

    def process(self, iprot, oprot):
        n = len(iprot.trans.read(1 << 30))
        oprot.trans.write(struct.pack('!i', n))


def recv_exactly(sock, n):
    data = b''
    while len(data) < n:
        chunk = sock.recv(n - len(data))
        if not chunk:
            raise EOFError('connection closed after %d of %d bytes' % (len(data), n))
        data += chunk
    return data


class TLSServerTest(unittest.TestCase):

    def start(self, **kwargs):
        lsocket = TSSLServerSocket(
            host='127.0.0.1', port=0, certfile=SERVER_CERT, keyfile=SERVER_KEY,
            cert_reqs=ssl.CERT_NONE,
            ssl_version=ssl.PROTOCOL_TLS_SERVER, **kwargs)
        server = TNonblockingServer.TNonblockingServer(
            LengthProcessor(), lsocket, threads=1)
        server.prepare()
        thread = threading.Thread(target=self.serve, args=(server,))
        thread.daemon = True
        thread.start()

        def stop():
            server.stop()
            thread.join(10)
            server.close()
        self.addCleanup(stop)
        return server, lsocket.handle.getsockname()[1]

    @staticmethod
    def serve(server):
        while not server._stop:
            server.handle()

    def client(self, port):
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_CLIENT)
        context.check_hostname = False
        context.verify_mode = ssl.CERT_NONE
        sock = context.wrap_socket(socket.create_connection(('127.0.0.1', port)))
        sock.settimeout(10)
        self.addCleanup(sock.close)
        return sock

    def test_a_request_of_many_tls_records_is_answered(self):
        _, port = self.start()
        sock = self.client(port)
        for size in (100, 20000, 300000):
            payload = b'x' * size
            sock.sendall(struct.pack('!i', size) + payload)
            reply = recv_exactly(sock, 8)
            self.assertEqual(struct.unpack('!ii', reply), (4, size))

    def test_the_frame_size_limit_still_applies_over_tls(self):
        _, port = self.start()
        sock = self.client(port)
        sock.sendall(struct.pack('!i', 1 << 30) + b'x' * 1000)
        with self.assertRaises((EOFError, ssl.SSLError, OSError)):
            recv_exactly(sock, 4)


if __name__ == '__main__':
    unittest.main()
