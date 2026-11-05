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
#include <stdexcept>

using std::map;
using std::string;
using cpp_generator_test_utils::parse_thrift_for_test;

namespace {

// Generates js code for a one-off IDL and removes the IDL file again.
void generate_js(const string& name, const string& source, const string& options)
{
    const string thrift_path = name + ".thrift";
    {
        std::ofstream thrift_file(thrift_path, std::ios::binary);
        REQUIRE(thrift_file.is_open());
        thrift_file << source;
    }

    map<string, string> parsed_options;
    std::unique_ptr<t_program> program(new t_program(thrift_path, name));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(
        t_generator_registry::get_generator(program.get(), "js", parsed_options, options));
    REQUIRE(gen != nullptr);
    try {
        gen->generate_program();
    } catch (...) {
        std::remove(thrift_path.c_str());
        throw;
    }
    std::remove(thrift_path.c_str());
}

} // namespace

// THRIFT-3315: the client implements each method a() with send_a() and recv_a(),
// so a method named recv_a or send_a silently replaced the generated one.
TEST_CASE("t_js_generator rejects a method that collides with a generated client method",
          "[functional]")
{
    REQUIRE_THROWS_WITH(
        generate_js("test_js_recv_clash",
                    "service Test {\n  i32 a()\n  i32 recv_a()\n}\n", "node"),
        Catch::Matchers::Contains("recv_a"));
    REQUIRE_THROWS_WITH(
        generate_js("test_js_send_clash",
                    "service Test {\n  void send_a()\n  void a()\n}\n", ""),
        Catch::Matchers::Contains("send_a"));
}

TEST_CASE("t_js_generator rejects a collision with a method of an extended service",
          "[functional]")
{
    REQUIRE_THROWS_WITH(
        generate_js("test_js_inherit_clash",
                    "service Base {\n  i32 a()\n}\nservice Child extends Base {\n  i32 recv_a()\n}\n",
                    "node"),
        Catch::Matchers::Contains("recv_a"));
}

TEST_CASE("t_js_generator accepts methods that do not collide", "[functional]")
{
    REQUIRE_NOTHROW(generate_js("test_js_no_clash",
                                "service Test {\n  i32 a()\n  i32 recv_b()\n  i32 send_c()\n}\n",
                                "node"));
}
