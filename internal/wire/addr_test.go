package wire

import (
	"bytes"
	"net"
	"testing"
)

func TestAddrPayloadRoundTrip(t *testing.T) {
	in := []NetAddress{{
		Timestamp: 1_779_235_200,
		Services:  1,
		IP:        net.ParseIP("203.0.113.10"),
		Port:      19555,
	}}
	payload, err := AddrPayload(in)
	if err != nil {
		t.Fatalf("AddrPayload: %v", err)
	}
	got, err := ReadAddrPayload(bytes.NewReader(payload))
	if err != nil {
		t.Fatalf("ReadAddrPayload: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len=%d want 1", len(got))
	}
	if got[0].Timestamp != in[0].Timestamp || got[0].Services != in[0].Services || got[0].Port != in[0].Port {
		t.Fatalf("addr metadata mismatch: %+v", got[0])
	}
	if !got[0].IP.Equal(in[0].IP) {
		t.Fatalf("ip=%s want %s", got[0].IP, in[0].IP)
	}
}
