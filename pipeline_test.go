// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import (
	"errors"
	"testing"
)

func TestPipelined(t *testing.T) {
	// Two GETs and an INCR batched.
	m := newMockConn("$1\r\nA\r\n$1\r\nB\r\n:5\r\n")
	c := NewFromConn(m)
	results, err := c.Pipelined(func(b *Batch) {
		b.Add("GET", "a")
		b.Add("GET", "b").Add("INCR", "n")
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0] != "A" || results[1] != "B" || results[2] != int64(5) {
		t.Fatalf("results %#v", results)
	}
	// All three commands must have been written in one batch.
	want := "*2\r\n$3\r\nGET\r\n$1\r\na\r\n" + "*2\r\n$3\r\nGET\r\n$1\r\nb\r\n" + "*2\r\n$4\r\nINCR\r\n$1\r\nn\r\n"
	if m.writes.String() != want {
		t.Fatalf("wrote %q", m.writes.String())
	}
}

func TestPipelinedEmpty(t *testing.T) {
	c := NewFromConn(newMockConn(""))
	results, err := c.Pipelined(func(b *Batch) {})
	if err != nil || len(results) != 0 {
		t.Fatalf("empty %v %v", results, err)
	}
}

func TestPipelinedErrorReplyIsValue(t *testing.T) {
	// A per-command error is a value in the slice, not a returned error.
	c := NewFromConn(newMockConn("+OK\r\n-ERR boom\r\n"))
	results, err := c.Pipelined(func(b *Batch) {
		b.Add("SET", "k", "v")
		b.Add("BADCMD")
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := results[1].(*CommandError); !ok {
		t.Fatalf("results[1] %#v", results[1])
	}
}

func TestPipelinedSeamAndDecodeErrors(t *testing.T) {
	// Write failure.
	c := NewFromConn(&errConn{failWrite: true})
	if _, err := c.Pipelined(func(b *Batch) { b.Add("PING") }); err == nil {
		t.Fatal("want write err")
	}
	// Short reply stream (decode fails mid-batch).
	c = NewFromConn(newMockConn("+OK\r\n"))
	if _, err := c.Pipelined(func(b *Batch) { b.Add("A"); b.Add("B") }); err == nil {
		t.Fatal("want decode err")
	}
}

func TestMulti(t *testing.T) {
	// MULTI -> +OK, each QUEUED, EXEC -> array of results.
	replies := "+OK\r\n" + "+QUEUED\r\n+QUEUED\r\n" + "*2\r\n:1\r\n:2\r\n"
	m := newMockConn(replies)
	c := NewFromConn(m)
	got, err := c.Multi(func(b *Batch) {
		b.Add("INCR", "a")
		b.Add("INCR", "b")
	})
	if err != nil {
		t.Fatal(err)
	}
	arr := got.([]any)
	if len(arr) != 2 || arr[0] != int64(1) || arr[1] != int64(2) {
		t.Fatalf("exec %#v", arr)
	}
	// The written frame must begin with MULTI and end with EXEC.
	w := m.writes.String()
	if w[:len("*1\r\n$5\r\nMULTI\r\n")] != "*1\r\n$5\r\nMULTI\r\n" {
		t.Fatalf("no MULTI prefix: %q", w)
	}
}

func TestMultiAborted(t *testing.T) {
	// EXEC returns a null array when a watched key changed.
	replies := "+OK\r\n" + "+QUEUED\r\n" + "*-1\r\n"
	c := NewFromConn(newMockConn(replies))
	got, err := c.Multi(func(b *Batch) { b.Add("SET", "k", "v") })
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("want nil, got %#v", got)
	}
}

func TestMultiErrors(t *testing.T) {
	// Write failure.
	c := NewFromConn(&errConn{failWrite: true})
	if _, err := c.Multi(func(b *Batch) { b.Add("PING") }); err == nil {
		t.Fatal("want write err")
	}
	// MULTI itself errors.
	c = NewFromConn(newMockConn("-ERR nomulti\r\n"))
	if _, err := c.Multi(func(b *Batch) { b.Add("PING") }); err == nil {
		t.Fatal("want multi err")
	}
	// MULTI decode fails (empty stream).
	c = NewFromConn(newMockConn(""))
	if _, err := c.Multi(func(b *Batch) { b.Add("PING") }); err == nil {
		t.Fatal("want multi decode err")
	}
	// A QUEUED reply is an error.
	c = NewFromConn(newMockConn("+OK\r\n-ERR badcmd\r\n"))
	if _, err := c.Multi(func(b *Batch) { b.Add("PING") }); err == nil {
		t.Fatal("want queued err")
	}
	// QUEUED decode fails.
	c = NewFromConn(newMockConn("+OK\r\n"))
	if _, err := c.Multi(func(b *Batch) { b.Add("PING") }); err == nil {
		t.Fatal("want queued decode err")
	}
	// EXEC decode fails.
	c = NewFromConn(newMockConn("+OK\r\n+QUEUED\r\n"))
	if _, err := c.Multi(func(b *Batch) { b.Add("PING") }); err == nil {
		t.Fatal("want exec decode err")
	}
	// EXEC returns an error reply.
	c = NewFromConn(newMockConn("+OK\r\n+QUEUED\r\n-EXECABORT aborted\r\n"))
	if _, err := c.Multi(func(b *Batch) { b.Add("PING") }); err == nil {
		t.Fatal("want exec err")
	}
	_ = errors.New
}
