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

// A struct that holds itself by value (THRIFT-4484) cannot compile in C++, so the generator has to
// stop with advice instead of writing a header that fails with an incomplete-type error.
TEST_CASE("t_cpp_generator rejects a struct field that holds its own struct by value", "[functional]")
{
    string path = join_path(source_dir(), "test_self_reference.thrift");
    string name = "test_self_reference";
    map<string, string> parsed_options;
    string option_string = "";

    std::unique_ptr<t_program> program(new t_program(path, name));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(
        t_generator_registry::get_generator(program.get(), "cpp", parsed_options, option_string));
    REQUIRE(gen != nullptr);
    REQUIRE_THROWS_WITH(gen->generate_program(),
                        Catch::Matchers::Contains("field \"next\" of SelfByValue")
                            && Catch::Matchers::Contains("optional SelfByValue & next"));
}

TEST_CASE("t_cpp_generator accepts self references through '&' and containers", "[functional]")
{
    string path = join_path(source_dir(), "test_self_reference_ref.thrift");
    string name = "test_self_reference_ref";
    map<string, string> parsed_options;
    string option_string = "";

    std::unique_ptr<t_program> program(new t_program(path, name));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(
        t_generator_registry::get_generator(program.get(), "cpp", parsed_options, option_string));
    REQUIRE(gen != nullptr);
    REQUIRE_NOTHROW(gen->generate_program());

    string generated = read_file("gen-cpp/test_self_reference_ref_types.h");
    REQUIRE(generated.find("::std::shared_ptr<SelfByRef> next;") != string::npos);
}
