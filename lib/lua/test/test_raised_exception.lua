--
-- Licensed to the Apache Software Foundation (ASF) under one
-- or more contributor license agreements. See the NOTICE file
-- distributed with this work for additional information
-- regarding copyright ownership. The ASF licenses this file
-- to you under the Apache License, Version 2.0 (the
-- "License"); you may not use this file except in compliance
-- with the License. You may obtain a copy of the License at
--
--   http://www.apache.org/licenses/LICENSE-2.0
--
-- Unless required by applicable law or agreed to in writing,
-- software distributed under the License is distributed on an
-- "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
-- KIND, either express or implied. See the License for the
-- specific language governing permissions and limitations
-- under the License.
--
-- THRIFT-6499: a handler that raises a declared exception with error() must
-- reach the caller as that exception, and a raised value that is not a
-- declared exception (a table, a string) must reach it as a
-- TApplicationException whose message is a string. The client and the
-- processor run in one process over in-memory buffers; the processor reads
-- from and writes to the same pair of buffers for every call, so a reply left
-- half written would break the call after it.
--
-- Run after generating the code next to this file:
--   thrift -o lib/lua/test --gen lua lib/lua/test/RaisedException.thrift
--   lua lib/lua/test/test_raised_exception.lua

local script_dir = arg[0]:match('(.*[/\\])') or './'
package.path = script_dir .. '../?.lua;' ..
               script_dir .. 'gen-lua/?.lua;' .. package.path

-- Pure-Lua stand-ins for the C extensions, as in test_recursion_depth.lua.
package.preload['libluabitwise'] = function()
  return {
    bor    = function(a, b) return a | b end,
    band   = function(a, b) return a & b end,
    bxor   = function(a, b) return a ~ b end,
    shiftl = function(a, n) return (a << n) & 0xFFFFFFFF end,
    shiftr = function(a, n) return a >> n end,
  }
end
package.preload['libluabpack'] = function()
  local fmt = {c = '>i1', C = '>I1', s = '>i2', S = '>I2',
               i = '>i4', I = '>I4', l = '>i8', d = '>d'}
  return {
    bpack   = function(code, val) return string.pack(fmt[code], val) end,
    bunpack = function(code, data) return (string.unpack(fmt[code], data)) end,
  }
end
package.preload['liblualongnumber'] = function()
  return {new = function(_, v) return v or 0 end, tonumber = function(v) return v end}
end

require('Thrift')
require('TTransport')
require('TMemoryBuffer')
require('TBinaryProtocol')
require('RaisedException_Raiser')

local failures = 0
local function check(cond, what)
  if cond then
    print('ok   - ' .. what)
  else
    failures = failures + 1
    print('FAIL - ' .. what)
  end
end

-- The handler decides what to do from its argument.
local handler = RaiserIface:new{}
function handler:boom(who)
  if who == 'oops' then
    error(Oops:new{message = 'raised'})
  elseif who == 'other' then
    error(Other:new{code = 5})
  elseif who == 'returned' then
    return Oops:new{message = 'returned'}
  elseif who == 'table' then
    error({code = 1})
  elseif who == 'string' then
    error('plain failure', 0)
  end
  return 42
end
function handler:quiet()
  error(Oops:new{message = 'quiet'})
end
function handler:ping()
  return 7
end

local processor = RaiserProcessor:new{handler = handler}

-- The client's transport. flush() hands what was written to the processor and
-- keeps its reply for the client to read.
local serverIn = TMemoryBuffer:new{}
local serverOut = TMemoryBuffer:new{}
local serverIprot = TBinaryProtocol:new{trans = serverIn}
local serverOprot = TBinaryProtocol:new{trans = serverOut}
local processorErrors = {}

local transport = TMemoryBuffer:new{}
local clientIn = TMemoryBuffer:new{}
transport.read = function(_, len) return clientIn:read(len) end
transport.readAll = function(_, len) return clientIn:readAll(len) end
transport.flush = function(self)
  serverIn:resetBuffer(self.buffer)
  self:resetBuffer('')
  local ok, err = pcall(processor.process, processor, serverIprot, serverOprot)
  if not ok then
    processorErrors[#processorErrors + 1] = tostring(err)
  end
  clientIn:resetBuffer(serverOut:getBuffer())
  serverOut:resetBuffer('')
end

local protocol = TBinaryProtocol:new{trans = transport}
local client = RaiserClient:new{protocol = protocol}

local function call(fn, ...)
  local ok, err = pcall(fn, client, ...)
  return ok, err
end

local function replyIsConsumed()
  return clientIn:available() == 0
end

local ok, err = call(client.boom, 'oops')
check(not ok and ttype(err) == 'Oops' and err.message == 'raised',
      'a declared exception raised with error() reaches the client: ' .. tostring(err))
check(#processorErrors == 0, 'the processor did not fail: ' .. table.concat(processorErrors, '; '))
check(replyIsConsumed(), 'the whole reply was read')

ok, err = call(client.boom, 'other')
check(not ok and ttype(err) == 'Other' and err.code == 5,
      'the second declared exception raised with error() reaches the client: ' .. tostring(err))

ok, err = call(client.boom, 'returned')
check(not ok and ttype(err) == 'Oops' and err.message == 'returned',
      'a declared exception that is returned still reaches the client: ' .. tostring(err))

ok, err = call(client.boom, 'table')
check(not ok and ttype(err) == 'TApplicationException' and type(err.message) == 'string'
      and err.message:find('^table: ') ~= nil,
      'a raised table that is not a declared exception becomes a TApplicationException ' ..
      'with a string message: ' .. tostring(err))
check(#processorErrors == 0, 'the processor did not fail: ' .. table.concat(processorErrors, '; '))
check(replyIsConsumed(), 'the whole reply was read')

ok, err = call(client.ping)
check(ok and err == 7, 'the connection serves the next call after a raised table: ' .. tostring(err))

ok, err = call(client.boom, 'string')
check(not ok and ttype(err) == 'TApplicationException' and err.message == 'plain failure',
      'a raised string becomes a TApplicationException with that message: ' .. tostring(err))

ok, err = call(client.boom, 'ok')
check(ok and err == 42, 'the connection serves the next call after a raised string: ' .. tostring(err))

-- The generated client does not raise declared exceptions of a void function
-- here, so read the reply of quiet() directly.
client:send_quiet()
local fname, mtype = protocol:readMessageBegin()
local result = quiet_result:new{}
result:read(protocol)
protocol:readMessageEnd()
check(fname == 'quiet' and mtype == TMessageType.REPLY and ttype(result.oops) == 'Oops'
      and result.oops.message == 'quiet',
      'a declared exception raised by a void function is sent as a REPLY')
check(#processorErrors == 0, 'the processor did not fail: ' .. table.concat(processorErrors, '; '))

ok, err = call(client.ping)
check(ok and err == 7, 'the connection serves the next call after a void function raised')

if failures > 0 then
  print(failures .. ' failure(s)')
  os.exit(1)
end
print('all ok')
