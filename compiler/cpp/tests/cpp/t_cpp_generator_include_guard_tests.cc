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

// Generates test_include_guard.thrift into gen-cpp/ with the given options.
void generate_include_guard_test(const map<string, string>& parsed_options) {
    string path = join_path(source_dir(), "test_include_guard.thrift");
    string name = "test_include_guard";
    string option_string = "";

    std::unique_ptr<t_program> program(new t_program(path, name));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(
        t_generator_registry::get_generator(program.get(), "cpp", parsed_options, option_string));
    REQUIRE(gen != nullptr);
    REQUIRE_NOTHROW(gen->generate_program());
}

bool has_guard(const string& file, const string& guard) {
    string content = read_file(file);
    REQUIRE(!content.empty());
    return content.find("#ifndef " + guard + "\n#define " + guard + "\n") != string::npos;
}

} // namespace

TEST_CASE("t_cpp_generator keeps the bare include guards by default", "[functional]")
{
    generate_include_guard_test({{"templates", ""}});

    REQUIRE(has_guard("gen-cpp/test_include_guard_types.h", "test_include_guard_TYPES_H"));
    REQUIRE(has_guard("gen-cpp/test_include_guard_types.tcc", "test_include_guard_TYPES_TCC"));
    REQUIRE(has_guard("gen-cpp/test_include_guard_constants.h", "test_include_guard_CONSTANTS_H"));
    REQUIRE(has_guard("gen-cpp/Echo.h", "Echo_H"));
    REQUIRE(has_guard("gen-cpp/Echo.tcc", "Echo_TCC"));
}

TEST_CASE("t_cpp_generator prepends include_guard_prefix to every include guard", "[functional]")
{
    generate_include_guard_test({{"templates", ""}, {"include_guard_prefix", "CUSTOM_GUARD_"}});

    REQUIRE(has_guard("gen-cpp/test_include_guard_types.h", "CUSTOM_GUARD_test_include_guard_TYPES_H"));
    REQUIRE(has_guard("gen-cpp/test_include_guard_types.tcc", "CUSTOM_GUARD_test_include_guard_TYPES_TCC"));
    REQUIRE(has_guard("gen-cpp/test_include_guard_constants.h", "CUSTOM_GUARD_test_include_guard_CONSTANTS_H"));
    REQUIRE(has_guard("gen-cpp/Echo.h", "CUSTOM_GUARD_Echo_H"));
    REQUIRE(has_guard("gen-cpp/Echo.tcc", "CUSTOM_GUARD_Echo_TCC"));
}
