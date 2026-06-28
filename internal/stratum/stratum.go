package stratum

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/mempool"
	"legacycoin/legacy-go/internal/mining"
	"legacycoin/legacy-go/internal/pow"
	"legacycoin/legacy-go/internal/wire"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/consensus"
)

type StratumRequest struct {
	ID     any             `json:"id"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params"`
}

type StratumResponse struct {
	ID     any    `json:"id"`
	Result any    `json:"result"`
	Error  any    `json:"error,omitempty"`
}

type StratumNotify struct {
	ID     any    `json:"id"`
	Method string `json:"method"`
	Params any    `json:"params"`
}

const (
	maxConnsPerIP      = 3
	maxConnsGlobal     = 100
	shareRateWindow    = 30 * time.Second
	shareRateLimit     = 10
	idleTimeout        = 5 * time.Minute
	submitTimeout      = 30 * time.Second
)

type Miner struct {
	conn         net.Conn
	enc          *json.Encoder
	worker       string
	difficulty   float64
	ip           string
	sharesInWin  int
	windowStart  time.Time
}

type miningJob struct {
	jobID     string
	block     *wire.MsgBlock
	height    int32
	created   time.Time
	cleanJobs bool
}

type Server struct {
	params      chaincfg.Params
	chain       *blockchain.Chain
	pool        *mempool.Pool
	listener    net.Listener
	mu          sync.Mutex
	miners      map[*Miner]struct{}
	ipCounts    map[string]int
	activeJob   *miningJob
	jobCounter  uint64
	shareDiff   float64
	pow         pow.YespowerHasher
	done        chan struct{}
	startedAt   time.Time
	sharesFound int64
	blocksFound int64
	Port        int
}

func New(params chaincfg.Params, chain *blockchain.Chain, pool *mempool.Pool) *Server {
	return &Server{
		params:    params,
		chain:     chain,
		pool:      pool,
		miners:    make(map[*Miner]struct{}),
		ipCounts:  make(map[string]int),
		shareDiff: 1.0,
		pow:       pow.YespowerHasher{Personalization: params.YespowerPers},
		done:      make(chan struct{}),
		startedAt: time.Now(),
	}
}

func (s *Server) Start(addr string) error {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("stratum listen %s: %w", addr, err)
	}
	s.listener = ln
	log.Printf("[Stratum] Listening on %s", addr)
	go s.acceptLoop()
	go s.jobBroadcaster()
	return nil
}

func (s *Server) Stop() {
	if s.listener != nil {
		s.listener.Close()
	}
	s.mu.Lock()
	for m := range s.miners {
		m.conn.Close()
	}
	s.miners = nil
	s.mu.Unlock()
	close(s.done)
}

func (s *Server) SetShareDiff(diff float64) {
	if diff > 0 {
		s.shareDiff = diff
	}
}

func (s *Server) Stats() map[string]any {
	return map[string]any{
		"clients": len(s.miners),
		"shares":  atomic.LoadInt64(&s.sharesFound),
		"blocks":  atomic.LoadInt64(&s.blocksFound),
		"uptime":  time.Since(s.startedAt).String(),
	}
}

func (s *Server) acceptLoop() {
	for {
		conn, err := s.listener.Accept()
		if err != nil {
			return
		}
		host, _, _ := net.SplitHostPort(conn.RemoteAddr().String())
		if host == "" {
			host = conn.RemoteAddr().String()
		}

		s.mu.Lock()
		if len(s.miners) >= maxConnsGlobal {
			s.mu.Unlock()
			conn.Close()
			log.Printf("[Stratum] rejected connection (global limit %d)", maxConnsGlobal)
			continue
		}
		if s.ipCounts[host] >= maxConnsPerIP {
			s.mu.Unlock()
			conn.Close()
			log.Printf("[Stratum] rejected connection from %s (per-IP limit %d)", host, maxConnsPerIP)
			continue
		}
		s.ipCounts[host]++
		s.mu.Unlock()

		m := &Miner{
			conn:        conn,
			enc:         json.NewEncoder(conn),
			difficulty:  s.shareDiff,
			ip:          host,
			windowStart: time.Now(),
		}
		s.mu.Lock()
		s.miners[m] = struct{}{}
		s.mu.Unlock()
		go s.handleMiner(m)
	}
}

