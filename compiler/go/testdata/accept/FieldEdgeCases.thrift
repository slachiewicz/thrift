#
# Licensed to the Apache Software Foundation (ASF) under one
# or more contributor license agreements. See the NOTICE file
# distributed with this work for additional information
# regarding copyright ownership. The ASF licenses this file
# to you under the Apache License, Version 2.0 (the
# "License"); you may not use this file except in compliance
# with the License. You may obtain a copy of the License at
#
#   http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing,
# software distributed under the License is distributed on an
# "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
# KIND, either express or implied. See the License for the
# specific language governing permissions and limitations
# under the License.
#
# Edge cases the unit tests found that the shipped corpus does not cover.
# Every file here must be accepted by both compilers with identical output.

# Field ids: implicit ones count down from -1 per field list.
struct Implicit {
  i32 a
  5: i32 b
  i32 c
  0: i32 d
  -2: i32 e
}

exception Err {
  1: i32 code
}

service Args {
  void f(i32 a, i32 b, 1: i32 c, i32 d)
  void g(1: i32 a) throws (Err e)
}

# Keywords as field names, and a bare sign as a double constant.
struct Keywords {
  1: i32 string
  2: i32 list
  3: i32 service
  4: i32 include
  5: double minus = -
  6: double plus = +
  7: i32 hex = -0x10
  8: string quoted = 'single'
}

# Forward typedef resolved on first use.
typedef Later T
struct Later {
  1: i32 x
}
struct UsesT {
  1: T t
  2: list<T> ts
}

# Trailing cpp_type on a list and annotations everywhere.
struct Cpp {
  1: list<i32> cpp_type "vec" a
  2: map cpp_type "umap" <string (a = "1"), i32> b (c)
  3: set cpp_type "hs" <i32> (d = "2") c
}
