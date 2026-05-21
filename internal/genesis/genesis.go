package genesis

import (
	"context"
	"encoding/hex"
	"fmt"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/consensus"
	"legacycoin/legacy-go/internal/pow"
	"legacycoin/legacy-go/internal/wire"
)

func NewBlock(params chaincfg.Params) (*wire.MsgBlock, error) {
	coinbaseScript := append([]byte{0x04, 0xff, 0xff, 0x00, 0x1d, 0x01, 0x04, byte(len(params.GenesisTimestamp))}, []byte(params.GenesisTimestamp)...)
	pubKey, err := hex.DecodeString("5F1DF16B2B704C8A578D0BBAF74D385CDE12C11EE50455F3C438EF4C3FBCF649B6DE611FEAE06279A60939E028A8D65C10B73071A6F16719274855FEB0FD8A6704")
	if err != nil {
		return nil, err
	}
	pkScript := append([]byte{byte(len(pubKey))}, pubKey...)
	pkScript = append(pkScript, 0xac)

	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: math.MaxUint32},
			SignatureScript:  coinbaseScript,
			Sequence:         math.MaxUint32,
		}},
		TxOut: []wire.TxOut{{
			Value:    chaincfg.Subsidy,
			PkScript: pkScript,
		}},
	}
	block := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			Timestamp: params.GenesisTime,
			Bits:      params.GenesisBits,
			Nonce:     params.GenesisNonce,
		},
		Transactions: []*wire.MsgTx{tx},
	}
	root, err := block.BuildMerkleRoot()
	if err != nil {
		return nil, err
	}
	block.Header.MerkleRoot = root
	return block, nil
}

func Mine(params chaincfg.Params, hasher pow.Hasher) (*wire.MsgBlock, chainhash.Hash, error) {
	block, err := NewBlock(params)
	if err != nil {
		return nil, chainhash.Hash{}, err
	}
	for {
		hash, err := hasher.HashHeader(block.Header)
		if err != nil {
			return nil, chainhash.Hash{}, err
		}
		if consensus.CheckProofOfWork(hash, block.Header.Bits) == nil {
			return block, hash, nil
		}
		block.Header.Nonce++
		if block.Header.Nonce == 0 {
			block.Header.Timestamp++
		}
	}
}

type MineProgress struct {
	Attempts uint64
	Nonce    uint32
	Rate     float64
}

func MineParallel(ctx context.Context, params chaincfg.Params, hasher pow.Hasher, workers int, progress func(MineProgress)) (*wire.MsgBlock, chainhash.Hash, error) {
	if workers <= 0 {
		workers = 1
	}
	template, err := NewBlock(params)
	if err != nil {
		return nil, chainhash.Hash{}, err
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type result struct {
		block *wire.MsgBlock
		hash  chainhash.Hash
		err   error
	}
	resultc := make(chan result, 1)
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
					case resultc <- result{err: err}:
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
					case resultc <- result{block: &found, hash: hash}:
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
			return res.block, res.hash, res.err
		case <-ticker.C:
			if progress != nil {
				elapsed := time.Since(start).Seconds()
				count := attempts.Load()
				rate := float64(0)
				if elapsed > 0 {
					rate = float64(count) / elapsed
				}
				progress(MineProgress{Attempts: count, Nonce: lastNonce.Load(), Rate: rate})
			}
		case <-done:
			select {
			case res := <-resultc:
				return res.block, res.hash, res.err
			default:
				return nil, chainhash.Hash{}, fmt.Errorf("genesis nonce not found")
			}
		case <-ctx.Done():
			wg.Wait()
			return nil, chainhash.Hash{}, ctx.Err()
		}
	}
}

func Describe(params chaincfg.Params) (string, error) {
	block, err := NewBlock(params)
	if err != nil {
		return "", err
	}
	headerHash, err := block.Header.Hash()
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("timestamp=%q time=%d bits=%08x nonce=%d merkle=%s header_sha256d=%s",
		params.GenesisTimestamp,
		block.Header.Timestamp,
		block.Header.Bits,
		block.Header.Nonce,
		block.Header.MerkleRoot.String(),
		headerHash.String(),
	), nil
}
