package p2p

import (
	"bytes"
	"io"
	"log"
	"net"
	"sync"
	"testing"

	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/wire"
)

func TestAnnounceBlockSendsInvToAllConnectedPeers(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))

	peerCount := 3
	type pipePair struct {
		server, client net.Conn
	}
	pipes := make([]pipePair, peerCount)
	for i := 0; i < peerCount; i++ {
		serverEnd, clientEnd := net.Pipe()
		pipes[i] = pipePair{serverEnd, clientEnd}
		s.activeMu.Lock()
		s.activePeers[&peer{
			conn:    serverEnd,
			remote:  "mock-peer-" + string(rune('A'+i)),
			writeMu: sync.Mutex{},
		}] = struct{}{}
		s.activeMu.Unlock()
	}

	type result struct {
		index int
		msg   wire.Message
		err   error
	}
	results := make(chan result, peerCount)

	for i := 0; i < peerCount; i++ {
		go func(idx int, client net.Conn) {
			msg, err := wire.ReadMessage(client, chaincfg.MainNet.MessageStart)
			results <- result{idx, msg, err}
		}(i, pipes[i].client)
	}

	hash := chainhash.Hash{0xab, 0xcd, 0xef}
	s.AnnounceBlock(hash)

	for i := 0; i < peerCount; i++ {
		r := <-results
		if r.err != nil {
			t.Fatalf("peer %d: read message: %v", r.index, r.err)
		}
		if r.msg.Command != wire.CommandInv {
			t.Fatalf("peer %d: command=%q want=%q", r.index, r.msg.Command, wire.CommandInv)
		}
		inv, err := wire.ReadInvPayload(bytes.NewReader(r.msg.Payload))
		if err != nil {
			t.Fatalf("peer %d: read inv payload: %v", r.index, err)
		}
		if len(inv) != 1 {
			t.Fatalf("peer %d: inv count=%d want=1", r.index, len(inv))
		}
		if inv[0].Hash != hash {
			t.Fatalf("peer %d: inv hash=%x want=%x", r.index, inv[0].Hash[:], hash[:])
		}
	}

	for _, p := range pipes {
		p.client.Close()
		p.server.Close()
	}
}

func TestAnnounceBlockSkipsNilPeer(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))

	serverEnd, clientEnd := net.Pipe()
	defer clientEnd.Close()
	defer serverEnd.Close()

	s.activeMu.Lock()
	s.activePeers[nil] = struct{}{}
	s.activePeers[&peer{
		conn:    serverEnd,
		remote:  "real-peer",
		writeMu: sync.Mutex{},
	}] = struct{}{}
	s.activeMu.Unlock()

	msgCh := make(chan wire.Message, 1)
	errCh := make(chan error, 1)
	go func() {
		msg, err := wire.ReadMessage(clientEnd, chaincfg.MainNet.MessageStart)
		msgCh <- msg
		errCh <- err
	}()

	hash := chainhash.Hash{0x01, 0x02, 0x03}
	s.AnnounceBlock(hash)

	msg := <-msgCh
	err := <-errCh
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if msg.Command != wire.CommandInv {
		t.Fatalf("command=%q want=%q", msg.Command, wire.CommandInv)
	}
	inv, err := wire.ReadInvPayload(bytes.NewReader(msg.Payload))
	if err != nil {
		t.Fatalf("read inv payload: %v", err)
	}
	if len(inv) != 1 {
		t.Fatalf("inv count=%d want=1", len(inv))
	}
	if inv[0].Hash != hash {
		t.Fatalf("inv hash=%x want=%x", inv[0].Hash[:], hash[:])
	}
}

func TestAnnounceBlockTargetCountNotReceiptConfirmation(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))

	serverEnd, clientEnd := net.Pipe()
	defer clientEnd.Close()
	defer serverEnd.Close()

	before := s.blocksAnnounced
	s.activeMu.Lock()
	s.activePeers[&peer{
		conn:    serverEnd,
		remote:  "target-peer",
		writeMu: sync.Mutex{},
	}] = struct{}{}
	s.activeMu.Unlock()

	msgCh := make(chan struct{}, 1)
	go func() {
		wire.ReadMessage(clientEnd, chaincfg.MainNet.MessageStart)
		msgCh <- struct{}{}
	}()

	hash := chainhash.Hash{0xfe, 0xdc, 0xba}
	s.AnnounceBlock(hash)

	after := s.blocksAnnounced
	if after-before != 1 {
		t.Fatalf("blocksAnnounced delta=%d want=1 (target count counts peers targeted, not successful receipt)", after-before)
	}

	<-msgCh
}
