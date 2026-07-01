// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

// This file holds the batched command surfaces — pipelining and MULTI/EXEC
// transactions — plus pub/sub message framing. Each batches several encoded
// commands into one write and reads the matching run of replies, exactly as the
// gem's redis.pipelined and redis.multi blocks do.

// Batch accumulates commands to be sent together. redis.pipelined and redis.multi
// yield an object like this; each queued command records its arguments, and the
// batch is flushed as one write with the replies read back in order.
type Batch struct {
	cmds [][]any
}

// Add queues a command (name plus arguments) on the batch. It mirrors calling a
// command method inside a pipelined/multi block: the command is not sent until
// the batch is executed.
func (b *Batch) Add(args ...any) *Batch {
	b.cmds = append(b.cmds, args)
	return b
}

// Len reports how many commands are queued.
func (b *Batch) Len() int { return len(b.cmds) }

// encode renders every queued command back-to-back into one request buffer.
func (b *Batch) encode() []byte {
	var buf []byte
	for _, cmd := range b.cmds {
		buf = append(buf, EncodeCommand(cmd...)...)
	}
	return buf
}

// Pipelined runs fn to queue commands on a fresh [Batch], sends them all in one
// write, then reads and returns each reply in order (redis.pipelined). A Redis
// error reply for a queued command is returned as a *CommandError value in the
// results slice (not a returned error), so one failed command does not mask the
// others — matching the gem, which collects per-command results. A stream or
// protocol fault aborts with a non-nil error.
func (c *Client) Pipelined(fn func(*Batch)) ([]any, error) {
	b := &Batch{}
	fn(b)
	if b.Len() == 0 {
		return []any{}, nil
	}
	dec, err := c.send(b.encode())
	if err != nil {
		return nil, err
	}
	results := make([]any, b.Len())
	for i := range results {
		reply, err := dec.Decode()
		if err != nil {
			return nil, err
		}
		results[i] = reply
	}
	return results, nil
}

// Multi runs fn to queue commands, wraps them in a MULTI/EXEC transaction and
// returns the array of results EXEC yields (redis.multi). The wire framing is
// MULTI, then every queued command (each answered "+QUEUED"), then EXEC, whose
// reply is the array of the queued commands' results. A nil EXEC reply means the
// transaction was aborted (a watched key changed); it is returned as nil with no
// error, as the gem returns nil from a discarded multi block.
func (c *Client) Multi(fn func(*Batch)) (any, error) {
	b := &Batch{}
	fn(b)

	var req []byte
	req = append(req, EncodeCommand("MULTI")...)
	req = append(req, b.encode()...)
	req = append(req, EncodeCommand("EXEC")...)

	dec, err := c.send(req)
	if err != nil {
		return nil, err
	}

	// MULTI's own reply ("+OK").
	if reply, err := dec.Decode(); err != nil {
		return nil, err
	} else if e := asError(reply); e != nil {
		return nil, e
	}
	// Each queued command replies "+QUEUED"; drain them.
	for range b.cmds {
		if reply, err := dec.Decode(); err != nil {
			return nil, err
		} else if e := asError(reply); e != nil {
			return nil, e
		}
	}
	// EXEC's reply: the array of results, or nil if the transaction aborted.
	exec, err := dec.Decode()
	if err != nil {
		return nil, err
	}
	if e := asError(exec); e != nil {
		return nil, e
	}
	return exec, nil
}
