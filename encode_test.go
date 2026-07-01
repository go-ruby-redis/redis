// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import (
	"math"
	"math/big"
	"testing"
)

func TestEncodeCommand(t *testing.T) {
	cases := []struct {
		name string
		args []any
		want string
	}{
		{"get", []any{"GET", "foo"}, "*2\r\n$3\r\nGET\r\n$3\r\nfoo\r\n"},
		{"set", []any{"SET", "k", "v"}, "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$1\r\nv\r\n"},
		{"empty-arg", []any{"SET", "k", ""}, "*3\r\n$3\r\nSET\r\n$1\r\nk\r\n$0\r\n\r\n"},
		{"no-args", []any{"PING"}, "*1\r\n$4\r\nPING\r\n"},
		{"int-arg", []any{"INCRBY", "n", 5}, "*3\r\n$6\r\nINCRBY\r\n$1\r\nn\r\n$1\r\n5\r\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := string(EncodeCommand(tc.args...))
			if got != tc.want {
				t.Fatalf("EncodeCommand(%v) = %q, want %q", tc.args, got, tc.want)
			}
		})
	}
}

func TestEncodeInline(t *testing.T) {
	got := string(EncodeInline("PING", "hi"))
	if got != "PING hi\r\n" {
		t.Fatalf("inline = %q", got)
	}
	if got := string(EncodeInline()); got != "\r\n" {
		t.Fatalf("empty inline = %q", got)
	}
}

func TestArgConversions(t *testing.T) {
	bi, _ := new(big.Int).SetString("123456789012345678901234567890", 10)
	cases := []struct {
		in   any
		want string
	}{
		{nil, ""},
		{"str", "str"},
		{[]byte("bytes"), "bytes"},
		{Symbol("sym"), "sym"},
		{true, "1"},
		{false, "0"},
		{int(7), "7"},
		{int64(-9), "-9"},
		{int32(3), "3"},
		{uint(4), "4"},
		{uint64(5), "5"},
		{float64(3.5), "3.5"},
		{float32(2.5), "2.5"},
		{bi, "123456789012345678901234567890"},
		{stringerT{}, "STR"},
		{struct{ X int }{1}, "?"},
	}
	for _, tc := range cases {
		if got := arg(tc.in); got != tc.want {
			t.Errorf("arg(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

type stringerT struct{}

func (stringerT) String() string { return "STR" }

func TestFormatFloat(t *testing.T) {
	cases := []struct {
		in   float64
		want string
	}{
		{math.NaN(), "nan"},
		{math.Inf(1), "inf"},
		{math.Inf(-1), "-inf"},
		{1.5, "1.5"},
	}
	for _, tc := range cases {
		if got := formatFloat(tc.in); got != tc.want {
			t.Errorf("formatFloat(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSprint(t *testing.T) {
	bi := big.NewInt(42)
	cases := []struct {
		in   any
		want string
	}{
		{"s", "s"},
		{int64(1), "1"},
		{int(2), "2"},
		{true, "true"},
		{false, "false"},
		{3.5, "3.5"},
		{bi, "42"},
		{struct{}{}, "?"},
	}
	for _, tc := range cases {
		if got := sprint(tc.in); got != tc.want {
			t.Errorf("sprint(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
