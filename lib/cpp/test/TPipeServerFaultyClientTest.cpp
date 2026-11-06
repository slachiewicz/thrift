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

#ifdef _WIN32

#include <atomic>
#include <chrono>
#include <memory>
#include <mutex>
#include <string>
#include <thread>
#include <vector>

#include <thrift/transport/TPipe.h>
#include <thrift/transport/TPipeServer.h>

#define BOOST_TEST_MODULE TPipeServerFaultyClientTest
#include <boost/test/unit_test.hpp>

using apache::thrift::transport::TPipe;
using apache::thrift::transport::TPipeServer;
using apache::thrift::transport::TTransport;
using apache::thrift::transport::TTransportException;

// THRIFT-3238: a client that opens the pipe and closes it again at once must not
// make TPipeServer::accept() fail with anything but CLIENT_DISCONNECT, because
// TServerFramework::serve() leaves its loop on any other exception.  The window
// is a race inside accept(), so the test hammers the server with such clients;
// every wait is bounded, so a broken server fails the test instead of hanging it.

namespace {

// Everything the accept thread touches lives here, so that the thread can outlive
// a test that gave up on it.
struct AcceptState {
  AcceptState() : stop(false), died(false) {}
  std::shared_ptr<TPipeServer> server;
  std::atomic<bool> stop;
  std::atomic<bool> died;
  std::mutex mutex;
  std::string diedWith;
  std::vector<std::shared_ptr<TTransport> > accepted;
};

void acceptLoop(std::shared_ptr<AcceptState> state) {
  while (!state->stop) {
    try {
      std::shared_ptr<TTransport> client = state->server->accept();
      std::lock_guard<std::mutex> lock(state->mutex);
      state->accepted.push_back(client);
    } catch (const TTransportException& ex) {
      if (ex.getType() == TTransportException::CLIENT_DISCONNECT
          || ex.getType() == TTransportException::TIMED_OUT) {
        continue; // what TServerFramework::serve() does
      }
      std::lock_guard<std::mutex> lock(state->mutex);
      if (ex.getType() != TTransportException::INTERRUPTED) {
        state->died = true;
        state->diedWith = "type " + std::to_string(static_cast<int>(ex.getType())) + ": "
                          + ex.what();
      }
      return;
    } catch (const std::exception& ex) {
      std::lock_guard<std::mutex> lock(state->mutex);
      state->died = true;
      state->diedWith = ex.what();
      return;
    }
  }
}

void faultyClient(const std::string& pipename, const std::atomic<bool>& stop) {
  while (!stop) {
    try {
      TPipe client(pipename);
      client.setConnTimeout(1);
      client.open();
      client.close(); // gone before the server has looked at it
    } catch (...) {
      // the pipe may be busy or gone; only the server's fate matters
    }
  }
}

} // namespace

BOOST_AUTO_TEST_CASE(test_server_survives_clients_that_disconnect_at_once) {
  const std::string pipename = "TPipeServerFaultyClientTest";

  std::shared_ptr<AcceptState> state(new AcceptState);
  state->server.reset(new TPipeServer(pipename));
  state->server->listen();
  std::thread acceptThread(acceptLoop, state);

  std::atomic<bool> stopFaulty(false);
  std::vector<std::thread> faulty;
  for (int i = 0; i < 4; ++i) {
    faulty.push_back(std::thread(faultyClient, pipename, std::ref(stopFaulty)));
  }

  // Run until the server dies or enough time has passed for the race to show up.
  auto deadline = std::chrono::steady_clock::now() + std::chrono::seconds(5);
  while (!state->died && std::chrono::steady_clock::now() < deadline) {
    std::this_thread::sleep_for(std::chrono::milliseconds(20));
  }
  stopFaulty = true;
  for (auto& t : faulty) {
    t.join();
  }
  {
    std::lock_guard<std::mutex> lock(state->mutex);
    BOOST_CHECK_MESSAGE(!state->died, "accept() failed after a client disconnected at once: "
                                          << state->diedWith);
  }

  // A well-behaved client must still be served.
  bool served = false;
  if (!state->died) {
    try {
      TPipe good(pipename);
      good.setConnTimeout(5);
      good.open();
      const uint8_t marker = 'G';
      good.write(&marker, 1);
      good.flush();

      auto readDeadline = std::chrono::steady_clock::now() + std::chrono::seconds(10);
      while (!served && !state->died && std::chrono::steady_clock::now() < readDeadline) {
        std::vector<std::shared_ptr<TTransport> > accepted;
        {
          std::lock_guard<std::mutex> lock(state->mutex);
          accepted = state->accepted;
        }
        // The marker is already written, so a read on the good client's pipe returns
        // at once; pipes of the faulty clients report end of file or an error.
        for (auto& t : accepted) {
          try {
            uint8_t got = 0;
            if (t->read(&got, 1) == 1 && got == marker) {
              served = true;
              break;
            }
          } catch (...) {
          }
        }
        if (!served) {
          std::this_thread::sleep_for(std::chrono::milliseconds(50));
        }
      }
    } catch (const std::exception& ex) {
      BOOST_ERROR(std::string("the good client could not be served: ") + ex.what());
    }
    BOOST_CHECK_MESSAGE(served, "the good client was not served");
  }

  state->stop = true;
  state->server->interrupt();
  acceptThread.join(); // interrupt() wakes accept(), which then returns
}

#endif // _WIN32
