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
package org.apache.thrift.partial;

import static org.junit.jupiter.api.Assertions.assertEquals;

import java.nio.ByteBuffer;
import java.nio.charset.StandardCharsets;
import java.util.Arrays;
import org.junit.jupiter.api.Test;

public class TestThriftStructProcessor {
  @Test
  public void testPrepareStringFromAnyByteBuffer() {
    byte[] data = "xxhello worldyy".getBytes(StandardCharsets.UTF_8);
    ByteBuffer slice = ByteBuffer.wrap(data, 2, 11).slice();
    ByteBuffer direct = ByteBuffer.allocateDirect(data.length).put(data);
    direct.limit(13).position(2);
    ByteBuffer readOnly = ByteBuffer.wrap(data).asReadOnlyBuffer();
    readOnly.limit(13).position(2);
    ThriftStructProcessor processor = new ThriftStructProcessor();
    for (ByteBuffer bb : Arrays.asList(slice, direct, readOnly)) {
      int pos = bb.position();
      assertEquals("hello world", processor.prepareString(bb));
      assertEquals(pos, bb.position());
    }
  }
}
