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

// THRIFT-4846: a struct is written after the structs it holds by value, and a typedef that names
// a struct declared later in the file is written after that struct's forward declaration.
TEST_CASE("t_cpp_generator orders typedefs and structs by what they need", "[functional]")
{
    string path = join_path(source_dir(), "test_definition_order.thrift");
    string name = "test_definition_order";
    map<string, string> parsed_options;
    string option_string = "";

    std::unique_ptr<t_program> program(new t_program(path, name));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(
        t_generator_registry::get_generator(program.get(), "cpp", parsed_options, option_string));
    REQUIRE(gen != nullptr);
    REQUIRE_NOTHROW(gen->generate_program());

    string generated = read_file("gen-cpp/test_definition_order_types.h");
    REQUIRE(!generated.empty());

    string::size_type forward_later = generated.find("class Later;");
    string::size_type typedef_later = generated.find("typedef Later LaterT;");
    string::size_type typedef_list = generated.find("typedef std::vector<Later>  LaterList;");
    string::size_type def_later = generated.find("class Later :");
    string::size_type def_holder = generated.find("class Holder :");
    REQUIRE(forward_later != string::npos);
    REQUIRE(typedef_later != string::npos);
    REQUIRE(typedef_list != string::npos);
    REQUIRE(def_later != string::npos);
    REQUIRE(def_holder != string::npos);
    REQUIRE(forward_later < typedef_later);
    REQUIRE(forward_later < typedef_list);
    REQUIRE(typedef_later < def_holder);
    REQUIRE(def_later < def_holder);

    // A chain declared in reverse comes out as C, B, A.
    string::size_type def_a = generated.find("class OrderA :");
    string::size_type def_b = generated.find("class OrderB :");
    string::size_type def_c = generated.find("class OrderC :");
    REQUIRE(def_c != string::npos);
    REQUIRE(def_c < def_b);
    REQUIRE(def_b < def_a);

    // A cycle broken by '&' needs no reordering beyond what the by-value member asks for.
    string::size_type def_first = generated.find("class BrokenFirst :");
    string::size_type def_second = generated.find("class BrokenSecond :");
    REQUIRE(def_first != string::npos);
    REQUIRE(def_first < def_second);
}

TEST_CASE("t_cpp_generator rejects structs that hold each other by value", "[functional]")
{
    string path = join_path(source_dir(), "test_definition_cycle.thrift");
    string name = "test_definition_cycle";
    map<string, string> parsed_options;
    string option_string = "";

    std::unique_ptr<t_program> program(new t_program(path, name));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(
        t_generator_registry::get_generator(program.get(), "cpp", parsed_options, option_string));
    REQUIRE(gen != nullptr);
    REQUIRE_THROWS_WITH(gen->generate_program(),
                        Catch::Matchers::Contains("CycleA.b -> CycleB.a -> CycleA")
                            && Catch::Matchers::Contains("optional CycleB & b"));
}
