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

#include "t_cpp_generator_test_utils.h"

using std::string;
using std::map;
using cpp_generator_test_utils::read_file;
using cpp_generator_test_utils::source_dir;
using cpp_generator_test_utils::join_path;
using cpp_generator_test_utils::parse_thrift_for_test;

namespace {

// Returns the body of the stub whose signature starts with the given text.
string stub_body(const string& source, const string& signature) {
    size_t start = source.find(signature);
    REQUIRE(start != string::npos);
    size_t end = source.find("\n  }\n", start);
    REQUIRE(end != string::npos);
    return source.substr(start, end - start);
}

void check_skeleton(const string& source) {
    REQUIRE(!source.empty());

    // Functions returning a value through the C++ return statement end with a value-initialized
    // return, so the stub compiles without -Wreturn-type warnings.
    REQUIRE(stub_body(source, "int32_t number()").find("\n    return int32_t();") != string::npos);
    REQUIRE(stub_body(source, "bool flag()").find("\n    return bool();") != string::npos);
    REQUIRE(stub_body(source, "Color::type color()").find("\n    return Color::type();")
            != string::npos);
    REQUIRE(stub_body(source, "Id id()").find("\n    return Id();") != string::npos);
    REQUIRE(stub_body(source, "Shade shade()").find("\n    return Shade();") != string::npos);

    // Void functions and functions returning through _return are unchanged.
    REQUIRE(stub_body(source, "void nothing()").find("\n    return ") == string::npos);
    REQUIRE(stub_body(source, "void text(std::string& _return)").find("\n    return ") == string::npos);
    REQUIRE(stub_body(source, "void numbers(std::vector<int32_t> & _return)").find("\n    return ")
            == string::npos);
    REQUIRE(stub_body(source, "void pair(Pair& _return)").find("\n    return ") == string::npos);
}

} // namespace

TEST_CASE("t_cpp_generator server skeleton stubs return a value for non-void functions",
          "[functional]")
{
    string path = join_path(source_dir(), "test_skeleton_return.thrift");
    string name = "test_skeleton_return";
    map<string, string> parsed_options = {{"cob_style", ""}}; // also generates the async skeleton
    string option_string = "cob_style";

    std::unique_ptr<t_program> program(new t_program(path, name));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(
        t_generator_registry::get_generator(program.get(), "cpp", parsed_options, option_string));
    REQUIRE(gen != nullptr);
    REQUIRE_NOTHROW(gen->generate_program());

    check_skeleton(read_file("gen-cpp/ReturnService_server.skeleton.cpp"));
    // The synchronous handler inside the asynchronous skeleton is built the same way.
    check_skeleton(read_file("gen-cpp/ReturnService_async_server.skeleton.cpp"));
}
