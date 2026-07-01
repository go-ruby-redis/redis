// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import (
	"bufio"
	"errors"
	"io"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// ErrProtocol reports a malformed RESP stream — a byte the grammar does not
// permit, or a truncated frame. The gem raises Redis::ProtocolError in the same
// situations; a host may map this error onto that class.
var ErrProtocol = errors.New("redis: protocol error")

// Decoder reads RESP replies from a stream. It handles both RESP2 and RESP3: the
// two protocols share request framing and every RESP2 reply type, and RESP3 adds
// the typed replies (null, double, boolean, big number, verbatim string, map,
// set, push, attribute, blob error). One Decoder may read any mix of the two,
// because the leading type byte disambiguates every frame.
type Decoder struct {
	r *bufio.Reader
	// attrs holds the most recent RESP3 attribute (`|`) dictionary, which the
	// protocol attaches to the reply that follows it. Decode returns the reply
	// and leaves the attribute available via LastAttribute.
	attrs *Map
}

// NewDecoder wraps r in a Decoder.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{r: bufio.NewReader(r)}
}

// LastAttribute returns the RESP3 attribute dictionary that preceded the most
// recently decoded reply, or nil if none did. Attributes are out-of-band
// metadata (e.g. key popularity, client-side-caching hints); the gem exposes
// them separately from the value, and so does this decoder.
func (d *Decoder) LastAttribute() *Map { return d.attrs }

// Decode reads and returns exactly one reply, mapping it to the value model. A
// Redis error reply (RESP2 `-` or RESP3 `!`) is returned as a value of type
// *CommandError with a nil error; only stream/protocol faults produce a non-nil
// error. This mirrors the gem, which turns an error reply into a raised
// Redis::CommandError the caller handles, distinct from a broken connection.
func (d *Decoder) Decode() (any, error) {
	d.attrs = nil
	return d.decode()
}

func (d *Decoder) decode() (any, error) {
	typ, err := d.r.ReadByte()
	if err != nil {
		return nil, err
	}
	line, err := d.readLine()
	if err != nil {
		return nil, err
	}
	switch typ {
	case '+': // simple string
		return line, nil
	case '-': // simple error
		return &CommandError{Message: line}, nil
	case ':': // integer
		return parseInt(line)
	case '$': // bulk string (RESP2) — also verbatim in RESP3 uses '='
		return d.readBulk(line)
	case '*': // array
		return d.readAggregate(line, aggArray)
	case '_': // RESP3 null
		return nil, nil
	case '#': // RESP3 boolean
		return parseBool(line)
	case ',': // RESP3 double
		return parseDouble(line)
	case '(': // RESP3 big number
		return parseBigNumber(line)
	case '=': // RESP3 verbatim string
		return d.readVerbatim(line)
	case '!': // RESP3 blob error
		return d.readBlobError(line)
	case '%': // RESP3 map
		return d.readMap(line)
	case '~': // RESP3 set
		return d.readAggregate(line, aggSet)
	case '>': // RESP3 push
		return d.readAggregate(line, aggPush)
	case '|': // RESP3 attribute — metadata for the *next* reply
		return d.readAttribute(line)
	default:
		return nil, ErrProtocol
	}
}

// readLine reads up to the next CRLF and returns the line without it. A bare LF
// or a missing terminator is a protocol error.
func (d *Decoder) readLine() (string, error) {
	s, err := d.r.ReadString('\n')
	if err != nil {
		if err == io.EOF && s != "" {
			return "", ErrProtocol
		}
		return "", err
	}
	if len(s) < 2 || s[len(s)-2] != '\r' {
		return "", ErrProtocol
	}
	return s[:len(s)-2], nil
}

func parseInt(line string) (any, error) {
	n, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return nil, ErrProtocol
	}
	return n, nil
}

func parseBool(line string) (any, error) {
	switch line {
	case "t":
		return true, nil
	case "f":
		return false, nil
	default:
		return nil, ErrProtocol
	}
}

func parseDouble(line string) (any, error) {
	switch line {
	case "inf":
		return math.Inf(1), nil
	case "-inf":
		return math.Inf(-1), nil
	case "nan", "-nan":
		return math.NaN(), nil
	}
	f, err := strconv.ParseFloat(line, 64)
	if err != nil {
		return nil, ErrProtocol
	}
	return f, nil
}

func parseBigNumber(line string) (any, error) {
	n, ok := new(big.Int).SetString(line, 10)
	if !ok {
		return nil, ErrProtocol
	}
	return n, nil
}

