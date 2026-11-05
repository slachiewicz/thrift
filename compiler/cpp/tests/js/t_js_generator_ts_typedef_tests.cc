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

// THRIFT-3185: the TypeScript declarations dropped typedefs and spelled out
// the underlying type everywhere.
TEST_CASE("t_js_generator keeps typedef names in TypeScript declarations", "[functional]")
{
    const string thrift_path = "test_js_ts_typedef.thrift";
    const string thrift_source =
        "typedef string userID\n"
        "typedef list<userID> userIDs\n"
        "struct Foo { 1: userID s, 2: userIDs l, 3: map<userID, i32> m }\n"
        "service Svc { userID get(1: userID id) }\n";

    {
        std::ofstream thrift_file(thrift_path, std::ios::binary);
        REQUIRE(thrift_file.is_open());
        thrift_file << thrift_source;
    }

    map<string, string> parsed_options;
    parsed_options["ts"] = "";
    std::unique_ptr<t_program> program(new t_program(thrift_path, "test_js_ts_typedef"));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(
        t_generator_registry::get_generator(program.get(), "js", parsed_options, ""));
    REQUIRE(gen != nullptr);
    REQUIRE_NOTHROW(gen->generate_program());

    const string types = read_file("gen-js/test_js_ts_typedef_types.d.ts");
    REQUIRE(types.find("type userID = string;") != string::npos);
    REQUIRE(types.find("type userIDs = userID[];") != string::npos);
    REQUIRE(types.find("s: userID;") != string::npos);
    REQUIRE(types.find("l: userIDs;") != string::npos);
    // An index signature cannot use an alias, so a map key stays the base type.
    REQUIRE(types.find("m: { [k: string]: number; };") != string::npos);

    const string service = read_file("gen-js/Svc.d.ts");
    REQUIRE(service.find("get(id: userID): userID;") != string::npos);

    std::remove(thrift_path.c_str());
}
