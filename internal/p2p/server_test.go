package p2p

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/genesis"
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
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))
	in := []string{"127.0.0.1:19555", "legacycoinseed.space"}
	s.SetBootstrapPeers(in)
	want := []string{"127.0.0.1:19555", "legacycoinseed.space:19555"}
	got := s.BootstrapPeers()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("bootstrap=%v want=%v", got, want)
	}
	in[0] = "changed"
	if got2 := s.BootstrapPeers(); got2[0] != "127.0.0.1:19555" {
		t.Fatalf("bootstrap slice was not copied")
	}
}

func TestSetPeerPolicyNormalizesConnectOnlyPeers(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))
	s.SetPeerPolicy(chaincfg.MainNet.ChainID, false, true, 100, true, []string{"legacycoinseed.space", "192.0.2.10:19555"})
	if _, ok := s.connectOnly["legacycoinseed.space:19555"]; !ok {
		t.Fatalf("connect-only host without port was not normalized: %#v", s.connectOnly)
	}
	if _, ok := s.connectOnly["192.0.2.10:19555"]; !ok {
		t.Fatalf("connect-only host with port missing: %#v", s.connectOnly)
	}
	if err := s.AddNode(context.Background(), "legacycoinseed2.space"); err == nil || !strings.Contains(err.Error(), "connect-only") {
		t.Fatalf("unexpected AddNode error for disallowed peer: %v", err)
	}
}

func TestConnectSeedsDialsFixedSeeds(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()

	accepted := make(chan struct{})
	go func() {
		conn, err := ln.Accept()
		if err == nil {
			_ = conn.Close()
			close(accepted)
		}
	}()

	params := chaincfg.MainNet
	params.DNSSeeds = nil
	params.FixedSeeds = []string{ln.Addr().String()}
	chain, err := blockchain.New(params, p2pTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	s := New(params, chain, nil, log.New(io.Discard, "", 0))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	s.connectSeeds(ctx)
	select {
	case <-accepted:
	case <-ctx.Done():
		t.Fatal("fixed seed was not dialed")
	}
	cancel()
	s.closeActivePeerConnections()
	s.wg.Wait()
}

func TestHandleAddrPayloadCachesDiscoveredPeer(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))
	s.SetPeerPolicy(chaincfg.MainNet.ChainID, false, true, 100, true, []string{"127.0.0.1:65535"})
	payload, err := wire.AddrPayload([]wire.NetAddress{{
		Timestamp: uint32(time.Now().Unix()),
		Services:  1,
		IP:        net.ParseIP("127.0.0.1"),
		Port:      19555,
	}})
	if err != nil {
		t.Fatalf("AddrPayload: %v", err)
	}
	p := &peer{remote: "127.0.0.1:20000", lastSeen: time.Now(), lastPong: time.Now()}
	if err := s.handleAddrPayload(context.Background(), p, payload); err != nil {
		t.Fatalf("handleAddrPayload: %v", err)
	}
	if got := s.KnownAddresses(); len(got) != 1 || got[0] != "127.0.0.1:19555" {
		t.Fatalf("known addresses=%v", got)
	}
}

func TestKnownAddressCacheCapAndPublicFiltering(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))
	for i := 0; i < maxKnownPeerAddresses+25; i++ {
		if !s.rememberPeerAddress(fmt.Sprintf("198.51.100.10:%d", 10_000+i), "test") {
			t.Fatalf("expected new address at %d", i)
		}
	}
	if got := s.KnownAddressCount(); got != maxKnownPeerAddresses {
		t.Fatalf("known address count=%d want %d", got, maxKnownPeerAddresses)
	}

	privatePayload, err := wire.AddrPayload([]wire.NetAddress{{
		Timestamp: uint32(time.Now().Unix()),
		Services:  1,
		IP:        net.ParseIP("10.1.2.3"),
		Port:      19555,
	}})
	if err != nil {
		t.Fatalf("AddrPayload: %v", err)
	}
	publicPeer := &peer{remote: "203.0.113.7:19555", lastSeen: time.Now(), lastPong: time.Now()}
	if err := s.handleAddrPayload(context.Background(), publicPeer, privatePayload); err != nil {
		t.Fatalf("handleAddrPayload: %v", err)
	}
	for _, addr := range s.KnownAddresses() {
		if strings.HasPrefix(addr, "10.1.2.3:") {
			t.Fatalf("private address from public peer was cached: %s", addr)
		}
	}
}

