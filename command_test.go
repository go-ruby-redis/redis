// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import (
	"errors"
	"reflect"
	"testing"
)

// TestStringCommands covers the string family and its coercions.
func TestStringCommands(t *testing.T) {
	runCmd(t, "$5\r\nhello\r\n", "*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n",
		func(c *Client) (any, error) { return c.Get("foo") }, "hello")
	runCmd(t, "$-1\r\n", "", func(c *Client) (any, error) { return c.Get("missing") }, nil)
	runCmd(t, "+OK\r\n", "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n",
		func(c *Client) (any, error) { return c.Set("k", "v") }, "OK")
	runCmd(t, "+OK\r\n", "*5\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n$2\r\nEX\r\n$2\r\n10\r\n",
		func(c *Client) (any, error) { return c.Set("k", "v", "EX", 10) }, "OK")
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.SetNX("k", "v") }, true)
	runCmd(t, ":0\r\n", "", func(c *Client) (any, error) { return c.SetNX("k", "v") }, false)
	runCmd(t, "$3\r\nold\r\n", "", func(c *Client) (any, error) { return c.GetSet("k", "v") }, "old")
	runCmd(t, ":8\r\n", "", func(c *Client) (any, error) { return c.Append("k", "v") }, int64(8))
	runCmd(t, ":5\r\n", "", func(c *Client) (any, error) { return c.Strlen("k") }, int64(5))
	runCmd(t, "$3\r\nell\r\n", "", func(c *Client) (any, error) { return c.GetRange("k", 1, 3) }, "ell")
	runCmd(t, ":7\r\n", "", func(c *Client) (any, error) { return c.SetRange("k", 2, "x") }, int64(7))
	runCmd(t, ":6\r\n", "", func(c *Client) (any, error) { return c.Incr("k") }, int64(6))
	runCmd(t, ":10\r\n", "", func(c *Client) (any, error) { return c.IncrBy("k", 4) }, int64(10))
	runCmd(t, "$4\r\n3.14\r\n", "", func(c *Client) (any, error) { return c.IncrByFloat("k", 3.14) }, 3.14)
	runCmd(t, ":4\r\n", "", func(c *Client) (any, error) { return c.Decr("k") }, int64(4))
	runCmd(t, ":2\r\n", "", func(c *Client) (any, error) { return c.DecrBy("k", 2) }, int64(2))
	runCmd(t, "*2\r\n$1\r\na\r\n$-1\r\n", "", func(c *Client) (any, error) { return c.MGet("x", "y") }, []any{"a", nil})
	runCmd(t, "+OK\r\n", "", func(c *Client) (any, error) { return c.MSet("a", "1", "b", "2") }, "OK")
}

// TestKeyCommands covers the key family.
func TestKeyCommands(t *testing.T) {
	runCmd(t, ":2\r\n", "", func(c *Client) (any, error) { return c.Del("a", "b") }, int64(2))
	runCmd(t, ":1\r\n", "*2\r\n$6\r\nEXISTS\r\n$1\r\nk\r\n", func(c *Client) (any, error) { return c.Exists("k") }, int64(1))
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.Expire("k", 60) }, true)
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.ExpireAt("k", 99999) }, true)
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.Persist("k") }, true)
	runCmd(t, ":-1\r\n", "", func(c *Client) (any, error) { return c.TTL("k") }, int64(-1))
	runCmd(t, ":5000\r\n", "", func(c *Client) (any, error) { return c.PTTL("k") }, int64(5000))
	runCmd(t, "+string\r\n", "", func(c *Client) (any, error) { return c.Type("k") }, "string")
	runCmd(t, "+OK\r\n", "", func(c *Client) (any, error) { return c.Rename("a", "b") }, "OK")
	runCmd(t, "*2\r\n$1\r\na\r\n$1\r\nb\r\n", "", func(c *Client) (any, error) { return c.Keys("*") }, []any{"a", "b"})
}

