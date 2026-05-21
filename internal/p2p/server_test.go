package p2p

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"reflect"
	"testing"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/storage"
	"legacycoin/legacy-go/internal/wire"
)

func TestLimitInv(t *testing.T) {
	inv := make([]wire.InvVect, 10)
	got := limitInv(inv, 3)
	if len(got) != 3 {
		t.Fatalf("len=%d want=3", len(got))
	}
	got = limitInv(inv, 20)
	if len(got) != 10 {
		t.Fatalf("len=%d want=10", len(got))
	}
	got = limitInv(inv, 0)
	if len(got) != 0 {
		t.Fatalf("len=%d want=0", len(got))
	}
}

func TestBootstrapPeersSetGet(t *testing.T) {
	s := &Server{}
	in := []string{"127.0.0.1:19555", "legacycoinseed.space"}
	s.SetBootstrapPeers(in)
	got := s.BootstrapPeers()
	if !reflect.DeepEqual(got, in) {
		t.Fatalf("bootstrap=%v want=%v", got, in)
	}
	in[0] = "changed"
	if got2 := s.BootstrapPeers(); got2[0] != "127.0.0.1:19555" {
		t.Fatalf("bootstrap slice was not copied")
	}
}

func TestPostHandshakeIdlePeerIsDisconnected(t *testing.T) {
	oldHandshake := peerHandshakeTimeout
	oldIdle := peerIdleTimeout
	peerHandshakeTimeout = 250 * time.Millisecond
	peerIdleTimeout = 50 * time.Millisecond
	defer func() {
		peerHandshakeTimeout = oldHandshake
		peerIdleTimeout = oldIdle
	}()

	chain, err := blockchain.New(chaincfg.MainNet, p2pTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	s := New(chaincfg.MainNet, chain, nil, log.New(io.Discard, "", 0))

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		s.handleConn(ctx, serverConn, false)
		close(done)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := wire.ReadMessage(clientConn, chaincfg.MainNet.MessageStart); err != nil {
		t.Fatalf("read server version: %v", err)
	}
	payload, err := s.versionPayload(serverConn.LocalAddr())
	if err != nil {
		t.Fatalf("build client version payload: %v", err)
	}
	if err := wire.WriteMessage(clientConn, chaincfg.MainNet.MessageStart, wire.CommandVersion, payload); err != nil {
		t.Fatalf("write client version: %v", err)
	}
	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if msg, err := wire.ReadMessage(clientConn, chaincfg.MainNet.MessageStart); err != nil || msg.Command != wire.CommandVerAck {
		t.Fatalf("read server verack: msg=%v err=%v", msg, err)
	}
	if err := wire.WriteMessage(clientConn, chaincfg.MainNet.MessageStart, wire.CommandVerAck, nil); err != nil {
		t.Fatalf("write client verack: %v", err)
	}
	_ = clientConn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	if msg, err := wire.ReadMessage(clientConn, chaincfg.MainNet.MessageStart); err != nil {
		if err != io.EOF {
			t.Fatalf("read optional sync getheaders: msg=%v err=%v", msg, err)
		}
	} else if msg.Command != wire.CommandGetHeaders {
		t.Fatalf("read sync getheaders: msg=%v err=%v", msg, err)
	}
	_ = clientConn.SetReadDeadline(time.Time{})

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("idle post-handshake peer was not disconnected")
	}
}

type p2pTestHasher struct{}

func (p2pTestHasher) HashHeader(header wire.BlockHeader) (chainhash.Hash, error) {
	b, err := header.Bytes()
	if err != nil {
		return chainhash.Hash{}, err
	}
	return chainhash.DoubleHashB(b), nil
}

func TestRequestUnknownBlockInvSendsGetData(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, p2pTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	s := New(chaincfg.MainNet, chain, nil, log.New(io.Discard, "", 0))

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{conn: serverConn, remote: "pipe-test", lastSeen: time.Now(), lastPong: time.Now()}

	unknown := chainhash.DoubleHashB([]byte("unknown block announced by inv"))
	done := make(chan error, 1)
	go func() {
		done <- s.requestUnknownBlocks(p, []wire.InvVect{{Type: wire.InvTypeBlock, Hash: unknown}})
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msg, err := wire.ReadMessage(clientConn, chaincfg.MainNet.MessageStart)
	if err != nil {
		t.Fatalf("read getdata: %v", err)
	}
	if msg.Command != wire.CommandGetData {
		t.Fatalf("first command=%s want %s", msg.Command, wire.CommandGetData)
	}

	inv, err := wire.ReadInvPayload(bytes.NewReader(msg.Payload))
	if err != nil {
		t.Fatalf("parse getdata payload: %v", err)
	}
	if len(inv) != 1 || inv[0].Type != wire.InvTypeBlock || inv[0].Hash != unknown {
		t.Fatalf("getdata inv=%+v want block %s", inv, unknown.String())
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("requestUnknownBlocks: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("requestUnknownBlocks did not return")
	}
}
