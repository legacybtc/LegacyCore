package wire

import (
	"bytes"
	"errors"
	"testing"
)

func TestMessageRoundTrip(t *testing.T) {
	var buf bytes.Buffer
	magic := [4]byte{0xe2, 0x36, 0x24, 0x18}
	if err := WriteMessage(&buf, magic, CommandPing, []byte("legacy")); err != nil {
		t.Fatal(err)
	}
	msg, err := ReadMessage(&buf, magic)
	if err != nil {
		t.Fatal(err)
	}
	if msg.Command != CommandPing {
		t.Fatalf("command=%q", msg.Command)
	}
	if string(msg.Payload) != "legacy" {
		t.Fatalf("payload=%q", msg.Payload)
	}
}

func TestMessageRejectsWrongMagic(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteMessage(&buf, [4]byte{1, 2, 3, 4}, CommandVerAck, nil); err != nil {
		t.Fatal(err)
	}
	_, err := ReadMessage(&buf, [4]byte{0xe2, 0x36, 0x24, 0x18})
	if !errors.Is(err, ErrBadMessageMagic) {
		t.Fatalf("err=%v", err)
	}
}
