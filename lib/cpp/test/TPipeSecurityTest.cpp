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

#define BOOST_TEST_MODULE TPipeSecurityTest
#include <boost/test/unit_test.hpp>

#include <thrift/transport/TPipe.h>
#include <thrift/transport/TPipeServer.h>

#include <windows.h>

#include <chrono>
#include <future>
#include <memory>
#include <string>
#include <thread>

using apache::thrift::transport::TPipe;
using apache::thrift::transport::TPipeServer;

namespace {

const std::chrono::seconds kTimeout(20);

// THRIFT-5388: TPipe::open() has to ask for SecurityIdentification.  A raw
// server (not TPipeServer) lets the test do what a hostile server would do:
// impersonate the client and look at the level it was granted.
BOOST_AUTO_TEST_CASE(client_grants_identification_level_only) {
  const std::string name = "TPipeSecurityTest.impersonation";
  HANDLE server = CreateNamedPipeA(("\\\\.\\pipe\\" + name).c_str(),
                                   PIPE_ACCESS_DUPLEX,
                                   PIPE_TYPE_BYTE | PIPE_READMODE_BYTE | PIPE_WAIT,
                                   1, 1024, 1024, 0, nullptr);
  BOOST_REQUIRE(server != INVALID_HANDLE_VALUE);

  // The worker owns the promise, so a test that times out leaves a thread behind
  // instead of destroying state it still uses.
  auto level = std::make_shared<std::promise<int>>();
  std::future<int> result = level->get_future();
  std::thread worker([server, level]() {
    int value = -1;
    if (ConnectNamedPipe(server, nullptr) || GetLastError() == ERROR_PIPE_CONNECTED) {
      if (ImpersonateNamedPipeClient(server)) {
        HANDLE token = nullptr;
        if (OpenThreadToken(GetCurrentThread(), TOKEN_QUERY, TRUE, &token)) {
          SECURITY_IMPERSONATION_LEVEL l;
          DWORD len = 0;
          if (GetTokenInformation(token, TokenImpersonationLevel, &l, sizeof(l), &len)) {
            value = static_cast<int>(l);
          }
          CloseHandle(token);
        }
        RevertToSelf();
      }
    }
    level->set_value(value);
  });
  worker.detach();

  TPipe client(name);
  client.open();

  BOOST_REQUIRE(result.wait_for(kTimeout) == std::future_status::ready);
  BOOST_CHECK_EQUAL(result.get(), static_cast<int>(SecurityIdentification));
  client.close();
}

// PIPE_REJECT_REMOTE_CLIENTS cannot be exercised without a second machine; this
// checks that opting in still serves a local client, with both the setter and
// the default.
void roundTrip(TPipeServer& server, const std::string& name) {
  server.listen();
  TPipe client(name);
  client.open();
  auto accepted = server.accept();
  const uint8_t out = 0x5a;
  client.write(&out, 1);
  client.flush();
  uint8_t in = 0;
  BOOST_REQUIRE_EQUAL(accepted->read(&in, 1), 1u);
  BOOST_CHECK_EQUAL(in, out);
}

BOOST_AUTO_TEST_CASE(reject_remote_clients_serves_local_client) {
  TPipeServer server("TPipeSecurityTest.reject");
  server.setRejectRemoteClients(true);
  roundTrip(server, "TPipeSecurityTest.reject");
}

BOOST_AUTO_TEST_CASE(default_server_serves_local_client) {
  TPipeServer server("TPipeSecurityTest.default");
  roundTrip(server, "TPipeSecurityTest.default");
}

BOOST_AUTO_TEST_CASE(reject_remote_clients_after_listen_throws) {
  TPipeServer server("TPipeSecurityTest.late");
  server.listen();
  BOOST_CHECK_THROW(server.setRejectRemoteClients(true), std::logic_error);
}

} // namespace

#endif // _WIN32
