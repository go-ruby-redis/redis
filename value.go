// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

// Package redis is a pure-Go (CGO-free) reimplementation of the RESP protocol
// codec and command layer of Ruby's redis / redis-client gem. It encodes a
// command (an array of bulk strings) to the RESP wire format and decodes every
// RESP2 and RESP3 reply type into a small, explicit Ruby value model, then maps
// each command's reply to the typed value the gem returns (a Hash for HGETALL, a
// Set for SMEMBERS, an Integer for EXISTS, and so on).
//
// The actual TCP/TLS socket is deliberately not part of this package: it is a
// host seam, mirroring the go-ruby-net-* pattern. A [Client] drives its commands
// over an injected [Conn] (an io.ReadWriter) or a [RoundTripper] the host wires
// to a live connection; this package owns only the deterministic, ruby-free wire
// grammar and command surface.
//
// # Ruby value model
//
// A decoded reply is represented by an [any] drawn from a small, fixed set of Go
// types so a host (such as go-embedded-ruby) can map replies onto its own object
// graph:
//
//	RESP                       Go value
//	----                       --------
//	null ($-1, *-1, _)         nil
//	simple string (+)          string
//	simple error (-)           *CommandError
//	blob error (!)             *CommandError
//	integer (:)                int64
//	double (,)                 float64
//	big number (()             *big.Int
//	boolean (#)                bool
//	bulk string ($)            string
//	verbatim string ($=/=)     *VerbatimString
//	array (*)                  []any
//	map (%)                    *Map (ordered)
//	set (~)                    *Set
//	push (>)                   *Push
//	attribute (|)              carried on the following reply, not a value
//
// The command layer then coerces these primitives into the types the gem's
// methods return (see reply.go).
package redis

import "strings"

// Value is the interface satisfied by every Ruby value this package produces. It
// is purely documentary — the public API uses any — but a host may use it to
// constrain its own adapters.
type Value = any

// Symbol is a Ruby Symbol (`:name`). A host may pass a Symbol as a command
// argument (the gem accepts symbols and stringifies them by name); the encoder
// serialises it as its bare name.
type Symbol string

// CommandError is a Redis error reply — the RESP2 `-` simple error, or the RESP3
// `!` blob error. Ruby's redis gem raises a Redis::CommandError carrying this
// message; the value model surfaces it as an error so a host can decide whether
// to raise. It satisfies the Go error interface.
type CommandError struct {
	// Message is the full error text, e.g. "WRONGTYPE Operation against a key
	// holding the wrong kind of value".
	Message string
}

// Error implements the error interface.
func (e *CommandError) Error() string { return e.Message }

// Code returns the leading upper-case error code (the word before the first
// space), e.g. "WRONGTYPE" or "ERR". Redis conventionally prefixes error
// messages with such a code; the gem exposes it likewise. An empty message
// yields "".
func (e *CommandError) Code() string {
	if e == nil {
		return ""
	}
	i := strings.IndexByte(e.Message, ' ')
	if i < 0 {
		return e.Message
	}
	return e.Message[:i]
}

// VerbatimString is a RESP3 verbatim string (`=`): a bulk string carrying a
// three-byte format hint (e.g. "txt" or "mkd"). The gem treats it as a plain
// String; this model keeps the format so a host may inspect it.
type VerbatimString struct {
	// Format is the three-character type hint (e.g. "txt", "mkd").
	Format string
	// Text is the string payload (without the "fmt:" prefix).
	Text string
}

// String returns the payload, so a VerbatimString stringifies like the plain
// String the gem exposes.
func (v *VerbatimString) String() string {
	if v == nil {
		return ""
	}
	return v.Text
}

// Push is a RESP3 out-of-band push message (`>`): the frame pub/sub and other
// server-initiated notifications arrive on. Its Values are the decoded elements
// (the first is the kind, e.g. "message", "subscribe", "pmessage").
type Push struct {
	Values []any
}

// Kind returns the first element of a Push as a string (the message kind), or ""
// if the push is empty or its head is not a string.
func (p *Push) Kind() string {
	if p == nil || len(p.Values) == 0 {
		return ""
	}
	if s, ok := p.Values[0].(string); ok {
		return s
	}
	return ""
}
