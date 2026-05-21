package p2p

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/mempool"
	"legacycoin/legacy-go/internal/wire"
)

const (
	protocolVersion  int32  = 70015
	nodeNetwork      uint64 = 1
	userAgent               = "/Legacy-GO:0.1.0/"
	maxPeers                = 125
	maxOutboundPeers        = 8
	maxGetDataItems         = 2048
	maxServeInvItems        = 2048
)

var (
	peerHandshakeTimeout = 2 * time.Minute
	peerIdleTimeout      = 2 * time.Minute
	peerPingInterval     = 30 * time.Second
	peerPongTimeout      = 90 * time.Second
	peerReconnectEvery   = 15 * time.Second
)

const (
	MaxPeers         = maxPeers
	MaxOutboundPeers = maxOutboundPeers
)

type peer struct {
	conn      net.Conn
	outbound  bool
	remote    string
	writeMu   sync.Mutex
	lastMu    sync.Mutex
	connected time.Time
	lastSeen  time.Time
	lastPong  time.Time
	lastPing  time.Time
	lastRTT   time.Duration
	minRTT    time.Duration
	version   int32
	subver    string
	height    int32
	chainID   string
	banScore  int
	bytesSent uint64
	bytesRecv uint64
}

type Server struct {
	params            chaincfg.Params
	chain             *blockchain.Chain
	pool              *mempool.Pool
	log               *log.Logger
	pretty            bool
	heartbeat         bool
	compactHeartbeat  bool
	showLatency       bool
	showPeerHeight    bool
	trustedPeerName   string
	heartbeatInterval time.Duration
	chainID           string
	enforceChainID    bool
	peerSafety        bool
	banThreshold      int
	connectOnly       map[string]struct{}
	seedPeers         bool

	listener      net.Listener
	peers         atomic.Int32
	outbound      atomic.Int32
	knownMu       sync.Mutex
	knownOutbound map[string]struct{}
	bootstrap     []string
	listenHost    string
	activeMu      sync.Mutex
	activePeers   map[*peer]struct{}
	bannedMu      sync.Mutex
	bannedUntil   map[string]time.Time
	seedMu        sync.Mutex
	seedFailures  map[string]int
	seedLastLog   map[string]time.Time
	rejectMu      sync.Mutex
	rejectCounts  map[string]int
	rejectLastLog map[string]time.Time
	wg            sync.WaitGroup
}

func New(params chaincfg.Params, chain *blockchain.Chain, pool *mempool.Pool, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	return &Server{
		params:        params,
		chain:         chain,
		pool:          pool,
		log:           logger,
		knownOutbound: make(map[string]struct{}),
		activePeers:   make(map[*peer]struct{}),
		bannedUntil:   make(map[string]time.Time),
		seedFailures:  make(map[string]int),
		seedLastLog:   make(map[string]time.Time),
		rejectCounts:  make(map[string]int),
		rejectLastLog: make(map[string]time.Time),
		chainID:       params.ChainID,
		peerSafety:    true,
		banThreshold:  100,
		seedPeers:     true,
	}
}

func (s *Server) ListenAddr() string {
	if s.listener == nil {
		return ""
	}
	return s.listener.Addr().String()
}

func (s *Server) PeerCount() int32 {
	return s.peers.Load()
}

func (s *Server) OutboundCount() int32 {
	return s.outbound.Load()
}

func (s *Server) SetPeerPolicy(chainID string, enforce bool, peerSafety bool, banThreshold int, seedPeers bool, connectOnly []string) {
	if strings.TrimSpace(chainID) != "" {
		s.chainID = strings.TrimSpace(chainID)
	}
	s.enforceChainID = enforce
	s.peerSafety = peerSafety
	if banThreshold > 0 {
		s.banThreshold = banThreshold
	}
	s.seedPeers = seedPeers
	if len(connectOnly) > 0 {
		s.connectOnly = make(map[string]struct{}, len(connectOnly))
		for _, addr := range connectOnly {
			addr = strings.TrimSpace(addr)
			if addr != "" {
				s.connectOnly[addr] = struct{}{}
			}
		}
	}
}

func (s *Server) SetPrettyLogging(enabled bool, heartbeat bool, compact bool, showLatency bool, showPeerHeight bool, trustedPeerName string, heartbeatSeconds int) {
	s.pretty = enabled
	s.heartbeat = heartbeat
	s.compactHeartbeat = compact
	s.showLatency = showLatency
	s.showPeerHeight = showPeerHeight
	s.trustedPeerName = trustedPeerName
	if heartbeatSeconds >= 10 {
		s.heartbeatInterval = time.Duration(heartbeatSeconds) * time.Second
	}
}

