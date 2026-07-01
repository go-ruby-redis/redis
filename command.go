// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

// This file is the command layer: the client method surface that builds RESP
// commands, sends them over the host seam and coerces each reply to the type the
// redis / redis-client gem returns. The generic entry points are Call (raw) and
// the typed helpers; the named methods (Get, Set, HGetAll, …) wrap them with the
// gem's argument order and reply coercion.

// Call sends one command — a variadic name plus arguments — and returns the raw
// decoded reply (the value model, no per-command coercion), mirroring
// redis.call([...]). A Redis error reply is promoted to a returned error, as the
// gem raises Redis::CommandError. This is the escape hatch for commands the typed
// surface does not wrap.
func (c *Client) Call(args ...any) (any, error) {
	dec, err := c.send(EncodeCommand(args...))
	if err != nil {
		return nil, err
	}
	reply, err := dec.Decode()
	if err != nil {
		return nil, err
	}
	if e := asError(reply); e != nil {
		return nil, e
	}
	return reply, nil
}

// callCoerce runs Call and applies a coercion to the successful reply.
func (c *Client) callCoerce(coerce func(any) (any, error), args ...any) (any, error) {
	reply, err := c.Call(args...)
	if err != nil {
		return nil, err
	}
	return coerce(reply)
}

// --- Strings -------------------------------------------------------------

// Get returns the string value of key, or nil if the key is absent (GET).
func (c *Client) Get(key any) (any, error) {
	return c.callCoerce(toString, "GET", key)
}

// Set stores value at key and returns the "OK" status (SET). Extra options
// (EX/PX/NX/XX/…) may be appended as further args.
func (c *Client) Set(key, value any, opts ...any) (any, error) {
	args := append([]any{"SET", key, value}, opts...)
	return c.callCoerce(toString, args...)
}

// SetNX sets key to value only if it does not exist, returning a boolean (SETNX).
func (c *Client) SetNX(key, value any) (any, error) {
	return c.callCoerce(toBoolFromInt, "SETNX", key, value)
}

// GetSet atomically sets key to value and returns its old value (GETSET).
func (c *Client) GetSet(key, value any) (any, error) {
	return c.callCoerce(toString, "GETSET", key, value)
}

// Append appends value to key and returns the new length (APPEND).
func (c *Client) Append(key, value any) (any, error) {
	return c.callCoerce(toInt, "APPEND", key, value)
}

// Strlen returns the length of the string at key (STRLEN).
func (c *Client) Strlen(key any) (any, error) {
	return c.callCoerce(toInt, "STRLEN", key)
}

// GetRange returns the substring of key between start and stop (GETRANGE).
func (c *Client) GetRange(key, start, stop any) (any, error) {
	return c.callCoerce(toString, "GETRANGE", key, start, stop)
}

// SetRange overwrites part of key starting at offset, returning the new length
// (SETRANGE).
func (c *Client) SetRange(key, offset, value any) (any, error) {
	return c.callCoerce(toInt, "SETRANGE", key, offset, value)
}

// Incr increments the integer at key by one (INCR).
func (c *Client) Incr(key any) (any, error) {
	return c.callCoerce(toInt, "INCR", key)
}

// IncrBy increments the integer at key by n (INCRBY).
func (c *Client) IncrBy(key, n any) (any, error) {
	return c.callCoerce(toInt, "INCRBY", key, n)
}

// IncrByFloat increments the float at key by n, returning a Float (INCRBYFLOAT).
func (c *Client) IncrByFloat(key, n any) (any, error) {
	return c.callCoerce(toFloat, "INCRBYFLOAT", key, n)
}

// Decr decrements the integer at key by one (DECR).
func (c *Client) Decr(key any) (any, error) {
	return c.callCoerce(toInt, "DECR", key)
}

// DecrBy decrements the integer at key by n (DECRBY).
func (c *Client) DecrBy(key, n any) (any, error) {
	return c.callCoerce(toInt, "DECRBY", key, n)
}

// MGet returns the values of the given keys as an Array with nil for missing
// keys (MGET).
func (c *Client) MGet(keys ...any) (any, error) {
	return c.callCoerce(toStringArray, append([]any{"MGET"}, keys...)...)
}

// MSet sets the given key/value pairs and returns "OK" (MSET).
func (c *Client) MSet(pairs ...any) (any, error) {
	return c.callCoerce(toString, append([]any{"MSET"}, pairs...)...)
}

