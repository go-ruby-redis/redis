// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import (
	"math/big"
	"testing"
)

func TestToStringVariants(t *testing.T) {
	if v, _ := toString(&VerbatimString{Format: "txt", Text: "hi"}); v != "hi" {
		t.Fatalf("verbatim %v", v)
	}
	if v, _ := toString(int64(42)); v != "42" {
		t.Fatalf("int %v", v)
	}
	if _, err := toString([]any{}); err == nil {
		t.Fatal("want type err")
	}
}

func TestToBoolFromInt(t *testing.T) {
	if v, _ := toBoolFromInt(true); v != true {
		t.Fatal("resp3 bool")
	}
	if _, err := toBoolFromInt("x"); err == nil {
		t.Fatal("want err")
	}
}

func TestToInt(t *testing.T) {
	if v, _ := toInt(true); v != int64(1) {
		t.Fatal("bool true")
	}
	if v, _ := toInt(false); v != int64(0) {
		t.Fatal("bool false")
	}
	if v, _ := toInt(nil); v != nil {
		t.Fatal("nil")
	}
	if _, err := toInt("x"); err == nil {
		t.Fatal("want err")
	}
}

func TestToFloat(t *testing.T) {
	if v, _ := toFloat(3.5); v != 3.5 {
		t.Fatal("double")
	}
	if _, err := toFloat("notafloat"); err == nil {
		t.Fatal("bad string")
	}
	if _, err := toFloat([]any{}); err == nil {
		t.Fatal("bad type")
	}
}

func TestToArrayVariants(t *testing.T) {
	s := NewSet()
	s.Add("a")
	if v, _ := toArray(s); len(v.([]any)) != 1 {
		t.Fatal("set to array")
	}
	if v, _ := toArray(&Push{Values: []any{"a"}}); len(v.([]any)) != 1 {
		t.Fatal("push to array")
	}
	if _, err := toArray("x"); err == nil {
		t.Fatal("want err")
	}
}

func TestToStringArrayNilAndErr(t *testing.T) {
	if v, _ := toStringArray(nil); v != nil {
		t.Fatal("nil array")
	}
	if _, err := toStringArray("x"); err == nil {
		t.Fatal("bad type")
	}
	// Element that cannot be coerced.
	if _, err := toStringArray([]any{[]any{}}); err == nil {
		t.Fatal("bad element")
	}
}

func TestToSetNilAndErr(t *testing.T) {
	if v, _ := toSet(nil); v.(*Set).Len() != 0 {
		t.Fatal("nil set")
	}
	if _, err := toSet(int64(1)); err == nil {
		t.Fatal("bad type")
	}
	// Element that cannot be stringified.
	if _, err := toSet([]any{[]any{}}); err == nil {
		t.Fatal("bad element")
	}
}

func TestToMapFromPairsEdges(t *testing.T) {
	if v, _ := toMapFromPairs(nil); v.(*Map).Len() != 0 {
		t.Fatal("nil map")
	}
	if _, err := toMapFromPairs([]any{"a"}); err == nil {
		t.Fatal("odd pairs")
	}
	if _, err := toMapFromPairs(int64(1)); err == nil {
		t.Fatal("bad type")
	}
	// Bad key.
	if _, err := toMapFromPairs([]any{[]any{}, "v"}); err == nil {
		t.Fatal("bad key")
	}
	// Bad value.
	if _, err := toMapFromPairs([]any{"k", []any{}}); err == nil {
		t.Fatal("bad value")
	}
}

func TestToScanResultErrors(t *testing.T) {
	if _, err := toScanResult("x"); err == nil {
		t.Fatal("bad type")
	}
	if _, err := toScanResult([]any{"only-one"}); err == nil {
		t.Fatal("wrong len")
	}
	// Bad cursor.
	if _, err := toScanResult([]any{[]any{}, []any{}}); err == nil {
		t.Fatal("bad cursor")
	}
	// Bad elements.
	if _, err := toScanResult([]any{"0", "notarray"}); err == nil {
		t.Fatal("bad elements")
	}
	// nil elements -> empty keys.
	sr, err := toScanResult([]any{"0", nil})
	if err != nil || len(sr.(*ScanResult).Elements) != 0 {
		t.Fatalf("nil elems %v %v", sr, err)
	}
}

func TestKindOf(t *testing.T) {
	cases := []struct {
		v    any
		want string
	}{
		{nil, "nil"},
		{"s", "String"},
		{int64(1), "Integer"},
		{3.5, "Float"},
		{true, "Boolean"},
		{big.NewInt(1), "BigNumber"},
		{[]any{}, "Array"},
		{NewMap(), "Hash"},
		{NewSet(), "Set"},
		{&Push{}, "Push"},
		{&VerbatimString{}, "VerbatimString"},
		{&CommandError{}, "Error"},
		{struct{}{}, "unknown"},
	}
	for _, tc := range cases {
		if got := kindOf(tc.v); got != tc.want {
			t.Errorf("kindOf(%T) = %q, want %q", tc.v, got, tc.want)
		}
	}
}

func TestAsError(t *testing.T) {
	if asError("ok") != nil {
		t.Fatal("non-error")
	}
	if asError(&CommandError{Message: "x"}) == nil {
		t.Fatal("error")
	}
}
