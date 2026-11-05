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

typedef Later LaterT
typedef list<Later> LaterList

struct Holder {
  1: LaterT x
  2: LaterList xs
}

struct Later {
  1: i16 v
}

struct OrderA {
  1: OrderB b
}

struct OrderB {
  1: OrderC c
}

struct OrderC {
  1: i16 v
}

struct BrokenFirst {
  1: optional BrokenSecond & next
}

struct BrokenSecond {
  1: BrokenFirst prev
}
