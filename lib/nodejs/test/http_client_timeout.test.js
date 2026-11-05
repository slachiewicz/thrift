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

// The HTTP client's "timeout" option aborts a request whose server never
// answers and reports it through the connection's "error" event. Without the
// option the request waits for the server, as before.

const test = require("tape");
const http = require("http");
const thrift = require("thrift");
const ThriftTest = require("./gen-nodejs/ThriftTest");

// Accepts requests and never responds; remembers them so they can be closed.
function startSilentServer(onListening) {
  const sockets = [];
  const server = http.createServer(function () {});
  server.on("connection", function (socket) {
    sockets.push(socket);
  });
  server.listen(0, "127.0.0.1", function () {
    onListening(server, sockets);
  });
}

function stop(server, sockets) {
  sockets.forEach(function (s) {
    s.destroy();
  });
  server.close();
}

function connect(server, extra) {
  const options = Object.assign(
    {
      transport: thrift.TBufferedTransport,
      protocol: thrift.TBinaryProtocol,
      path: "/",
    },
    extra,
  );
  return thrift.createHttpConnection(
    "127.0.0.1",
    server.address().port,
    options,
  );
}

test("a request to a silent server fails with ETIMEDOUT when timeout is set", function (assert) {
  startSilentServer(function (server, sockets) {
    const connection = connect(server, { timeout: 200 });
    const client = thrift.createHttpClient(ThriftTest, connection);
    const guard = setTimeout(function () {
      assert.fail("no error event within 3 seconds");
      stop(server, sockets);
      assert.end();
    }, 3000);
    connection.on("error", function (err) {
      clearTimeout(guard);
      assert.equal(err.code, "ETIMEDOUT", "error code is ETIMEDOUT");
      stop(server, sockets);
      assert.end();
    });
    client.testString("hello", function () {});
  });
});

test("a request to a silent server is not aborted without the option", function (assert) {
  startSilentServer(function (server, sockets) {
    const connection = connect(server, {});
    const client = thrift.createHttpClient(ThriftTest, connection);
    let failed = false;
    connection.on("error", function () {
      failed = true;
    });
    client.testString("hello", function () {});
    setTimeout(function () {
      assert.notOk(failed, "no error after 600 ms");
      stop(server, sockets);
      assert.end();
    }, 600);
  });
});

test("a timeout does not disturb a request that is answered in time", function (assert) {
  const processor = new ThriftTest.Processor({
    testString: function (thing, result) {
      result(null, thing);
    },
  });
  const server = thrift.createWebServer({
    services: {
      "/": {
        processor: processor,
        protocol: thrift.TBinaryProtocol,
        transport: thrift.TBufferedTransport,
      },
    },
  });
  server.listen(0, "127.0.0.1", function () {
    const connection = connect(server, { timeout: 2000 });
    const client = thrift.createHttpClient(ThriftTest, connection);
    let failed = false;
    connection.on("error", function () {
      failed = true;
    });
    client.testString("hello", function (err, res) {
      assert.error(err, "no error");
      assert.equal(res, "hello", "response received");
      setTimeout(function () {
        assert.notOk(failed, "no late timeout error");
        server.close();
        assert.end();
      }, 2300);
    });
  });
});