func TestScan(t *testing.T) {
	m := newMockConn("*2\r\n$2\r\n17\r\n*2\r\n$3\r\nkey\r\n$4\r\nkey2\r\n")
	c := NewFromConn(m)
	got, err := c.Scan("0", "MATCH", "key*", "COUNT", 100)
	if err != nil {
		t.Fatal(err)
	}
	sr := got.(*ScanResult)
	if sr.Cursor != "17" || sr.Done() {
		t.Fatalf("cursor %q done %v", sr.Cursor, sr.Done())
	}
	if !reflect.DeepEqual(sr.Elements, []any{"key", "key2"}) {
		t.Fatalf("elems %#v", sr.Elements)
	}
	// Completed scan.
	c = NewFromConn(newMockConn("*2\r\n$1\r\n0\r\n*0\r\n"))
	got, _ = c.Scan("17")
	if !got.(*ScanResult).Done() {
		t.Fatal("want done")
	}
}

// TestHashCommands covers the hash family, including the Hash coercion.
func TestHashCommands(t *testing.T) {
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.HSet("h", "f", "v") }, int64(1))
	runCmd(t, "$1\r\nv\r\n", "", func(c *Client) (any, error) { return c.HGet("h", "f") }, "v")
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.HDel("h", "f") }, int64(1))
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.HExists("h", "f") }, true)
	runCmd(t, "*1\r\n$1\r\nf\r\n", "", func(c *Client) (any, error) { return c.HKeys("h") }, []any{"f"})
	runCmd(t, "*1\r\n$1\r\nv\r\n", "", func(c *Client) (any, error) { return c.HVals("h") }, []any{"v"})
	runCmd(t, ":2\r\n", "", func(c *Client) (any, error) { return c.HLen("h") }, int64(2))
	runCmd(t, ":5\r\n", "", func(c *Client) (any, error) { return c.HIncrBy("h", "f", 5) }, int64(5))
	runCmd(t, "$3\r\n1.5\r\n", "", func(c *Client) (any, error) { return c.HIncrByFloat("h", "f", 1.5) }, 1.5)
	runCmd(t, "*2\r\n$1\r\na\r\n$-1\r\n", "", func(c *Client) (any, error) { return c.HMGet("h", "f1", "f2") }, []any{"a", nil})

	m := newMockConn("*4\r\n$1\r\na\r\n$1\r\n1\r\n$1\r\nb\r\n$1\r\n2\r\n")
	c := NewFromConn(m)
	got, err := c.HGetAll("h")
	if err != nil {
		t.Fatal(err)
	}
	h := got.(*Map)
	if v, _ := h.Get("a"); v != "1" {
		t.Fatalf("a = %v", v)
	}
	if h.Keys()[0] != "a" || h.Keys()[1] != "b" {
		t.Fatalf("order %v", h.Keys())
	}
}

func TestListCommands(t *testing.T) {
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.LPush("l", "a") }, int64(1))
	runCmd(t, ":2\r\n", "", func(c *Client) (any, error) { return c.RPush("l", "b") }, int64(2))
	runCmd(t, "$1\r\na\r\n", "", func(c *Client) (any, error) { return c.LPop("l") }, "a")
	runCmd(t, "$1\r\nb\r\n", "", func(c *Client) (any, error) { return c.RPop("l") }, "b")
	runCmd(t, ":3\r\n", "", func(c *Client) (any, error) { return c.LLen("l") }, int64(3))
	runCmd(t, "*2\r\n$1\r\na\r\n$1\r\nb\r\n", "", func(c *Client) (any, error) { return c.LRange("l", 0, -1) }, []any{"a", "b"})
	runCmd(t, "$1\r\nx\r\n", "", func(c *Client) (any, error) { return c.LIndex("l", 0) }, "x")
	runCmd(t, "+OK\r\n", "", func(c *Client) (any, error) { return c.LSet("l", 0, "y") }, "OK")
	runCmd(t, "+OK\r\n", "", func(c *Client) (any, error) { return c.LTrim("l", 0, 1) }, "OK")
}