type PeerInfo struct {
	Addr                string  `json:"addr"`
	Direction           string  `json:"direction"`
	Outbound            bool    `json:"outbound"`
	ConnectedForSeconds int64   `json:"connected_for_seconds"`
	LastSeenAgoSeconds  float64 `json:"last_seen_ago_seconds"`
	LastPongAgoSeconds  float64 `json:"last_pong_ago_seconds"`
	LastPingMS          float64 `json:"last_ping_ms"`
	MinPingMS           float64 `json:"min_ping_ms"`
	Version             int32   `json:"version"`
	SubVer              string  `json:"subver"`
	SyncedBlocks        int32   `json:"synced_blocks"`
	StartingHeight      int32   `json:"starting_height"`
	ChainID             string  `json:"chain_id"`
	BanScore            int     `json:"ban_score"`
	BytesSent           uint64  `json:"bytes_sent"`
	BytesRecv           uint64  `json:"bytes_recv"`
	ConnectionType      string  `json:"connection_type"`
}

func (s *Server) PeerInfos() []PeerInfo {
	now := time.Now()
	peers := s.snapshotPeers()
	out := make([]PeerInfo, 0, len(peers))
	for _, p := range peers {
		p.lastMu.Lock()
		connected := p.connected
		seen := p.lastSeen
		pong := p.lastPong
		rtt := p.lastRTT
		minRTT := p.minRTT
		version := p.version
		subver := p.subver
		height := p.height
		chainID := p.chainID
		banScore := p.banScore
		bytesSent := p.bytesSent
		bytesRecv := p.bytesRecv
		p.lastMu.Unlock()
		direction := "inbound"
		if p.outbound {
			direction = "outbound"
		}
		out = append(out, PeerInfo{
			Addr:                p.remote,
			Direction:           direction,
			Outbound:            p.outbound,
			ConnectedForSeconds: int64(now.Sub(connected).Seconds()),
			LastSeenAgoSeconds:  now.Sub(seen).Seconds(),
			LastPongAgoSeconds:  now.Sub(pong).Seconds(),
			LastPingMS:          float64(rtt.Microseconds()) / 1000,
			MinPingMS:           float64(minRTT.Microseconds()) / 1000,
			Version:             version,
			SubVer:              subver,
			StartingHeight:      height,
			SyncedBlocks:        height,
			ChainID:             chainID,
			BanScore:            banScore,
			BytesSent:           bytesSent,
			BytesRecv:           bytesRecv,
			ConnectionType:      direction + "-full-relay",
		})
	}
	return out
}

func (s *Server) SetBootstrapPeers(peers []string) {
	s.knownMu.Lock()
	defer s.knownMu.Unlock()
	s.bootstrap = append([]string(nil), peers...)
}

func (s *Server) BootstrapPeers() []string {
	return s.bootstrapPeers()
}

func (s *Server) addBootstrapPeer(addr string) {
	s.knownMu.Lock()
	defer s.knownMu.Unlock()
	for _, existing := range s.bootstrap {
		if existing == addr {
			return
		}
	}
	s.bootstrap = append(s.bootstrap, addr)
}

func (s *Server) SetListenHost(host string) {
	s.knownMu.Lock()
	defer s.knownMu.Unlock()
	s.listenHost = host
}

func (s *Server) ListenHost() string {
	s.knownMu.Lock()
	defer s.knownMu.Unlock()
	return s.listenHost
}

func (s *Server) AddNode(ctx context.Context, addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = strconv.Itoa(int(s.params.DefaultPort))
	}
	if host == "" {
		return fmt.Errorf("empty peer host")
	}
	addr = net.JoinHostPort(host, port)
	if len(s.connectOnly) > 0 {
		if _, ok := s.connectOnly[addr]; !ok {
			return fmt.Errorf("peer %s is not allowed by connect-only policy", addr)
		}
	}
	s.addBootstrapPeer(addr)
	if s.outbound.Load() >= maxOutboundPeers || s.peers.Load() >= maxPeers {
		s.log.Printf("p2p addnode %s queued but peer capacity is full", addr)
		return nil
	}
	if !s.markOutbound(addr) {
		return nil
	}
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer s.unmarkOutbound(addr)
		dialer := net.Dialer{Timeout: 15 * time.Second}
		conn, err := dialer.DialContext(ctx, "tcp", addr)
		if err != nil {
			s.log.Printf("p2p dial %s: %v", addr, err)
			return
		}
		s.log.Printf("p2p dial %s connected", addr)
		s.handleConn(ctx, conn, true)
	}()
	return nil
}

func (s *Server) Start(ctx context.Context) error {
	addr := net.JoinHostPort(s.ListenHost(), strconv.Itoa(int(s.params.DefaultPort)))
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = ln
	s.log.Printf("Legacy Coin P2P listening on %s", ln.Addr())

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		<-ctx.Done()
		_ = ln.Close()
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.acceptLoop(ctx, ln)
	}()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		s.seedLoop(ctx)
	}()

	<-ctx.Done()
	_ = ln.Close()
	s.wg.Wait()
	return nil
}

