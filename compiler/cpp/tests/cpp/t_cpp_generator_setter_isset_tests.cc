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

// Returns the body of the out-of-line setter "S::__set_<field>(...) { ... }" from the .cpp file.
static string setter_body(const string& content, const string& field) {
  string head = "SetterIsset::__set_" + field + "(";
  size_t start = content.find(head);
  REQUIRE(start != string::npos);
  size_t end = content.find("\n}\n", start);
  REQUIRE(end != string::npos);
  return content.substr(start, end - start);
}

static void check_setters(const string& content, bool string_setter_in_cpp) {
  REQUIRE(!content.empty());

  // default requiredness: the field has an __isset flag, so the setter must set it
  REQUIRE(setter_body(content, "default_field").find("__isset.default_field = true;")
          != string::npos);
  if (string_setter_in_cpp) {
    REQUIRE(setter_body(content, "default_string").find("__isset.default_string = true;")
            != string::npos);
  }

  // optional: unchanged
  REQUIRE(setter_body(content, "optional_field").find("__isset.optional_field = true;")
          != string::npos);

  // required: no __isset flag exists
  REQUIRE(setter_body(content, "required_field").find("__isset") == string::npos);
}

TEST_CASE("t_cpp_generator setters set __isset for every non-required field", "[functional]") {
  string path = join_path(source_dir(), "test_setter_isset.thrift");
  string name = "test_setter_isset";
  map<string, string> parsed_options;
  string option_string = "";

  std::unique_ptr<t_program> program(new t_program(path, name));
  parse_thrift_for_test(program.get());

  std::unique_ptr<t_generator> gen(
      t_generator_registry::get_generator(program.get(), "cpp", parsed_options, option_string));
  REQUIRE(gen != nullptr);
  REQUIRE_NOTHROW(gen->generate_program());

  check_setters(read_file("gen-cpp/test_setter_isset_types.cpp"), true);
}

TEST_CASE("t_cpp_generator forward_setter setters set __isset for every non-required field",
          "[functional]") {
  string path = join_path(source_dir(), "test_setter_isset.thrift");
  string name = "test_setter_isset";
  map<string, string> parsed_options = {{"moveable_types", "forward_setter"}};
  string option_string = "";

  std::unique_ptr<t_program> program(new t_program(path, name));
  parse_thrift_for_test(program.get());

  std::unique_ptr<t_generator> gen(
      t_generator_registry::get_generator(program.get(), "cpp", parsed_options, option_string));
  REQUIRE(gen != nullptr);
  REQUIRE_NOTHROW(gen->generate_program());

  // only complex fields get a forwarding template setter, in the header or the .tcc file
  check_setters(read_file("gen-cpp/test_setter_isset_types.cpp"), false);
  string header = read_file("gen-cpp/test_setter_isset_types.h");
  string tcc = read_file("gen-cpp/test_setter_isset_types.tcc");
  string fwd = header + tcc;
  size_t pos = fwd.find("SetterIsset::__set_default_string(T_&& val)");
  REQUIRE(pos != string::npos);
  size_t end = fwd.find("\n}\n", pos);
  REQUIRE(fwd.substr(pos, end - pos).find("__isset.default_string = true;") != string::npos);
}
