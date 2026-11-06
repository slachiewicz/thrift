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

using cpp_generator_test_utils::join_path;
using cpp_generator_test_utils::parse_thrift_for_test;
using cpp_generator_test_utils::read_file;
using std::map;
using std::string;

// source_dir() in the shared header resolves to the cpp test directory, so locate this one here.
static string netstd_test_dir() {
  string file = __FILE__;
  std::replace(file.begin(), file.end(), '\\', '/');
  size_t slash = file.rfind('/');
  return (slash == string::npos) ? string(".") : file.substr(0, slash);
}

// A required field of exception type used to abort the generator with "UNEXPECTED TYPE IN
// initialize_field" when targeting net6 or later.
TEST_CASE("t_netstd_generator accepts a required field of exception type", "[functional]") {
  const char* targets[] = {"net8", "net9", "net10"};
  for (const char* target : targets) {
    string path = join_path(netstd_test_dir(), "test_xception_field.thrift");
    string name = "test_xception_field";
    map<string, string> parsed_options = {{target, ""}};
    string option_string = "";

    std::unique_ptr<t_program> program(new t_program(path, name));
    parse_thrift_for_test(program.get());

    std::unique_ptr<t_generator> gen(t_generator_registry::get_generator(program.get(), "netstd",
                                                                         parsed_options,
                                                                         option_string));
    REQUIRE(gen != nullptr);
    REQUIRE_NOTHROW(gen->generate_program());

    string generated = read_file("gen-netstd/test/xception_field/XceptionField.cs");
    REQUIRE(!generated.empty());

    // Exception members stay nullable, like struct members: no eager allocation.
    REQUIRE(generated.find("Boom? Req_boom { get; set; }") != string::npos);
    REQUIRE(generated.find("Boom? Default_boom") != string::npos);
    REQUIRE(generated.find("Boom? Opt_boom") != string::npos);
  }
}
