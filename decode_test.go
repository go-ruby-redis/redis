// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import (
	"errors"
	"io"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"
)

// TestDecodeGoldenRESP2 decodes hand-built RESP2 byte streams from the spec and
// checks the value model — the ruby-free golden path.
func TestDecodeGoldenRESP2(t *testing.T) {
	cases := []struct {
		name string
		wire string
		want any
	}{
		{"simple-string", "+OK\r\n", "OK"},
		{"empty-simple", "+\r\n", ""},
		{"integer", ":1000\r\n", int64(1000)},
		{"neg-integer", ":-5\r\n", int64(-5)},
		{"bulk", "$5\r\nhello\r\n", "hello"},
		{"empty-bulk", "$0\r\n\r\n", ""},
		{"null-bulk", "$-1\r\n", nil},
		{"array", "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n", []any{"foo", "bar"}},
		{"empty-array", "*0\r\n", []any{}},
		{"null-array", "*-1\r\n", nil},
		{"int-array", "*3\r\n:1\r\n:2\r\n:3\r\n", []any{int64(1), int64(2), int64(3)}},
		{"nested-array", "*2\r\n*1\r\n:1\r\n*1\r\n:2\r\n", []any{[]any{int64(1)}, []any{int64(2)}}},
		{"array-with-null", "*2\r\n$3\r\nfoo\r\n$-1\r\n", []any{"foo", nil}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeBytes([]byte(tc.wire))
			if err != nil {
				t.Fatalf("DecodeBytes(%q): %v", tc.wire, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("DecodeBytes(%q) = %#v, want %#v", tc.wire, got, tc.want)
			}
		})
	}
}

func TestDecodeError(t *testing.T) {
	got, err := DecodeBytes([]byte("-WRONGTYPE bad\r\n"))
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	ce, ok := got.(*CommandError)
	if !ok {
		t.Fatalf("got %T", got)
	}
	if ce.Message != "WRONGTYPE bad" {
		t.Fatalf("message %q", ce.Message)
	}
	if ce.Code() != "WRONGTYPE" {
		t.Fatalf("code %q", ce.Code())
	}
	if ce.Error() != "WRONGTYPE bad" {
		t.Fatalf("Error() %q", ce.Error())
	}
}

func TestCommandErrorCode(t *testing.T) {
	if (&CommandError{Message: "ERR"}).Code() != "ERR" {
		t.Fatal("single-word code")
	}
	var nilErr *CommandError
	if nilErr.Code() != "" {
		t.Fatal("nil code")
	}
}

// TestDecodeGoldenRESP3 covers every RESP3-only reply type.
func TestDecodeGoldenRESP3(t *testing.T) {
	bi, _ := new(big.Int).SetString("3492890328409238509324850943850943825024385", 10)
	cases := []struct {
		name string
		wire string
		want any
	}{
		{"null", "_\r\n", nil},
		{"true", "#t\r\n", true},
		{"false", "#f\r\n", false},
		{"double", ",3.14\r\n", 3.14},
		{"double-int", ",10\r\n", 10.0},
		{"double-inf", ",inf\r\n", math.Inf(1)},
		{"double-ninf", ",-inf\r\n", math.Inf(-1)},
		{"bignum", "(" + "3492890328409238509324850943850943825024385" + "\r\n", bi},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeBytes([]byte(tc.wire))
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("= %#v, want %#v", got, tc.want)
			}
		})
	}
}

