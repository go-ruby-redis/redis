<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-redis/brand/main/social/go-ruby-redis-redis.png" alt="go-ruby-redis/redis" width="720"></p>

# redis — go-ruby-redis

[![Docs](https://img.shields.io/badge/docs-mkdocs--material-DC2626)](https://go-ruby-redis.github.io/docs/)
[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#tests--coverage)

**A pure-Go (no cgo) reimplementation of the [`redis`](https://github.com/redis/redis-rb) /
[`redis-client`](https://github.com/redis-rb/redis-client) gem's RESP protocol
codec and command layer** — the deterministic, interpreter-independent core that
encodes a Redis command to the RESP wire format and decodes every RESP2 and RESP3
reply into a small Ruby value model, then coerces each command's reply to the
type the gem returns (a `Hash` for `HGETALL`, a `Set` for `SMEMBERS`, an `Integer`
for `EXISTS`, a `Float` for `ZSCORE`). It builds and parses the wire **without any
Ruby runtime and without a live redis-server** — you feed it bytes.

It is the Redis backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module — a sibling of
[go-ruby-regexp](https://github.com/go-ruby-regexp/regexp) and
[go-ruby-marshal](https://github.com/go-ruby-marshal/marshal).

> **What it is — and isn't.** The RESP grammar and the command→reply-type mapping
> are fully deterministic and need **no interpreter and no server**, so they live
> here as pure Go. The actual TCP/TLS **socket is a host seam**, mirroring the
> go-ruby-net-\* pattern: a [`Client`](client.go) drives its commands over an
> injected `Conn` (an `io.ReadWriter`) or a `RoundTripper`, and the host owns the
> connection, timeouts and TLS.

## Features

- **RESP encode** — serialise a command (an array of bulk strings) to the
  multi-bulk wire form `*N\r\n$len\r\n…`, plus the legacy inline form. Every
  argument kind the gem accepts (String, Integer, Float, Symbol, big integers) is
  stringified exactly as the gem does.
- **RESP decode** — parse **every** reply type:
  - **RESP2**: simple string `+`, error `-`, integer `:`, bulk string `$` (incl.
    null `$-1`), array `*` (incl. null `*-1` and arbitrary nesting).
  - **RESP3**: null `_`, double `,` (incl. `inf`/`-inf`/`nan`), boolean `#`,
    verbatim string `=`, big number `(`, blob error `!`, map `%`, set `~`, push
    `>`, and attribute `|` (out-of-band metadata carried onto the next reply).
- **Value model** — `nil` / `string` / `int64` / `float64` / `bool` / `*big.Int`
  / `[]any` / ordered `*Map` (Hash) / `*Set` / `*Push` / `*VerbatimString` /
  `*CommandError`.
- **Command layer** — the gem's method surface, each building a RESP command and
  coercing its reply to the gem's type: strings (`GET`/`SET`/`INCR`/`APPEND`/
  `GETRANGE`/`MGET`…), hashes (`HGETALL`→`Hash`…), lists, sets (`SMEMBERS`→`Set`…),
  sorted sets (with `Float` score parsing), keys (`EXPIRE`/`TTL`/`TYPE`/`SCAN`→
  cursor+keys), server (`PING`/`SELECT`/`AUTH`/`HELLO`/`CONFIG GET`→`Hash`),
  pub/sub message framing, `MULTI`/`EXEC` transactions and pipelining.
- **Generic escape hatch** — `Call([...])` for any command, mirroring
  `redis.call`.
- **CGO-free** and validated on all six supported 64-bit targets (amd64, arm64,
  riscv64, loong64, ppc64le, s390x) across Linux, macOS and Windows.

## Usage

```go
import "github.com/go-ruby-redis/redis"

// The socket is the host's; wire any io.ReadWriter (a *net.Conn, a TLS conn, …).
c := redis.NewFromConn(conn)

c.Set("greeting", "hello")             // -> "OK"
v, _ := c.Get("greeting")              // -> "hello"
c.HSet("h", "a", "1", "b", "2")
h, _ := c.HGetAll("h")                 // -> *redis.Map{ "a"=>"1", "b"=>"2" }
c.SAdd("s", "x", "y")
s, _ := c.SMembers("s")                // -> *redis.Set{ "x", "y" }

// Pipelines and transactions batch commands over one write.
res, _ := c.Pipelined(func(b *redis.Batch) {
    b.Add("INCR", "n").Add("INCR", "n")
})                                     // -> []any{int64(1), int64(2)}

// Or drive the wire codec directly, no client needed:
req := redis.EncodeCommand("GET", "foo")   // -> *2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n
val, _ := redis.DecodeBytes([]byte("$5\r\nhello\r\n")) // -> "hello"
```

## Tests & coverage

The deterministic, ruby-free suite (golden RESP byte vectors built from the spec,
plus a scripted `Conn`/`RoundTripper` for the command layer) holds **100%
statement coverage on its own**, so every CI lane passes the gate. A differential
**oracle** additionally validates the codec against the reference implementation —
the `redis-client` gem's `RedisClient::RESP3.dump` / `.parse` (no live server
needed) — on the lanes where Ruby ≥ 4.0 with the gem is present; it skips itself
elsewhere.

```sh
GOWORK=off go test ./...
```

## License

BSD-3-Clause — see [LICENSE](LICENSE). Copyright the go-ruby-redis/redis authors.
