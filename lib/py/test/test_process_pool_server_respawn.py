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
# TProcessPoolServer forks its workers once at startup. A worker that dies
# afterwards (killed, crashed) has to be replaced, or the pool shrinks until
# the server stops answering.
#

import multiprocessing
import os
import signal
import sys
import threading
import time
import unittest

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))

import _import_local_thrift  # noqa
from thrift.protocol import TBinaryProtocol  # noqa
from thrift.server import TProcessPoolServer  # noqa
from thrift.transport import TSocket, TTransport  # noqa


class NoopProcessor(object):
    def process(self, iprot, oprot):
        pass


@unittest.skipUnless(hasattr(os, 'fork'), 'requires fork')
class ProcessPoolRespawnTest(unittest.TestCase):

    TIMEOUT = 10.0

    def setUp(self):
        # The workers inherit the listening socket, which cannot be pickled
        # for the spawn or forkserver start methods.
        self._start_method = multiprocessing.get_start_method(allow_none=True)
        multiprocessing.set_start_method('fork', force=True)
        transport = TSocket.TServerSocket(host='127.0.0.1', port=0)
        self.server = TProcessPoolServer.TProcessPoolServer(
            NoopProcessor(), transport,
            TTransport.TTransportFactoryBase(),
            TBinaryProtocol.TBinaryProtocolFactory())
        self.server.setNumWorkers(2)
        self.server.workerCheckInterval = 0.1
        self.thread = threading.Thread(target=self.server.serve)
        self.thread.daemon = True
        self.thread.start()

    def tearDown(self):
        self.server.stop()
        self.thread.join(self.TIMEOUT)
        for w in list(self.server.workers):
            if w.is_alive():
                w.terminate()
            w.join(self.TIMEOUT)
        multiprocessing.set_start_method(self._start_method, force=True)

    def wait_for(self, condition):
        deadline = time.time() + self.TIMEOUT
        while time.time() < deadline:
            if condition():
                return True
            time.sleep(0.05)
        return condition()

    def test_dead_worker_is_replaced(self):
        self.assertTrue(self.wait_for(lambda: len(self.server.workers) == 2))
        victim = self.server.workers[0]
        self.assertTrue(self.wait_for(victim.is_alive))
        os.kill(victim.pid, signal.SIGKILL)

        def replaced():
            workers = list(self.server.workers)
            return (len(workers) == 2
                    and victim not in workers
                    and all(w.is_alive() for w in workers))

        self.assertTrue(self.wait_for(replaced))


if __name__ == '__main__':
    unittest.main()
