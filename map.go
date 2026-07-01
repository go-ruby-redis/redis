// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

// Map is an insertion-ordered key/value collection — the value model's Ruby
// Hash. RESP3 maps (`%`) decode to a Map, and command coercions that build a
// Hash reply (HGETALL, CONFIG GET, XPENDING summaries, …) return one so key
// order is preserved on round-trip, exactly as Ruby's ordered Hash does.
type Map struct {
	keys   []any
	values map[string]mapEntry
}

// mapEntry stores a key/value pair keyed by the key's canonical string form so
// lookups are O(1) while the keys slice keeps insertion order.
type mapEntry struct {
	key   any
	value any
}

// NewMap returns an empty ordered Map.
func NewMap() *Map {
	return &Map{values: make(map[string]mapEntry)}
}

// mapKey renders a key to its canonical string form for the backing map. Keys in
// Redis replies are always strings; other comparable values fall back to their
// Go default formatting so a host may still use the Map generically.
func mapKey(k any) string {
	switch v := k.(type) {
	case string:
		return "s:" + v
	case int64:
		return "i:" + itoa(v)
	default:
		return "o:" + sprint(k)
	}
}

// Set inserts or updates key with value, preserving first-insertion order.
func (m *Map) Set(key, value any) {
	mk := mapKey(key)
	if _, ok := m.values[mk]; !ok {
		m.keys = append(m.keys, key)
	}
	m.values[mk] = mapEntry{key: key, value: value}
}

// Get returns the value stored for key and whether it was present.
func (m *Map) Get(key any) (any, bool) {
	e, ok := m.values[mapKey(key)]
	if !ok {
		return nil, false
	}
	return e.value, true
}

// Len reports the number of entries.
func (m *Map) Len() int { return len(m.keys) }

// Keys returns the keys in insertion order.
func (m *Map) Keys() []any {
	out := make([]any, len(m.keys))
	copy(out, m.keys)
	return out
}

// Each calls fn for every entry in insertion order.
func (m *Map) Each(fn func(key, value any)) {
	for _, k := range m.keys {
		fn(k, m.values[mapKey(k)].value)
	}
}

// Set is the value model's Ruby Set: an insertion-ordered collection of unique
// members. RESP3 sets (`~`) decode to a Set, and SMEMBERS / SUNION / SDIFF /
// SINTER coerce their array reply into one, matching the gem.
type Set struct {
	members []any
	index   map[string]struct{}
}

// NewSet returns an empty Set.
func NewSet() *Set {
	return &Set{index: make(map[string]struct{})}
}

// Add inserts member if it is not already present, preserving insertion order.
// It reports whether the member was newly added.
func (s *Set) Add(member any) bool {
	mk := mapKey(member)
	if _, ok := s.index[mk]; ok {
		return false
	}
	s.index[mk] = struct{}{}
	s.members = append(s.members, member)
	return true
}

// Include reports whether member is in the set.
func (s *Set) Include(member any) bool {
	_, ok := s.index[mapKey(member)]
	return ok
}

// Len reports the number of members.
func (s *Set) Len() int { return len(s.members) }

// Members returns the members in insertion order.
func (s *Set) Members() []any {
	out := make([]any, len(s.members))
	copy(out, s.members)
	return out
}
