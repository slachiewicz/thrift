/*
 * Licensed to the Apache Software Foundation (ASF) under one
 * or more contributor license agreements. See the NOTICE file
 * distributed with this work for additional information
 * regarding copyright ownership. The ASF licenses this file
 * to you under the Apache License, Version 2.0 (the
 * "License"); you may not use this file except in compliance
 * with the License. You may obtain a copy of the License at
 *
 *   http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */


#include <boost/test/unit_test.hpp>
#include <thrift/Thrift.h>

#include <sstream>
#include <string>

// The macros must be usable in preprocessor conditionals, as in
// "#if THRIFT_VERSION_MAJOR > 0 || THRIFT_VERSION_MINOR >= 14".
#if !(THRIFT_VERSION_MAJOR > 0 || THRIFT_VERSION_MINOR >= 14)
#error "THRIFT_VERSION_MAJOR and THRIFT_VERSION_MINOR must be numeric preprocessor values"
#endif

BOOST_AUTO_TEST_SUITE(VersionTest)

BOOST_AUTO_TEST_CASE(version_macros_match_package_version) {
  std::ostringstream version;
  version << THRIFT_VERSION_MAJOR << '.' << THRIFT_VERSION_MINOR << '.' << THRIFT_VERSION_PATCH;
  BOOST_CHECK_EQUAL(version.str(), std::string(PACKAGE_VERSION));
}

BOOST_AUTO_TEST_SUITE_END()
