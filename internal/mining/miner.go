package mining

import (
	"context"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/consensus"
	"legacycoin/legacy-go/internal/mempool"
	"legacycoin/legacy-go/internal/pow"
	"legacycoin/legacy-go/internal/script"
	"legacycoin/legacy-go/internal/wire"
)

type Progress struct {
	Attempts uint64
	Nonce    uint32
	Rate     float64
}

type Result struct {
	Block  *wire.MsgBlock
	Hash   chainhash.Hash
	Height int32
}

func NewBlockTemplate(chain *blockchain.Chain, pool *mempool.Pool, pubKeyHash []byte) (*wire.MsgBlock, int32, error) {
	if len(pubKeyHash) != 20 {
		return nil, 0, fmt.Errorf("mining address hash must be 20 bytes")
	}
	tip := chain.Tip()
	if tip == nil || tip.Hash == "" {
		return nil, 0, fmt.Errorf("chain tip is not initialized")
	}
	prev, err := chainhash.FromString(tip.Hash)
	if err != nil {
		return nil, 0, err
	}
	bits, err := chain.NextRequiredBits()
	if err != nil {
		return nil, 0, err
	}
	height := tip.Height + 1
	totalFees := int64(0)
	selected := make([]*wire.MsgTx, 0)
	if pool != nil {
		entries := pool.Entries()
		for _, entry := range entries {
			totalFees += entry.Fee
			selected = append(selected, entry.Tx)
		}
	}
	coinbase, err := NewCoinbaseTx(height, pubKeyHash, chaincfg.BlockSubsidy(height)+totalFees)
	if err != nil {
		return nil, 0, err
	}
	now := time.Now().Unix()
	// Safe conversion: check bounds before converting to uint32
	if now < 0 || now > 0xffffffff {
		return nil, 0, fmt.Errorf("timestamp out of range")
	}
	timestamp := uint32(now)
	if timestamp <= tip.Time {
		timestamp = tip.Time + 1
	}
	block := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			PrevBlock: prev,
			Timestamp: timestamp,
			Bits:      bits,
		},
		Transactions: append([]*wire.MsgTx{coinbase}, selected...),
	}
	root, err := block.BuildMerkleRoot()
	if err != nil {
		return nil, 0, err
	}
	block.Header.MerkleRoot = root
	return block, height, nil
}

func NewCoinbaseTx(height int32, pubKeyHash []byte, value int64) (*wire.MsgTx, error) {
	pkScript, err := script.PayToPubKeyHashScript(pubKeyHash)
	if err != nil {
		return nil, err
	}
	// Safe conversion: height is always >= 0 for valid blocks, fits in uint32
	if height < 0 {
		return nil, fmt.Errorf("invalid negative height")
	}
	h := uint32(height)
	heightScript := []byte{byte(h & 0xff), byte((h >> 8) & 0xff), byte((h >> 16) & 0xff), byte((h >> 24) & 0xff)}
	sigScript := append([]byte{byte(len(heightScript))}, heightScript...)
	sigScript = append(sigScript, []byte("/Legacy-GO/")...)
	return &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: math.MaxUint32},
			SignatureScript:  sigScript,
			Sequence:         math.MaxUint32,
		}},
		TxOut: []wire.TxOut{{
			Value:    value,
			PkScript: pkScript,
		}},
	}, nil
}

