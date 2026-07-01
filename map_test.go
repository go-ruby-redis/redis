// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import "testing"

func TestMapOrderAndOverwrite(t *testing.T) {
	m := NewMap()
	m.Set("b", 1)
	m.Set("a", 2)
	m.Set("b", 3) // overwrite keeps position
	if m.Len() != 2 {
		t.Fatalf("len %d", m.Len())
	}
	keys := m.Keys()
	if keys[0] != "b" || keys[1] != "a" {
		t.Fatalf("order %v", keys)
	}
	if v, ok := m.Get("b"); !ok || v != 3 {
		t.Fatalf("b = %v", v)
	}
	if _, ok := m.Get("missing"); ok {
		t.Fatal("missing present")
	}
	var seen []any
	m.Each(func(k, v any) { seen = append(seen, k) })
	if len(seen) != 2 || seen[0] != "b" {
		t.Fatalf("each %v", seen)
	}
}

func TestMapKeyKinds(t *testing.T) {
	m := NewMap()
	m.Set(int64(1), "int")
	m.Set("1", "str") // distinct from int64(1)
	m.Set(struct{ X int }{}, "obj")
	if m.Len() != 3 {
		t.Fatalf("len %d", m.Len())
	}
	if v, _ := m.Get(int64(1)); v != "int" {
		t.Fatalf("int key %v", v)
	}
	if v, _ := m.Get("1"); v != "str" {
		t.Fatalf("str key %v", v)
	}
}

func TestSetOps(t *testing.T) {
	s := NewSet()
	if !s.Add("a") {
		t.Fatal("first add")
	}
	if s.Add("a") {
		t.Fatal("dup add")
	}
	s.Add("b")
	if s.Len() != 2 {
		t.Fatalf("len %d", s.Len())
	}
	if !s.Include("a") || s.Include("z") {
		t.Fatal("membership")
	}
	mem := s.Members()
	if mem[0] != "a" || mem[1] != "b" {
		t.Fatalf("order %v", mem)
	}
}