// readBulk reads a bulk-string body given its length line. A length of -1 is the
// RESP2 null bulk string, decoded as nil.
func (d *Decoder) readBulk(line string) (any, error) {
	n, err := strconv.Atoi(line)
	if err != nil {
		return nil, ErrProtocol
	}
	if n < 0 {
		return nil, nil // $-1: null bulk string
	}
	buf, err := d.readN(n)
	if err != nil {
		return nil, err
	}
	return string(buf), nil
}

// readVerbatim reads a RESP3 verbatim string: a bulk body of the form
// "fmt:text" where fmt is a three-character type hint.
func (d *Decoder) readVerbatim(line string) (any, error) {
	n, err := strconv.Atoi(line)
	if err != nil || n < 0 {
		return nil, ErrProtocol
	}
	buf, err := d.readN(n)
	if err != nil {
		return nil, err
	}
	s := string(buf)
	if len(s) < 4 || s[3] != ':' {
		return nil, ErrProtocol
	}
	return &VerbatimString{Format: s[:3], Text: s[4:]}, nil
}

// readBlobError reads a RESP3 blob error: a length-prefixed error body.
func (d *Decoder) readBlobError(line string) (any, error) {
	n, err := strconv.Atoi(line)
	if err != nil || n < 0 {
		return nil, ErrProtocol
	}
	buf, err := d.readN(n)
	if err != nil {
		return nil, err
	}
	return &CommandError{Message: string(buf)}, nil
}

// readN reads exactly n payload bytes plus the trailing CRLF.
func (d *Decoder) readN(n int) ([]byte, error) {
	buf := make([]byte, n+2)
	if _, err := io.ReadFull(d.r, buf); err != nil {
		if err == io.ErrUnexpectedEOF {
			return nil, ErrProtocol
		}
		return nil, err
	}
	if buf[n] != '\r' || buf[n+1] != '\n' {
		return nil, ErrProtocol
	}
	return buf[:n], nil
}

// aggKind selects how readAggregate shapes a count-prefixed reply.
type aggKind int

const (
	aggArray aggKind = iota
	aggSet
	aggPush
)

// readAggregate reads a count-prefixed reply (array `*`, set `~`, or push `>`).
// A count of -1 is the RESP2 null array, decoded as nil. Elements are decoded
// recursively so nested aggregates work to arbitrary depth.
func (d *Decoder) readAggregate(line string, kind aggKind) (any, error) {
	n, err := strconv.Atoi(line)
	if err != nil {
		return nil, ErrProtocol
	}
	if n < 0 {
		return nil, nil // *-1: null array
	}
	elems := make([]any, n)
	for i := range elems {
		v, err := d.decodeInner()
		if err != nil {
			return nil, err
		}
		elems[i] = v
	}
	switch kind {
	case aggSet:
		s := NewSet()
		for _, e := range elems {
			s.Add(e)
		}
		return s, nil
	case aggPush:
		return &Push{Values: elems}, nil
	default:
		return elems, nil
	}
}

// readMap reads a RESP3 map (`%`): a count of key/value *pairs*, decoded into an
// ordered [Map].
func (d *Decoder) readMap(line string) (any, error) {
	n, err := strconv.Atoi(line)
	if err != nil || n < 0 {
		return nil, ErrProtocol
	}
	m := NewMap()
	for range n {
		k, err := d.decodeInner()
		if err != nil {
			return nil, err
		}
		v, err := d.decodeInner()
		if err != nil {
			return nil, err
		}
		m.Set(k, v)
	}
	return m, nil
}

// readAttribute reads a RESP3 attribute (`|`) — itself a map of metadata — then
// decodes and returns the reply it decorates, stashing the attribute so a caller
// can retrieve it via LastAttribute.
func (d *Decoder) readAttribute(line string) (any, error) {
	m, err := d.readMap(line)
	if err != nil {
		return nil, err
	}
	d.attrs = m.(*Map)
	return d.decodeInner()
}

// decodeInner decodes a nested element without clearing the top-level attribute
// state (only the outermost Decode resets it).
func (d *Decoder) decodeInner() (any, error) { return d.decode() }

// DecodeBytes decodes a single reply from a complete byte slice — the golden-
// vector path used by the ruby-free tests: hand-built RESP bytes in, a value
// model out. It errors if the bytes hold no complete reply.
func DecodeBytes(b []byte) (any, error) {
	d := NewDecoder(strings.NewReader(string(b)))
	return d.Decode()
}
