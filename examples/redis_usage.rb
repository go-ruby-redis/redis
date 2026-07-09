# frozen_string_literal: true
#
# Pure-Ruby usage of the Redis client, as provided by go-embedded-ruby (rbgo).
# Run it with:  rbgo examples/redis_usage.rb
#
# The RESP codec and command layer are deterministic and need no live server:
# the socket is a host seam. Redis.new drives its commands over any object that
# responds to #read / #write, so here we inject a tiny in-memory connection that
# replays a canned RESP reply stream.

require "redis"

# A duck-typed connection: #write captures the RESP the client sends, #read
# drains the canned replies (here: +PONG, +OK, a bulk string, an integer).
class FakeConnection
  def initialize(reply) ; @in = reply.dup.force_encoding("ASCII-8BIT") ; @pos = 0 ; end
  def write(s) ; s.bytesize ; end
  def read(n = nil)
    avail = @in.bytesize - @pos
    return "".b if avail <= 0
    n = avail if n.nil? || n > avail
    chunk = @in.byteslice(@pos, n) ; @pos += n ; chunk
  end
end

r = Redis.new(connection: FakeConnection.new("+PONG\r\n+OK\r\n$5\r\nworld\r\n:1\r\n"))
p r.ping                # => "PONG"  (simple string)
p r.set("hello", "x")   # => "OK"
p r.get("hello")        # => "world" (bulk string)
p r.exists("hello")     # => 1       (integer)

# HGETALL: a RESP map is coerced to a Ruby Hash, SMEMBERS to a Set.
h = Redis.new(connection: FakeConnection.new("%1\r\n$4\r\nname\r\n$3\r\nAda\r\n"))
p h.hgetall("user:1")   # => {"name" => "Ada"}
s = Redis.new(connection: FakeConnection.new("*2\r\n$1\r\na\r\n$1\r\nb\r\n"))
p s.smembers("tags").class # => Set

# A Redis error reply is raised as Redis::CommandError.
e = Redis.new(connection: FakeConnection.new("-WRONGTYPE nope\r\n"))
begin
  e.get("k")
rescue Redis::CommandError => err
  puts "error: #{err.message}" # => error: WRONGTYPE nope
end

# #pipelined batches commands into one write; replies come back as an Array.
pl = Redis.new(connection: FakeConnection.new("$1\r\na\r\n$1\r\nb\r\n"))
p pl.pipelined { |p| p.get("x"); p.get("y") } # => ["a", "b"]
