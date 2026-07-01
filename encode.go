// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import (
	"math"
	"math/big"
	"strconv"
)

// itoa renders an int64 in base 10.
func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// sprint renders an arbitrary value for use as a Map/Set canonical key. It stays
// small and dependency-free — the value model only ever keys on strings and
// integers in practice.
func sprint(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	case bool:
		if x {
			return "true"
		}
		return "false"
	case float64:
		return strconv.FormatFloat(x, 'g', -1, 64)
	case *big.Int:
		return x.String()
	default:
		return "?"
	}
}

// arg converts a single command argument to its wire bytes. Redis commands are
// arrays of bulk strings, so every argument is stringified exactly as the gem
// does: strings verbatim, integers/floats in canonical base-10, symbols by name.
// A nil argument is the empty string (the gem rejects nil, but the codec is
// lenient so a host can enforce that policy).
func arg(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case []byte:
		return string(x)
	case Symbol:
		return string(x)
	case bool:
		if x {
			return "1"
		}
		return "0"
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case uint:
		return strconv.FormatUint(uint64(x), 10)
	case uint64:
		return strconv.FormatUint(x, 10)
	case float64:
		return formatFloat(x)
	case float32:
		return formatFloat(float64(x))
	case *big.Int:
		return x.String()
	case Stringer:
		return x.String()
	default:
		return sprint(v)
	}
}

// Stringer mirrors fmt.Stringer without pulling fmt into the arg fast path.
type Stringer interface{ String() string }

// formatFloat renders a float as Redis expects: "inf"/"-inf" for infinities and
// the shortest round-tripping decimal otherwise (matching the gem, which relies
// on Ruby's Float#to_s).
func formatFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "nan"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	return strconv.FormatFloat(f, 'g', -1, 64)
}

// EncodeCommand serialises one command — a name plus its arguments — to the RESP
// multi-bulk wire format: `*N\r\n$len\r\n<arg>\r\n…`. This is the RESP2 request
// framing every Redis command uses on the wire (RESP3 requests are identical;
// only replies differ). Every argument is stringified via [arg].
func EncodeCommand(args ...any) []byte {
	var b []byte
	b = append(b, '*')
	b = strconv.AppendInt(b, int64(len(args)), 10)
	b = append(b, '\r', '\n')
	for _, a := range args {
		s := arg(a)
		b = append(b, '$')
		b = strconv.AppendInt(b, int64(len(s)), 10)
		b = append(b, '\r', '\n')
		b = append(b, s...)
		b = append(b, '\r', '\n')
	}
	return b
}

// EncodeInline serialises a command in the legacy inline form: the arguments
// separated by single spaces and terminated by CRLF. Redis accepts inline
// commands when a request does not begin with `*`; the gem never emits them, but
// a host may need to (e.g. a bare "PING\r\n"). Arguments must not contain spaces
// or CRLF; callers that need those must use [EncodeCommand].
func EncodeInline(args ...any) []byte {
	var b []byte
	for i, a := range args {
		if i > 0 {
			b = append(b, ' ')
		}
		b = append(b, arg(a)...)
	}
	b = append(b, '\r', '\n')
	return b
}
