// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

// This file holds pub/sub message framing: subscribing/unsubscribing and
// classifying the messages the server pushes on the connection. The gem's
// redis.subscribe block dispatches on the message kind; this package exposes the
// same classification over the value model so a host can build that loop.

// MessageKind classifies a pub/sub frame.
type MessageKind string

const (
	// KindSubscribe is a "subscribe" confirmation (channel subscribed).
	KindSubscribe MessageKind = "subscribe"
	// KindUnsubscribe is an "unsubscribe" confirmation.
	KindUnsubscribe MessageKind = "unsubscribe"
	// KindPSubscribe is a "psubscribe" pattern-subscription confirmation.
	KindPSubscribe MessageKind = "psubscribe"
	// KindPUnsubscribe is a "punsubscribe" confirmation.
	KindPUnsubscribe MessageKind = "punsubscribe"
	// KindMessage is a "message" delivery on a subscribed channel.
	KindMessage MessageKind = "message"
	// KindPMessage is a "pmessage" delivery matching a subscribed pattern.
	KindPMessage MessageKind = "pmessage"
)

// Message is a decoded pub/sub frame. Depending on Kind, some fields are unset:
// a "message" carries Channel and Payload; a "pmessage" also carries Pattern; a
// subscribe/unsubscribe confirmation carries Channel and Count (the number of
// channels/patterns still subscribed).
type Message struct {
	Kind    MessageKind
	Pattern string // set for pmessage / (p)subscribe patterns
	Channel string // channel (or pattern for psubscribe confirmations)
	Payload string // set for message / pmessage
	Count   int64  // set for (un)subscribe confirmations
}

// Subscribe sends a SUBSCRIBE for the given channels and returns the request's
// first reply (the first subscribe confirmation) as a [Message]. Redis answers
// one confirmation per channel; a host reads the remaining confirmations and the
// subsequent deliveries with [Client.NextMessage].
func (c *Client) Subscribe(channels ...any) (*Message, error) {
	return c.subscribeCmd("SUBSCRIBE", channels...)
}

// PSubscribe sends a PSUBSCRIBE for the given patterns (PSUBSCRIBE).
func (c *Client) PSubscribe(patterns ...any) (*Message, error) {
	return c.subscribeCmd("PSUBSCRIBE", patterns...)
}

// Unsubscribe sends an UNSUBSCRIBE (UNSUBSCRIBE). With no channels it
// unsubscribes from all.
func (c *Client) Unsubscribe(channels ...any) (*Message, error) {
	return c.subscribeCmd("UNSUBSCRIBE", channels...)
}

// PUnsubscribe sends a PUNSUBSCRIBE (PUNSUBSCRIBE).
func (c *Client) PUnsubscribe(patterns ...any) (*Message, error) {
	return c.subscribeCmd("PUNSUBSCRIBE", patterns...)
}

// subscribeCmd sends a (p)(un)subscribe command and reads its first reply frame.
func (c *Client) subscribeCmd(name string, args ...any) (*Message, error) {
	dec, err := c.send(EncodeCommand(append([]any{name}, args...)...))
	if err != nil {
		return nil, err
	}
	reply, err := dec.Decode()
	if err != nil {
		return nil, err
	}
	return classifyMessage(reply)
}

// Publish sends a PUBLISH and returns the number of subscribers that received
// the message (PUBLISH).
func (c *Client) Publish(channel, message any) (any, error) {
	return c.callCoerce(toInt, "PUBLISH", channel, message)
}

// NextMessage reads and classifies the next pub/sub frame from the connection —
// the read side of a redis.subscribe loop. It only works on a streaming [Conn]
// client (pub/sub is inherently multi-reply); on a [RoundTripper] client it
// returns an error.
func (c *Client) NextMessage() (*Message, error) {
	if c.dec == nil {
		return nil, &CommandError{Message: "ERR NextMessage requires a streaming connection"}
	}
	reply, err := c.dec.Decode()
	if err != nil {
		return nil, err
	}
	return classifyMessage(reply)
}

// classifyMessage turns a decoded frame — an Array (RESP2) or a Push (RESP3) —
// into a typed [Message]. It is the framing logic shared by the subscribe
// commands and NextMessage.
func classifyMessage(reply any) (*Message, error) {
	var elems []any
	switch x := reply.(type) {
	case []any:
		elems = x
	case *Push:
		elems = x.Values
	default:
		return nil, &CommandError{Message: "ERR unexpected pub/sub frame; got " + kindOf(reply)}
	}
	if len(elems) < 3 {
		return nil, &CommandError{Message: "ERR malformed pub/sub frame"}
	}
	kind, ok := elems[0].(string)
	if !ok {
		return nil, &CommandError{Message: "ERR malformed pub/sub frame kind"}
	}
	m := &Message{Kind: MessageKind(kind)}
	switch MessageKind(kind) {
	case KindMessage:
		m.Channel = asStr(elems[1])
		m.Payload = asStr(elems[2])
	case KindPMessage:
		if len(elems) < 4 {
			return nil, &CommandError{Message: "ERR malformed pmessage frame"}
		}
		m.Pattern = asStr(elems[1])
		m.Channel = asStr(elems[2])
		m.Payload = asStr(elems[3])
	default: // (p)(un)subscribe confirmation: [kind, channel, count]
		m.Channel = asStr(elems[1])
		if n, ok := elems[2].(int64); ok {
			m.Count = n
		}
	}
	return m, nil
}

// asStr renders a value-model element to a string (empty for nil).
func asStr(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case *VerbatimString:
		return x.Text
	default:
		return ""
	}
}
