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

// Round trips through a named TPipe/TPipeServer pair.  TPipe is only a
// Windows class (elsewhere it is a TSocket), so the whole file is Windows only.
#ifdef _WIN32

#include <thrift/Thrift.h>
#include <thrift/TProcessor.h>
#include <thrift/protocol/TBinaryProtocol.h>
#include <thrift/server/TSimpleServer.h>
#include <thrift/transport/TPipe.h>
#include <thrift/transport/TPipeServer.h>
#include <thrift/transport/TTransportUtils.h>

#include <chrono>
#include <cstdint>
#include <exception>
#include <future>
#include <memory>
#include <string>
#include <thread>
#include <vector>

#define BOOST_TEST_MODULE TPipeTest
#include <boost/test/unit_test.hpp>

using apache::thrift::TProcessor;
using apache::thrift::protocol::TBinaryProtocol;
using apache::thrift::protocol::TBinaryProtocolFactory;
using apache::thrift::protocol::TProtocol;
using apache::thrift::protocol::T_CALL;
using apache::thrift::protocol::T_I32;
using apache::thrift::protocol::T_REPLY;
using apache::thrift::protocol::T_STOP;
using apache::thrift::server::TServerEventHandler;
using apache::thrift::server::TSimpleServer;
using apache::thrift::transport::TPipe;
using apache::thrift::transport::TPipeServer;
using apache::thrift::transport::TTransport;
using apache::thrift::transport::TTransportException;
using apache::thrift::transport::TTransportFactory;

namespace {

// The pipe buffer is deliberately small, so that the large payloads below do
// not fit in it and the transfer has to wait for the reader.
const uint32_t kPipeBufferSize = 4096;

// Fills a buffer with a pattern that shows lost, repeated or reordered bytes.
std::vector<uint8_t> makePayload(uint32_t size) {
  std::vector<uint8_t> payload(size);
  for (uint32_t i = 0; i < size; ++i) {
    payload[i] = static_cast<uint8_t>((i * 31 + (i >> 8)) & 0xFF);
  }
  return payload;
}

uint32_t readLength(TTransport& trans) {
  uint8_t raw[4];
  trans.readAll(raw, 4);
  return (static_cast<uint32_t>(raw[0]) << 24) | (static_cast<uint32_t>(raw[1]) << 16)
         | (static_cast<uint32_t>(raw[2]) << 8) | static_cast<uint32_t>(raw[3]);
}

void writeLength(TTransport& trans, uint32_t len) {
  uint8_t raw[4] = {static_cast<uint8_t>(len >> 24),
                    static_cast<uint8_t>(len >> 16),
                    static_cast<uint8_t>(len >> 8),
                    static_cast<uint8_t>(len)};
  trans.write(raw, 4);
}

// Serves one client: for every frame (a 4 byte length and that many bytes) it
// reads the whole frame and then writes the same frame back.  A length of zero
// ends the conversation.  Reading the whole frame before answering means the
// client can write it completely before it starts to read.
void echoOneClient(TPipeServer* server, std::promise<void>* result) {
  try {
    std::shared_ptr<TTransport> conn = server->accept();
    for (;;) {
      uint32_t len = readLength(*conn);
      if (len == 0) {
        break;
      }
      std::vector<uint8_t> frame(len);
      conn->readAll(frame.data(), len);
      writeLength(*conn, len);
      conn->write(frame.data(), len);
      conn->flush();
    }
    conn->close();
    result->set_value();
  } catch (...) {
    result->set_exception(std::current_exception());
  }
}

// Sends one frame and returns what the server echoes back.
std::vector<uint8_t> echo(TPipe& client, const std::vector<uint8_t>& payload) {
  writeLength(client, static_cast<uint32_t>(payload.size()));
  client.write(payload.data(), static_cast<uint32_t>(payload.size()));
  client.flush();

  uint32_t len = readLength(client);
  std::vector<uint8_t> reply(len);
  client.readAll(reply.data(), len);
  return reply;
}

// A processor with a single method, "add", that takes a struct of two i32
// fields (1 and 2) and replies with a struct holding their sum in field 0.  It
// stands in for a generated processor, which this test does not need.
class AddProcessor : public TProcessor {
public:
  bool process(std::shared_ptr<TProtocol> in,
               std::shared_ptr<TProtocol> out,
               void* /*connectionContext*/) override {
    std::string name;
    apache::thrift::protocol::TMessageType type;
    int32_t seqid;
    in->readMessageBegin(name, type, seqid);
    BOOST_CHECK_EQUAL(name, "add");
    BOOST_CHECK_EQUAL(static_cast<int>(type), static_cast<int>(T_CALL));

    int32_t a = 0;
    int32_t b = 0;
    std::string structName;
    in->readStructBegin(structName);
    for (;;) {
      std::string fieldName;
      apache::thrift::protocol::TType fieldType;
      int16_t fieldId;
      in->readFieldBegin(fieldName, fieldType, fieldId);
      if (fieldType == T_STOP) {
        break;
      }
      if (fieldType == T_I32 && fieldId == 1) {
        in->readI32(a);
      } else if (fieldType == T_I32 && fieldId == 2) {
        in->readI32(b);
      } else {
        in->skip(fieldType);
      }
      in->readFieldEnd();
    }
    in->readStructEnd();
    in->readMessageEnd();

    out->writeMessageBegin(name, T_REPLY, seqid);
    out->writeStructBegin("add_result");
    out->writeFieldBegin("success", T_I32, 0);
    out->writeI32(a + b);
    out->writeFieldEnd();
    out->writeFieldStop();
    out->writeStructEnd();
    out->writeMessageEnd();
    out->getTransport()->writeEnd();
    out->getTransport()->flush();
    return true;
  }
};

class ServeSignal : public TServerEventHandler {
public:
  void preServe() override { ready.set_value(); }
  std::promise<void> ready;
};

int32_t callAdd(TBinaryProtocol& proto, int32_t a, int32_t b, int32_t seqid) {
  proto.writeMessageBegin("add", T_CALL, seqid);
  proto.writeStructBegin("add_args");
  proto.writeFieldBegin("a", T_I32, 1);
  proto.writeI32(a);
  proto.writeFieldEnd();
  proto.writeFieldBegin("b", T_I32, 2);
  proto.writeI32(b);
  proto.writeFieldEnd();
  proto.writeFieldStop();
  proto.writeStructEnd();
  proto.writeMessageEnd();
  proto.getTransport()->writeEnd();
  proto.getTransport()->flush();

  std::string name;
  apache::thrift::protocol::TMessageType type;
  int32_t replySeqid;
  proto.readMessageBegin(name, type, replySeqid);
  BOOST_CHECK_EQUAL(name, "add");
  BOOST_CHECK_EQUAL(static_cast<int>(type), static_cast<int>(T_REPLY));
  BOOST_CHECK_EQUAL(replySeqid, seqid);

  int32_t sum = 0;
  std::string structName;
  proto.readStructBegin(structName);
  for (;;) {
    std::string fieldName;
    apache::thrift::protocol::TType fieldType;
    int16_t fieldId;
    proto.readFieldBegin(fieldName, fieldType, fieldId);
    if (fieldType == T_STOP) {
      break;
    }
    BOOST_CHECK_EQUAL(fieldId, 0);
    proto.readI32(sum);
    proto.readFieldEnd();
  }
  proto.readStructEnd();
  proto.readMessageEnd();
  return sum;
}

} // namespace

