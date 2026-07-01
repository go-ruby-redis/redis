// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import (
	"bytes"
	"errors"
	"io"
	"reflect"
	"testing"
)

// mockConn is a scripted [Conn]: it records everything written and serves canned
// reply bytes on read. It is the ruby-free harness for the command layer — feed
// wire bytes in, assert the coerced value out.
type mockConn struct {
	writes bytes.Buffer
	reads  *bytes.Reader
}

func newMockConn(replies string) *mockConn {
	return &mockConn{reads: bytes.NewReader([]byte(replies))}
}

func (m *mockConn) Write(p []byte) (int, error) { return m.writes.Write(p) }
func (m *mockConn) Read(p []byte) (int, error)  { return m.reads.Read(p) }

// errConn fails on the requested operation to exercise seam-error paths.
type errConn struct {
	failWrite bool
	failRead  bool
}

func (e *errConn) Write(p []byte) (int, error) {
	if e.failWrite {
		return 0, errors.New("write boom")
	}
	return len(p), nil
}
func (e *errConn) Read(p []byte) (int, error) {
	if e.failRead {
		return 0, errors.New("read boom")
	}
	return 0, io.EOF
}

// mockRT is a scripted [RoundTripper].
type mockRT struct {
	lastReq []byte
	reply   string
	err     error
}

func (m *mockRT) RoundTrip(req []byte) ([]byte, error) {
	m.lastReq = req
	if m.err != nil {
		return nil, m.err
	}
	return []byte(m.reply), nil
}

func TestCallOverConn(t *testing.T) {
	m := newMockConn("+PONG\r\n")
	c := NewFromConn(m)
	got, err := c.Call("PING")
	if err != nil {
		t.Fatal(err)
	}
	if got != "PONG" {
		t.Fatalf("got %v", got)
	}
	if m.writes.String() != "*1\r\n$4\r\nPING\r\n" {
		t.Fatalf("wrote %q", m.writes.String())
	}
}

func TestCallOverRoundTripper(t *testing.T) {
	rt := &mockRT{reply: "+OK\r\n"}
	c := NewFromRoundTripper(rt)
	got, err := c.Call("SET", "k", "v")
	if err != nil {
		t.Fatal(err)
	}
	if got != "OK" {
		t.Fatalf("got %v", got)
	}
	if string(rt.lastReq) != "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n" {
		t.Fatalf("req %q", rt.lastReq)
	}
}

func TestCallErrorReply(t *testing.T) {
	c := NewFromConn(newMockConn("-ERR nope\r\n"))
	_, err := c.Call("GET", "x")
	var ce *CommandError
	if !errors.As(err, &ce) || ce.Message != "ERR nope" {
		t.Fatalf("err %v", err)
	}
}

func TestCallSeamErrors(t *testing.T) {
	// Write failure.
	c := NewFromConn(&errConn{failWrite: true})
	if _, err := c.Call("PING"); err == nil {
		t.Fatal("want write err")
	}
	// RoundTripper failure.
	c = NewFromRoundTripper(&mockRT{err: errors.New("rt boom")})
	if _, err := c.Call("PING"); err == nil {
		t.Fatal("want rt err")
	}
	// Decode failure (EOF before a reply).
	c = NewFromConn(newMockConn(""))
	if _, err := c.Call("PING"); err == nil {
		t.Fatal("want decode err")
	}
}

func TestNewAndConstructors(t *testing.T) {
	m := newMockConn("")
	c, err := New(m, Options{DB: 1, Protocol: 2})
	if err != nil {
		t.Fatal(err)
	}
	if c.conn == nil {
		t.Fatal("no conn")
	}
	if c.IsRESP3() {
		t.Fatal("should not be resp3")
	}
}

func TestByteReader(t *testing.T) {
	r := byteReader([]byte("abc"))
	buf := make([]byte, 2)
	n, err := r.Read(buf)
	if n != 2 || err != nil {
		t.Fatalf("read1 %d %v", n, err)
	}
	n, err = r.Read(buf)
	if n != 1 || err != nil {
		t.Fatalf("read2 %d %v", n, err)
	}
	if _, err := r.Read(buf); err != io.EOF {
		t.Fatalf("read3 %v", err)
	}
}

// runCmd is a helper: build a client over canned replies, run fn, and assert the
// result equals want and the exact bytes were written.
func runCmd(t *testing.T, reply string, wantWrite string, fn func(*Client) (any, error), want any) {
	t.Helper()
	m := newMockConn(reply)
	c := NewFromConn(m)
	got, err := fn(c)
	if err != nil {
		t.Fatalf("cmd error: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("result = %#v, want %#v", got, want)
	}
	if wantWrite != "" && m.writes.String() != wantWrite {
		t.Fatalf("wrote %q, want %q", m.writes.String(), wantWrite)
	}
}
