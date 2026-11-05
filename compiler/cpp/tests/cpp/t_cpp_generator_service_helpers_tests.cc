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

namespace {

string generate_service_cpp(const map<string, string>& parsed_options) {
  string path = join_path(source_dir(), "test_service_helpers.thrift");
  std::unique_ptr<t_program> program(new t_program(path, "test_service_helpers"));
  parse_thrift_for_test(program.get());

  std::unique_ptr<t_generator> gen(
      t_generator_registry::get_generator(program.get(), "cpp", parsed_options, ""));
  REQUIRE(gen != nullptr);
  REQUIRE_NOTHROW(gen->generate_program());

  string content = read_file("gen-cpp/HelperService.cpp");
  REQUIRE(!content.empty());
  return content;
}

bool has(const string& haystack, const string& needle) {
  return haystack.find(needle) != string::npos;
}

// The header declares these members for every args and result helper, so the
// .cpp has to define them or any use fails at link time.
void require_value_helpers(const string& cpp, const string& cls, bool moveable) {
  REQUIRE(has(cpp, cls + "::" + cls + "(const " + cls + "&"));
  REQUIRE(has(cpp, cls + "& " + cls + "::operator=(const " + cls + "&"));
  REQUIRE(has(cpp, "bool " + cls + "::operator==(const " + cls + " &"));
  REQUIRE(has(cpp, cls + "::" + cls + "(" + cls + "&&") == moveable);
  REQUIRE(has(cpp, cls + "& " + cls + "::operator=(" + cls + "&&") == moveable);
}

} // namespace

TEST_CASE("t_cpp_generator defines value members of service args and result helpers",
          "[functional]") {
  string cpp = generate_service_cpp({});

  require_value_helpers(cpp, "HelperService_echo_args", false);
  require_value_helpers(cpp, "HelperService_echo_result", false);
  require_value_helpers(cpp, "HelperService_ping_args", false);
  require_value_helpers(cpp, "HelperService_ping_result", false);
  require_value_helpers(cpp, "HelperService_fire_args", false);

  REQUIRE(has(cpp, "void HelperService_echo_args::__set_p("));
  REQUIRE(has(cpp, "void HelperService_echo_args::__set_names("));
  REQUIRE(has(cpp, "void HelperService_echo_args::__set_text("));
  REQUIRE(has(cpp, "void HelperService_echo_result::__set_success("));
  REQUIRE(has(cpp, "void HelperService_echo_result::__set_oops("));
  REQUIRE(has(cpp, "void HelperService_ping_args::__set_n("));

  // Pointer-based helpers declare none of these members.
  REQUIRE(!has(cpp, "HelperService_echo_pargs::HelperService_echo_pargs(const"));
  REQUIRE(!has(cpp, "HelperService_echo_presult::HelperService_echo_presult(const"));
  REQUIRE(!has(cpp, "HelperService_echo_pargs::operator=("));
}

TEST_CASE("t_cpp_generator defines move members of service helpers with moveable_types",
          "[functional]") {
  string cpp = generate_service_cpp({{"moveable_types", ""}});

  require_value_helpers(cpp, "HelperService_echo_args", true);
  require_value_helpers(cpp, "HelperService_echo_result", true);
  REQUIRE(has(cpp, "void HelperService_echo_args::__set_names("));
}

TEST_CASE("t_cpp_generator keeps service helper members out of the .tcc with templates",
          "[functional]") {
  string cpp = generate_service_cpp({{"templates", ""}});
  require_value_helpers(cpp, "HelperService_echo_args", false);
  require_value_helpers(cpp, "HelperService_echo_result", false);
  REQUIRE(has(cpp, "void HelperService_echo_args::__set_p("));

  string tcc = read_file("gen-cpp/HelperService.tcc");
  REQUIRE(!has(tcc, "HelperService_echo_args::__set_p("));
  REQUIRE(!has(tcc, "HelperService_echo_args::HelperService_echo_args(const"));
}

TEST_CASE("t_cpp_generator omits value members of service helpers with no_constructors",
          "[functional]") {
  string cpp = generate_service_cpp({{"no_constructors", ""}, {"no_default_operators", ""}});
  REQUIRE(!has(cpp, "HelperService_echo_args::HelperService_echo_args(const"));
  REQUIRE(!has(cpp, "HelperService_echo_args::operator=("));
  REQUIRE(!has(cpp, "HelperService_echo_args::operator==("));
  REQUIRE(has(cpp, "void HelperService_echo_args::__set_p("));
}

TEST_CASE("t_cpp_generator defines forwarding setters of service helpers in the header",
          "[functional]") {
  string cpp = generate_service_cpp({{"moveable_types", "forward_setter"}});
  require_value_helpers(cpp, "HelperService_echo_args", true);
  require_value_helpers(cpp, "HelperService_echo_result", true);

  // Complex types get template setters, defined out of class in the header.
  string header = read_file("gen-cpp/HelperService.h");
  REQUIRE(has(header, "void HelperService_echo_args::__set_p(T_&& val) {"));
  REQUIRE(has(header, "void HelperService_echo_args::__set_names(T_&& val) {"));
  REQUIRE(has(header, "void HelperService_echo_args::__set_text(T_&& val) {"));
  REQUIRE(has(header, "void HelperService_echo_result::__set_success(T_&& val) {"));
  REQUIRE(has(header, "void HelperService_echo_result::__set_oops(T_&& val) {"));

  // Primitive setters stay ordinary functions in the .cpp.
  REQUIRE(has(cpp, "void HelperService_ping_args::__set_n("));
  REQUIRE(!has(cpp, "HelperService_echo_args::__set_p("));
}
