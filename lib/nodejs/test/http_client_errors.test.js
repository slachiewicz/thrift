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
const http = require("http");
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

// Starts a server whose handler answers every request, then calls done(port, close).
function withServer(handler, done) {
  const srv = http.createServer(handler);
  srv.listen(0, "127.0.0.1", function () {
    done(srv.address().port, function () {
      srv.closeAllConnections();
      srv.close();
    });
  });
}

// Runs one call and checks it settles once with an error and that the
// connection emits "error" once, also after the rest of the reply was handled.
function expectOneFailure(assert, multiplexed, handler, check) {
  withServer(handler, function (port, close) {
    const connection = thrift.createHttpConnection("127.0.0.1", port, {
      transport: thrift.TBufferedTransport,
      protocol: thrift.TBinaryProtocol,
      path: "/",
    });
    const emitted = [];
    connection.on("error", function (e) {
      emitted.push(e);
    });
    const client = multiplexed
      ? new thrift.Multiplexer().createClient(
          "ThriftTest",
          ThriftTest,
          connection,
        )
      : thrift.createHttpClient(ThriftTest, connection);
    const settled = [];
    const timer = setTimeout(function () {
      assert.fail("the call never settled");
      close();
      assert.end();
      process.exit(1);
    }, 5000);
    client.testString("x", function (err) {
      settled.push(err);
    });
    // Leave time for the rest of the reply to be processed.
    const poll = setInterval(function () {
      if (!settled.length) return;
      clearInterval(poll);
      setTimeout(function () {
        clearTimeout(timer);
        close();
        assert.equal(settled.length, 1, "the call settled once");
        assert.equal(emitted.length, 1, "the connection emitted error once");
        check(settled[0], emitted[0]);
        assert.end();
      }, 200);
    }, 10);
  });
}

[false, true].forEach(function (multiplexed) {
  const kind = multiplexed ? "multiplexed " : "";

  test(`the ${kind}HTTP client rejects a call answered with status 500`, function (assert) {
    expectOneFailure(
      assert,
      multiplexed,
      function (req, res) {
        req.resume();
        res.writeHead(500, { "Content-Type": "text/plain" });
        // Bytes a Thrift binary reader rejects as a bad version.
        res.end(Buffer.from([0x80, 0x02, 0x00, 0x01, 0, 0, 0, 0]));
      },
      function (err) {
        assert.ok(
          err instanceof thrift.Thrift.TApplicationException,
          "THTTPException",
        );
        assert.equal(err.statusCode, 500, "carries the status");
      },
    );
  });

  test(`the ${kind}HTTP client rejects a call when the reply is cut mid-body`, function (assert) {
    expectOneFailure(
      assert,
      multiplexed,
      function (req, res) {
        req.resume();
        res.writeHead(200, { "Content-Length": 100 });
        res.write("abc");
        setTimeout(function () {
          res.destroy();
        }, 50);
      },
      function (err) {
        assert.ok(err instanceof Error, "rejected with an error");
      },
    );
  });
});
