// Copyright (c) the go-ruby-redis/redis authors
//
// SPDX-License-Identifier: BSD-3-Clause

package redis

import "testing"

func TestSubscribeConfirmation(t *testing.T) {
	// SUBSCRIBE ch -> ["subscribe", "ch", 1]
	m := newMockConn("*3\r\n$9\r\nsubscribe\r\n$2\r\nch\r\n:1\r\n")
	c := NewFromConn(m)
	msg, err := c.Subscribe("ch")
	if err != nil {
		t.Fatal(err)
	}
	if msg.Kind != KindSubscribe || msg.Channel != "ch" || msg.Count != 1 {
		t.Fatalf("msg %#v", msg)
	}
	if m.writes.String() != "*2\r\n$9\r\nSUBSCRIBE\r\n$2\r\nch\r\n" {
		t.Fatalf("wrote %q", m.writes.String())
	}
}

func TestSubscribeVariants(t *testing.T) {
	c := NewFromConn(newMockConn("*3\r\n$10\r\npsubscribe\r\n$3\r\nch*\r\n:1\r\n"))
	msg, err := c.PSubscribe("ch*")
	if err != nil || msg.Kind != KindPSubscribe {
		t.Fatalf("psub %#v %v", msg, err)
	}
	c = NewFromConn(newMockConn("*3\r\n$11\r\nunsubscribe\r\n$2\r\nch\r\n:0\r\n"))
	msg, err = c.Unsubscribe("ch")
	if err != nil || msg.Kind != KindUnsubscribe {
		t.Fatalf("unsub %#v %v", msg, err)
	}
	c = NewFromConn(newMockConn("*3\r\n$12\r\npunsubscribe\r\n$3\r\nch*\r\n:0\r\n"))
	msg, err = c.PUnsubscribe("ch*")
	if err != nil || msg.Kind != KindPUnsubscribe {
		t.Fatalf("punsub %#v %v", msg, err)
	}
}

func TestSubscribeSeamError(t *testing.T) {
	c := NewFromConn(&errConn{failWrite: true})
	if _, err := c.Subscribe("ch"); err == nil {
		t.Fatal("want write err")
	}
	// Decode error.
	c = NewFromConn(newMockConn(""))
	if _, err := c.Subscribe("ch"); err == nil {
		t.Fatal("want decode err")
	}
}

func TestPublish(t *testing.T) {
	runCmd(t, ":2\r\n", "*3\r\n$7\r\nPUBLISH\r\n$2\r\nch\r\n$2\r\nhi\r\n",
		func(c *Client) (any, error) { return c.Publish("ch", "hi") }, int64(2))
}

func TestNextMessage(t *testing.T) {
	// A "message" delivery.
	m := newMockConn("*3\r\n$7\r\nmessage\r\n$2\r\nch\r\n$5\r\nhello\r\n")
	c := NewFromConn(m)
	msg, err := c.NextMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Kind != KindMessage || msg.Channel != "ch" || msg.Payload != "hello" {
		t.Fatalf("msg %#v", msg)
	}
}

func TestNextMessagePMessage(t *testing.T) {
	m := newMockConn("*4\r\n$8\r\npmessage\r\n$3\r\nch*\r\n$2\r\nch\r\n$2\r\nhi\r\n")
	c := NewFromConn(m)
	msg, err := c.NextMessage()
	if err != nil {
		t.Fatal(err)
	}
	if msg.Kind != KindPMessage || msg.Pattern != "ch*" || msg.Channel != "ch" || msg.Payload != "hi" {
		t.Fatalf("pmsg %#v", msg)
	}
}

func TestNextMessagePush(t *testing.T) {
	// RESP3 delivers pub/sub as a push frame.
	m := newMockConn(">3\r\n$7\r\nmessage\r\n$2\r\nch\r\n$2\r\nhi\r\n")
	c := NewFromConn(m)
	msg, err := c.NextMessage()
	if err != nil || msg.Kind != KindMessage || msg.Payload != "hi" {
		t.Fatalf("push msg %#v %v", msg, err)
	}
}

func TestNextMessageRoundTripperRejected(t *testing.T) {
	c := NewFromRoundTripper(&mockRT{})
	if _, err := c.NextMessage(); err == nil {
		t.Fatal("want streaming-required err")
	}
}

func TestNextMessageDecodeError(t *testing.T) {
	c := NewFromConn(newMockConn(""))
	if _, err := c.NextMessage(); err == nil {
		t.Fatal("want decode err")
	}
}

func TestClassifyMessageErrors(t *testing.T) {
	// Wrong top-level type.
	if _, err := classifyMessage(int64(1)); err == nil {
		t.Fatal("want type err")
	}
	// Too few elements.
	if _, err := classifyMessage([]any{"message", "ch"}); err == nil {
		t.Fatal("want short err")
	}
	// Non-string kind.
	if _, err := classifyMessage([]any{int64(1), "ch", "p"}); err == nil {
		t.Fatal("want kind err")
	}
	// pmessage too short.
	if _, err := classifyMessage([]any{"pmessage", "ch*", "ch"}); err == nil {
		t.Fatal("want pmessage short err")
	}
}

func TestAsStr(t *testing.T) {
	if asStr(&VerbatimString{Text: "v"}) != "v" {
		t.Fatal("verbatim")
	}
	if asStr(int64(1)) != "" {
		t.Fatal("non-string")
	}
}

func TestSubscribeCountNonInt(t *testing.T) {
	// A confirmation whose count is not an integer leaves Count at 0.
	msg, err := classifyMessage([]any{"subscribe", "ch", "notint"})
	if err != nil || msg.Count != 0 {
		t.Fatalf("msg %#v %v", msg, err)
	}
}
