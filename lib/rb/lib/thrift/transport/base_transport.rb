# encoding: ascii-8bit
# frozen_string_literal: true
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

module Thrift
  class TransportException < Exception
    UNKNOWN = 0
    NOT_OPEN = 1
    ALREADY_OPEN = 2
    TIMED_OUT = 3
    END_OF_FILE = 4
    NEGATIVE_SIZE = 5
    SIZE_LIMIT = 6

    attr_reader :type

    def initialize(type = UNKNOWN, message = nil)
      super(message)
      @type = type
    end

    # TransportException.new(TYPE, message) returns an instance of the subclass
    # registered for TYPE, so callers can rescue a specific condition by class
    # while code that rescues TransportException and reads #type keeps working.
    def self.new(*args, &block)
      if equal?(TransportException)
        klass = SUBCLASSES[args.fetch(0, UNKNOWN)]
        return klass.new(*args, &block) if klass
      end
      super
    end

    # Base for the per-type subclasses. Accepts (message) or (type, message);
    # the type defaults to the one the subclass stands for.
    class Typed < TransportException
      def initialize(type_or_message = nil, message = nil)
        if type_or_message.is_a?(Integer)
          super(type_or_message, message)
        else
          super(self.class::TYPE, type_or_message)
        end
      end
    end

    class NotOpen < Typed
      TYPE = NOT_OPEN
    end

    class AlreadyOpen < Typed
      TYPE = ALREADY_OPEN
    end

    class TimedOut < Typed
      TYPE = TIMED_OUT
    end

    class EndOfFile < Typed
      TYPE = END_OF_FILE
    end

    class NegativeSize < Typed
      TYPE = NEGATIVE_SIZE
    end

    class SizeLimit < Typed
      TYPE = SIZE_LIMIT
    end

    class Unknown < Typed
      TYPE = UNKNOWN
    end

    SUBCLASSES = {
      NOT_OPEN => NotOpen,
      ALREADY_OPEN => AlreadyOpen,
      TIMED_OUT => TimedOut,
      END_OF_FILE => EndOfFile,
      NEGATIVE_SIZE => NegativeSize,
      SIZE_LIMIT => SizeLimit,
      UNKNOWN => Unknown,
    }.freeze
  end

  module TransportUtils
    # Deprecated: Use Thrift::Bytes instead
    def self.get_string_byte(string, index)
      Bytes.get_string_byte(string, index)
    end

    # Deprecated: Use Thrift::Bytes instead
    def self.set_string_byte(string, index, byte)
      Bytes.set_string_byte(string, index, byte)
    end
  end

  class BaseTransport
    def open?; end

    def open; end

    def close; end

    # Returns true when the transport needs protocol message boundary notifications.
    def message_boundaries?
      false
    end

    # Reads a number of bytes from the transports. The String returned will have a BINARY (aka ASCII-8BIT) encoding.
    #
    # sz - The number of bytes to read from the transport.
    #
    # Returns a String acting as a byte buffer.
    def read(sz)
      raise NotImplementedError
    end

    # Returns an unsigned byte as a Integer in the range (0..255).
    def read_byte
      buf = read_all(1)
      Bytes.get_string_byte(buf, 0)
    end

    # Reads size bytes and copies them into buffer[0..size].
    def read_into_buffer(buffer, size)
      tmp = read_all(size)
      i = 0
      tmp.each_byte do |byte|
        Bytes.set_string_byte(buffer, i, byte)
        i += 1
      end
      i
    end

    def read_all(size)
      raise TransportException.new(TransportException::NEGATIVE_SIZE, "Negative size") unless size >= 0
      return Bytes.empty_byte_buffer if size == 0

      buf = read(size)
      if buf.nil? || buf.empty?
        raise TransportException.new(TransportException::END_OF_FILE, "No more data available")
      end

      buf = Bytes.force_binary_encoding(buf)
      return buf if buf.length >= size

      buf = buf.dup if buf.frozen?
      while (buf.length < size)
        chunk = read(size - buf.length)
        if chunk.nil? || chunk.empty?
          raise TransportException.new(TransportException::END_OF_FILE, "No more data available")
        end

        buf << Bytes.force_binary_encoding(chunk)
      end

      buf
    end

    # Writes the byte buffer to the transport. The buffer will be forced into BINARY encoding.
    #
    # buf - A String acting as a byte buffer.
    #
    # Returns nothing.
    def write(buf); end
    alias_method :<<, :write

    def flush; end

    def to_s
      "base"
    end
  end

  class BaseTransportFactory
    def get_transport(trans)
      trans
    end

    def to_s
      "base"
    end
  end
end