BOOST_AUTO_TEST_CASE(test_open_close_state) {
  TPipeServer server("TPipeTestState", kPipeBufferSize);
  server.listen();
  BOOST_CHECK(server.isOpen());

  std::promise<void> result;
  std::future<void> done = result.get_future();
  std::thread serverThread(echoOneClient, &server, &result);

  TPipe client("TPipeTestState");
  client.setConnTimeout(10);
  BOOST_CHECK(!client.isOpen());
  client.open();
  BOOST_CHECK(client.isOpen());

  writeLength(client, 0);
  client.flush();
  serverThread.join();
  done.get();

  client.close();
  BOOST_CHECK(!client.isOpen());
  server.close();
}

BOOST_AUTO_TEST_CASE(test_round_trip_sizes) {
  TPipeServer server("TPipeTestSizes", kPipeBufferSize);
  server.listen();

  std::promise<void> result;
  std::future<void> done = result.get_future();
  std::thread serverThread(echoOneClient, &server, &result);

  TPipe client("TPipeTestSizes");
  client.setConnTimeout(10);
  client.open();

  // one byte, a few bytes around the buffer size, 64 KiB, and more than the
  // pipe buffer holds many times over
  const uint32_t sizes[] = {1, 2, kPipeBufferSize - 1, kPipeBufferSize, kPipeBufferSize + 1,
                            64 * 1024, 1024 * 1024 + 7};
  for (uint32_t size : sizes) {
    std::vector<uint8_t> payload = makePayload(size);
    std::vector<uint8_t> reply = echo(client, payload);
    BOOST_CHECK_EQUAL(reply.size(), payload.size());
    BOOST_CHECK(reply == payload);
  }

  writeLength(client, 0);
  client.flush();
  serverThread.join();
  done.get();
  client.close();
  server.close();
}

BOOST_AUTO_TEST_CASE(test_reopen_with_next_client) {
  TPipeServer server("TPipeTestReopen", kPipeBufferSize);
  server.listen();

  for (int round = 0; round < 3; ++round) {
    std::promise<void> result;
    std::future<void> done = result.get_future();
    std::thread serverThread(echoOneClient, &server, &result);

    TPipe client("TPipeTestReopen");
    client.setConnTimeout(10);
    client.open();
    std::vector<uint8_t> payload = makePayload(100 + round);
    BOOST_CHECK(echo(client, payload) == payload);
    writeLength(client, 0);
    client.flush();
    serverThread.join();
    done.get();
    client.close();
  }
  server.close();
}

BOOST_AUTO_TEST_CASE(test_thrift_call) {
  std::shared_ptr<TPipeServer> transport(new TPipeServer("TPipeTestCall", kPipeBufferSize));
  std::shared_ptr<TProcessor> processor(new AddProcessor);
  std::shared_ptr<TTransportFactory> transportFactory(new TTransportFactory);
  std::shared_ptr<TBinaryProtocolFactory> protocolFactory(new TBinaryProtocolFactory);
  TSimpleServer server(processor, transport, transportFactory, protocolFactory);

  std::shared_ptr<ServeSignal> signal(new ServeSignal);
  server.setServerEventHandler(signal);
  std::future<void> ready = signal->ready.get_future();
  std::thread serverThread([&server]() { server.serve(); });

  BOOST_REQUIRE(ready.wait_for(std::chrono::seconds(30)) == std::future_status::ready);

  {
    std::shared_ptr<TPipe> pipe(new TPipe("TPipeTestCall"));
    pipe->setConnTimeout(10);
    TBinaryProtocol proto(pipe);
    pipe->open();

    BOOST_CHECK_EQUAL(callAdd(proto, 2, 3, 1), 5);
    BOOST_CHECK_EQUAL(callAdd(proto, -7, 7, 2), 0);
    BOOST_CHECK_EQUAL(callAdd(proto, 1000000, 2345678, 3), 3345678);
    pipe->close();
  }

  server.stop();
  serverThread.join();
}

#endif // _WIN32
