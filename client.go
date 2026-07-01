// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import (
	"bufio"
	"io"
)

// RoundTripper is the host seam for a single request/response exchange: given
// the encoded RESP bytes of a command, it returns the raw RESP bytes of the
// reply. A host wires this to a live TCP/TLS connection (the socket lives outside
// this package, mirroring the go-ruby-net-* design). A Client built on a
// RoundTripper drives one command at a time; pipelines and transactions send a
// batch and read a matching run of replies, so a RoundTripper-backed Client
// serialises those onto a [Conn]-style stream internally.
type RoundTripper interface {
	// RoundTrip sends one request frame and returns its reply frame(s). For a
	// single command the reply is one RESP value; the Client parses it.
	RoundTrip(request []byte) (reply []byte, err error)
}

// Conn is the streaming host seam: a bidirectional byte pipe (an io.ReadWriter)
// the Client writes requests to and reads replies from. A live *net.Conn or a
// TLS connection satisfies it directly. This is the primary seam — it supports
// pipelining, transactions and pub/sub, which need many replies read from one
// stream — while [RoundTripper] suits simple request/reply transports.
type Conn = io.ReadWriter

// Client is a Redis client bound to a host connection seam. It builds RESP
// commands, sends them over the seam, decodes the replies and coerces each to
// the typed value the redis / redis-client gem returns. It owns no socket: the
// caller supplies a [Conn] (streaming, the default) or a [RoundTripper].
//
// A zero Client is not usable; construct one with [New], [NewFromConn] or
// [NewFromRoundTripper].
type Client struct {
	// conn is the streaming seam, when configured. Exactly one of conn / rt is
	// non-nil.
	conn Conn
	// dec decodes replies from conn (nil for the RoundTripper path).
	dec *Decoder
	// w buffers writes to conn.
	w *bufio.Writer
	// rt is the request/reply seam, when configured.
	rt RoundTripper

	// resp3 records whether the connection was switched to RESP3 (via HELLO 3).
	// It does not change how replies are decoded — the type byte is
	// self-describing — but a host may consult it.
	resp3 bool

	// opts records the configuration New captured, for Handshake to apply.
	opts Options
}

// Options configures a Client. The fields mirror the gem's Redis.new keywords
// that affect the protocol layer; transport keywords (host/port/ssl/timeouts)
// belong to the host that owns the socket, not to this codec.
type Options struct {
	// Username and Password are sent via AUTH (or HELLO … AUTH) when non-empty,
	// matching Redis.new(username:, password:). This package does not itself
	// perform the handshake — call [Client.Auth] / [Client.Hello] — but the
	// options are recorded so a host driver can.
	Username string
	Password string
	// DB is the logical database index selected with SELECT after connecting
	// (Redis.new(db:)). Zero means the default database.
	DB int
	// Protocol is 2 or 3; 3 requests the RESP3 handshake via HELLO 3. Zero
	// defaults to 2, matching the gem's default.
	Protocol int
}

// NewFromConn builds a Client that speaks over a streaming [Conn]. This is the
// primary constructor: it supports pipelining, MULTI/EXEC and pub/sub.
func NewFromConn(c Conn) *Client {
	return &Client{
		conn: c,
		dec:  NewDecoder(c),
		w:    bufio.NewWriter(c),
	}
}

// NewFromRoundTripper builds a Client that speaks over a request/reply
// [RoundTripper]. Pipelining and transactions still work: the batch is encoded
// as one request and its concatenated replies are decoded in order.
func NewFromRoundTripper(rt RoundTripper) *Client {
	return &Client{rt: rt}
}

// New builds a Client over a streaming [Conn] and records opts for a host driver
// to apply during its handshake. It mirrors Redis.new(**opts) at the protocol
// layer; the actual AUTH/SELECT/HELLO round-trips are the caller's to issue (see
// [Client.Handshake]).
func New(c Conn, opts Options) (*Client, error) {
	cl := NewFromConn(c)
	cl.opts = opts
	return cl, nil
}

// opts is stored on the Client for Handshake; kept unexported and appended here
// to keep the struct definition focused on the seam fields above.
//
// (Declared via a separate field set so New can record configuration.)

// send writes an encoded request to the seam and returns the decoder to read
// replies from. On the RoundTripper path it performs the exchange eagerly and
// returns a decoder over the reply bytes.
func (c *Client) send(req []byte) (*Decoder, error) {
	if c.rt != nil {
		reply, err := c.rt.RoundTrip(req)
		if err != nil {
			return nil, err
		}
		return NewDecoder(byteReader(reply)), nil
	}
	if _, err := c.w.Write(req); err != nil {
		return nil, err
	}
	if err := c.w.Flush(); err != nil {
		return nil, err
	}
	return c.dec, nil
}

// byteReader adapts a byte slice to an io.Reader for the RoundTripper reply path
// without allocating a bytes.Reader import churn.
func byteReader(b []byte) io.Reader { return &sliceReader{b: b} }

type sliceReader struct {
	b []byte
	i int
}

func (r *sliceReader) Read(p []byte) (int, error) {
	if r.i >= len(r.b) {
		return 0, io.EOF
	}
	n := copy(p, r.b[r.i:])
	r.i += n
	return n, nil
}
