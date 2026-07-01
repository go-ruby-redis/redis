// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

// This file holds targeted tests for the last error/edge branches that the
// behavioural tests do not reach, so the deterministic, ruby-free suite alone
// keeps coverage at 100%.

package redis

import (
	"errors"
	"strings"
	"testing"
)

// TestReadLineImmediateEOF hits readLine's bare `return "", err` branch: EOF with
// no bytes buffered after the type byte (input is just the type byte).
func TestReadLineImmediateEOF(t *testing.T) {
	if _, err := DecodeBytes([]byte(":")); err == nil {
		t.Fatal("want err on lone type byte")
	}
}

// TestReadNPropagatesReadError hits readN's `return nil, err` (a non-EOF reader
// error) via a reader that fails partway through a bulk body.
func TestReadNPropagatesReadError(t *testing.T) {
	r := &failAfterReader{data: []byte("$5\r\nhe"), fail: errors.New("io boom")}
	d := NewDecoder(r)
	if _, err := d.Decode(); err == nil {
		t.Fatal("want read error")
	}
}

// TestReadVerbatimReadError hits readVerbatim's readN-error branch.
func TestReadVerbatimReadError(t *testing.T) {
	r := &failAfterReader{data: []byte("=5\r\nte"), fail: errors.New("io boom")}
	d := NewDecoder(r)
	if _, err := d.Decode(); err == nil {
		t.Fatal("want verbatim read error")
	}
}

// TestReadBlobErrorReadError hits readBlobError's readN-error branch.
func TestReadBlobErrorReadError(t *testing.T) {
	r := &failAfterReader{data: []byte("!5\r\ner"), fail: errors.New("io boom")}
	d := NewDecoder(r)
	if _, err := d.Decode(); err == nil {
		t.Fatal("want blob-error read error")
	}
}

// TestReadMapKeyReadError hits readMap's key-decode error branch.
func TestReadMapKeyReadError(t *testing.T) {
	r := &failAfterReader{data: []byte("%1\r\n$"), fail: errors.New("io boom")}
	d := NewDecoder(r)
	if _, err := d.Decode(); err == nil {
		t.Fatal("want map key read error")
	}
}

// TestToSetPassThrough hits toSet's `case *Set` pass-through branch.
func TestToSetPassThrough(t *testing.T) {
	s := NewSet()
	s.Add("a")
	got, err := toSet(s)
	if err != nil || got.(*Set) != s {
		t.Fatalf("pass-through %#v %v", got, err)
	}
}

// TestSendWriteError hits client.send's Write-error branch: a request larger than
// bufio's buffer forces Write through to the failing underlying writer.
func TestSendWriteError(t *testing.T) {
	c := NewFromConn(&errConn{failWrite: true})
	big := strings.Repeat("x", 8192)
	if _, err := c.Call("SET", "k", big); err == nil {
		t.Fatal("want write error")
	}
}

// failAfterReader serves data once, then returns fail on the next read — used to
// drive io.ReadFull into a non-EOF error mid-frame.
type failAfterReader struct {
	data []byte
	i    int
	fail error
}

func (r *failAfterReader) Read(p []byte) (int, error) {
	if r.i < len(r.data) {
		n := copy(p, r.data[r.i:])
		r.i += n
		return n, nil
	}
	return 0, r.fail
}
