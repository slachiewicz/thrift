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

using std::map;
using std::string;
using cpp_generator_test_utils::parse_thrift_for_test;
using cpp_generator_test_utils::read_file;

// THRIFT-4852: recv_<fn> of a void function dropped its declared exceptions
// and returned normally, so the caller never saw them.
TEST_CASE("t_lua_generator rethrows declared exceptions from a void recv", "[functional]")
{
    const string thrift_path = "test_lua_void_exception.thrift";
    const string thrift_source =
        "exception X { 1: string message }\n"
        "exception Y { 1: string message }\n"
        "service S {\n"
        "  void f() throws (1: X x, 2: Y y)\n"
        "  i32 g() throws (1: X x)\n"
        "  void h()\n"
        "}\n";

    {
        std::ofstream thrift_file(thrift_path, std::ios::binary);
        REQUIRE(thrift_file.is_open());
        thrift_file << thrift_source;
    }

    map<string, string> parsed_options;
    std::unique_ptr<t_program> program(new t_program(thrift_path, "test_lua_void_exception"));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(
        t_generator_registry::get_generator(program.get(), "lua", parsed_options, ""));
    REQUIRE(gen != nullptr);
    REQUIRE_NOTHROW(gen->generate_program());

    const string generated = read_file("gen-lua/test_lua_void_exception_S.lua");
    REQUIRE(!generated.empty());

    auto body_of = [&](const string& header) {
        const size_t begin = generated.find(header);
        REQUIRE(begin != string::npos);
        const size_t end = generated.find("\nend\n", begin);
        REQUIRE(end != string::npos);
        return generated.substr(begin, end - begin);
    };

    const string recv_f = body_of("function SClient:recv_f(");
    REQUIRE(recv_f.find("if result.x then\n    error(result.x)") != string::npos);
    REQUIRE(recv_f.find("elseif result.y then\n    error(result.y)") != string::npos);
    // A void function has no success value and must not raise MISSING_RESULT.
    REQUIRE(recv_f.find("MISSING_RESULT") == string::npos);
    REQUIRE(recv_f.find("result.success") == string::npos);

    const string recv_g = body_of("function SClient:recv_g(");
    REQUIRE(recv_g.find("return result.success") != string::npos);
    REQUIRE(recv_g.find("elseif result.x then") != string::npos);
    REQUIRE(recv_g.find("MISSING_RESULT") != string::npos);

    const string recv_h = body_of("function SClient:recv_h(");
    REQUIRE(recv_h.find("error(result.") == string::npos);

    std::remove(thrift_path.c_str());
}