// --- Keys ----------------------------------------------------------------

// Del removes the given keys and returns the count removed (DEL).
func (c *Client) Del(keys ...any) (any, error) {
	return c.callCoerce(toInt, append([]any{"DEL"}, keys...)...)
}

// Exists returns how many of the given keys exist, as an Integer (EXISTS).
func (c *Client) Exists(keys ...any) (any, error) {
	return c.callCoerce(toInt, append([]any{"EXISTS"}, keys...)...)
}

// Expire sets a key's TTL in seconds and returns a boolean (EXPIRE).
func (c *Client) Expire(key, seconds any) (any, error) {
	return c.callCoerce(toBoolFromInt, "EXPIRE", key, seconds)
}

// ExpireAt sets a key's expiry to a UNIX timestamp, returning a boolean
// (EXPIREAT).
func (c *Client) ExpireAt(key, ts any) (any, error) {
	return c.callCoerce(toBoolFromInt, "EXPIREAT", key, ts)
}

// Persist removes a key's TTL, returning a boolean (PERSIST).
func (c *Client) Persist(key any) (any, error) {
	return c.callCoerce(toBoolFromInt, "PERSIST", key)
}

// TTL returns a key's remaining time-to-live in seconds (TTL): -2 if the key is
// gone, -1 if it has no expiry.
func (c *Client) TTL(key any) (any, error) {
	return c.callCoerce(toInt, "TTL", key)
}

// PTTL returns a key's TTL in milliseconds (PTTL).
func (c *Client) PTTL(key any) (any, error) {
	return c.callCoerce(toInt, "PTTL", key)
}

// Type returns a key's type name as a String (TYPE).
func (c *Client) Type(key any) (any, error) {
	return c.callCoerce(toString, "TYPE", key)
}

// Rename renames a key, returning "OK" (RENAME).
func (c *Client) Rename(key, newkey any) (any, error) {
	return c.callCoerce(toString, "RENAME", key, newkey)
}

// Keys returns all keys matching pattern as an Array of String (KEYS).
func (c *Client) Keys(pattern any) (any, error) {
	return c.callCoerce(toStringArray, "KEYS", pattern)
}

// Scan performs one SCAN iteration from cursor, returning a [ScanResult]. Extra
// options (MATCH/COUNT/TYPE) may be appended as further args.
func (c *Client) Scan(cursor any, opts ...any) (any, error) {
	return c.callCoerce(toScanResult, append([]any{"SCAN", cursor}, opts...)...)
}

// --- Hashes --------------------------------------------------------------

// HSet sets one or more field/value pairs on a hash and returns the number of
// new fields (HSET).
func (c *Client) HSet(key any, pairs ...any) (any, error) {
	return c.callCoerce(toInt, append([]any{"HSET", key}, pairs...)...)
}

// HGet returns the value of a hash field, or nil (HGET).
func (c *Client) HGet(key, field any) (any, error) {
	return c.callCoerce(toString, "HGET", key, field)
}

// HGetAll returns a hash's fields and values as an ordered [Map] (HGETALL).
func (c *Client) HGetAll(key any) (any, error) {
	return c.callCoerce(toMapFromPairs, "HGETALL", key)
}

// HDel removes hash fields and returns the count removed (HDEL).
func (c *Client) HDel(key any, fields ...any) (any, error) {
	return c.callCoerce(toInt, append([]any{"HDEL", key}, fields...)...)
}

// HExists reports whether a hash field exists, as a boolean (HEXISTS).
func (c *Client) HExists(key, field any) (any, error) {
	return c.callCoerce(toBoolFromInt, "HEXISTS", key, field)
}

// HKeys returns a hash's field names as an Array (HKEYS).
func (c *Client) HKeys(key any) (any, error) {
	return c.callCoerce(toStringArray, "HKEYS", key)
}

// HVals returns a hash's values as an Array (HVALS).
func (c *Client) HVals(key any) (any, error) {
	return c.callCoerce(toStringArray, "HVALS", key)
}

// HLen returns the number of fields in a hash (HLEN).
func (c *Client) HLen(key any) (any, error) {
	return c.callCoerce(toInt, "HLEN", key)
}

// HIncrBy increments a hash field by an integer (HINCRBY).
func (c *Client) HIncrBy(key, field, n any) (any, error) {
	return c.callCoerce(toInt, "HINCRBY", key, field, n)
}

