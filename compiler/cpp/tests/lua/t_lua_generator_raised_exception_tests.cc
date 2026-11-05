// Licensed to the Apache Software Foundation(ASF) under one
// or more contributor license agreements.See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License. You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied. See the License for the
// specific language governing permissions and limitations
// under the License.

#include "../cpp/t_cpp_generator_test_utils.h"

#include <cstdio>
#include <fstream>
#include <memory>

using cpp_generator_test_utils::parse_thrift_for_test;
using cpp_generator_test_utils::read_file;
using std::map;
using std::string;

// THRIFT-6499: a handler that raises a declared exception with error() must
// reach the result struct like one that is returned, and any other raised value
// must be turned into a string before it becomes a TApplicationException message.
TEST_CASE("t_lua_generator processor handles exceptions raised by the handler", "[functional]") {
  const string thrift_path = "test_lua_raised_exception.thrift";
  const string thrift_source
      = "exception X { 1: string message }\n"
        "exception Y { 1: string message }\n"
        "service S {\n"
        "  i32 f() throws (1: X x, 2: Y y)\n"
        "  void g()\n"
        "  oneway void h()\n"
        "}\n";

  {
    std::ofstream thrift_file(thrift_path, std::ios::binary);
    REQUIRE(thrift_file.is_open());
    thrift_file << thrift_source;
  }

  map<string, string> parsed_options;
  std::unique_ptr<t_program> program(new t_program(thrift_path, "test_lua_raised_exception"));
  parse_thrift_for_test(program.get());

  std::unique_ptr<t_generator> gen(
      t_generator_registry::get_generator(program.get(), "lua", parsed_options, ""));
  REQUIRE(gen != nullptr);
  REQUIRE_NOTHROW(gen->generate_program());

  const string generated = read_file("gen-lua/test_lua_raised_exception_S.lua");
  REQUIRE(!generated.empty());

  auto body_of = [&](const string& header) {
    const size_t begin = generated.find(header);
    REQUIRE(begin != string::npos);
    const size_t end = generated.find("\nend\n", begin);
    REQUIRE(end != string::npos);
    return generated.substr(begin, end - begin);
  };

  const string process_f = body_of("function SProcessor:process_f(");
  const size_t raised = process_f.find(
      "  if not status then\n"
      "    if ttype(res) == 'X' then\n"
      "      result.x = res\n"
      "    elseif ttype(res) == 'Y' then\n"
      "      result.y = res\n"
      "    else\n"
      "      reply_type = TMessageType.EXCEPTION\n"
      "      result = TApplicationException:new{message = tostring(res)}\n"
      "    end\n"
      "  elseif ttype(res) == 'X' then\n");
  REQUIRE(raised != string::npos);
  // Only a returned value reaches the success assignment.
  REQUIRE(process_f.find("  else\n    result.success = res\n  end") != string::npos);

  const string process_g = body_of("function SProcessor:process_g(");
  REQUIRE(process_g.find("  if not status then\n"
                         "    reply_type = TMessageType.EXCEPTION\n"
                         "    result = TApplicationException:new{message = tostring(res)}\n"
                         "  else\n")
          != string::npos);

  // A oneway call sends no reply, so it has nothing to convert.
  const string process_h = body_of("function SProcessor:process_h(");
  REQUIRE(process_h.find("TApplicationException") == string::npos);

  std::remove(thrift_path.c_str());
}