func (s *Server) acceptLoop(ctx context.Context, ln net.Listener) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				return
			}
			s.log.Printf("p2p accept: %v", err)
			continue
		}
		if s.peers.Load() >= maxPeers {
			_ = conn.Close()
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.handleConn(ctx, conn, false)
		}()
	}
}

func (s *Server) seedLoop(ctx context.Context) {
	ticker := time.NewTicker(peerReconnectEvery)
	defer ticker.Stop()
	s.connectSeeds(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.connectSeeds(ctx)
		}
	}
}

func (s *Server) connectSeeds(ctx context.Context) {
	if s.outbound.Load() >= maxOutboundPeers || s.peers.Load() >= maxPeers {
		return
	}
	for _, peer := range s.bootstrapPeers() {
		if s.outbound.Load() >= maxOutboundPeers || s.peers.Load() >= maxPeers {
			return
		}
		if err := s.AddNode(ctx, peer); err != nil {
			s.log.Printf("p2p add bootstrap peer %s: %v", peer, err)
		}
	}
	if !s.seedPeers || len(s.connectOnly) > 0 {
		return
	}
	for _, seed := range s.params.DNSSeeds {
		hosts, err := net.DefaultResolver.LookupHost(ctx, seed)
		if err != nil {
			s.logSeedError(seed, err)
			continue
		}
		for _, host := range hosts {
			if s.outbound.Load() >= maxOutboundPeers || s.peers.Load() >= maxPeers {
				return
			}
			addr := net.JoinHostPort(host, strconv.Itoa(int(s.params.DefaultPort)))
			if err := s.AddNode(ctx, addr); err != nil {
				s.log.Printf("p2p add seed peer %s: %v", addr, err)
			}
		}
	}
}

func (s *Server) logSeedError(seed string, err error) {
	if !s.pretty {
		s.log.Printf("p2p seed %s: %v", seed, err)
		return
	}
	now := time.Now()
	s.seedMu.Lock()
	s.seedFailures[seed]++
	count := s.seedFailures[seed]
	last := s.seedLastLog[seed]
	if last.IsZero() || now.Sub(last) >= 5*time.Minute || count == 1 {
		s.seedLastLog[seed] = now
		s.seedMu.Unlock()
		if count == 1 {
			s.log.Printf("🌱 DNS seed unavailable | %s | normal if seeds are offline/private test", seed)
		} else {
			s.log.Printf("🌱 DNS seed warning repeated | %s | repeats %d | suppressing noise", seed, count)
		}
		return
	}
	s.seedMu.Unlock()
}

func (s *Server) logConnectOnlyReject(addr string) {
	if !s.pretty {
		s.log.Printf("p2p rejected inbound peer %s by connect-only developer policy", addr)
		return
	}
	host, _, err := net.SplitHostPort(addr)
	key := addr
	if err == nil && host != "" {
		key = host
	}
	now := time.Now()
	s.rejectMu.Lock()
	s.rejectCounts[key]++
	count := s.rejectCounts[key]
	last := s.rejectLastLog[key]
	// Show first rejection from an address, then summarize at most every 5 minutes.
	if count == 1 || last.IsZero() || now.Sub(last) >= 5*time.Minute {
		s.rejectLastLog[key] = now
		s.rejectMu.Unlock()
		if count == 1 {
			s.log.Printf("🛡️ Connect-only active | rejected inbound peer %s | allowed peers only", addr)
		} else {
			s.log.Printf("🛡️ Connect-only summary | rejected inbound peer %s | repeats %d | suppressing repeats for 5m", key, count)
		}
		return
	}
	s.rejectMu.Unlock()
}

func (s *Server) bootstrapPeers() []string {
	s.knownMu.Lock()
	defer s.knownMu.Unlock()
	return append([]string(nil), s.bootstrap...)
}

func (s *Server) markOutbound(addr string) bool {
	s.knownMu.Lock()
	defer s.knownMu.Unlock()
	if _, ok := s.knownOutbound[addr]; ok {
		return false
	}
	s.knownOutbound[addr] = struct{}{}
	return true
}