// HIncrByFloat increments a hash field by a float, returning a Float
// (HINCRBYFLOAT).
func (c *Client) HIncrByFloat(key, field, n any) (any, error) {
	return c.callCoerce(toFloat, "HINCRBYFLOAT", key, field, n)
}

// HMGet returns the values of the given hash fields as an Array (HMGET).
func (c *Client) HMGet(key any, fields ...any) (any, error) {
	return c.callCoerce(toStringArray, append([]any{"HMGET", key}, fields...)...)
}

// --- Lists ---------------------------------------------------------------

// LPush prepends values to a list and returns the new length (LPUSH).
func (c *Client) LPush(key any, values ...any) (any, error) {
	return c.callCoerce(toInt, append([]any{"LPUSH", key}, values...)...)
}

// RPush appends values to a list and returns the new length (RPUSH).
func (c *Client) RPush(key any, values ...any) (any, error) {
	return c.callCoerce(toInt, append([]any{"RPUSH", key}, values...)...)
}

// LPop pops from the head of a list, returning a String or nil (LPOP).
func (c *Client) LPop(key any) (any, error) {
	return c.callCoerce(toString, "LPOP", key)
}

// RPop pops from the tail of a list, returning a String or nil (RPOP).
func (c *Client) RPop(key any) (any, error) {
	return c.callCoerce(toString, "RPOP", key)
}

// LLen returns the length of a list (LLEN).
func (c *Client) LLen(key any) (any, error) {
	return c.callCoerce(toInt, "LLEN", key)
}

// LRange returns the elements of a list between start and stop as an Array
// (LRANGE).
func (c *Client) LRange(key, start, stop any) (any, error) {
	return c.callCoerce(toStringArray, "LRANGE", key, start, stop)
}

// LIndex returns the list element at index, or nil (LINDEX).
func (c *Client) LIndex(key, index any) (any, error) {
	return c.callCoerce(toString, "LINDEX", key, index)
}

// LSet sets the list element at index, returning "OK" (LSET).
func (c *Client) LSet(key, index, value any) (any, error) {
	return c.callCoerce(toString, "LSET", key, index, value)
}

// LTrim trims a list to the given range, returning "OK" (LTRIM).
func (c *Client) LTrim(key, start, stop any) (any, error) {
	return c.callCoerce(toString, "LTRIM", key, start, stop)
}

// --- Sets ----------------------------------------------------------------

// SAdd adds members to a set and returns the count added (SADD).
func (c *Client) SAdd(key any, members ...any) (any, error) {
	return c.callCoerce(toInt, append([]any{"SADD", key}, members...)...)
}

// SRem removes members from a set and returns the count removed (SREM).
func (c *Client) SRem(key any, members ...any) (any, error) {
	return c.callCoerce(toInt, append([]any{"SREM", key}, members...)...)
}

// SMembers returns all members of a set as a Ruby [Set] (SMEMBERS).
func (c *Client) SMembers(key any) (any, error) {
	return c.callCoerce(toSet, "SMEMBERS", key)
}

// SIsMember reports whether a value is a set member, as a boolean (SISMEMBER).
func (c *Client) SIsMember(key, member any) (any, error) {
	return c.callCoerce(toBoolFromInt, "SISMEMBER", key, member)
}

// SCard returns the cardinality of a set (SCARD).
func (c *Client) SCard(key any) (any, error) {
	return c.callCoerce(toInt, "SCARD", key)
}

// SUnion returns the union of the given sets as a Ruby [Set] (SUNION).
func (c *Client) SUnion(keys ...any) (any, error) {
	return c.callCoerce(toSet, append([]any{"SUNION"}, keys...)...)
}

// SInter returns the intersection of the given sets as a Ruby [Set] (SINTER).
func (c *Client) SInter(keys ...any) (any, error) {
	return c.callCoerce(toSet, append([]any{"SINTER"}, keys...)...)
}

// SDiff returns the difference of the given sets as a Ruby [Set] (SDIFF).
func (c *Client) SDiff(keys ...any) (any, error) {
	return c.callCoerce(toSet, append([]any{"SDIFF"}, keys...)...)
}

// --- Sorted sets ---------------------------------------------------------

// ZAdd adds scored members to a sorted set, returning the count added (ZADD).
// Arguments alternate score, member (plus any leading NX/XX/GT/LT/CH options).
func (c *Client) ZAdd(key any, scoreMembers ...any) (any, error) {
	return c.callCoerce(toInt, append([]any{"ZADD", key}, scoreMembers...)...)
}