func TestInboundHandshakeDoesNotCacheEphemeralPeerAddress(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))

	s.markKnownPeerConnected("203.0.113.9:36420", false)
	if got := s.KnownAddresses(); len(got) != 0 {
		t.Fatalf("inbound ephemeral address was cached: %v", got)
	}

	const known = "203.0.113.9:19555"
	if !s.rememberPeerAddress(known, "seed") {
		t.Fatalf("expected %s to be remembered", known)
	}
	s.markKnownPeerConnected(known, false)

	s.knownMu.Lock()
	info, ok := s.knownAddresses[known]
	_, ephemeral := s.knownAddresses["203.0.113.9:36420"]
	s.knownMu.Unlock()
	if ephemeral {
		t.Fatal("inbound ephemeral address was cached after known-peer update")
	}
	if !ok {
		t.Fatalf("known peer %s disappeared", known)
	}
	if info.LastDirection != "inbound" || info.Successes != 1 || info.LastConnected.IsZero() {
		t.Fatalf("known inbound peer metadata not updated: %+v", info)
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
	for i := 0; i < 4; i++ {
		_ = clientConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
		msg, err := wire.ReadMessage(clientConn, chaincfg.MainNet.MessageStart)
		if err != nil {
			if err != io.EOF {
				if netErr, ok := err.(net.Error); !ok || !netErr.Timeout() {
					t.Fatalf("read optional post-handshake message: msg=%v err=%v", msg, err)
				}
			}
			break
		}
		if msg.Command != wire.CommandGetHeaders && msg.Command != wire.CommandGetBlocks && msg.Command != wire.CommandAddr && msg.Command != wire.CommandGetAddr && msg.Command != wire.CommandSendHeaders {
			t.Fatalf("read post-handshake maintenance message: msg=%v err=%v", msg, err)
		}
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

	s.getdataMu.Lock()
	_, tracked := s.getdataReqs[unknown.String()]
	s.getdataMu.Unlock()
	if !tracked {
		t.Fatalf("unknown INV block request was not tracked for timeout recovery")
	}
}

func TestRequestMissingParentSendsGetDataForOrphanPrev(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, p2pTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	s := New(chaincfg.MainNet, chain, nil, log.New(io.Discard, "", 0))

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{conn: serverConn, remote: "pipe-orphan", lastSeen: time.Now(), lastPong: time.Now()}

	parent := chainhash.DoubleHashB([]byte("missing parent of an orphan block"))
	done := make(chan error, 1)
	go func() {
		if payload, ok := s.tryClaimMissingParent(parent.String()); ok {
			s.sendMissingParentRequest(p, parent.String(), payload)
		}
		done <- nil
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msg, err := wire.ReadMessage(clientConn, chaincfg.MainNet.MessageStart)
	if err != nil {
		t.Fatalf("read getdata: %v", err)
	}
	if msg.Command != wire.CommandGetData {
		t.Fatalf("command=%s want %s", msg.Command, wire.CommandGetData)
	}
	inv, err := wire.ReadInvPayload(bytes.NewReader(msg.Payload))
	if err != nil {
		t.Fatalf("parse getdata payload: %v", err)
	}
	if len(inv) != 1 || inv[0].Type != wire.InvTypeBlock || inv[0].Hash != parent {
		t.Fatalf("getdata inv=%+v want block %s", inv, parent.String())
	}

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("requestMissingParent did not return")
	}

	// Dedupe: a second call for the same parent within the TTL must NOT
	// queue another write on the pipe (the reader would otherwise block).
	if payload, ok := s.tryClaimMissingParent(parent.String()); ok {
		s.sendMissingParentRequest(p, parent.String(), payload)
		t.Fatal("duplicate missing-parent request was not deduped")
	}
	_ = clientConn.SetReadDeadline(time.Now().Add(300 * time.Millisecond))
	if _, err := wire.ReadMessage(clientConn, chaincfg.MainNet.MessageStart); err == nil {
		t.Fatal("duplicate missing-parent request was re-sent within TTL")
	}
}

func TestDuplicateOrphanResultRetriesMissingParentAfterTTL(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, p2pTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	s := New(chaincfg.MainNet, chain, nil, log.New(io.Discard, "", 0))

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{conn: serverConn, remote: "pipe-duplicate-orphan", lastSeen: time.Now(), lastPong: time.Now()}

	parent := chainhash.DoubleHashB([]byte("missing parent of a duplicate orphan"))
	parentHash := parent.String()
	s.missingParentMu.Lock()
	s.missingParentSeen = map[string]time.Time{
		parentHash: time.Now().Add(-missingParentTTL - time.Second),
	}
	s.missingParentMu.Unlock()

	result := blockchain.BlockProcessResult{
		Hash:        chainhash.DoubleHashB([]byte("duplicate orphan")).String(),
		PrevHash:    parentHash,
		Orphan:      true,
		ParentKnown: false,
		Status:      blockchain.BlockStatusDuplicate,
		Reason:      "orphan already stored",
	}

	done := make(chan bool, 1)
	go func() {
		done <- s.requestMissingParentForResult(p, result)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msg, err := wire.ReadMessage(clientConn, chaincfg.MainNet.MessageStart)
	if err != nil {
		t.Fatalf("read getdata: %v", err)
	}
	if msg.Command != wire.CommandGetData {
		t.Fatalf("command=%s want %s", msg.Command, wire.CommandGetData)
	}
	inv, err := wire.ReadInvPayload(bytes.NewReader(msg.Payload))
	if err != nil {
		t.Fatalf("parse getdata payload: %v", err)
	}
	if len(inv) != 1 || inv[0].Type != wire.InvTypeBlock || inv[0].Hash != parent {
		t.Fatalf("getdata inv=%+v want block %s", inv, parentHash)
	}

	select {
	case requested := <-done:
		if !requested {
			t.Fatal("duplicate orphan did not refresh missing-parent request after TTL")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("requestMissingParentForResult did not return")
	}
}

type legacyAliasTestHasher struct{}

func (legacyAliasTestHasher) HashHeader(header wire.BlockHeader) (chainhash.Hash, error) {
	var out chainhash.Hash
	out[0] = 0x01
	out[1] = byte(header.Nonce)
	out[2] = byte(header.Nonce >> 8)
	out[3] = byte(header.Nonce >> 16)
	out[4] = byte(header.Nonce >> 24)
	return out, nil
}

func TestServeInventoryAcceptsLegacyWireHash(t *testing.T) {
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := legacyAliasTestHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = canonical.String()
	chain, err := blockchain.New(params, legacyAliasTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	legacy, err := chain.LegacyHeaderHash(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	if legacy == canonical {
		t.Fatal("test setup expected legacy and canonical hashes to differ")
	}

	s := New(params, chain, nil, log.New(io.Discard, "", 0))
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{conn: serverConn, remote: "legacy-wire-peer", lastSeen: time.Now(), lastPong: time.Now()}

	done := make(chan error, 1)
	go func() {
		done <- s.serveInventory(p, []wire.InvVect{{Type: wire.InvTypeBlock, Hash: legacy}})
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msg, err := wire.ReadMessage(clientConn, params.MessageStart)
	if err != nil {
		t.Fatalf("read served block: %v", err)
	}
	if msg.Command != wire.CommandBlock {
		t.Fatalf("command=%s want %s", msg.Command, wire.CommandBlock)
	}
	served, err := wire.ReadBlock(bytes.NewReader(msg.Payload))
	if err != nil {
		t.Fatalf("parse served block: %v", err)
	}
	servedHash, err := chain.BlockHash(served)
	if err != nil {
		t.Fatal(err)
	}
	if servedHash != canonical {
		t.Fatalf("served hash=%s want %s", servedHash, canonical)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveInventory: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("serveInventory did not return")
	}
}

func TestClearGetdataForBlockClearsCanonicalAndLegacyHashes(t *testing.T) {
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := legacyAliasTestHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = canonical.String()
	chain, err := blockchain.New(params, legacyAliasTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	legacy, err := chain.LegacyHeaderHash(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	s := New(params, chain, nil, log.New(io.Discard, "", 0))
	s.recordGetdataReq(canonical.String(), "peer-a")
	s.recordGetdataReq(legacy.String(), "peer-a")

	s.clearGetdataForBlock(genesisBlock)

	s.getdataMu.Lock()
	remaining := len(s.getdataReqs)
	s.getdataMu.Unlock()
	if remaining != 0 {
		t.Fatalf("remaining getdata requests=%d want 0", remaining)
	}
}

func TestRequestUnknownBlocksTreatsKnownLegacyWireHashAsKnown(t *testing.T) {
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	canonical, err := legacyAliasTestHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = canonical.String()
	chain, err := blockchain.New(params, legacyAliasTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	legacy, err := chain.LegacyHeaderHash(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}

	s := New(params, chain, nil, log.New(io.Discard, "", 0))
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{conn: serverConn, remote: "legacy-inv-peer", lastSeen: time.Now(), lastPong: time.Now()}

	if err := s.requestUnknownBlocks(p, []wire.InvVect{{Type: wire.InvTypeBlock, Hash: legacy}}); err != nil {
		t.Fatalf("requestUnknownBlocks: %v", err)
	}
	s.getdataMu.Lock()
	remaining := len(s.getdataReqs)
	s.getdataMu.Unlock()
	if remaining != 0 {
		t.Fatalf("known legacy hash was tracked as unknown; requests=%d", remaining)
	}
	_ = clientConn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	if msg, err := wire.ReadMessage(clientConn, params.MessageStart); err == nil {
		t.Fatalf("unexpected message for known legacy INV: %s", msg.Command)
	}
}

func TestRequestBlockHashFromCandidatesSendsExactGetData(t *testing.T) {
	s, params, cleanup := newP2PTestServerWithGenesis(t)
	defer cleanup()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{
		conn:             serverConn,
		remote:           "127.0.0.1:19555",
		height:           3,
		lastSeen:         time.Now(),
		lastPong:         time.Now(),
		lastHeightUpdate: time.Now(),
	}
	s.registerPeer(p)

	hash := chainhash.DoubleHashB([]byte("timed out body"))
	done := make(chan int, 1)
	go func() {
		done <- s.requestBlockHashFromCandidates(hash.String(), 1)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	msg, err := wire.ReadMessage(clientConn, params.MessageStart)
	if err != nil {
		t.Fatalf("read getdata: %v", err)
	}
	if msg.Command != wire.CommandGetData {
		t.Fatalf("command=%s want %s", msg.Command, wire.CommandGetData)
	}
	inv, err := wire.ReadInvPayload(bytes.NewReader(msg.Payload))
	if err != nil {
		t.Fatalf("parse getdata payload: %v", err)
	}
	if len(inv) != 1 || inv[0].Type != wire.InvTypeBlock || inv[0].Hash != hash {
		t.Fatalf("getdata inv=%+v want block %s", inv, hash.String())
	}
	select {
	case requested := <-done:
		if requested != 1 {
			t.Fatalf("requested=%d want 1", requested)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("requestBlockHashFromCandidates did not return")
	}
}

func TestRequestSyncFromPeerIfBehindSendsHeadersAndBlocks(t *testing.T) {
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	genesisHash, err := p2pTestHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = genesisHash.String()

	chain, err := blockchain.New(params, p2pTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	s := New(params, chain, nil, log.New(io.Discard, "", 0))

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{conn: serverConn, remote: "pipe-test", height: 3, lastSeen: time.Now(), lastPong: time.Now()}

	done := make(chan error, 1)
	go func() {
		done <- s.requestSyncFromPeerIfBehind(p, false)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	first, err := wire.ReadMessage(clientConn, params.MessageStart)
	if err != nil {
		t.Fatalf("read first sync request: %v", err)
	}
	if first.Command != wire.CommandGetHeaders {
		t.Fatalf("first command=%s want %s", first.Command, wire.CommandGetHeaders)
	}
	second, err := wire.ReadMessage(clientConn, params.MessageStart)
	if err != nil {
		t.Fatalf("read second sync request: %v", err)
	}
	if second.Command != wire.CommandGetBlocks {
		t.Fatalf("second command=%s want %s", second.Command, wire.CommandGetBlocks)
	}

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("requestSyncFromPeerIfBehind: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("requestSyncFromPeerIfBehind did not return")
	}
}

func TestRequestSyncFromPeerIfBehindForceBypassesThrottle(t *testing.T) {
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	genesisHash, err := p2pTestHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = genesisHash.String()

	chain, err := blockchain.New(params, p2pTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	s := New(params, chain, nil, log.New(io.Discard, "", 0))

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{conn: serverConn, remote: "pipe-test", height: 3, lastSeen: time.Now(), lastPong: time.Now(), lastSyncRequest: time.Now()}

	done := make(chan error, 1)
	go func() {
		done <- s.requestSyncFromPeerIfBehind(p, true)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	first, err := wire.ReadMessage(clientConn, params.MessageStart)
	if err != nil {
		t.Fatalf("read forced sync request: %v", err)
	}
	if first.Command != wire.CommandGetHeaders {
		t.Fatalf("first command=%s want %s", first.Command, wire.CommandGetHeaders)
	}
	second, err := wire.ReadMessage(clientConn, params.MessageStart)
	if err != nil {
		t.Fatalf("read forced second sync request: %v", err)
	}
	if second.Command != wire.CommandGetBlocks {
		t.Fatalf("second command=%s want %s", second.Command, wire.CommandGetBlocks)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("forced requestSyncFromPeerIfBehind: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("forced requestSyncFromPeerIfBehind did not return")
	}
}

func TestSyncStatusIncludesLoopHealth(t *testing.T) {
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	genesisHash, err := p2pTestHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = genesisHash.String()

	chain, err := blockchain.New(params, p2pTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	s := New(params, chain, nil, log.New(io.Discard, "", 0))
	s.setP2PRunning(true)
	s.setSyncRunning(true)
	s.noteSyncBeat()
	s.noteSyncRequest()
	s.notePeerMessage()

	status := s.SyncStatus()
	health, ok := status["health"].(map[string]any)
	if !ok {
		t.Fatalf("health missing or wrong type: %#v", status["health"])
	}
	if health["p2p_loop_running"] != true {
		t.Fatalf("p2p_loop_running=%v want true", health["p2p_loop_running"])
	}
	if health["sync_loop_running"] != true {
		t.Fatalf("sync_loop_running=%v want true", health["sync_loop_running"])
	}
	if got := health["last_p2p_sync_request_ago_seconds"].(float64); got < 0 {
		t.Fatalf("last_p2p_sync_request_ago_seconds=%v", got)
	}
	if got := health["last_peer_message_ago_seconds"].(float64); got < 0 {
		t.Fatalf("last_peer_message_ago_seconds=%v", got)
	}
}

func TestPeerHeightMetadataUpdatesOnBestChainBlock(t *testing.T) {
	p := &peer{height: 411, lastHeightUpdate: time.Now().Add(-20 * time.Minute)}
	p.setLastBlockResult(blockchain.BlockProcessResult{
		Hash:          "new-tip",
		NewBestHeight: 419,
		BestChanged:   true,
		Connected:     true,
		Status:        blockchain.BlockStatusConnected,
	})

	p.lastMu.Lock()
	height := p.height
	age := time.Since(p.lastHeightUpdate)
	errText := p.lastSyncError
	p.lastMu.Unlock()

	if height != 419 {
		t.Fatalf("height=%d want 419", height)
	}
	if age > time.Second {
		t.Fatalf("lastHeightUpdate age=%s, want fresh", age)
	}
	if errText != "" {
		t.Fatalf("lastSyncError=%q want empty", errText)
	}
}

func TestPeerSetAdvertisedHeightRefreshesMetadataWithoutLoweringHeight(t *testing.T) {
	p := &peer{height: 411, lastHeightUpdate: time.Now().Add(-20 * time.Minute)}
	p.setAdvertisedHeight(430)
	p.lastMu.Lock()
	gotHeight := p.height
	age := time.Since(p.lastHeightUpdate)
	p.lastMu.Unlock()
	if gotHeight != 430 {
		t.Fatalf("height=%d want 430", gotHeight)
	}
	if age > time.Second {
		t.Fatalf("lastHeightUpdate age=%s want fresh", age)
	}

	p.setAdvertisedHeight(420)
	p.lastMu.Lock()
	gotHeight = p.height
	p.lastMu.Unlock()
	if gotHeight != 430 {
		t.Fatalf("height downgraded to %d; expected to keep 430", gotHeight)
	}
}

func TestSyncStatusReportsPeerAheadCatchUpPending(t *testing.T) {
	s, _, cleanup := newP2PTestServerWithGenesis(t)
	defer cleanup()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{
		conn:             serverConn,
		remote:           "127.0.0.1:19555",
		height:           12,
		lastSeen:         time.Now(),
		lastPong:         time.Now(),
		lastHeightUpdate: time.Now(),
	}
	s.registerPeer(p)

	status := s.SyncStatus()
	if status["sync_state"] != "requesting_blocks" {
		t.Fatalf("sync_state=%v want requesting_blocks", status["sync_state"])
	}
	if status["catch_up_pending"] != true {
		t.Fatalf("catch_up_pending=%v want true", status["catch_up_pending"])
	}
	if status["peer_reported_height"] != int32(12) {
		t.Fatalf("peer_reported_height=%v want 12", status["peer_reported_height"])
	}
	if status["local_best_hash"] == "" {
		t.Fatalf("local_best_hash missing")
	}
}

func TestSyncStatusClearsRequestInFlightWhenCurrent(t *testing.T) {
	s, _, cleanup := newP2PTestServerWithGenesis(t)
	defer cleanup()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{
		conn:             serverConn,
		remote:           "127.0.0.1:19555",
		height:           0,
		lastSeen:         time.Now(),
		lastPong:         time.Now(),
		lastHeightUpdate: time.Now(),
	}
	s.registerPeer(p)
	s.noteSyncRequest()

	status := s.SyncStatus()
	if status["sync_state"] != "current" {
		t.Fatalf("sync_state=%v want current", status["sync_state"])
	}
	if status["blocks_behind"] != int32(0) {
		t.Fatalf("blocks_behind=%v want 0", status["blocks_behind"])
	}
	if status["request_in_flight"] != false {
		t.Fatalf("request_in_flight=%v want false for current node", status["request_in_flight"])
	}
}

func TestRequestSyncFromPeerIfBehindDoesNotSpamRetry(t *testing.T) {
	s, params, cleanup := newP2PTestServerWithGenesis(t)
	defer cleanup()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{
		conn:             serverConn,
		remote:           "pipe-test",
		height:           3,
		lastSeen:         time.Now(),
		lastPong:         time.Now(),
		lastHeightUpdate: time.Now(),
		lastSyncRequest:  time.Now(),
	}

	done := make(chan error, 1)
	go func() {
		done <- s.requestSyncFromPeerIfBehind(p, false)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("requestSyncFromPeerIfBehind: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("requestSyncFromPeerIfBehind did not return")
	}

	_ = clientConn.SetReadDeadline(time.Now().Add(50 * time.Millisecond))
	if msg, err := wire.ReadMessage(clientConn, params.MessageStart); err == nil {
		t.Fatalf("unexpected sync retry message: %s", msg.Command)
	}
}

func TestSyncStatusCountsStalePeerMetadata(t *testing.T) {
	s, _, cleanup := newP2PTestServerWithGenesis(t)
	defer cleanup()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{
		conn:             serverConn,
		remote:           "127.0.0.1:19555",
		height:           2,
		lastSeen:         time.Now().Add(-getPeerStaleThreshold() - time.Minute),
		lastPong:         time.Now(),
		lastHeightUpdate: time.Now().Add(-getPeerStaleThreshold() - time.Minute),
	}
	s.registerPeer(p)

	status := s.SyncStatus()
	if status["stale_peer_count"] != 1 {
		t.Fatalf("stale_peer_count=%v want 1", status["stale_peer_count"])
	}
	if status["catch_up_pending"] != true {
		t.Fatalf("catch_up_pending=%v want true", status["catch_up_pending"])
	}
}

func TestDisconnectNodeAllowsReplacementPeerMetadata(t *testing.T) {
	s, _, cleanup := newP2PTestServerWithGenesis(t)
	defer cleanup()

	serverConn, clientConn := net.Pipe()
	p := &peer{
		conn:             serverConn,
		remote:           "127.0.0.1:19555",
		height:           2,
		lastSeen:         time.Now(),
		lastPong:         time.Now(),
		lastHeightUpdate: time.Now(),
	}
	s.registerPeer(p)
	if !s.DisconnectNode("127.0.0.1:19555") {
		t.Fatalf("DisconnectNode returned false")
	}
	_ = clientConn.Close()
	s.unregisterPeer(p)

	replacementServer, replacementClient := net.Pipe()
	defer replacementServer.Close()
	defer replacementClient.Close()
	replacement := &peer{
		conn:             replacementServer,
		remote:           "127.0.0.1:19555",
		height:           4,
		lastSeen:         time.Now(),
		lastPong:         time.Now(),
		lastHeightUpdate: time.Now(),
	}
	s.registerPeer(replacement)

	status := s.SyncStatus()
	if status["peer_count"] != 1 {
		t.Fatalf("peer_count=%v want 1", status["peer_count"])
	}
	if status["peer_reported_height"] != int32(4) {
		t.Fatalf("peer_reported_height=%v want 4", status["peer_reported_height"])
	}
}

func TestPeerMarkPingPongTracksMissedPongs(t *testing.T) {
	p := &peer{}
	p.markPing()
	p.markPing()

	p.lastMu.Lock()
	missed := p.missedPongs
	p.lastMu.Unlock()
	if missed != 2 {
		t.Fatalf("missedPongs=%d want 2", missed)
	}

	_ = p.markPong()
	p.lastMu.Lock()
	missed = p.missedPongs
	p.lastMu.Unlock()
	if missed != 0 {
		t.Fatalf("missedPongs=%d want 0 after pong", missed)
	}
}

func TestClassifyPeerSafetyKeepsSmallHeightLagCompatible(t *testing.T) {
	category, good, reason := classifyPeerSafety(3310, 3309, true, 0, "healthy", "", "", "", "", 75*time.Millisecond)
	if category != "lagging_1_block" || !good {
		t.Fatalf("1-block lag category=%q good=%t reason=%q", category, good, reason)
	}
	category, good, reason = classifyPeerSafety(3310, 3308, true, 0, "healthy", "", "", "", "", 75*time.Millisecond)
	if category != "lagging_2_blocks" || !good {
		t.Fatalf("2-block lag category=%q good=%t reason=%q", category, good, reason)
	}
	category, good, reason = classifyPeerSafety(3310, 3307, true, 0, "healthy", "", "", "", "", 75*time.Millisecond)
	if category != "stale_chain_data" || good {
		t.Fatalf("stale >2-block lag category=%q good=%t reason=%q", category, good, reason)
	}
}

func TestClassifyPeerSafetyKeepsSmallLagCompatibleWithOldMetadata(t *testing.T) {
	category, good, reason := classifyPeerSafety(3315, 3314, true, 0, "poor", "", "", "", "", 75*time.Millisecond)
	if category != "lagging_1_block" || !good {
		t.Fatalf("stale metadata 1-block lag category=%q good=%t reason=%q", category, good, reason)
	}
	category, good, reason = classifyPeerSafety(3315, 3313, true, 0, "poor", "", "", "", "", 75*time.Millisecond)
	if category != "lagging_2_blocks" || !good {
		t.Fatalf("stale metadata 2-block lag category=%q good=%t reason=%q", category, good, reason)
	}
}

func TestPeerInfosIncludePingAndSyncFields(t *testing.T) {
	s, _, cleanup := newP2PTestServerWithGenesis(t)
	defer cleanup()

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{
		conn:             serverConn,
		remote:           "127.0.0.1:19555",
		height:           5,
		lastSeen:         time.Now(),
		lastPong:         time.Now(),
		lastPing:         time.Now(),
		lastRTT:          25 * time.Millisecond,
		minRTT:           10 * time.Millisecond,
		missedPongs:      1,
		lastHeightUpdate: time.Now().Add(-getPeerStaleThreshold() - time.Second),
	}
	s.registerPeer(p)

	infos := s.PeerInfos()
	if len(infos) != 1 {
		t.Fatalf("len(infos)=%d want 1", len(infos))
	}
	info := infos[0]
	if info.LastPingTime == 0 || info.LastPongTime == 0 {
		t.Fatalf("expected non-zero ping/pong unix times: %+v", info)
	}
	if info.MissedPongs != 1 {
		t.Fatalf("missed_pongs=%d want 1", info.MissedPongs)
	}
	if info.PingLatencyMS <= 0 {
		t.Fatalf("ping_latency_ms=%f want > 0", info.PingLatencyMS)
	}
	if !info.Stale {
		t.Fatalf("expected stale=true when height metadata is old")
	}
	if info.ReportedHeight != 5 {
		t.Fatalf("reported_height=%d want 5", info.ReportedHeight)
	}
	if info.SyncState == "" {
		t.Fatalf("sync_state should not be empty")
	}
}

func TestRequestSyncForcedEqualHeightLogsRefreshNotBehind(t *testing.T) {
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	genesisHash, err := p2pTestHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = genesisHash.String()

	chain, err := blockchain.New(params, p2pTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	var logBuf bytes.Buffer
	s := New(params, chain, nil, log.New(&logBuf, "", 0))

	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{
		conn:             serverConn,
		remote:           "pipe-test",
		height:           0,
		lastSeen:         time.Now(),
		lastPong:         time.Now(),
		lastHeightUpdate: time.Now().Add(-5 * time.Minute),
	}
	done := make(chan error, 1)
	go func() {
		done <- s.requestSyncFromPeerIfBehind(p, true)
	}()

	_ = clientConn.SetReadDeadline(time.Now().Add(2 * time.Second))
	if _, err := wire.ReadMessage(clientConn, params.MessageStart); err != nil {
		t.Fatalf("read first sync message: %v", err)
	}
	if _, err := wire.ReadMessage(clientConn, params.MessageStart); err != nil {
		t.Fatalf("read second sync message: %v", err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("forced requestSyncFromPeerIfBehind: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("forced requestSyncFromPeerIfBehind did not return")
	}
	logText := logBuf.String()
	if !strings.Contains(logText, "sync metadata refresh") {
		t.Fatalf("expected refresh log, got: %s", logText)
	}
	if strings.Contains(logText, "sync behind peer") {
		t.Fatalf("unexpected behind log for equal-height forced refresh: %s", logText)
	}
}

func newP2PTestServerWithGenesis(t *testing.T) (*Server, chaincfg.Params, func()) {
	t.Helper()
	params := chaincfg.MainNet
	genesisBlock, err := genesis.NewBlock(params)
	if err != nil {
		t.Fatal(err)
	}
	genesisHash, err := p2pTestHasher{}.HashHeader(genesisBlock.Header)
	if err != nil {
		t.Fatal(err)
	}
	params.GenesisHash = genesisHash.String()

	chain, err := blockchain.New(params, p2pTestHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.EnsureGenesis(); err != nil {
		t.Fatal(err)
	}
	s := New(params, chain, nil, log.New(io.Discard, "", 0))
	return s, params, func() {
		for _, p := range s.snapshotPeers() {
			_ = p.conn.Close()
			s.unregisterPeer(p)
		}
	}
}

func TestAllowPeerMessageRateLimit(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))
	s.SetRuntimePolicy(64, 60, true, 15, 2, 8, 32, 100, 60, 900)
	p := &peer{remote: "127.0.0.1:19555"}
	if !s.allowPeerMessage(p, wire.CommandPing) {
		t.Fatalf("first message should pass rate limiter")
	}
	if !s.allowPeerMessage(p, wire.CommandPing) {
		t.Fatalf("second message should pass rate limiter")
	}
	if s.allowPeerMessage(p, wire.CommandPing) {
		t.Fatalf("third message should be rate-limited")
	}
}

func TestPeerBanExpiresAfterConfiguredDuration(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))
	s.SetPeerPolicy(chaincfg.MainNet.ChainID, false, true, 5, true, nil)
	s.SetRuntimePolicy(64, 1, true, 15, 1000, 8, 32, 10_000, 60, 900)
	serverConn, clientConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	p := &peer{conn: serverConn, remote: "127.0.0.1:20555"}
	s.scorePeer(p, 5, "test ban")
	if !s.isBanned("127.0.0.1:9999") {
		t.Fatalf("expected peer host to be temporarily banned")
	}
	time.Sleep(1200 * time.Millisecond)
	if s.isBanned("127.0.0.1:9999") {
		t.Fatalf("expected temporary ban to expire")
	}
}

func TestDuplicateInboundHostDetected(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	p := &peer{conn: serverConn, remote: "127.0.0.1:20001", lastSeen: time.Now(), lastPong: time.Now()}
	s.registerPeer(p)
	if !s.duplicateInboundHost("127.0.0.1") {
		t.Fatalf("expected duplicate inbound host detection")
	}
	if s.duplicateInboundHost("198.51.100.9") {
		t.Fatalf("unexpected duplicate detection for different host")
	}
}

func TestMisbehaviorScoreDecay(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))
	s.SetPeerPolicy(chaincfg.MainNet.ChainID, false, true, 100, true, nil)
	s.SetRuntimePolicy(64, 60, true, 15, 1000, 8, 32, 10_000, 1, 900)
	serverConn, clientConn := net.Pipe()
	defer serverConn.Close()
	defer clientConn.Close()
	p := &peer{conn: serverConn, remote: "127.0.0.1:20002"}
	s.scorePeer(p, 5, "initial")
	time.Sleep(1100 * time.Millisecond)
	s.scorePeer(p, 1, "second")
	p.lastMu.Lock()
	score := p.banScore
	p.lastMu.Unlock()
	if score >= 6 {
		t.Fatalf("expected score decay before second penalty, got %d", score)
	}
}

func TestAddNodeBoundedConcurrency(t *testing.T) {
	s := New(chaincfg.MainNet, nil, nil, log.New(io.Discard, "", 0))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

	g0 := runtime.NumGoroutine()
	for i := 0; i < 200; i++ {
		addr := fmt.Sprintf("10.0.0.%d:19555", i%254+1)
		_ = s.AddNode(ctx, addr)
	}
	cancel()
	time.Sleep(time.Second)
	g1 := runtime.NumGoroutine()
	// With 32 semaphore + cleanup goroutines, tolerance at 60
	if g1 > g0+60 {
		t.Fatalf("goroutine explosion: before=%d after=%d", g0, g1)
	}
}