func MineBlock(ctx context.Context, chain *blockchain.Chain, pool *mempool.Pool, hasher pow.Hasher, pubKeyHash []byte, workers int, progress func(Progress)) (Result, error) {
	if workers <= 0 {
		workers = 1
	}
	template, height, err := NewBlockTemplate(chain, pool, pubKeyHash)
	if err != nil {
		return Result{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type mineResult struct {
		block *wire.MsgBlock
		hash  chainhash.Hash
		err   error
	}
	resultc := make(chan mineResult, 1)
	var attempts atomic.Uint64
	var lastNonce atomic.Uint32
	var wg sync.WaitGroup
	start := time.Now()

	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			block := *template
			block.Transactions = template.Transactions
			step := uint32(workers)
			for nonce := uint32(worker); ; nonce += step {
				select {
				case <-ctx.Done():
					return
				default:
				}
				block.Header.Nonce = nonce
				hash, err := hasher.HashHeader(block.Header)
				if err != nil {
					select {
					case resultc <- mineResult{err: err}:
						cancel()
					default:
					}
					return
				}
				attempts.Add(1)
				lastNonce.Store(nonce)
				if consensus.CheckProofOfWork(hash, block.Header.Bits) == nil {
					found := block
					select {
					case resultc <- mineResult{block: &found, hash: hash}:
						cancel()
					default:
					}
					return
				}
				if nonce > math.MaxUint32-step {
					return
				}
			}
		}(worker)
	}

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case res := <-resultc:
			wg.Wait()
			if res.err != nil {
				return Result{}, res.err
			}
			if err := chain.ConnectBlock(res.block); err != nil {
				return Result{}, err
			}
			if pool != nil {
				pool.RemoveForBlock(res.block)
			}
			return Result{Block: res.block, Hash: res.hash, Height: height}, nil
		case <-ticker.C:
			if progress != nil {
				elapsed := time.Since(start).Seconds()
				count := attempts.Load()
				rate := float64(0)
				if elapsed > 0 {
					rate = float64(count) / elapsed
				}
				progress(Progress{Attempts: count, Nonce: lastNonce.Load(), Rate: rate})
			}
		case <-done:
			select {
			case res := <-resultc:
				if res.err != nil {
					return Result{}, res.err
				}
				if err := chain.ConnectBlock(res.block); err != nil {
					return Result{}, err
				}
				if pool != nil {
					pool.RemoveForBlock(res.block)
				}
				return Result{Block: res.block, Hash: res.hash, Height: height}, nil
			default:
				return Result{}, fmt.Errorf("block nonce not found")
			}
		case <-ctx.Done():
			wg.Wait()
			return Result{}, ctx.Err()
		}
	}
}

type BenchmarkResult struct {
	DurationSeconds int64
	Threads         int
	Hashes          uint64
	HashPS          float64
	HashesPerThread float64
	LastNonce       uint32
	CurrentBits     uint32
}

func BenchmarkHashrate(ctx context.Context, chain *blockchain.Chain, pool *mempool.Pool, hasher pow.Hasher, pubKeyHash []byte, workers int, duration time.Duration) (BenchmarkResult, error) {
	if workers <= 0 {
		workers = 1
	}
	if duration <= 0 {
		duration = 10 * time.Second
	}
	template, _, err := NewBlockTemplate(chain, pool, pubKeyHash)
	if err != nil {
		return BenchmarkResult{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()
	var attempts atomic.Uint64
	var lastNonce atomic.Uint32
	var wg sync.WaitGroup
	start := time.Now()
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			block := *template
			block.Transactions = template.Transactions
			step := uint32(workers)
			for nonce := uint32(worker); ; nonce += step {
				select {
				case <-ctx.Done():
					return
				default:
				}
				block.Header.Nonce = nonce
				_, err := hasher.HashHeader(block.Header)
				if err != nil {
					return
				}
				attempts.Add(1)
				lastNonce.Store(nonce)
				if nonce > math.MaxUint32-step {
					return
				}
			}
		}(worker)
	}
	wg.Wait()
	elapsed := time.Since(start).Seconds()
	hashes := attempts.Load()
	hps := float64(0)
	if elapsed > 0 {
		hps = float64(hashes) / elapsed
	}
	return BenchmarkResult{
		DurationSeconds: int64(elapsed),
		Threads:         workers,
		Hashes:          hashes,
		HashPS:          hps,
		HashesPerThread: hashesPerThreadMining(hps, workers),
		LastNonce:       lastNonce.Load(),
		CurrentBits:     template.Header.Bits,
	}, nil
}

func hashesPerThreadMining(total float64, threads int) float64 {
	if threads <= 0 {
		return 0
	}
	return total / float64(threads)
}