// TestSetCommands covers the set family and its Set coercion.
func TestSetCommands(t *testing.T) {
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.SAdd("s", "a") }, int64(1))
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.SRem("s", "a") }, int64(1))
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.SIsMember("s", "a") }, true)
	runCmd(t, ":3\r\n", "", func(c *Client) (any, error) { return c.SCard("s") }, int64(3))

	for _, tc := range []struct {
		name string
		fn   func(*Client) (any, error)
	}{
		{"SMembers", func(c *Client) (any, error) { return c.SMembers("s") }},
		{"SUnion", func(c *Client) (any, error) { return c.SUnion("a", "b") }},
		{"SInter", func(c *Client) (any, error) { return c.SInter("a", "b") }},
		{"SDiff", func(c *Client) (any, error) { return c.SDiff("a", "b") }},
	} {
		c := NewFromConn(newMockConn("*2\r\n$1\r\nx\r\n$1\r\ny\r\n"))
		got, err := tc.fn(c)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		s, ok := got.(*Set)
		if !ok || !s.Include("x") || !s.Include("y") || s.Len() != 2 {
			t.Fatalf("%s: %#v", tc.name, got)
		}
	}
}

func TestSortedSetCommands(t *testing.T) {
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.ZAdd("z", 1.0, "a") }, int64(1))
	runCmd(t, "$3\r\n2.5\r\n", "", func(c *Client) (any, error) { return c.ZScore("z", "a") }, 2.5)
	runCmd(t, "$-1\r\n", "", func(c *Client) (any, error) { return c.ZScore("z", "missing") }, nil)
	runCmd(t, "*2\r\n$1\r\na\r\n$1\r\nb\r\n", "", func(c *Client) (any, error) { return c.ZRange("z", 0, -1) }, []any{"a", "b"})
	runCmd(t, ":0\r\n", "", func(c *Client) (any, error) { return c.ZRank("z", "a") }, int64(0))
	runCmd(t, ":5\r\n", "", func(c *Client) (any, error) { return c.ZCard("z") }, int64(5))
	runCmd(t, "$3\r\n3.0\r\n", "", func(c *Client) (any, error) { return c.ZIncrBy("z", 1, "a") }, 3.0)
	runCmd(t, ":1\r\n", "", func(c *Client) (any, error) { return c.ZRem("z", "a") }, int64(1))
	runCmd(t, "*3\r\n$1\r\na\r\n$3\r\n1.5\r\n$1\r\nb\r\n", "",
		func(c *Client) (any, error) { return c.ZRange("z", 0, -1, "WITHSCORES") }, []any{"a", "1.5", "b"})
}

func TestServerCommands(t *testing.T) {
	runCmd(t, "+PONG\r\n", "", func(c *Client) (any, error) { return c.Ping() }, "PONG")
	runCmd(t, "$2\r\nhi\r\n", "", func(c *Client) (any, error) { return c.Ping("hi") }, "hi")
	runCmd(t, "$2\r\nhi\r\n", "", func(c *Client) (any, error) { return c.Echo("hi") }, "hi")
	runCmd(t, "+OK\r\n", "", func(c *Client) (any, error) { return c.Select(1) }, "OK")
	runCmd(t, "+OK\r\n", "", func(c *Client) (any, error) { return c.Auth("pw") }, "OK")
	runCmd(t, "+OK\r\n", "", func(c *Client) (any, error) { return c.FlushDB() }, "OK")

	m := newMockConn("*2\r\n$6\r\nmaxmem\r\n$1\r\n0\r\n")
	c := NewFromConn(m)
	got, err := c.ConfigGet("maxmem")
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := got.(*Map).Get("maxmem"); v != "0" {
		t.Fatalf("config = %v", v)
	}
}