func (s *Server) unmarkOutbound(addr string) {
	s.knownMu.Lock()
	defer s.knownMu.Unlock()
	delete(s.knownOutbound, addr)
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn, outbound bool) {
	now := time.Now()
	p := &peer{conn: conn, outbound: outbound, remote: conn.RemoteAddr().String(), connected: now, lastSeen: now, lastPong: now}
	defer conn.Close()
	if s.isBanned(conn.RemoteAddr().String()) {
		s.log.Printf("p2p rejected banned peer %s", conn.RemoteAddr())
		return
	}
	if len(s.connectOnly) > 0 && !outbound {
		if _, ok := s.connectOnly[conn.RemoteAddr().String()]; !ok {
			s.logConnectOnlyReject(conn.RemoteAddr().String())
			return
		}
	}
	s.registerPeer(p)
	defer s.unregisterPeer(p)
	pingDone := make(chan struct{})
	go s.pingLoop(ctx, p, pingDone)
	defer close(pingDone)
	s.peers.Add(1)
	defer s.peers.Add(-1)
	if outbound {
		s.outbound.Add(1)
		defer s.outbound.Add(-1)
	}

	_ = conn.SetReadDeadline(time.Now().Add(peerHandshakeTimeout))
	if err := s.writeVersion(p, conn.RemoteAddr()); err != nil {
		s.log.Printf("p2p write version to %s: %v", conn.RemoteAddr(), err)
		return
	}

	gotVersion := false
	gotVerAck := false
	didSyncRequest := false
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		msg, err := wire.ReadMessage(conn, s.params.MessageStart)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				s.log.Printf("p2p read from %s: %v", conn.RemoteAddr(), err)
			}
			return
		}
		p.addBytesRecv(uint64(len(msg.Payload) + 24))
		p.markSeen()
		if gotVersion && gotVerAck {
			_ = conn.SetReadDeadline(time.Now().Add(peerIdleTimeout))
		}
		if !gotVersion || !gotVerAck {
			switch msg.Command {
			case wire.CommandVersion, wire.CommandVerAck, wire.CommandPing, wire.CommandPong:
				// Allowed during handshake.
			default:
				s.log.Printf("p2p protocol violation from %s: %s before handshake complete", conn.RemoteAddr(), msg.Command)
				return
			}
		}
		switch msg.Command {
		case wire.CommandVersion:
			gotVersion = true
			meta, err := s.parseVersionPayload(msg.Payload)
			if err != nil {
				s.scorePeer(p, 20, "bad version payload")
				s.log.Printf("p2p bad version from %s: %v", conn.RemoteAddr(), err)
				return
			}
			p.setVersionMeta(meta)
			if s.enforceChainID && s.chainID != "" && meta.ChainID != s.chainID {
				s.scorePeer(p, 100, "wrong or empty chain id")
				if s.pretty {
					s.log.Printf("🚫 Peer rejected | %s | chain_id=%q expected=%q", conn.RemoteAddr(), meta.ChainID, s.chainID)
				} else {
					s.log.Printf("p2p reject wrong-chain peer %s chain_id=%q expected=%q", conn.RemoteAddr(), meta.ChainID, s.chainID)
				}
				return
			}
			if err := s.writePeerMessage(p, wire.CommandVerAck, nil); err != nil {
				s.log.Printf("p2p write verack to %s: %v", conn.RemoteAddr(), err)
				return
			}
		case wire.CommandVerAck:
			gotVerAck = true
		case wire.CommandPing:
			if err := s.writePeerMessage(p, wire.CommandPong, msg.Payload); err != nil {
				s.log.Printf("p2p write pong to %s: %v", conn.RemoteAddr(), err)
				return
			}
		case wire.CommandPong:
			rtt := p.markPong()
			if s.pretty && s.heartbeat {
				height := int32(-1)
				if tip := s.chain.Tip(); tip != nil {
					height = tip.Height
				}
				name := s.peerLabel(p)
				if s.compactHeartbeat {
					s.log.Printf("🏓 %s pong %.1fms | peers %d | height %d | storage ✅", name, float64(rtt.Microseconds())/1000, s.PeerCount(), height)
				} else {
					s.log.Printf("🟢 PONG ← %s | latency %.1fms | height %d | connection stable", name, float64(rtt.Microseconds())/1000, height)
				}
			}
		case wire.CommandBlock:
			block, err := wire.ReadBlock(bytes.NewReader(msg.Payload))
			if err != nil {
				s.log.Printf("p2p parse block from %s: %v", conn.RemoteAddr(), err)
				return
			}
			if err := s.chain.ProcessBlock(block); err != nil {
				s.log.Printf("p2p reject block from %s: %v", conn.RemoteAddr(), err)
				return
			}
			if s.pretty {
				height := int32(-1)
				if tip := s.chain.Tip(); tip != nil {
					height = tip.Height
				}
				s.log.Printf("📦 Block accepted | height %d | txs %d | from %s | storage ✅", height, len(block.Transactions), s.peerLabel(p))
			} else {
				s.log.Printf("p2p accepted block from %s", conn.RemoteAddr())
			}
			if s.pool != nil {
				s.pool.RemoveForBlock(block)
			}
			hash, err := s.chain.BlockHash(block)
			if err == nil {
				s.log.Printf("p2p connected block %s from %s", hash.String(), conn.RemoteAddr())
				// Relay the accepted block using the canonical Yespower block hash.
				// Do not announce back to the peer that supplied the block.
				s.announceBlockToPeersExcept(hash, p)
				_ = s.requestHeaders(p)
			}
		case wire.CommandTx:
			if s.pool == nil {
				continue
			}
			tx, err := wire.ReadTx(bytes.NewReader(msg.Payload))
			if err != nil {
				s.log.Printf("p2p parse tx from %s: %v", conn.RemoteAddr(), err)
				return
			}
			entry, err := s.pool.Add(s.chain, tx)
			if err != nil {
				if errors.Is(err, mempool.ErrOrphanTx) {
					continue
				}
				s.log.Printf("p2p reject tx from %s: %v", conn.RemoteAddr(), err)
				continue
			}
			// Relay accepted transactions onward so wallet-created transactions can
			// propagate beyond the first peer and receivers can see pending funds.
			if s.pretty {
				s.log.Printf("💸 TX accepted to mempool | %s | from %s", entry.TxID, s.peerLabel(p))
			}
			if h, err := chainhash.FromString(entry.TxID); err == nil {
				s.announceTxToPeersExcept(h, p)
				if s.pretty {
					s.log.Printf("📣 TX relayed | %s | peers %d", entry.TxID, s.PeerCount())
				}
			}
		case wire.CommandInv:
			inv, err := wire.ReadInvPayload(bytes.NewReader(msg.Payload))
			if err != nil {
				s.log.Printf("p2p parse inv from %s: %v", conn.RemoteAddr(), err)
				return
			}
			if err := s.requestUnknownBlocks(p, inv); err != nil {
				s.log.Printf("p2p request blocks from %s: %v", conn.RemoteAddr(), err)
				return
			}
		case wire.CommandGetData:
			inv, err := wire.ReadInvPayload(bytes.NewReader(msg.Payload))
			if err != nil {
				s.log.Printf("p2p parse getdata from %s: %v", conn.RemoteAddr(), err)
				return
			}
			if err := s.serveInventory(p, inv); err != nil {
				s.log.Printf("p2p serve getdata to %s: %v", conn.RemoteAddr(), err)
				return
			}
		case wire.CommandGetBlocks:
			req, err := wire.ReadGetBlocks(bytes.NewReader(msg.Payload))
			if err != nil {
				s.log.Printf("p2p parse getblocks from %s: %v", conn.RemoteAddr(), err)
				return
			}
			if err := s.serveBlockInventory(p, req); err != nil {
				s.log.Printf("p2p serve getblocks to %s: %v", conn.RemoteAddr(), err)
				return
			}
		case wire.CommandGetHeaders:
			req, err := wire.ReadGetBlocks(bytes.NewReader(msg.Payload))
			if err != nil {
				s.log.Printf("p2p parse getheaders from %s: %v", conn.RemoteAddr(), err)
				return
			}
			if err := s.serveHeaders(p, req); err != nil {
				s.log.Printf("p2p serve headers to %s: %v", conn.RemoteAddr(), err)
				return
			}
		case wire.CommandHeaders:
			headers, err := wire.ReadHeadersPayload(bytes.NewReader(msg.Payload))
			if err != nil {
				s.log.Printf("p2p parse headers from %s: %v", conn.RemoteAddr(), err)
				return
			}
			if err := s.requestHeaderBlocks(p, headers); err != nil {
				// A peer that is behind us can legitimately send a stale header
				// batch when we also asked it for headers. Do not disconnect for
				// non-connecting header batches; keep the peer available so its
				// own sync request can fetch our blocks.
				s.log.Printf("p2p ignore non-syncable headers from %s: %v", conn.RemoteAddr(), err)
			}
		}
		if gotVersion && gotVerAck && !didSyncRequest {
			didSyncRequest = true
			if s.pretty {
				s.log.Printf("🌐 Connected peer | %s | outbound=%v | height %d | chain_id=%s", s.peerLabel(p), outbound, p.height, p.chainID)
			} else {
				s.log.Printf("p2p handshake complete with %s outbound=%v", conn.RemoteAddr(), outbound)
			}
			_ = conn.SetReadDeadline(time.Now().Add(peerIdleTimeout))
			if err := s.requestHeaders(p); err != nil {
				s.log.Printf("p2p request headers from %s: %v", conn.RemoteAddr(), err)
			}
			if err := s.requestBlocks(p); err != nil {
				s.log.Printf("p2p request blocks from %s: %v", conn.RemoteAddr(), err)
			}
		}
	}
}

