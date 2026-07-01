// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import (
	"math"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

// The oracle tests differentially validate this package against the reference
// implementation — the redis-client gem's RedisClient::RESP3 codec, which
// encodes commands and parses replies with no live server. They are gated on
// RUBY_VERSION >= "4.0" and on the gem being installed, and skip themselves
// otherwise (the qemu cross-arch lanes and the Windows lane have no such ruby),
// so the deterministic, ruby-free suite alone drives the 100% coverage gate.
//
// Two directions are checked:
//   - encode: EncodeCommand(args) must equal RESP3.dump(args).
//   - decode: DecodeBytes(wire) must equal RESP3.parse(wire) (compared via the
//     gem's own #inspect of the parsed value).

// rubyOracle locates a usable `ruby` (>= 4.0) with redis-client installed, once.
// It skips the calling test when any prerequisite is missing.
func rubyOracle(t *testing.T) string {
	t.Helper()
	bin, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("ruby not on PATH; skipping redis-client oracle")
	}
	// Version gate + gem presence in one probe.
	out, err := exec.Command(bin, "-e", `
exit(1) if RUBY_VERSION < "4.0"
begin
  require "redis_client"
rescue LoadError
  exit(2)
end
print "ok"
`).CombinedOutput()
	if err != nil || strings.TrimSpace(string(out)) != "ok" {
		t.Skipf("ruby >= 4.0 with redis-client not available; skipping oracle (%s)", strings.TrimSpace(string(out)))
	}
	return bin
}

// oracleShim is the Ruby preamble providing a StringIO adapter that satisfies
// RESP3.parse's IO contract (getbyte / gets_chomp / gets_integer / read_chomp /
// skip), plus a helper to dump a command and one to parse a wire string.
const oracleShim = `
require "redis_client"
require "stringio"

class ShimIO
  def initialize(str) = @io = StringIO.new(str)
  def getbyte = @io.getbyte
  def gets_chomp
    line = @io.gets("\r\n")
    line.nil? ? "" : line.chomp("\r\n")
  end
  def gets_integer = Integer(gets_chomp)
  def read_chomp(n)
    s = @io.read(n)
    @io.read(2) # drop CRLF
    s
  end
  def skip(n) = @io.read(n)
end

def resp3_dump(args)
  buf = +""
  RedisClient::RESP3.dump(args, buf)
  buf
end

def resp3_parse(wire)
  RedisClient::RESP3.parse(ShimIO.new(wire))
end
`

// rubyRun executes a Ruby script (with the shim preamble) and returns trimmed
// stdout, binmoded so Windows text mode cannot corrupt the bytes.
func rubyRun(t *testing.T, bin, script string) string {
	t.Helper()
	cmd := exec.Command(bin, "-e", "$stdout.binmode\n"+oracleShim+"\n"+script)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ruby error: %v\nscript:\n%s\noutput:\n%s", err, script, out)
	}
	return string(out)
}

// TestOracleEncode checks EncodeCommand matches RESP3.dump for a corpus of
// commands spanning the argument kinds a host passes.
func TestOracleEncode(t *testing.T) {
	bin := rubyOracle(t)
	cases := [][]any{
		{"GET", "foo"},
		{"SET", "k", "v"},
		{"SET", "k", ""},
		{"MSET", "a", "1", "b", "2"},
		{"INCRBY", "n", 5},
		{"HSET", "h", "f", "value with spaces"},
		{"PING"},
		{"SUBSCRIBE", "chan"},
	}
	for _, args := range cases {
		got := string(EncodeCommand(args...))
		// Build the equivalent Ruby array literal (all string args, since the
		// gem stringifies before dumping just as EncodeCommand does).
		var lits []string
		for _, a := range args {
			lits = append(lits, strconv.Quote(arg(a)))
		}
		script := "print resp3_dump([" + strings.Join(lits, ",") + "])"
		want := rubyRun(t, bin, script)
		if got != want {
			t.Errorf("EncodeCommand(%v) = %q, gem RESP3.dump = %q", args, got, want)
		}
	}
}

// TestOracleDecode checks DecodeBytes agrees with RESP3.parse on canned RESP2 and
// RESP3 wire streams, comparing the gem's #inspect of its parsed value against a
// rendering of this package's value model.
func TestOracleDecode(t *testing.T) {
	bin := rubyOracle(t)
	cases := []struct {
		name string
		wire string
	}{
		{"simple", "+OK\r\n"},
		{"integer", ":1000\r\n"},
		{"bulk", "$5\r\nhello\r\n"},
		{"null-bulk", "$-1\r\n"},
		{"array", "*2\r\n$3\r\nfoo\r\n$3\r\nbar\r\n"},
		{"int-array", "*3\r\n:1\r\n:2\r\n:3\r\n"},
		{"nested", "*2\r\n*1\r\n:1\r\n*1\r\n:2\r\n"},
		{"resp3-null", "_\r\n"},
		{"resp3-true", "#t\r\n"},
		{"resp3-false", "#f\r\n"},
		{"resp3-double", ",3.14\r\n"},
		{"resp3-double-inf", ",inf\r\n"},
		{"resp3-verbatim", "=15\r\ntxt:Some string\r\n"},
		{"resp3-map", "%2\r\n$1\r\na\r\n:1\r\n$1\r\nb\r\n:2\r\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := DecodeBytes([]byte(tc.wire))
			if err != nil {
				t.Fatalf("DecodeBytes: %v", err)
			}
			script := "p resp3_parse(" + strconv.Quote(tc.wire) + ")"
			want := strings.TrimRight(rubyRun(t, bin, script), "\n")
			if inspect(got) != want {
				t.Errorf("wire %q: DecodeBytes -> %s, gem -> %s", tc.wire, inspect(got), want)
			}
		})
	}
}

// inspect renders a decoded value model the way Ruby's #inspect renders the gem's
// parsed value, so the two can be compared for the oracle. It covers the shapes
// TestOracleDecode exercises.
func inspect(v any) string {
	switch x := v.(type) {
	case nil:
		return "nil"
	case bool:
		return strconv.FormatBool(x)
	case string:
		return strconv.Quote(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		switch {
		case math.IsInf(x, 1):
			return "Infinity"
		case math.IsInf(x, -1):
			return "-Infinity"
		case x == math.Trunc(x):
			return strconv.FormatFloat(x, 'f', 1, 64)
		default:
			return strconv.FormatFloat(x, 'g', -1, 64)
		}
	case *VerbatimString:
		return strconv.Quote(x.Text)
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = inspect(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case *Map:
		var parts []string
		x.Each(func(k, val any) {
			parts = append(parts, inspect(k)+" => "+inspect(val))
		})
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return "?"
	}
}
