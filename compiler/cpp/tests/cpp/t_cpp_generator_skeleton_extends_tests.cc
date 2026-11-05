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

TEST_CASE("t_cpp_generator server skeleton stubs functions of extended services", "[functional]")
{
    string path = join_path(source_dir(), "test_skeleton_extends.thrift");
    string name = "test_skeleton_extends";
    map<string, string> parsed_options;
    string option_string = "";

    std::unique_ptr<t_program> program(new t_program(path, name));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(
        t_generator_registry::get_generator(program.get(), "cpp", parsed_options, option_string));
    REQUIRE(gen != nullptr);
    REQUIRE_NOTHROW(gen->generate_program());

    // The handler derives only from DerivedServiceIf, whose pure virtual functions include
    // those of MiddleService and of BaseService.
    string derived = read_file("gen-cpp/DerivedService_server.skeleton.cpp");
    REQUIRE(!derived.empty());
    REQUIRE(derived.find("class DerivedServiceHandler") != string::npos);
    REQUIRE(derived.find("void getBase(BaseResult& _return, const int32_t key)")
            != string::npos);
    REQUIRE(derived.find("void basePing()") != string::npos);
    REQUIRE(derived.find("void middlePing()") != string::npos);
    REQUIRE(derived.find("int32_t add(const int32_t a, const int32_t b)") != string::npos);

    // A service extending one level only gets the base functions, not the derived ones.
    string middle = read_file("gen-cpp/MiddleService_server.skeleton.cpp");
    REQUIRE(middle.find("void basePing()") != string::npos);
    REQUIRE(middle.find("void middlePing()") != string::npos);
    REQUIRE(middle.find("add(") == string::npos);

    // A service without a base is unchanged.
    string base = read_file("gen-cpp/BaseService_server.skeleton.cpp");
    REQUIRE(base.find("void basePing()") != string::npos);
    REQUIRE(base.find("middlePing") == string::npos);
}
