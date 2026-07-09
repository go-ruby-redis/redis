# Ruby examples

Pure-Ruby examples for the `redis` client as provided by
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby) (rbgo). The socket
is a host seam, so the example injects an in-memory connection replaying canned
RESP replies. Run it with the `rbgo` interpreter:

```sh
rbgo examples/redis_usage.rb
```

| File | Shows |
| --- | --- |
| [`redis_usage.rb`](redis_usage.rb) | `Redis.new(connection:)` over a duck-typed socket; `ping`/`set`/`get`/`exists`, `hgetall` → Hash and `smembers` → Set, `Redis::CommandError` rescue, and `pipelined`. |

Each example is executed as-is under rbgo (`require "redis"`).