// ZScore returns a member's score as a Float, or nil (ZSCORE).
func (c *Client) ZScore(key, member any) (any, error) {
	return c.callCoerce(toFloat, "ZSCORE", key, member)
}

// ZRange returns members in the given rank range as an Array (ZRANGE). Append
// "WITHSCORES" to interleave scores.
func (c *Client) ZRange(key, start, stop any, opts ...any) (any, error) {
	return c.callCoerce(toStringArray, append([]any{"ZRANGE", key, start, stop}, opts...)...)
}

// ZRank returns a member's rank, or nil (ZRANK).
func (c *Client) ZRank(key, member any) (any, error) {
	return c.callCoerce(toInt, "ZRANK", key, member)
}

// ZCard returns the cardinality of a sorted set (ZCARD).
func (c *Client) ZCard(key any) (any, error) {
	return c.callCoerce(toInt, "ZCARD", key)
}

// ZIncrBy increments a member's score, returning the new score as a Float
// (ZINCRBY).
func (c *Client) ZIncrBy(key, increment, member any) (any, error) {
	return c.callCoerce(toFloat, "ZINCRBY", key, increment, member)
}

// ZRem removes members from a sorted set, returning the count removed (ZREM).
func (c *Client) ZRem(key any, members ...any) (any, error) {
	return c.callCoerce(toInt, append([]any{"ZREM", key}, members...)...)
}

// --- Server / connection -------------------------------------------------

// Ping returns "PONG" (or echoes its message argument) (PING).
func (c *Client) Ping(message ...any) (any, error) {
	return c.callCoerce(toString, append([]any{"PING"}, message...)...)
}

// Echo returns its message argument (ECHO).
func (c *Client) Echo(message any) (any, error) {
	return c.callCoerce(toString, "ECHO", message)
}

// Select switches the connection to database index, returning "OK" (SELECT).
func (c *Client) Select(index any) (any, error) {
	return c.callCoerce(toString, "SELECT", index)
}

// Auth authenticates the connection, returning "OK" (AUTH). Pass one argument
// for password-only auth or two for username+password (ACL) auth.
func (c *Client) Auth(args ...any) (any, error) {
	return c.callCoerce(toString, append([]any{"AUTH"}, args...)...)
}

// FlushDB empties the current database, returning "OK" (FLUSHDB).
func (c *Client) FlushDB() (any, error) {
	return c.callCoerce(toString, "FLUSHDB")
}

// ConfigGet returns matching configuration parameters as an ordered [Map]
// (CONFIG GET).
func (c *Client) ConfigGet(pattern any) (any, error) {
	return c.callCoerce(toMapFromPairs, "CONFIG", "GET", pattern)
}

// Hello performs the RESP3 handshake, returning the server's HELLO reply as an
// ordered [Map] (HELLO). Pass the protocol version (2 or 3) and any AUTH/SETNAME
// options; requesting 3 flips the connection into RESP3 for subsequent replies.
func (c *Client) Hello(args ...any) (any, error) {
	reply, err := c.callCoerce(toMapFromPairs, append([]any{"HELLO"}, args...)...)
	if err != nil {
		return nil, err
	}
	for _, a := range args {
		if arg(a) == "3" {
			c.resp3 = true
			break
		}
	}
	return reply, nil
}

// Handshake applies the recorded [Options] to a fresh connection the way the gem
// does after connecting: HELLO for RESP3 (or AUTH for RESP2), then SELECT. It is
// a convenience over the individual commands; a host that manages its own
// handshake need not call it.
func (c *Client) Handshake() error {
	if c.opts.Protocol == 3 {
		args := []any{3}
		if c.opts.Password != "" {
			user := c.opts.Username
			if user == "" {
				user = "default"
			}
			args = append(args, "AUTH", user, c.opts.Password)
		}
		if _, err := c.Hello(args...); err != nil {
			return err
		}
	} else if c.opts.Password != "" {
		var authArgs []any
		if c.opts.Username != "" {
			authArgs = append(authArgs, c.opts.Username)
		}
		authArgs = append(authArgs, c.opts.Password)
		if _, err := c.Auth(authArgs...); err != nil {
			return err
		}
	}
	if c.opts.DB != 0 {
		if _, err := c.Select(c.opts.DB); err != nil {
			return err
		}
	}
	return nil
}

// IsRESP3 reports whether the connection has been switched to RESP3.
func (c *Client) IsRESP3() bool { return c.resp3 }
