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

// A request that fails before any response arrives (ECONNREFUSED, a timeout)
// is reported to the pending client callback, so a promise-based or callback
// client settles instead of waiting forever. The connection still emits
// "error" as before.

const test = require("tape");
const net = require("net");
const thrift = require("thrift");
const ThriftTest = require("./gen-nodejs/ThriftTest");

// A port nothing listens on: bind to an ephemeral port, then release it.
function closedPort(done) {
  const srv = net.createServer();
  srv.listen(0, "127.0.0.1", function () {
    const port = srv.address().port;
    srv.close(function () {
      done(port);
    });
  });
}

function once(fn) {
  let called = false;
  return function () {
    if (!called) {
      called = true;
      fn.apply(this, arguments);
    }
  };
}

test("the HTTP client rejects a call when the connection is refused", function (assert) {
  closedPort(function (port) {
    const connection = thrift.createHttpConnection("127.0.0.1", port, {
      transport: thrift.TBufferedTransport,
      protocol: thrift.TBinaryProtocol,
      path: "/",
    });
    let emitted = null;
    connection.on("error", function (e) {
      emitted = e;
    });
    const client = thrift.createHttpClient(ThriftTest, connection);

    const timer = setTimeout(function () {
      assert.fail("the call never settled");
      assert.end();
      process.exit(1);
    }, 5000);

    client.testString("x").then(
      function () {
        clearTimeout(timer);
        assert.fail("the call must not succeed");
        assert.end();
      },
      function (err) {
        clearTimeout(timer);
        assert.equal(
          err && err.code,
          "ECONNREFUSED",
          "rejected with the socket error",
        );
        assert.ok(emitted, "the connection still emitted error");
        assert.deepEqual(
          Object.keys(client._reqs),
          [],
          "no pending callback is left",
        );
        assert.end();
      },
    );
  });
});

test("the multiplexed HTTP client rejects a call when the connection is refused", function (assert) {
  closedPort(function (port) {
    const connection = thrift.createHttpConnection("127.0.0.1", port, {
      transport: thrift.TBufferedTransport,
      protocol: thrift.TBinaryProtocol,
      path: "/",
    });
    connection.on("error", function () {});
    const multiplexer = new thrift.Multiplexer();
    const client = multiplexer.createClient(
      "ThriftTest",
      ThriftTest,
      connection,
    );

    const timer = setTimeout(function () {
      assert.fail("the call never settled");
      assert.end();
      process.exit(1);
    }, 5000);

    const done = once(function (err) {
      clearTimeout(timer);
      assert.equal(
        err && err.code,
        "ECONNREFUSED",
        "rejected with the socket error",
      );
      assert.deepEqual(
        connection.seqId2Service,
        {},
        "service map entry released",
      );
      assert.end();
    });
    client.testString("x", done);
  });
});
