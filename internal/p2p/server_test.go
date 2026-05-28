package p2p

import (
	"bytes"
	"context"
	"io"
	"log"
	"net"
	"reflect"
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
	if status["sync_state"] != "syncing" {
		t.Fatalf("sync_state=%v want syncing", status["sync_state"])
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
		lastSeen:         time.Now().Add(-peerStaleThreshold - time.Minute),
		lastPong:         time.Now(),
		lastHeightUpdate: time.Now().Add(-peerStaleThreshold - time.Minute),
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
		lastHeightUpdate: time.Now().Add(-peerStaleThreshold - time.Second),
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
