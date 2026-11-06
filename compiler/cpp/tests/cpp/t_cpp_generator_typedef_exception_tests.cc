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

// An exception reached through a typedef must be generated like the exception itself: a value
// member of the *_presult struct, not a pointer that the client never initializes.
TEST_CASE("t_cpp_generator treats a typedef'd exception like the exception", "[functional]")
{
    string path = join_path(source_dir(), "test_typedef_exception.thrift");
    string name = "test_typedef_exception";
    map<string, string> parsed_options;
    string option_string = "";

    std::unique_ptr<t_program> program(new t_program(path, name));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(
        t_generator_registry::get_generator(program.get(), "cpp", parsed_options, option_string));
    REQUIRE(gen != nullptr);
    REQUIRE_NOTHROW(gen->generate_program());

    string header = read_file("gen-cpp/FooBar.h");
    REQUIRE(!header.empty());
    REQUIRE(header.find("AliasedError* e;") == string::npos);
    REQUIRE(header.find("AliasedError e;") != string::npos);
    REQUIRE(header.find("FooError* e;") == string::npos);

    string impl = read_file("gen-cpp/FooBar.cpp");
    REQUIRE(!impl.empty());
    REQUIRE(impl.find("(*(this->e))") == string::npos);
}