func TestDecodeDoubleNaN(t *testing.T) {
	got, err := DecodeBytes([]byte(",nan\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if f, ok := got.(float64); !ok || !math.IsNaN(f) {
		t.Fatalf("got %#v", got)
	}
	got, err = DecodeBytes([]byte(",-nan\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	if f, ok := got.(float64); !ok || !math.IsNaN(f) {
		t.Fatalf("got %#v", got)
	}
}

func TestDecodeVerbatim(t *testing.T) {
	got, err := DecodeBytes([]byte("=15\r\ntxt:Some string\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	v, ok := got.(*VerbatimString)
	if !ok {
		t.Fatalf("got %T", got)
	}
	if v.Format != "txt" || v.Text != "Some string" {
		t.Fatalf("verbatim %#v", v)
	}
	if v.String() != "Some string" {
		t.Fatalf("String() %q", v.String())
	}
	var nilV *VerbatimString
	if nilV.String() != "" {
		t.Fatal("nil String")
	}
}

func TestDecodeBlobError(t *testing.T) {
	got, err := DecodeBytes([]byte("!21\r\nSYNTAX invalid syntax\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	ce, ok := got.(*CommandError)
	if !ok || ce.Message != "SYNTAX invalid syntax" {
		t.Fatalf("got %#v", got)
	}
}

func TestDecodeMap(t *testing.T) {
	got, err := DecodeBytes([]byte("%2\r\n$5\r\nfirst\r\n:1\r\n$6\r\nsecond\r\n:2\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	m, ok := got.(*Map)
	if !ok {
		t.Fatalf("got %T", got)
	}
	if m.Len() != 2 {
		t.Fatalf("len %d", m.Len())
	}
	if v, _ := m.Get("first"); v != int64(1) {
		t.Fatalf("first = %v", v)
	}
	keys := m.Keys()
	if keys[0] != "first" || keys[1] != "second" {
		t.Fatalf("order %v", keys)
	}
}

func TestDecodeSet(t *testing.T) {
	got, err := DecodeBytes([]byte("~3\r\n$3\r\nfoo\r\n$3\r\nbar\r\n$3\r\nfoo\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	s, ok := got.(*Set)
	if !ok {
		t.Fatalf("got %T", got)
	}
	if s.Len() != 2 { // duplicate folded
		t.Fatalf("len %d", s.Len())
	}
	if !s.Include("foo") || !s.Include("bar") {
		t.Fatal("membership")
	}
}

func TestDecodePush(t *testing.T) {
	got, err := DecodeBytes([]byte(">3\r\n$7\r\nmessage\r\n$2\r\nch\r\n$5\r\nhello\r\n"))
	if err != nil {
		t.Fatal(err)
	}
	p, ok := got.(*Push)
	if !ok {
		t.Fatalf("got %T", got)
	}
	if p.Kind() != "message" || len(p.Values) != 3 {
		t.Fatalf("push %#v", p)
	}
	var nilP *Push
	if nilP.Kind() != "" {
		t.Fatal("nil kind")
	}
	if (&Push{}).Kind() != "" {
		t.Fatal("empty kind")
	}
	if (&Push{Values: []any{int64(1)}}).Kind() != "" {
		t.Fatal("non-string kind")
	}
}

func TestDecodeAttribute(t *testing.T) {
	// An attribute precedes the reply it decorates.
	wire := "|1\r\n$14\r\nkey-popularity\r\n%1\r\n$1\r\na\r\n:1\r\n:42\r\n"
	d := NewDecoder(strings.NewReader(wire))
	got, err := d.Decode()
	if err != nil {
		t.Fatal(err)
	}
	if got != int64(42) {
		t.Fatalf("value = %v", got)
	}
	attr := d.LastAttribute()
	if attr == nil {
		t.Fatal("no attribute")
	}
	if _, ok := attr.Get("key-popularity"); !ok {
		t.Fatal("attr content")
	}
	// A fresh Decode clears the attribute.
	got, err = DecodeBytes([]byte(":1\r\n"))
	if err != nil || got != int64(1) {
		t.Fatalf("got %v %v", got, err)
	}
}

// TestDecodeProtocolErrors exercises every malformed-frame path.
func TestDecodeProtocolErrors(t *testing.T) {
	bad := []string{
		"?bad\r\n",         // unknown type byte
		":notanint\r\n",    // bad integer
		"#x\r\n",           // bad boolean
		",notafloat\r\n",   // bad double
		"(notabignum\r\n",  // bad bignum
		"$3\r\nhe\r\n",     // bulk shorter than declared -> CRLF mismatch
		"$x\r\n",           // bad bulk length
		"*x\r\n",           // bad array count
		"%x\r\n",           // bad map count
		"=x\r\n",           // bad verbatim length
		"=3\r\nabc\r\n",    // verbatim missing "fmt:" separator
		"!x\r\n",           // bad blob-error length
		"|x\r\n",           // bad attribute count
		"$5\r\nhello\rX\n", // bad bulk terminator
		"+missingcrlf\n",   // bare LF, no CR
	}
	for _, w := range bad {
		if _, err := DecodeBytes([]byte(w)); !errors.Is(err, ErrProtocol) {
			t.Errorf("DecodeBytes(%q) err = %v, want ErrProtocol", w, err)
		}
	}
}

func TestDecodeEOF(t *testing.T) {
	if _, err := DecodeBytes([]byte("")); err != io.EOF {
		t.Fatalf("empty: %v", err)
	}
	// Truncated after the type byte.
	if _, err := DecodeBytes([]byte(":123")); err == nil {
		t.Fatal("want err on truncated line")
	}
	// Truncated bulk payload.
	if _, err := DecodeBytes([]byte("$5\r\nhel")); !errors.Is(err, ErrProtocol) {
		t.Fatalf("truncated bulk: %v", err)
	}
	// Nested aggregate element truncated.
	if _, err := DecodeBytes([]byte("*2\r\n:1\r\n")); err == nil {
		t.Fatal("want err on short array")
	}
	// Map value truncated.
	if _, err := DecodeBytes([]byte("%1\r\n$1\r\na\r\n")); err == nil {
		t.Fatal("want err on short map")
	}
	// Attribute body truncated.
	if _, err := DecodeBytes([]byte("|1\r\n$1\r\na\r\n")); err == nil {
		t.Fatal("want err on short attribute")
	}
}

func TestReadLineBareLFAtEOF(t *testing.T) {
	// A read that ends with content but no newline surfaces as EOF or protocol.
	if _, err := DecodeBytes([]byte("+partial")); err == nil {
		t.Fatal("want err")
	}
}