func (p *peer) markSeen() {
	p.lastMu.Lock()
	p.lastSeen = time.Now()
	p.lastMu.Unlock()
}

func (p *peer) markPong() time.Duration {
	p.lastMu.Lock()
	now := time.Now()
	p.lastSeen = now
	p.lastPong = now
	if !p.lastPing.IsZero() {
		p.lastRTT = now.Sub(p.lastPing)
		if p.minRTT == 0 || p.lastRTT < p.minRTT {
			p.minRTT = p.lastRTT
		}
	}
	rtt := p.lastRTT
	p.lastMu.Unlock()
	return rtt
}

func (p *peer) markPing() {
	p.lastMu.Lock()
	p.lastPing = time.Now()
	p.lastMu.Unlock()
}

func (p *peer) lastActivity() (seen time.Time, pong time.Time) {
	p.lastMu.Lock()
	defer p.lastMu.Unlock()
	return p.lastSeen, p.lastPong
}

func (s *Server) peerLabel(p *peer) string {
	if s.trustedPeerName != "" {
		return s.trustedPeerName
	}
	return p.remote
}

func (s *Server) pingLoop(ctx context.Context, p *peer, done <-chan struct{}) {
	interval := peerPingInterval
	if s.heartbeatInterval > 0 {
		interval = s.heartbeatInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-done:
			return
		case <-ticker.C:
			seen, pong := p.lastActivity()
			if time.Since(seen) > peerPongTimeout && time.Since(pong) > peerPongTimeout {
				s.log.Printf("p2p peer %s liveness timeout; closing connection", p.remote)
				_ = p.conn.Close()
				return
			}
			nonce := make([]byte, 8)
			if _, err := rand.Read(nonce); err != nil {
				binary.LittleEndian.PutUint64(nonce, uint64(time.Now().UnixNano()))
			}
			p.markPing()
			if s.pretty && s.heartbeat && !s.compactHeartbeat {
				s.log.Printf("🏓 PING → %s", s.peerLabel(p))
			}
			if err := s.writePeerMessage(p, wire.CommandPing, nonce); err != nil {
				s.log.Printf("p2p ping %s failed: %v", p.remote, err)
				_ = p.conn.Close()
				return
			}
		}
	}
}

