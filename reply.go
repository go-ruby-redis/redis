// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import (
	"math/big"
	"strconv"
)

// This file holds the reply-coercion helpers: the functions that turn a decoded
// RESP value model into the specific Ruby type the redis / redis-client gem's
// methods return. Each command in command.go composes these so its result type
// matches the gem exactly (Integer for EXISTS, Hash for HGETALL, Set for
// SMEMBERS, Float for INCRBYFLOAT, and so on).

// asError returns the *CommandError carried in v, if any. A Redis error reply is
// decoded as a value of type *CommandError; the command layer promotes it to a
// returned error so callers see it like the gem's raised Redis::CommandError.
func asError(v any) error {
	if e, ok := v.(*CommandError); ok {
		return e
	}
	return nil
}

// toString coerces a reply to a Ruby String (Go string), or nil for a null
// reply. It accepts a bulk/simple string, a verbatim string (payload), or an
// integer/double rendered canonically (some commands answer "+OK").
func toString(v any) (any, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case string:
		return x, nil
	case *VerbatimString:
		return x.Text, nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	default:
		return nil, typeErr("String", v)
	}
}

// toBoolFromInt coerces a Redis integer flag (0/1) to a Ruby boolean, the shape
// the gem gives EXPIRE, SISMEMBER, HEXISTS, SETNX, EXPIREAT, MOVE, PERSIST, …
func toBoolFromInt(v any) (any, error) {
	switch x := v.(type) {
	case int64:
		return x != 0, nil
	case bool: // RESP3 boolean
		return x, nil
	default:
		return nil, typeErr("Integer(bool)", v)
	}
}

// toInt coerces a reply to a Ruby Integer (int64), the shape of INCR, DEL,
// EXISTS, LLEN, STRLEN, TTL, …
func toInt(v any) (any, error) {
	switch x := v.(type) {
	case int64:
		return x, nil
	case bool:
		if x {
			return int64(1), nil
		}
		return int64(0), nil
	case nil:
		return nil, nil
	default:
		return nil, typeErr("Integer", v)
	}
}

// toFloat coerces a reply to a Ruby Float, the shape of INCRBYFLOAT,
// HINCRBYFLOAT and ZSCORE. RESP2 returns these as a bulk string; RESP3 as a
// double. A null reply (e.g. ZSCORE on a missing member) stays nil.
func toFloat(v any) (any, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case float64:
		return x, nil
	case string:
		f, err := strconv.ParseFloat(x, 64)
		if err != nil {
			return nil, typeErr("Float", v)
		}
		return f, nil
	default:
		return nil, typeErr("Float", v)
	}
}

// toArray coerces a reply to a Ruby Array. A null array stays nil; a RESP3 set
// or push is flattened to its elements so list/array commands are protocol-
// agnostic.
func toArray(v any) (any, error) {
	switch x := v.(type) {
	case nil:
		return nil, nil
	case []any:
		return x, nil
	case *Set:
		return x.Members(), nil
	case *Push:
		return x.Values, nil
	default:
		return nil, typeErr("Array", v)
	}
}

// toStringArray coerces a reply to []any of strings (the gem returns arrays of
// String for MGET, HKEYS, HVALS, KEYS, LRANGE, …). Null elements survive as nil,
// as MGET returns for missing keys.
func toStringArray(v any) (any, error) {
	arr, err := toArray(v)
	if err != nil {
		return nil, err
	}
	if arr == nil {
		return nil, nil
	}
	in := arr.([]any)
	out := make([]any, len(in))
	for i, e := range in {
		s, err := toString(e)
		if err != nil {
			return nil, err
		}
		out[i] = s
	}
	return out, nil
}

// toSet coerces a reply to a Ruby Set, the shape of SMEMBERS, SUNION, SINTER and
// SDIFF. RESP3 already yields a *Set; RESP2 yields an array we fold into one.
func toSet(v any) (any, error) {
	switch x := v.(type) {
	case nil:
		return NewSet(), nil
	case *Set:
		return x, nil
	case []any:
		s := NewSet()
		for _, e := range x {
			str, err := toString(e)
			if err != nil {
				return nil, err
			}
			s.Add(str)
		}
		return s, nil
	default:
		return nil, typeErr("Set", v)
	}
}

// toMapFromPairs coerces a flat array of alternating key/value strings (the
// RESP2 shape of HGETALL and CONFIG GET) or a RESP3 [Map] into an ordered [Map].
// This is the gem's Hash coercion for HGETALL, CONFIG GET, XPENDING, …
func toMapFromPairs(v any) (any, error) {
	switch x := v.(type) {
	case nil:
		return NewMap(), nil
	case *Map:
		return x, nil
	case []any:
		if len(x)%2 != 0 {
			return nil, typeErr("Hash(even pairs)", v)
		}
		m := NewMap()
		for i := 0; i < len(x); i += 2 {
			k, err := toString(x[i])
			if err != nil {
				return nil, err
			}
			val, err := toString(x[i+1])
			if err != nil {
				return nil, err
			}
			m.Set(k, val)
		}
		return m, nil
	default:
		return nil, typeErr("Hash", v)
	}
}

// toScanResult coerces a SCAN/HSCAN/SSCAN/ZSCAN reply — a two-element array of
// [cursor, elements] — into a [ScanResult]. The cursor is returned as a string
// (it can exceed int64 range and the gem hands it back verbatim).
func toScanResult(v any) (any, error) {
	arr, err := toArray(v)
	if err != nil {
		return nil, err
	}
	pair, ok := arr.([]any)
	if !ok || len(pair) != 2 {
		return nil, typeErr("ScanResult", v)
	}
	cursor, err := toString(pair[0])
	if err != nil {
		return nil, err
	}
	elems, err := toStringArray(pair[1])
	if err != nil {
		return nil, err
	}
	var keys []any
	if elems != nil {
		keys = elems.([]any)
	}
	cs, _ := cursor.(string)
	return &ScanResult{Cursor: cs, Elements: keys}, nil
}

// ScanResult is the value model of a cursored scan reply. Cursor is the opaque
// continuation token ("0" when the scan is complete) and Elements are the keys /
// members / field-value pairs returned by this iteration.
type ScanResult struct {
	Cursor   string
	Elements []any
}

// Done reports whether the scan has completed (the cursor returned to "0").
func (r *ScanResult) Done() bool { return r.Cursor == "0" }

// typeErr builds a *CommandError describing a reply that did not match the type
// a command expected. It surfaces as a returned error so a host maps it onto
// Redis::CommandError, matching the gem's behaviour when the server answers with
// an unexpected shape.
func typeErr(want string, got any) error {
	return &CommandError{Message: "ERR unexpected reply type; wanted " + want + ", got " + kindOf(got)}
}

// kindOf names the value-model type of v for diagnostics.
func kindOf(v any) string {
	switch v.(type) {
	case nil:
		return "nil"
	case string:
		return "String"
	case int64:
		return "Integer"
	case float64:
		return "Float"
	case bool:
		return "Boolean"
	case *big.Int:
		return "BigNumber"
	case []any:
		return "Array"
	case *Map:
		return "Hash"
	case *Set:
		return "Set"
	case *Push:
		return "Push"
	case *VerbatimString:
		return "VerbatimString"
	case *CommandError:
		return "Error"
	default:
		return "unknown"
	}
}
