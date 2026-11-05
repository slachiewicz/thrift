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

using cpp_generator_test_utils::join_path;
using cpp_generator_test_utils::parse_thrift_for_test;
using cpp_generator_test_utils::read_file;
using cpp_generator_test_utils::source_dir;
using std::map;
using std::string;

// Returns the body of the out-of-line "XceptionField::write(...) const { ... }" from the .cpp file.
static string write_body(const string& content) {
  size_t start = content.find("XceptionField::write(");
  REQUIRE(start != string::npos);
  size_t end = content.find("\n}\n", start);
  REQUIRE(end != string::npos);
  return content.substr(start, end - start);
}

TEST_CASE("t_cpp_generator write() guards exception-typed fields with __isset only when one exists",
          "[functional]") {
  string path = join_path(source_dir(), "test_xception_field.thrift");
  string name = "test_xception_field";
  map<string, string> parsed_options;
  string option_string = "";

  std::unique_ptr<t_program> program(new t_program(path, name));
  parse_thrift_for_test(program.get());

  std::unique_ptr<t_generator> gen(
      t_generator_registry::get_generator(program.get(), "cpp", parsed_options, option_string));
  REQUIRE(gen != nullptr);
  REQUIRE_NOTHROW(gen->generate_program());

  string body = write_body(read_file("gen-cpp/test_xception_field_types.cpp"));

  // required fields have no __isset member, whatever their type
  REQUIRE(body.find("__isset.req_boom") == string::npos);
  REQUIRE(body.find("__isset.req_int") == string::npos);
  // default and optional exception-typed fields keep the guard
  REQUIRE(body.find("if (this->__isset.default_boom)") != string::npos);
  REQUIRE(body.find("if (this->__isset.opt_boom)") != string::npos);
}