func (s *Server) registerPeer(p *peer) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	if s.activePeers == nil {
		s.activePeers = make(map[*peer]struct{})
	}
	s.activePeers[p] = struct{}{}
}

func (s *Server) unregisterPeer(p *peer) {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	delete(s.activePeers, p)
}

func (s *Server) snapshotPeers() []*peer {
	s.activeMu.Lock()
	defer s.activeMu.Unlock()
	out := make([]*peer, 0, len(s.activePeers))
	for p := range s.activePeers {
		out = append(out, p)
	}
	return out
}

func (s *Server) writePeerMessage(p *peer, command string, payload []byte) error {
	p.writeMu.Lock()
	defer p.writeMu.Unlock()
	err := wire.WriteMessage(p.conn, s.params.MessageStart, command, payload)
	if err == nil {
		p.addBytesSent(uint64(len(payload) + 24))
	}
	return err
}

func (s *Server) writeVersion(p *peer, remote net.Addr) error {
	payload, err := s.versionPayload(remote)
	if err != nil {
		return err
	}
	return s.writePeerMessage(p, wire.CommandVersion, payload)
}

func (s *Server) versionPayload(remote net.Addr) ([]byte, error) {
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, protocolVersion); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, nodeNetwork); err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, time.Now().Unix()); err != nil {
		return nil, err
	}
	writeNetAddr(&buf, remote, s.params.DefaultPort)
	writeNetAddr(&buf, nil, s.params.DefaultPort)
	nonce, err := randomUint64()
	if err != nil {
		return nil, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, nonce); err != nil {
		return nil, err
	}
	if err := wire.WriteVarBytes(&buf, []byte(userAgent)); err != nil {
		return nil, err
	}
	height := int32(-1)
	if tip := s.chain.Tip(); tip != nil {
		height = tip.Height
	}
	if err := binary.Write(&buf, binary.LittleEndian, height); err != nil {
		return nil, err
	}
	if err := buf.WriteByte(0); err != nil {
		return nil, err
	}
	if err := wire.WriteVarBytes(&buf, []byte(s.chainID)); err != nil {
		return nil, err
	}
	if _, err := buf.Write(s.params.MessageStart[:]); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type versionMeta struct {
	Version int32
	SubVer  string
	Height  int32
	ChainID string
}

func (s *Server) parseVersionPayload(payload []byte) (versionMeta, error) {
	var meta versionMeta
	r := bytes.NewReader(payload)
	if err := binary.Read(r, binary.LittleEndian, &meta.Version); err != nil {
		return meta, err
	}
	// services, timestamp, addr_recv, addr_from, nonce
	if r.Len() < 8+8+26+26+8 {
		return meta, fmt.Errorf("short version payload")
	}
	if _, err := r.Seek(8+8+26+26+8, io.SeekCurrent); err != nil {
		return meta, err
	}
	ua, err := wire.ReadVarBytes(r, 256, "user agent")
	if err != nil {
		return meta, err
	}
	meta.SubVer = string(ua)
	if err := binary.Read(r, binary.LittleEndian, &meta.Height); err != nil {
		return meta, err
	}
	// relay byte is optional in older payloads.
	if r.Len() > 0 {
		_, _ = r.ReadByte()
	}
	// V5.12+ metadata: varbytes chain_id + 4-byte message start. Older peers may omit it.
	if r.Len() > 0 {
		chainID, err := wire.ReadVarBytes(r, 256, "chain id")
		if err == nil {
			meta.ChainID = string(chainID)
		}
	}
	return meta, nil
}