func (s *Server) handleMiner(m *Miner) {
	defer func() {
		m.conn.Close()
		s.mu.Lock()
		delete(s.miners, m)
		if m.ip != "" {
			s.ipCounts[m.ip]--
			if s.ipCounts[m.ip] <= 0 {
				delete(s.ipCounts, m.ip)
			}
		}
		s.mu.Unlock()
	}()
	sc := bufio.NewScanner(m.conn)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for {
		m.conn.SetReadDeadline(time.Now().Add(idleTimeout))
		if !sc.Scan() {
			break
		}
		m.conn.SetReadDeadline(time.Time{})
		line := sc.Text()
		var req StratumRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue
		}
		switch req.Method {
		case "mining.subscribe":
			s.handleSubscribe(m, &req)
		case "mining.authorize":
			s.handleAuthorize(m, &req)
		case "mining.submit":
			s.handleSubmit(m, &req)
		default:
			s.sendError(m, req.ID, -1, "unknown method")
		}
	}
}

func (s *Server) handleSubscribe(m *Miner, req *StratumRequest) {
	notifyID := fmt.Sprintf("%x", time.Now().UnixNano())
	s.sendResult(m, req.ID, []any{
		[]any{"mining.notify", notifyID},
		"0001",
		8,
	})
	s.mu.Lock()
	job := s.activeJob
	s.mu.Unlock()
	if job != nil {
		s.notifyMiner(m, job)
	}
}

func (s *Server) handleAuthorize(m *Miner, req *StratumRequest) {
	var params []string
	if err := json.Unmarshal(req.Params, &params); err != nil || len(params) < 1 {
		s.sendError(m, req.ID, -1, "invalid params")
		return
	}
	m.worker = params[0]
	s.sendResult(m, req.ID, true)
}

func (s *Server) handleSubmit(m *Miner, req *StratumRequest) {
	var params []string
	if err := json.Unmarshal(req.Params, &params); err != nil || len(params) < 5 {
		s.sendError(m, req.ID, 24, "invalid params")
		return
	}
	workerName := params[0]
	jobID := params[1]
	extraNonce2 := params[2]
	ntimeStr := params[3]
	nonceStr := params[4]

	if len(nonceStr) > 8 || len(nonceStr) == 0 {
		s.sendError(m, req.ID, 24, "invalid nonce")
		return
	}
	nonceStr = strings.Repeat("0", 8-len(nonceStr)) + nonceStr

	if len(ntimeStr) != 8 {
		s.sendError(m, req.ID, 24, "invalid ntime")
		return
	}

	if len(extraNonce2) > 16 || len(extraNonce2) == 0 {
		s.sendError(m, req.ID, 24, "invalid extranonce2")
		return
	}

	// Share rate limiting
	now := time.Now()
	if now.Sub(m.windowStart) > shareRateWindow {
		m.sharesInWin = 0
		m.windowStart = now
	}
	m.sharesInWin++
	if m.sharesInWin > shareRateLimit {
		s.sendError(m, req.ID, -1, "rate limited")
		return
	}

	s.mu.Lock()
	job := s.activeJob
	s.mu.Unlock()

	if job == nil || job.jobID != jobID {
		s.sendError(m, req.ID, 23, "job not found")
		return
	}

	nonce := new(big.Int)
	nonce.SetString(nonceStr, 16)
	ntime := new(big.Int)
	ntime.SetString(ntimeStr, 16)

	header := job.block.Header
	header.Nonce = uint32(nonce.Uint64())
	header.Timestamp = uint32(ntime.Uint64())

	hash, err := s.pow.HashHeader(header)
	if err != nil {
		s.sendError(m, req.ID, 24, "hash error")
		return
	}

	var hashBig big.Int
	hashBig.SetBytes(hash[:])

	shareTarget := difficultyToTarget(s.shareDiff)
	blockTarget := consensus.CompactToBig(job.block.Header.Bits)

	if hashBig.Cmp(blockTarget) <= 0 {
		atomic.AddInt64(&s.blocksFound, 1)
		if err := s.chain.ProcessBlock(job.block); err != nil {
			s.sendError(m, req.ID, 24, fmt.Sprintf("block rejected: %v", err))
			return
		}
		s.sendResult(m, req.ID, true)
		log.Printf("[Stratum] BLOCK FOUND by %s: height %d hash %s", workerName, job.height, hash.String())
		s.refreshJob(true)
		return
	}

	if hashBig.Cmp(&shareTarget) <= 0 {
		atomic.AddInt64(&s.sharesFound, 1)
		s.sendResult(m, req.ID, true)
		return
	}

	s.sendError(m, req.ID, 24, "low difficulty share")
}