func TestHelloAndHandshake(t *testing.T) {
	// HELLO 3 flips to RESP3.
	hello := "%3\r\n$6\r\nserver\r\n$5\r\nredis\r\n$5\r\nproto\r\n:3\r\n$2\r\nid\r\n:7\r\n"
	c := NewFromConn(newMockConn(hello))
	got, err := c.Hello(3)
	if err != nil {
		t.Fatal(err)
	}
	if !c.IsRESP3() {
		t.Fatal("want resp3")
	}
	if v, _ := got.(*Map).Get("proto"); v != int64(3) {
		t.Fatalf("proto %v", v)
	}

	// Handshake RESP3 with auth + db select: HELLO 3 AUTH, then SELECT.
	m := newMockConn(hello + "+OK\r\n")
	c, _ = New(m, Options{Protocol: 3, Password: "pw", DB: 2})
	if err := c.Handshake(); err != nil {
		t.Fatal(err)
	}
	// The request must include HELLO 3 AUTH default pw and SELECT 2.
	if !containsAll(m.writes.String(), "HELLO", "AUTH", "default", "pw", "SELECT") {
		t.Fatalf("handshake wrote %q", m.writes.String())
	}

	// Handshake RESP3 with username.
	m = newMockConn(hello + "+OK\r\n")
	c, _ = New(m, Options{Protocol: 3, Username: "alice", Password: "pw", DB: 1})
	if err := c.Handshake(); err != nil {
		t.Fatal(err)
	}
	if !containsAll(m.writes.String(), "alice") {
		t.Fatalf("username missing: %q", m.writes.String())
	}

	// Handshake RESP2 with password only.
	m = newMockConn("+OK\r\n")
	c, _ = New(m, Options{Password: "pw"})
	if err := c.Handshake(); err != nil {
		t.Fatal(err)
	}
	if !containsAll(m.writes.String(), "AUTH", "pw") {
		t.Fatalf("resp2 auth: %q", m.writes.String())
	}

	// Handshake RESP2 with username+password.
	m = newMockConn("+OK\r\n")
	c, _ = New(m, Options{Username: "u", Password: "p"})
	if err := c.Handshake(); err != nil {
		t.Fatal(err)
	}
	if !containsAll(m.writes.String(), "u", "p") {
		t.Fatalf("resp2 acl auth: %q", m.writes.String())
	}

	// Handshake with no auth, no db: no writes at all.
	m = newMockConn("")
	c, _ = New(m, Options{})
	if err := c.Handshake(); err != nil {
		t.Fatal(err)
	}
	if m.writes.Len() != 0 {
		t.Fatalf("unexpected write %q", m.writes.String())
	}
}

func TestHandshakeErrors(t *testing.T) {
	// HELLO fails.
	c, _ := New(newMockConn("-ERR no\r\n"), Options{Protocol: 3})
	if err := c.Handshake(); err == nil {
		t.Fatal("want hello err")
	}
	// AUTH fails (RESP2).
	c, _ = New(newMockConn("-ERR badpw\r\n"), Options{Password: "x"})
	if err := c.Handshake(); err == nil {
		t.Fatal("want auth err")
	}
	// SELECT fails after a good HELLO.
	hello := "%1\r\n$5\r\nproto\r\n:3\r\n"
	c, _ = New(newMockConn(hello+"-ERR db\r\n"), Options{Protocol: 3, DB: 3})
	if err := c.Handshake(); err == nil {
		t.Fatal("want select err")
	}
}

func TestHelloError(t *testing.T) {
	c := NewFromConn(newMockConn("-ERR unknown\r\n"))
	if _, err := c.Hello(3); err == nil {
		t.Fatal("want err")
	}
}

func TestCommandErrorPropagation(t *testing.T) {
	// A coerced command surfaces a Redis error reply as a returned error.
	c := NewFromConn(newMockConn("-WRONGTYPE bad\r\n"))
	_, err := c.Get("k")
	var ce *CommandError
	if !errors.As(err, &ce) {
		t.Fatalf("err %v", err)
	}
}

func containsAll(s string, subs ...string) bool {
	for _, sub := range subs {
		found := false
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}