func (p *peer) setVersionMeta(meta versionMeta) {
	p.lastMu.Lock()
	defer p.lastMu.Unlock()
	p.version = meta.Version
	p.subver = meta.SubVer
	p.height = meta.Height
	p.chainID = meta.ChainID
}

func (p *peer) addBytesSent(n uint64) {
	p.lastMu.Lock()
	p.bytesSent += n
	p.lastMu.Unlock()
}

func (p *peer) addBytesRecv(n uint64) {
	p.lastMu.Lock()
	p.bytesRecv += n
	p.lastMu.Unlock()
}

func (s *Server) scorePeer(p *peer, score int, reason string) {
	if !s.peerSafety || p == nil || score <= 0 {
		return
	}
	p.lastMu.Lock()
	p.banScore += score
	total := p.banScore
	p.lastMu.Unlock()
	if s.banThreshold > 0 && total >= s.banThreshold {
		s.banPeer(p.remote, time.Hour, reason)
		_ = p.conn.Close()
	}
}

func (s *Server) banPeer(addr string, d time.Duration, reason string) {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	s.bannedMu.Lock()
	if s.bannedUntil == nil {
		s.bannedUntil = make(map[string]time.Time)
	}
	s.bannedUntil[host] = time.Now().Add(d)
	s.bannedMu.Unlock()
	s.log.Printf("p2p temporarily banned peer %s for %s: %s", host, d, reason)
}

func (s *Server) isBanned(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	s.bannedMu.Lock()
	defer s.bannedMu.Unlock()
	until, ok := s.bannedUntil[host]
	if !ok {
		return false
	}
	if time.Now().After(until) {
		delete(s.bannedUntil, host)
		return false
	}
	return true
}

func (s *Server) DisconnectNode(addr string) bool {
	for _, p := range s.snapshotPeers() {
		if p.remote == addr || strings.HasPrefix(p.remote, addr) {
			_ = p.conn.Close()
			return true
		}
	}
	return false
}

func (s *Server) ConnectionCount() int32 { return s.PeerCount() }

func writeNetAddr(w io.Writer, addr net.Addr, defaultPort uint16) {
	_ = binary.Write(w, binary.LittleEndian, nodeNetwork)
	var ip [16]byte
	port := defaultPort
	if tcp, ok := addr.(*net.TCPAddr); ok {
		if parsed := tcp.IP.To16(); parsed != nil {
			copy(ip[:], parsed)
		}
		if tcp.Port > 0 {
			port = uint16(tcp.Port)
		}
	}
	_, _ = w.Write(ip[:])
	_ = binary.Write(w, binary.BigEndian, port)
}

func randomUint64() (uint64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, fmt.Errorf("random nonce: %w", err)
	}
	return binary.LittleEndian.Uint64(b[:]), nil
}

func (s *Server) requestHeaders(p *peer) error {
	locator := s.chain.Locator()
	if len(locator) == 0 {
		return nil
	}
	payload, err := (wire.GetBlocks{Version: protocolVersion, Locator: locator}).Bytes()
	if err != nil {
		return err
	}
	s.log.Printf("p2p send getheaders to %s locator_tip=%s", p.remote, locator[0].String())
	return s.writePeerMessage(p, wire.CommandGetHeaders, payload)
}

func (s *Server) requestBlocks(p *peer) error {
	locator := s.chain.Locator()
	if len(locator) == 0 {
		return nil
	}
	payload, err := (wire.GetBlocks{Version: protocolVersion, Locator: locator}).Bytes()
	if err != nil {
		return err
	}
	s.log.Printf("p2p send getblocks to %s locator_tip=%s", p.remote, locator[0].String())
	return s.writePeerMessage(p, wire.CommandGetBlocks, payload)
}

func (s *Server) requestUnknownBlocks(p *peer, inv []wire.InvVect) error {
	want := make([]wire.InvVect, 0, len(inv))
	requestedHeaders := false
	for _, v := range inv {
		switch v.Type {
		case wire.InvTypeBlock:
			if s.chain.HasBlock(v.Hash.String()) {
				s.log.Printf("p2p received inv block %s from %s: already known", v.Hash.String(), p.remote)
				continue
			}
			s.log.Printf("p2p received inv block %s from %s: unknown, requesting getdata", v.Hash.String(), p.remote)
			// Ask for headers once per inv batch so a node that is behind can
			// recover any missing ancestors, but do not wait for headers before
			// requesting the announced block body. The common case is a peer
			// announcing the next block after our tip; waiting for a header round
			// trip can leave followers stuck even though they received the INV.
			if !requestedHeaders {
				requestedHeaders = true
				if err := s.requestHeaders(p); err != nil {
					s.log.Printf("p2p request headers for inv %s: %v", v.Hash.String(), err)
				}
			}
			want = append(want, v)
		case wire.InvTypeTx:
			if s.pool == nil {
				continue
			}
			if _, ok := s.pool.Lookup(v.Hash.String()); ok {
				continue
			}
			s.log.Printf("p2p received inv tx %s from %s: unknown, requesting getdata", v.Hash.String(), p.remote)
			want = append(want, v)
		}
		if len(want) >= maxGetDataItems {
			break
		}
	}
	if len(want) == 0 {
		return nil
	}
	payload, err := wire.InvPayload(want)
	if err != nil {
		return err
	}
	s.log.Printf("p2p sent getdata for %d inventory items to %s", len(want), p.remote)
	return s.writePeerMessage(p, wire.CommandGetData, payload)
}