func (s *Server) notifyMiner(m *Miner, job *miningJob) {
	block := job.block
	root, err := block.BuildMerkleRoot()
	if err != nil {
		return
	}
	merkleHex := hex.EncodeToString(root[:])

	prevHashRev := reverseBytes32(job.block.Header.PrevBlock)
	prevHashHex := hex.EncodeToString(prevHashRev[:])

	notify := StratumNotify{
		Method: "mining.notify",
		Params: []any{
			job.jobID,
			prevHashHex,
			merkleHex,
			hex.EncodeToString(root[:]),
			stratumHexBE(uint32(job.block.Header.Version)),
			stratumHexBE(job.block.Header.Timestamp),
			stratumHexBE(job.block.Header.Bits),
			job.cleanJobs,
		},
	}
	m.enc.Encode(notify)
}

func (s *Server) broadcastJob(job *miningJob) {
	s.mu.Lock()
	for m := range s.miners {
		s.notifyMiner(m, job)
	}
	s.mu.Unlock()
}

func (s *Server) refreshJob(cleanJobs bool) {
	pubKeyHash := make([]byte, 20)
	pubKeyHash[0] = 0x6f
	pubKeyHash[19] = 0x01

	block, height, err := mining.NewBlockTemplate(s.chain, s.pool, pubKeyHash)
	if err != nil {
		log.Printf("[Stratum] template error: %v", err)
		return
	}

	job := &miningJob{
		jobID:     fmt.Sprintf("%d", atomic.AddUint64(&s.jobCounter, 1)),
		block:     block,
		height:    height,
		created:   time.Now(),
		cleanJobs: cleanJobs,
	}

	s.mu.Lock()
	s.activeJob = job
	s.mu.Unlock()
	s.broadcastJob(job)
}

func (s *Server) jobBroadcaster() {
	s.refreshJob(false)

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.done:
			return
		case <-ticker.C:
			tip := s.chain.Tip()
			best := int32(-1)
			if tip != nil {
				best = tip.Height
			}
			s.mu.Lock()
			height := int32(0)
			if s.activeJob != nil {
				height = s.activeJob.height
			}
			s.mu.Unlock()
			if best > height {
				s.refreshJob(true)
			}
		}
	}
}

func (s *Server) sendResult(m *Miner, id any, result any) {
	msg := StratumResponse{ID: id, Result: result}
	m.enc.Encode(msg)
}

func (s *Server) sendError(m *Miner, id any, code int, msg string) {
	resp := StratumResponse{ID: id, Result: nil, Error: []any{code, msg, nil}}
	m.enc.Encode(resp)
}

func difficultyToTarget(diff float64) big.Int {
	if diff <= 0 {
		diff = 1
	}
	maxTarget := new(big.Int).Lsh(big.NewInt(1), 256)
	maxTarget.Sub(maxTarget, big.NewInt(1))
	target := new(big.Int).Div(maxTarget, big.NewInt(int64(diff)))
	return *target
}

func reverseBytes32(h chainhash.Hash) chainhash.Hash {
	var rev chainhash.Hash
	for i := 0; i < 32; i++ {
		rev[31-i] = h[i]
	}
	return rev
}

func stratumHexBE(v uint32) string {
	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, v)
	return hex.EncodeToString(b)
}
