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

namespace go edge

enum E {
  A = 1
  B
  C = -3
  D
}

# A container constant named by identifier stays a reference.
const list<i32> L = [1, 2]
const list<i32> L2 = L
const map<string, i32> M = {"k": 1}
const map<string, i32> M2 = M
const set<i32> S = [1]
const set<i32> S2 = S

# A string for an enum type is read as the integer 0 and mapped to the
# member with that value; validate_const_rec never sees it.
enum Z {
  ZERO = 0
}
const Z STRING_FOR_ENUM = "s"

# Integers of enum type map back to the member name.
const E FROM_INT = 2
const list<E> ENUM_LIST = [E.A, 2, -3]
const map<E, E> ENUM_MAP = {1: E.B, 2: -3}

# An integer is accepted for a double.
const double INT_AS_DOUBLE = 1

# Nested containers and struct constants resolved field-wise.
struct Inner {
  1: E e
  2: list<E> es
}
struct Outer {
  1: Inner inner
  2: map<string, Inner> inners
  3: list<i32> l = L
  4: E e = 1
}
const Outer OUTER = {"inner": {"e": 1, "es": [1, E.B]}, "inners": {"x": {"e": E.C}}, "l": [3]}

# Exceptions are allowed as field and container types.
exception Err {
  1: i32 code
}
struct HoldsErr {
  1: Err e
  2: list<Err> es
  3: map<string, Err> em
  4: set<Err> ess
  5: optional Err oe
}
service ErrService {
  Err echo(1: Err e) throws (1: Err err)
}