func (s *Server) serveInventory(p *peer, inv []wire.InvVect) error {
	inv = limitInv(inv, maxServeInvItems)
	for _, v := range inv {
		switch v.Type {
		case wire.InvTypeBlock:
			s.log.Printf("p2p received getdata block %s from %s", v.Hash.String(), p.remote)
			block, _, err := s.chain.BlockByHash(v.Hash.String())
			if err != nil {
				s.log.Printf("p2p getdata block %s from %s: not found", v.Hash.String(), p.remote)
				continue
			}
			payload, err := block.Bytes()
			if err != nil {
				return err
			}
			if err := s.writePeerMessage(p, wire.CommandBlock, payload); err != nil {
				return err
			}
			s.log.Printf("p2p sent block %s to %s", v.Hash.String(), p.remote)
		case wire.InvTypeTx:
			if s.pool == nil {
				continue
			}
			tx, ok := s.pool.Lookup(v.Hash.String())
			if !ok {
				continue
			}
			payload, err := tx.Bytes()
			if err != nil {
				return err
			}
			if err := s.writePeerMessage(p, wire.CommandTx, payload); err != nil {
				return err
			}
		}
	}
	return nil
}

func limitInv(inv []wire.InvVect, max int) []wire.InvVect {
	if max <= 0 {
		return nil
	}
	if len(inv) <= max {
		return inv
	}
	return inv[:max]
}

func (s *Server) announceTip(p *peer) error {
	tip := s.chain.Tip()
	if tip == nil || tip.Hash == "" {
		return nil
	}
	blockHash, err := chainhash.FromString(tip.Hash)
	if err != nil {
		return err
	}
	payload, err := wire.InvPayload([]wire.InvVect{{Type: wire.InvTypeBlock, Hash: blockHash}})
	if err != nil {
		return err
	}
	return s.writePeerMessage(p, wire.CommandInv, payload)
}

func (s *Server) serveBlockInventory(p *peer, req wire.GetBlocks) error {
	headers, err := s.chain.HeadersAfter(req.Locator, req.Stop, maxServeInvItems)
	if err != nil {
		return err
	}
	if len(headers) == 0 {
		s.log.Printf("p2p no block inventory after locator for %s", p.remote)
		return nil
	}
	inv := make([]wire.InvVect, 0, len(headers))
	for _, header := range headers {
		hash, err := s.chain.HashHeader(header)
		if err != nil {
			return err
		}
		inv = append(inv, wire.InvVect{Type: wire.InvTypeBlock, Hash: hash})
	}
	payload, err := wire.InvPayload(inv)
	if err != nil {
		return err
	}
	s.log.Printf("p2p serve %d block inv items to %s", len(inv), p.remote)
	return s.writePeerMessage(p, wire.CommandInv, payload)
}

func (s *Server) serveHeaders(p *peer, req wire.GetBlocks) error {
	headers, err := s.chain.HeadersAfter(req.Locator, req.Stop, wire.MaxHeadersPerMessage)
	if err != nil {
		return err
	}
	payload, err := wire.HeadersPayload(headers)
	if err != nil {
		return err
	}
	s.log.Printf("p2p serve %d headers to %s", len(headers), p.remote)
	return s.writePeerMessage(p, wire.CommandHeaders, payload)
}

func (s *Server) requestHeaderBlocks(p *peer, headers []wire.BlockHeader) error {
	if len(headers) == 0 {
		return nil
	}
	hashes, err := s.chain.ValidateHeaderSequence(headers)
	if err != nil {
		return err
	}
	want := make([]wire.InvVect, 0, len(hashes))
	for _, hash := range hashes {
		if s.chain.HasBlock(hash.String()) {
			continue
		}
		want = append(want, wire.InvVect{Type: wire.InvTypeBlock, Hash: hash})
		if len(want) >= maxGetDataItems {
			break
		}
	}
	if len(want) == 0 {
		return nil
	}
	payload, err := wire.InvPayload(want)
	if err != nil {
		return err
	}
	s.log.Printf("p2p request %d inventory items from %s", len(want), p.remote)
	return s.writePeerMessage(p, wire.CommandGetData, payload)
}
