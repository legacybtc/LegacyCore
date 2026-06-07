package blockchain

import (
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/consensus"
	"legacycoin/legacy-go/internal/genesis"
	"legacycoin/legacy-go/internal/pow"
	"legacycoin/legacy-go/internal/script"
	"legacycoin/legacy-go/internal/wire"
)

var (
	ErrNoTransactions   = errors.New("block has no transactions")
	ErrBadMerkleRoot    = errors.New("bad merkle root")
	ErrBadPrevBlock     = errors.New("bad previous block")
	ErrBadGenesisHash   = errors.New("bad genesis hash")
	ErrGenesisPending   = errors.New("genesis hash is not locked")
	ErrBadCoinbase      = errors.New("bad coinbase transaction")
	ErrMissingTxOut     = errors.New("missing transaction output")
	ErrDuplicateSpend   = errors.New("duplicate spend")
	ErrBadTxValue       = errors.New("bad transaction value")
	ErrBadBits          = errors.New("bad difficulty bits")
	ErrImmatureCoinbase = errors.New("immature coinbase spend")
	ErrBadCoinbaseValue = errors.New("bad coinbase value")
	ErrSideChainBlock   = errors.New("side chain block not active yet")
	ErrNonFinalTx       = errors.New("non-final transaction")
	ErrDuplicateTxID    = errors.New("duplicate transaction id in block")
	ErrBadBlockSize     = errors.New("bad block size")
	ErrTimeTooOld       = errors.New("block timestamp too old")
	ErrTimeTooNew       = errors.New("block timestamp too far in future")
	ErrTooManySigOps    = errors.New("too many signature operations")
)

const lockTimeThreshold uint32 = 500000000
const (
	maxBlockSerializedSize = 1_000_000
	minCoinbaseScriptLen   = 2
	maxCoinbaseScriptLen   = 100
	MaxOrphanBlocks        = 128
)

type BlockIndex struct {
	Height    int32  `json:"height"`
	Hash      string `json:"hash"`
	Time      uint32 `json:"time"`
	Bits      uint32 `json:"bits"`
	Nonce     uint32 `json:"nonce"`
	Parent    string `json:"parent,omitempty"`
	ChainWork string `json:"chainwork,omitempty"`
}

type Store interface {
	LoadTip() (*BlockIndex, error)
	SaveTip(BlockIndex) error
	SaveBlock(*wire.MsgBlock, BlockIndex, []UTXOEntry, []string, []UTXOEntry) error
	DisconnectBlock(hash string, prevTip *BlockIndex, undo UndoData) error
	LoadBlock(hash string) (*wire.MsgBlock, *BlockIndex, error)
	LoadIndexByHeight(height int32) (*BlockIndex, error)
	LoadUTXO(key string) (*UTXOEntry, error)
	LoadUndo(hash string) (*UndoData, error)
	ListUTXO() ([]UTXOEntry, error)
	UTXOStats() (UTXOStats, error)
}

type TxIndexRecord struct {
	TxID        string `json:"txid"`
	BlockHash   string `json:"block_hash"`
	BlockHeight int32  `json:"block_height"`
	TxPosition  int    `json:"tx_position"`
}

type AddressIndexUTXO struct {
	Address     string `json:"address"`
	TxID        string `json:"txid"`
	Vout        uint32 `json:"vout"`
	Value       int64  `json:"value"`
	Height      int32  `json:"height"`
	Coinbase    bool   `json:"coinbase"`
	PkScriptHex string `json:"script_pub_key"`
}

type AddressHistoryEntry struct {
	Address     string `json:"address"`
	TxID        string `json:"txid"`
	Vout        uint32 `json:"vout"`
	Value       int64  `json:"value"`
	Height      int32  `json:"height"`
	Coinbase    bool   `json:"coinbase"`
	PkScriptHex string `json:"script_pub_key"`
	Spent       bool   `json:"spent"`
	SpendTxID   string `json:"spend_txid,omitempty"`
	SpendHeight int32  `json:"spend_height,omitempty"`
}

type txIndexStore interface {
	TxIndexEnabled() bool
	LookupTxIndex(txid string) (*TxIndexRecord, error)
}

type addressIndexStore interface {
	AddressIndexEnabled() bool
	AddressTxIDs(address string) ([]string, error)
	AddressUTXOs(address string) ([]AddressIndexUTXO, error)
	AddressBalance(address string) (int64, int64, error)
	AddressHistory(address string) ([]AddressHistoryEntry, error)
}

type UTXOEntry struct {
	Key      string `json:"key"`
	TxID     string `json:"txid"`
	Vout     uint32 `json:"vout"`
	Value    int64  `json:"value"`
	PkScript string `json:"pkscript"`
	Height   int32  `json:"height"`
	Coinbase bool   `json:"coinbase"`
}

type UTXOStats struct {
	Count int64 `json:"count"`
	Total int64 `json:"total"`
}

type UndoData struct {
	AddedKeys []string    `json:"added_keys"`
	Spent     []UTXOEntry `json:"spent"`
}

type Chain struct {
	params chaincfg.Params
	hasher pow.Hasher
	store  Store

	mu  sync.RWMutex
	tip *BlockIndex

	orphanByHash map[string]*wire.MsgBlock
	orphanByPrev map[string][]string
	orphanParent map[string]string
	orphanOrder  []string
	sideBlocks   map[string]*sideBlockNode
	workByHash   map[string]*big.Int
	parentByHash map[string]string
}

type sideBlockNode struct {
	hash      string
	parent    string
	height    int32
	block     *wire.MsgBlock
	chainwork *big.Int
}

type BlockProcessStatus string

const (
	BlockStatusConnected BlockProcessStatus = "connected"
	BlockStatusDuplicate BlockProcessStatus = "duplicate"
	BlockStatusSideChain BlockProcessStatus = "side-chain"
	BlockStatusOrphan    BlockProcessStatus = "orphan"
	BlockStatusRejected  BlockProcessStatus = "rejected"
	BlockStatusProposal  BlockProcessStatus = "valid-proposal"
)

type BlockProcessResult struct {
	Hash             string             `json:"hash"`
	PrevHash         string             `json:"prev_hash"`
	CalculatedHeight int32              `json:"calculated_height"`
	ParentKnown      bool               `json:"parent_known"`
	ParentActive     bool               `json:"parent_active"`
	ExtendsActiveTip bool               `json:"extends_active_tip"`
	Duplicate        bool               `json:"duplicate"`
	SideChain        bool               `json:"side_chain"`
	Orphan           bool               `json:"orphan"`
	Connected        bool               `json:"connected"`
	BestChanged      bool               `json:"best_changed"`
	OldBestHeight    int32              `json:"old_best_height"`
	OldBestHash      string             `json:"old_best_hash"`
	NewBestHeight    int32              `json:"new_best_height"`
	NewBestHash      string             `json:"new_best_hash"`
	Status           BlockProcessStatus `json:"status"`
	Reason           string             `json:"reason,omitempty"`
}

var nowUnix = func() uint32 {
	return uint32(time.Now().UTC().Unix())
}

// HashHeader returns the canonical consensus block hash for this chain.
// Legacy Coin uses Yespower as block identity, not the wire-level SHA256d
// header helper. P2P inventory, RPC block announcements, indexes, locators,
// and stop-hash comparisons must use this method whenever they mean the
// consensus block hash.
func (c *Chain) HashHeader(header wire.BlockHeader) (chainhash.Hash, error) {
	return c.hasher.HashHeader(header)
}

// BlockHash returns the canonical consensus hash for a full block.
func (c *Chain) BlockHash(block *wire.MsgBlock) (chainhash.Hash, error) {
	if block == nil {
		return chainhash.Hash{}, errors.New("nil block")
	}
	return c.HashHeader(block.Header)
}

func New(params chaincfg.Params, hasher pow.Hasher, store Store) (*Chain, error) {
	c := &Chain{
		params:       params,
		hasher:       hasher,
		store:        store,
		orphanByHash: make(map[string]*wire.MsgBlock),
		orphanByPrev: make(map[string][]string),
		orphanParent: make(map[string]string),
		orphanOrder:  make([]string, 0, MaxOrphanBlocks),
		sideBlocks:   make(map[string]*sideBlockNode),
		workByHash:   make(map[string]*big.Int),
		parentByHash: make(map[string]string),
	}
	tip, err := store.LoadTip()
	if err != nil {
		return nil, err
	}
	c.tip = tip
	if err := c.rebuildActiveChainworkLocked(); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Chain) ProcessBlock(block *wire.MsgBlock) error {
	_, err := c.ProcessBlockWithResult(block)
	return err
}

func cloneBig(n *big.Int) *big.Int {
	if n == nil {
		return big.NewInt(0)
	}
	return new(big.Int).Set(n)
}

func parseChainwork(v string) (*big.Int, bool) {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, false
	}
	out := new(big.Int)
	base := 10
	if strings.HasPrefix(v, "0x") || strings.HasPrefix(v, "0X") {
		base = 0
	}
	if _, ok := out.SetString(v, base); !ok {
		return nil, false
	}
	if out.Sign() < 0 {
		return nil, false
	}
	return out, true
}

func (c *Chain) rebuildActiveChainworkLocked() error {
	c.workByHash = make(map[string]*big.Int)
	c.parentByHash = make(map[string]string)
	if c.tip == nil || c.tip.Hash == "" || c.tip.Height < 0 {
		return nil
	}
	var running = big.NewInt(0)
	for height := int32(0); height <= c.tip.Height; height++ {
		idx, err := c.store.LoadIndexByHeight(height)
		if err != nil {
			// Older/synthetic stores can have sparse height indexes. Continue and
			// derive tip chainwork from parent links below.
			continue
		}
		parent := idx.Parent
		if parent == "" && idx.Height > 0 {
			block, _, err := c.store.LoadBlock(idx.Hash)
			if err != nil {
				return err
			}
			parent = block.Header.PrevBlock.String()
		}
		var cw *big.Int
		if parsed, ok := parseChainwork(idx.ChainWork); ok {
			cw = parsed
		} else {
			running = new(big.Int).Add(running, consensus.WorkForBits(idx.Bits))
			cw = cloneBig(running)
		}
		c.workByHash[idx.Hash] = cloneBig(cw)
		c.parentByHash[idx.Hash] = parent
		running = cloneBig(cw)
	}
	if _, ok := c.workByHash[c.tip.Hash]; !ok {
		work, err := c.chainworkForHashLocked(c.tip.Hash)
		if err != nil {
			return err
		}
		c.workByHash[c.tip.Hash] = cloneBig(work)
	}
	return nil
}

func (c *Chain) chainworkForHashLocked(hash string) (*big.Int, error) {
	if hash == "" {
		return big.NewInt(0), nil
	}
	if work, ok := c.workByHash[hash]; ok {
		return cloneBig(work), nil
	}
	if side, ok := c.sideBlocks[hash]; ok {
		if side.chainwork != nil {
			c.workByHash[hash] = cloneBig(side.chainwork)
			c.parentByHash[hash] = side.parent
			return cloneBig(side.chainwork), nil
		}
		parentWork, err := c.chainworkForHashLocked(side.parent)
		if err != nil {
			return nil, err
		}
		total := new(big.Int).Add(parentWork, consensus.WorkForBits(side.block.Header.Bits))
		side.chainwork = cloneBig(total)
		c.workByHash[hash] = cloneBig(total)
		c.parentByHash[hash] = side.parent
		return total, nil
	}
	block, idx, err := c.store.LoadBlock(hash)
	if err != nil {
		return nil, err
	}
	parent := idx.Parent
	if parent == "" && idx.Height > 0 {
		parent = block.Header.PrevBlock.String()
	}
	parentWork, err := c.chainworkForHashLocked(parent)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			parentWork = big.NewInt(0)
		} else {
			return nil, err
		}
	}
	work := consensus.WorkForBits(idx.Bits)
	total := new(big.Int).Add(parentWork, work)
	c.workByHash[hash] = cloneBig(total)
	c.parentByHash[hash] = parent
	return total, nil
}

func (c *Chain) computeChildChainworkLocked(parentHash string, bits uint32) (*big.Int, error) {
	parentWork, err := c.chainworkForHashLocked(parentHash)
	if err != nil {
		return nil, err
	}
	return new(big.Int).Add(parentWork, consensus.WorkForBits(bits)), nil
}

func (c *Chain) buildSideNodeLocked(hash string, parent string, height int32, block *wire.MsgBlock) (*sideBlockNode, error) {
	work, err := c.computeChildChainworkLocked(parent, block.Header.Bits)
	if err != nil {
		return nil, err
	}
	return &sideBlockNode{
		hash:      hash,
		parent:    parent,
		height:    height,
		block:     block,
		chainwork: work,
	}, nil
}

func (c *Chain) ProcessBlockWithResult(block *wire.MsgBlock) (BlockProcessResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.processBlockLocked(block)
}

func (c *Chain) ValidateBlockProposal(block *wire.MsgBlock) (BlockProcessResult, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	result, err := c.blockProcessPreviewLocked(block, false)
	if err != nil || result.Status != "" {
		return result, err
	}
	idx, _, _, _, _, err := c.validateActiveBlockLocked(block)
	if err != nil {
		result.Status = BlockStatusRejected
		result.Reason = err.Error()
		return result, err
	}
	result.CalculatedHeight = idx.Height
	result.Status = BlockStatusProposal
	result.Reason = "proposal would extend active best chain"
	return result, nil
}

func (c *Chain) EnsureGenesis() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.tip != nil {
		return nil
	}
	block, err := genesis.NewBlock(c.params)
	if err != nil {
		return err
	}
	hash, err := c.hasher.HashHeader(block.Header)
	if err != nil {
		return fmt.Errorf("hash genesis: %w", err)
	}
	if c.params.GenesisHash == "" {
		idx := BlockIndex{Height: -1, Hash: "", Time: block.Header.Timestamp, Bits: block.Header.Bits, Nonce: block.Header.Nonce, Parent: ""}
		c.tip = &idx
		return nil
	}
	if hash.String() != c.params.GenesisHash {
		return fmt.Errorf("%w: got %s, want %s", ErrBadGenesisHash, hash.String(), c.params.GenesisHash)
	}
	if err := consensus.CheckProofOfWork(hash, block.Header.Bits); err != nil {
		return fmt.Errorf("validate genesis pow: %w", err)
	}
	genesisWork := consensus.WorkForBits(block.Header.Bits)
	idx := BlockIndex{Height: 0, Hash: hash.String(), Time: block.Header.Timestamp, Bits: block.Header.Bits, Nonce: block.Header.Nonce, Parent: "", ChainWork: genesisWork.Text(10)}
	adds, spendKeys, spentEntries, err := c.validateBlockTransactions(block, idx.Height)
	if err != nil {
		return err
	}
	if err := c.store.SaveBlock(block, idx, adds, spendKeys, spentEntries); err != nil {
		return err
	}
	c.workByHash[idx.Hash] = cloneBig(genesisWork)
	c.parentByHash[idx.Hash] = ""
	c.tip = &idx
	return nil
}

func (c *Chain) ConnectBlock(block *wire.MsgBlock) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.connectBlockLocked(block)
}

func (c *Chain) connectBlockLocked(block *wire.MsgBlock) error {
	idx, chainwork, adds, spendKeys, spentEntries, err := c.validateActiveBlockLocked(block)
	if err != nil {
		return err
	}
	if err := c.store.SaveBlock(block, idx, adds, spendKeys, spentEntries); err != nil {
		return err
	}
	c.workByHash[idx.Hash] = cloneBig(chainwork)
	c.parentByHash[idx.Hash] = idx.Parent
	c.tip = &idx
	return nil
}

func (c *Chain) validateActiveBlockLocked(block *wire.MsgBlock) (BlockIndex, *big.Int, []UTXOEntry, []string, []UTXOEntry, error) {
	var idx BlockIndex
	if len(block.Transactions) == 0 {
		return idx, nil, nil, nil, nil, ErrNoTransactions
	}
	root, err := block.BuildMerkleRoot()
	if err != nil {
		return idx, nil, nil, nil, nil, err
	}
	if root != block.Header.MerkleRoot {
		return idx, nil, nil, nil, nil, ErrBadMerkleRoot
	}
	if c.tip != nil {
		if c.tip.Hash == "" {
			return idx, nil, nil, nil, nil, ErrGenesisPending
		}
		prev, err := chainhash.FromString(c.tip.Hash)
		if err != nil {
			return idx, nil, nil, nil, nil, err
		}
		if block.Header.PrevBlock != prev {
			return idx, nil, nil, nil, nil, ErrBadPrevBlock
		}
		mtp, err := c.medianTimePastLocked(c.tip.Hash)
		if err != nil {
			return idx, nil, nil, nil, nil, err
		}
		if block.Header.Timestamp <= mtp {
			return idx, nil, nil, nil, nil, ErrTimeTooOld
		}
	}
	maxFuture := nowUnix() + uint32(chaincfg.MaxFutureDrift.Seconds())
	if block.Header.Timestamp > maxFuture {
		return idx, nil, nil, nil, nil, ErrTimeTooNew
	}
	expectedBits, err := c.nextRequiredBitsLocked()
	if err != nil {
		return idx, nil, nil, nil, nil, err
	}
	if block.Header.Bits != expectedBits {
		return idx, nil, nil, nil, nil, fmt.Errorf("%w: got %08x, want %08x", ErrBadBits, block.Header.Bits, expectedBits)
	}
	hash, err := c.hasher.HashHeader(block.Header)
	if err != nil {
		return idx, nil, nil, nil, nil, err
	}
	if err := consensus.CheckProofOfWork(hash, block.Header.Bits); err != nil {
		return idx, nil, nil, nil, nil, err
	}
	height := int32(0)
	parentHash := ""
	if c.tip != nil {
		height = c.tip.Height + 1
		parentHash = c.tip.Hash
	}
	chainwork, err := c.computeChildChainworkLocked(parentHash, block.Header.Bits)
	if err != nil {
		return idx, nil, nil, nil, nil, err
	}
	idx = BlockIndex{
		Height:    height,
		Hash:      hash.String(),
		Time:      block.Header.Timestamp,
		Bits:      block.Header.Bits,
		Nonce:     block.Header.Nonce,
		Parent:    parentHash,
		ChainWork: chainwork.Text(10),
	}
	adds, spendKeys, spentEntries, err := c.validateBlockTransactions(block, idx.Height)
	if err != nil {
		return idx, nil, nil, nil, nil, err
	}
	return idx, chainwork, adds, spendKeys, spentEntries, nil
}

func (c *Chain) processBlockLocked(block *wire.MsgBlock) (BlockProcessResult, error) {
	result, err := c.blockProcessPreviewLocked(block, true)
	if err != nil || result.Status != "" {
		return result, err
	}
	hashStr := result.Hash
	if err := c.connectBlockLocked(block); err != nil {
		result.Status = BlockStatusRejected
		result.Reason = err.Error()
		return result, err
	}
	c.acceptOrphanChildrenLocked(hashStr)
	result.Connected = true
	result.Status = BlockStatusConnected
	result.Reason = "extends active best chain"
	result.finish(c.tip)
	return result, nil
}

func (c *Chain) blockProcessPreviewLocked(block *wire.MsgBlock, mutate bool) (BlockProcessResult, error) {
	result := BlockProcessResult{
		CalculatedHeight: -1,
		OldBestHeight:    -1,
		NewBestHeight:    -1,
	}
	if c.tip != nil {
		result.OldBestHeight = c.tip.Height
		result.OldBestHash = c.tip.Hash
		result.NewBestHeight = c.tip.Height
		result.NewBestHash = c.tip.Hash
	}
	hash, err := c.hasher.HashHeader(block.Header)
	if err != nil {
		return result, err
	}
	hashStr := hash.String()
	result.Hash = hashStr
	result.PrevHash = block.Header.PrevBlock.String()
	if c.HasBlock(hashStr) {
		result.Duplicate = true
		result.Status = BlockStatusDuplicate
		result.Reason = "block already stored"
		if h, ok := c.activeHeight(hashStr); ok {
			result.CalculatedHeight = h
			result.ParentKnown = true
			result.ParentActive = true
		} else if h, ok := c.indexedHeight(hashStr); ok {
			result.CalculatedHeight = h
			result.ParentKnown = true
		}
		return result, nil
	}
	if _, ok := c.orphanByHash[hashStr]; ok {
		result.Duplicate = true
		result.Orphan = true
		result.Status = BlockStatusDuplicate
		result.Reason = "orphan already stored"
		return result, nil
	}
	if c.tip != nil && c.tip.Hash != "" {
		parent := block.Header.PrevBlock.String()
		if parent != c.tip.Hash {
			if parentHeight, ok := c.activeHeight(parent); ok {
				result.ParentKnown = true
				result.ParentActive = true
				result.SideChain = true
				result.CalculatedHeight = parentHeight + 1
				if !mutate {
					result.finish(c.tip)
					result.Status = BlockStatusSideChain
					result.Reason = "would be side-chain block; active best chain not updated"
					return result, nil
				}
				sideNode, err := c.buildSideNodeLocked(hashStr, parent, parentHeight+1, block)
				if err != nil {
					return result, err
				}
				c.sideBlocks[hashStr] = sideNode
				if err := c.tryActivateSideChainLocked(hashStr); err != nil {
					return result, err
				}
				result.finish(c.tip)
				if result.BestChanged && result.NewBestHash == hashStr {
					result.Connected = true
					result.Status = BlockStatusConnected
					result.Reason = "side branch became active best chain"
				} else {
					result.Status = BlockStatusSideChain
					result.Reason = "stored as side-chain block; active best chain not updated"
				}
				return result, nil
			}
			if parentHeight, ok := c.indexedHeight(parent); ok {
				result.ParentKnown = true
				result.SideChain = true
				result.CalculatedHeight = parentHeight + 1
				if !mutate {
					result.finish(c.tip)
					result.Status = BlockStatusSideChain
					result.Reason = "would be side-chain block; active best chain not updated"
					return result, nil
				}
				sideNode, err := c.buildSideNodeLocked(hashStr, parent, parentHeight+1, block)
				if err != nil {
					return result, err
				}
				c.sideBlocks[hashStr] = sideNode
				if err := c.tryActivateSideChainLocked(hashStr); err != nil {
					return result, err
				}
				result.finish(c.tip)
				if result.BestChanged && result.NewBestHash == hashStr {
					result.Connected = true
					result.Status = BlockStatusConnected
					result.Reason = "stored branch became active best chain"
				} else {
					result.Status = BlockStatusSideChain
					result.Reason = "stored as side-chain block; active best chain not updated"
				}
				return result, nil
			}
			if parentNode, ok := c.sideBlocks[parent]; ok {
				result.ParentKnown = true
				result.SideChain = true
				result.CalculatedHeight = parentNode.height + 1
				if !mutate {
					result.finish(c.tip)
					result.Status = BlockStatusSideChain
					result.Reason = "would extend known side branch; active best chain not updated"
					return result, nil
				}
				sideNode, err := c.buildSideNodeLocked(hashStr, parent, parentNode.height+1, block)
				if err != nil {
					return result, err
				}
				c.sideBlocks[hashStr] = sideNode
				if err := c.tryActivateSideChainLocked(hashStr); err != nil {
					return result, err
				}
				result.finish(c.tip)
				if result.BestChanged && result.NewBestHash == hashStr {
					result.Connected = true
					result.Status = BlockStatusConnected
					result.Reason = "side branch became active best chain"
				} else {
					result.Status = BlockStatusSideChain
					result.Reason = "stored as side-chain block; active best chain not updated"
				}
				return result, nil
			}
			result.Orphan = true
			result.Status = BlockStatusOrphan
			result.Reason = "parent unknown"
			if !mutate {
				result.finish(c.tip)
				return result, nil
			}
			c.addOrphanLocked(hashStr, parent, block)
			result.finish(c.tip)
			return result, nil
		}
		result.ParentKnown = true
		result.ParentActive = true
		result.ExtendsActiveTip = true
		result.CalculatedHeight = c.tip.Height + 1
	} else {
		result.CalculatedHeight = 0
	}
	return result, nil
}

func (r *BlockProcessResult) finish(tip *BlockIndex) {
	if tip != nil {
		r.NewBestHeight = tip.Height
		r.NewBestHash = tip.Hash
	}
	r.BestChanged = r.OldBestHeight != r.NewBestHeight || r.OldBestHash != r.NewBestHash
}

func (c *Chain) acceptOrphanChildrenLocked(parentHash string) {
	queue := []string{parentHash}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		children := c.orphanByPrev[cur]
		delete(c.orphanByPrev, cur)
		for _, orphanHash := range children {
			orphanBlock, ok := c.orphanByHash[orphanHash]
			if !ok {
				continue
			}
			if c.tip == nil || c.tip.Hash != orphanBlock.Header.PrevBlock.String() {
				continue
			}
			if err := c.connectBlockLocked(orphanBlock); err != nil {
				continue
			}
			c.removeOrphanLocked(orphanHash)
			delete(c.sideBlocks, orphanHash)
			queue = append(queue, orphanHash)
		}
	}
}

func (c *Chain) addOrphanLocked(hash string, parent string, block *wire.MsgBlock) {
	if _, ok := c.orphanByHash[hash]; ok {
		return
	}
	if len(c.orphanOrder) >= MaxOrphanBlocks {
		c.removeOrphanLocked(c.orphanOrder[0])
	}
	c.orphanByHash[hash] = block
	c.orphanByPrev[parent] = append(c.orphanByPrev[parent], hash)
	c.orphanParent[hash] = parent
	c.orphanOrder = append(c.orphanOrder, hash)
}

func (c *Chain) removeOrphanLocked(hash string) {
	if _, ok := c.orphanByHash[hash]; !ok {
		return
	}
	delete(c.orphanByHash, hash)
	parent := c.orphanParent[hash]
	delete(c.orphanParent, hash)
	if parent != "" {
		children := c.orphanByPrev[parent]
		for i, h := range children {
			if h == hash {
				children = append(children[:i], children[i+1:]...)
				break
			}
		}
		if len(children) == 0 {
			delete(c.orphanByPrev, parent)
		} else {
			c.orphanByPrev[parent] = children
		}
	}
	for i, h := range c.orphanOrder {
		if h == hash {
			c.orphanOrder = append(c.orphanOrder[:i], c.orphanOrder[i+1:]...)
			break
		}
	}
}

func (c *Chain) activeHeight(hash string) (int32, bool) {
	_, idx, err := c.store.LoadBlock(hash)
	if err != nil {
		return 0, false
	}
	activeIdx, err := c.store.LoadIndexByHeight(idx.Height)
	if err != nil {
		return 0, false
	}
	if activeIdx.Hash != hash {
		return 0, false
	}
	return idx.Height, true
}

func (c *Chain) indexedHeight(hash string) (int32, bool) {
	_, idx, err := c.store.LoadBlock(hash)
	if err != nil {
		return 0, false
	}
	return idx.Height, true
}

func (c *Chain) tryActivateSideChainLocked(sideTipHash string) error {
	node, ok := c.sideBlocks[sideTipHash]
	if !ok || c.tip == nil {
		return nil
	}
	activeWork, err := c.chainworkForHashLocked(c.tip.Hash)
	if err != nil {
		return err
	}
	if node.chainwork == nil || node.chainwork.Cmp(activeWork) <= 0 {
		return nil
	}

	// Build path from side tip back to fork point
	attachRev := make([]*sideBlockNode, 0)
	cur := node
	var forkHash string
	var forkHeight int32
	for {
		attachRev = append(attachRev, cur)
		if h, ok := c.activeHeight(cur.parent); ok {
			forkHash = cur.parent
			forkHeight = h
			break
		}
		parentNode, ok := c.sideBlocks[cur.parent]
		if !ok {
			return nil
		}
		cur = parentNode
	}

	// Log reorg attempt
	if c.tip.Hash != forkHash {
		// This is a reorg - log it
		reorgDepth := c.tip.Height - forkHeight
		fmt.Printf("blockchain: attempting reorg from height %d to %d (depth: %d blocks)\n",
			c.tip.Height, node.height, reorgDepth)
	}

	if c.tip.Hash == forkHash {
		// Fast path: nothing to disconnect.
	} else {
		removed := make([]*wire.MsgBlock, 0)
		for c.tip != nil && c.tip.Height > forkHeight {
			block, _, err := c.store.LoadBlock(c.tip.Hash)
			if err != nil {
				return err
			}
			removed = append(removed, block)
			if err := c.disconnectTipLocked(); err != nil {
				fmt.Printf("blockchain: reorg failed during disconnect at height %d: %v\n", c.tip.Height, err)
				return err
			}
		}
		if c.tip == nil || c.tip.Hash != forkHash {
			// Reorg failed - restore old chain
			fmt.Printf("blockchain: reorg failed to reach fork point, restoring old chain\n")
			for i := len(removed) - 1; i >= 0; i-- {
				_ = c.connectBlockLocked(removed[i])
			}
			return fmt.Errorf("failed to reach fork point")
		}
		// Connect side branch from fork child to side tip.
		connected := make([]*wire.MsgBlock, 0, len(attachRev))
		for i := len(attachRev) - 1; i >= 0; i-- {
			if err := c.connectBlockLocked(attachRev[i].block); err != nil {
				// Roll back partial side activation.
				fmt.Printf("blockchain: reorg failed during connect at height %d: %v, rolling back\n", c.tip.Height+1, err)
				if rollbackErr := c.disconnectNLocked(len(connected)); rollbackErr != nil {
					return fmt.Errorf("side-branch rollback failed after connect error: %v (original: %w)", rollbackErr, err)
				}
				// Restore old main branch.
				if restoreErr := c.reconnectBlocksLocked(removed); restoreErr != nil {
					return fmt.Errorf("main-branch restore failed after connect error: %v (original: %w)", restoreErr, err)
				}
				return err
			}
			connected = append(connected, attachRev[i].block)
		}
		for _, n := range attachRev {
			delete(c.sideBlocks, n.hash)
		}
		fmt.Printf("blockchain: reorg completed successfully to height %d\n", c.tip.Height)
		return nil
	}

	// Fast path (already at fork): just connect side branch and rollback on failure.
	connected := make([]*wire.MsgBlock, 0, len(attachRev))
	for i := len(attachRev) - 1; i >= 0; i-- {
		if err := c.connectBlockLocked(attachRev[i].block); err != nil {
			if rollbackErr := c.disconnectNLocked(len(connected)); rollbackErr != nil {
				return fmt.Errorf("side-branch fast-path rollback failed: %v (original: %w)", rollbackErr, err)
			}
			return err
		}
		connected = append(connected, attachRev[i].block)
	}
	for _, n := range attachRev {
		delete(c.sideBlocks, n.hash)
	}
	return nil
}

func (c *Chain) disconnectNLocked(n int) error {
	for i := 0; i < n; i++ {
		if err := c.disconnectTipLocked(); err != nil {
			return err
		}
	}
	return nil
}

func (c *Chain) reconnectBlocksLocked(disconnected []*wire.MsgBlock) error {
	for i := len(disconnected) - 1; i >= 0; i-- {
		if err := c.connectBlockLocked(disconnected[i]); err != nil {
			return err
		}
	}
	return nil
}

func (c *Chain) validateBlockTransactions(block *wire.MsgBlock, height int32) ([]UTXOEntry, []string, []UTXOEntry, error) {
	if len(block.Transactions) == 0 {
		return nil, nil, nil, ErrNoTransactions
	}
	if !isCoinbase(block.Transactions[0]) {
		return nil, nil, nil, ErrBadCoinbase
	}
	blockBytes, err := block.Bytes()
	if err != nil {
		return nil, nil, nil, err
	}
	if len(blockBytes) > maxBlockSerializedSize {
		return nil, nil, nil, ErrBadBlockSize
	}
	coinbaseSigLen := len(block.Transactions[0].TxIn[0].SignatureScript)
	if coinbaseSigLen < minCoinbaseScriptLen || coinbaseSigLen > maxCoinbaseScriptLen {
		return nil, nil, nil, ErrBadCoinbase
	}
	blockTime := block.Header.Timestamp

	// pendingAdds is an in-memory view of outputs created earlier in this block.
	// This allows valid same-block spends while ensuring outputs spent inside the
	// same block are not written as unspent UTXOs at commit time.
	pendingAdds := make(map[string]UTXOEntry)
	pendingOrder := make([]string, 0)
	spends := make([]string, 0)
	spentEntries := make([]UTXOEntry, 0)
	seenSpends := make(map[string]struct{})
	seenTxIDs := make(map[string]struct{}, len(block.Transactions))
	coinbaseOut := int64(0)
	totalFees := int64(0)
	blockSigOps := 0

	for txIndex, tx := range block.Transactions {
		if txIndex > 0 && isCoinbase(tx) {
			return nil, nil, nil, ErrBadCoinbase
		}
		if !IsFinalizedTx(tx, height, blockTime) {
			return nil, nil, nil, ErrNonFinalTx
		}
		totalOut := int64(0)
		txSigOps := 0
		txHash, err := tx.TxHash()
		if err != nil {
			return nil, nil, nil, err
		}
		txID := txHash.String()
		if _, exists := seenTxIDs[txID]; exists {
			return nil, nil, nil, ErrDuplicateTxID
		}
		seenTxIDs[txID] = struct{}{}

		createdOutputs := make([]UTXOEntry, 0, len(tx.TxOut))
		for vout, out := range tx.TxOut {
			if !chaincfg.MoneyRange(out.Value) {
				return nil, nil, nil, ErrBadTxValue
			}
			totalOut += out.Value
			if !chaincfg.MoneyRange(totalOut) {
				return nil, nil, nil, ErrBadTxValue
			}
			createdOutputs = append(createdOutputs, UTXOEntry{
				Key:      OutPointKey(txID, uint32(vout)),
				TxID:     txID,
				Vout:     uint32(vout),
				Value:    out.Value,
				PkScript: hex.EncodeToString(out.PkScript),
				Height:   height,
				Coinbase: txIndex == 0,
			})
			ops := script.CountSigOps(out.PkScript)
			txSigOps += ops
			blockSigOps += ops
		}

		if txIndex == 0 {
			coinbaseOut = totalOut
			if txSigOps > script.MaxTxSigOps || blockSigOps > script.MaxBlockSigOps {
				return nil, nil, nil, ErrTooManySigOps
			}
			for _, entry := range createdOutputs {
				if _, exists := pendingAdds[entry.Key]; exists {
					return nil, nil, nil, ErrDuplicateTxID
				}
				pendingAdds[entry.Key] = entry
				pendingOrder = append(pendingOrder, entry.Key)
			}
			continue
		}

		totalIn := int64(0)
		for inputIndex, in := range tx.TxIn {
			key := OutPointKey(in.PreviousOutPoint.Hash.String(), in.PreviousOutPoint.Index)
			if _, ok := seenSpends[key]; ok {
				return nil, nil, nil, fmt.Errorf("%w: %s", ErrDuplicateSpend, key)
			}
			seenSpends[key] = struct{}{}

			prev, fromSameBlock := pendingAdds[key]
			if !fromSameBlock {
				loaded, err := c.store.LoadUTXO(key)
				if err != nil {
					return nil, nil, nil, fmt.Errorf("%w: %s", ErrMissingTxOut, key)
				}
				prev = *loaded
			}
			if prev.Coinbase && height-prev.Height < int32(chaincfg.CoinbaseMaturity) {
				return nil, nil, nil, fmt.Errorf("%w: %s", ErrImmatureCoinbase, key)
			}
			prevScript, err := hex.DecodeString(prev.PkScript)
			if err != nil {
				return nil, nil, nil, err
			}
			sigOps, err := script.SigOpsForSpend(in.SignatureScript, prevScript)
			if err != nil {
				return nil, nil, nil, err
			}
			txSigOps += sigOps
			blockSigOps += sigOps
			if err := script.VerifyInput(tx, inputIndex, prevScript); err != nil {
				return nil, nil, nil, err
			}
			totalIn += prev.Value
			if !chaincfg.MoneyRange(totalIn) {
				return nil, nil, nil, ErrBadTxValue
			}

			if fromSameBlock {
				delete(pendingAdds, key)
			} else {
				spends = append(spends, key)
				spentEntries = append(spentEntries, prev)
			}
		}
		if txSigOps > script.MaxTxSigOps || blockSigOps > script.MaxBlockSigOps {
			return nil, nil, nil, ErrTooManySigOps
		}
		if totalIn < totalOut {
			return nil, nil, nil, ErrBadTxValue
		}
		fee := totalIn - totalOut
		totalFees += fee
		if !chaincfg.MoneyRange(totalFees) {
			return nil, nil, nil, ErrBadTxValue
		}

		for _, entry := range createdOutputs {
			if _, exists := pendingAdds[entry.Key]; exists {
				return nil, nil, nil, ErrDuplicateTxID
			}
			pendingAdds[entry.Key] = entry
			pendingOrder = append(pendingOrder, entry.Key)
		}
	}
	maxCoinbase := chaincfg.BlockSubsidy(height) + totalFees
	if coinbaseOut > maxCoinbase || !chaincfg.MoneyRange(coinbaseOut) {
		return nil, nil, nil, ErrBadCoinbaseValue
	}

	adds := make([]UTXOEntry, 0, len(pendingAdds))
	for _, key := range pendingOrder {
		if entry, ok := pendingAdds[key]; ok {
			adds = append(adds, entry)
		}
	}
	return adds, spends, spentEntries, nil
}

func (c *Chain) nextRequiredBitsLocked() (uint32, error) {
	if c.tip == nil {
		return c.params.GenesisBits, nil
	}
	recent, err := c.recentEntriesLocked(c.tip.Height, consensus.DGWv3PastBlocks)
	if err != nil {
		return 0, err
	}
	return requiredBitsFromRecent(recent, c.params.GenesisBits, c.params.PostGenesisBits), nil
}

func (c *Chain) medianTimePastLocked(startHash string) (uint32, error) {
	if startHash == "" {
		return 0, nil
	}
	const mtpWindow = 11
	times := make([]uint32, 0, mtpWindow)
	cur := startHash
	for len(times) < mtpWindow && cur != "" {
		blk, idx, err := c.store.LoadBlock(cur)
		if err != nil {
			return 0, err
		}
		times = append(times, idx.Time)
		if idx.Height <= 0 {
			break
		}
		cur = blk.Header.PrevBlock.String()
	}
	if len(times) == 0 {
		return 0, nil
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	return times[len(times)/2], nil
}

func (c *Chain) NextRequiredBits() (uint32, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.nextRequiredBitsLocked()
}

// ValidateHeaderSequence validates an announced header batch against the active
// chain before P2P asks for corresponding block bodies.  It uses the canonical
// Legacy Coin Yespower header hash and a rolling active-chain view so every
// header in the batch is checked for parent linkage, DGWv3 difficulty, median
// time past, future-time drift, and proof of work before getdata is sent.
func (c *Chain) ValidateHeaderSequence(headers []wire.BlockHeader) ([]chainhash.Hash, error) {
	if len(headers) == 0 {
		return nil, nil
	}

	// Snapshot only the active-chain context needed for rolling validation while
	// holding the chain read lock. Yespower hashing can be expensive for large
	// batches, so it is intentionally performed after the lock is released. This
	// prevents public peers from stalling block connection/reorg writers with
	// header spam.
	c.mu.RLock()
	if c.tip == nil || c.tip.Hash == "" {
		c.mu.RUnlock()
		return nil, errors.New("cannot validate headers before local tip is initialized")
	}
	tipHashString := c.tip.Hash
	tipHeight := c.tip.Height
	genesisBits := c.params.GenesisBits
	postGenesisBits := c.params.PostGenesisBits
	hasher := c.hasher
	recent, err := c.recentEntriesLocked(tipHeight, consensus.DGWv3PastBlocks)
	c.mu.RUnlock()
	if err != nil {
		return nil, err
	}

	prevHash, err := chainhash.FromString(tipHashString)
	if err != nil {
		return nil, err
	}
	if headers[0].PrevBlock != prevHash {
		return nil, fmt.Errorf("header batch does not connect to active tip: got prev %s, want %s", headers[0].PrevBlock.String(), tipHashString)
	}

	maxFuture := nowUnix() + uint32(chaincfg.MaxFutureDrift.Seconds())
	hashes := make([]chainhash.Hash, 0, len(headers))
	nextHeight := tipHeight + 1

	for i, header := range headers {
		if header.PrevBlock != prevHash {
			return nil, fmt.Errorf("headers not linked at position %d", i)
		}
		expectedBits := requiredBitsFromRecent(recent, genesisBits, postGenesisBits)
		if header.Bits != expectedBits {
			return nil, fmt.Errorf("header %d has unexpected bits: got %08x, want %08x", i, header.Bits, expectedBits)
		}
		mtp := medianTimeFromRecent(recent)
		if header.Timestamp <= mtp {
			return nil, fmt.Errorf("header %d timestamp too old: got %d, median-time-past %d", i, header.Timestamp, mtp)
		}
		if header.Timestamp > maxFuture {
			return nil, fmt.Errorf("header %d timestamp too far in future", i)
		}
		hash, err := hasher.HashHeader(header)
		if err != nil {
			return nil, err
		}
		if err := consensus.CheckProofOfWork(hash, header.Bits); err != nil {
			return nil, fmt.Errorf("header %d failed proof-of-work: %w", i, err)
		}
		hashes = append(hashes, hash)
		prevHash = hash
		recent = prependRecentEntry(recent, consensus.BlockWindowEntry{Height: nextHeight, Time: header.Timestamp, Bits: header.Bits}, consensus.DGWv3PastBlocks)
		nextHeight++
	}

	// If the active tip changed while the expensive validation was running, the
	// caller must discard/retry against the new context instead of requesting
	// bodies for a stale view.
	c.mu.RLock()
	tipUnchanged := c.tip != nil && c.tip.Hash == tipHashString && c.tip.Height == tipHeight
	c.mu.RUnlock()
	if !tipUnchanged {
		return nil, errors.New("active tip changed during header validation")
	}
	return hashes, nil
}

func (c *Chain) recentEntriesLocked(startHeight int32, limit int) ([]consensus.BlockWindowEntry, error) {
	if limit <= 0 || startHeight < 0 {
		return nil, nil
	}
	recent := make([]consensus.BlockWindowEntry, 0, limit)
	for height := startHeight; height >= 0 && len(recent) < limit; height-- {
		idx, err := c.store.LoadIndexByHeight(height)
		if err != nil {
			return nil, err
		}
		recent = append(recent, consensus.BlockWindowEntry{Height: idx.Height, Time: idx.Time, Bits: idx.Bits})
	}
	return recent, nil
}

func requiredBitsFromRecent(recent []consensus.BlockWindowEntry, genesisBits uint32, postGenesisBits uint32) uint32 {
	if len(recent) == 0 {
		return genesisBits
	}
	if postGenesisBits == 0 {
		postGenesisBits = genesisBits
	}
	if len(recent) < consensus.DGWv3PastBlocks {
		return postGenesisBits
	}
	return consensus.DarkGravityWaveV3(recent[:consensus.DGWv3PastBlocks], int64(chaincfg.TargetSpacing.Seconds()), consensus.PowLimit, postGenesisBits)
}

func prependRecentEntry(recent []consensus.BlockWindowEntry, entry consensus.BlockWindowEntry, limit int) []consensus.BlockWindowEntry {
	out := make([]consensus.BlockWindowEntry, 0, limit)
	out = append(out, entry)
	out = append(out, recent...)
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

func medianTimeFromRecent(recent []consensus.BlockWindowEntry) uint32 {
	const mtpWindow = 11
	if len(recent) == 0 {
		return 0
	}
	count := len(recent)
	if count > mtpWindow {
		count = mtpWindow
	}
	times := make([]uint32, 0, count)
	for i := 0; i < count; i++ {
		times = append(times, recent[i].Time)
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	return times[len(times)/2]
}

func isCoinbase(tx *wire.MsgTx) bool {
	return len(tx.TxIn) == 1 && tx.TxIn[0].PreviousOutPoint.Hash.IsZero() && tx.TxIn[0].PreviousOutPoint.Index == math.MaxUint32
}

func IsFinalizedTx(tx *wire.MsgTx, height int32, blockTime uint32) bool {
	if tx.LockTime == 0 {
		return true
	}
	// Safe conversion: height is always >= 0 for valid blocks
	var threshold uint32
	if height < 0 {
		threshold = 0
	} else {
		threshold = uint32(height)
	}
	if tx.LockTime >= lockTimeThreshold {
		threshold = blockTime
	}
	if tx.LockTime < threshold {
		return true
	}
	for _, in := range tx.TxIn {
		if in.Sequence != math.MaxUint32 {
			return false
		}
	}
	return true
}

func OutPointKey(txid string, vout uint32) string {
	return fmt.Sprintf("%s:%d", txid, vout)
}

func (c *Chain) BlockByHash(hash string) (*wire.MsgBlock, *BlockIndex, error) {
	return c.store.LoadBlock(hash)
}

func (c *Chain) HasBlock(hash string) bool {
	_, _, err := c.store.LoadBlock(hash)
	return err == nil
}

func (c *Chain) HeadersAfter(locator []chainhash.Hash, stop chainhash.Hash, max int) ([]wire.BlockHeader, error) {
	if max <= 0 {
		return nil, nil
	}
	if max > wire.MaxHeadersPerMessage {
		max = wire.MaxHeadersPerMessage
	}
	c.mu.RLock()
	tip := c.tip
	c.mu.RUnlock()
	if tip == nil || tip.Hash == "" {
		return nil, nil
	}

	startHeight := int32(-1)
	matchedLocator := len(locator) == 0
	for _, hash := range locator {
		_, idx, err := c.store.LoadBlock(hash.String())
		if err == nil {
			startHeight = idx.Height
			matchedLocator = true
			break
		}
	}
	if !matchedLocator {
		// The peer is asking from a locator we do not know. Returning our
		// genesis header would create a non-connecting stale header batch for
		// an ahead peer. Ask the peer to retry with a common locator instead.
		return nil, nil
	}

	headers := make([]wire.BlockHeader, 0, max)
	for height := startHeight + 1; height <= tip.Height && len(headers) < max; height++ {
		idx, err := c.store.LoadIndexByHeight(height)
		if err != nil {
			return nil, err
		}
		block, _, err := c.store.LoadBlock(idx.Hash)
		if err != nil {
			return nil, err
		}
		headers = append(headers, block.Header)
		hash, err := c.HashHeader(block.Header)
		if err != nil {
			return nil, err
		}
		if !stop.IsZero() && hash == stop {
			break
		}
	}
	return headers, nil
}

func (c *Chain) IndexByHeight(height int32) (*BlockIndex, error) {
	return c.store.LoadIndexByHeight(height)
}

func (c *Chain) UTXOStats() (UTXOStats, error) {
	return c.store.UTXOStats()
}

func (c *Chain) UTXO(txid string, vout uint32) (*UTXOEntry, error) {
	return c.store.LoadUTXO(OutPointKey(txid, vout))
}

func (c *Chain) ListUTXO() ([]UTXOEntry, error) {
	return c.store.ListUTXO()
}

func (c *Chain) Locator() []chainhash.Hash {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.tip == nil || c.tip.Hash == "" {
		return nil
	}
	hash, err := chainhash.FromString(c.tip.Hash)
	if err != nil {
		return nil
	}
	return []chainhash.Hash{hash}
}

func (c *Chain) Tip() *BlockIndex {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.tip == nil {
		return nil
	}
	cp := *c.tip
	return &cp
}

func (c *Chain) Params() chaincfg.Params {
	return c.params
}

func (c *Chain) TipChainwork() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.tip == nil || c.tip.Hash == "" {
		return "0"
	}
	if w, ok := c.workByHash[c.tip.Hash]; ok && w != nil {
		return w.Text(10)
	}
	return ""
}

func (c *Chain) TxIndexEnabled() bool {
	if idx, ok := c.store.(txIndexStore); ok {
		return idx.TxIndexEnabled()
	}
	return false
}

func (c *Chain) AddressIndexEnabled() bool {
	if idx, ok := c.store.(addressIndexStore); ok {
		return idx.AddressIndexEnabled()
	}
	return false
}

func (c *Chain) LookupTransactionByIndex(txid string) (*wire.MsgTx, *BlockIndex, int, error) {
	idxStore, ok := c.store.(txIndexStore)
	if !ok || !idxStore.TxIndexEnabled() {
		return nil, nil, -1, os.ErrNotExist
	}
	rec, err := idxStore.LookupTxIndex(txid)
	if err != nil {
		return nil, nil, -1, err
	}
	block, idx, err := c.store.LoadBlock(rec.BlockHash)
	if err != nil {
		return nil, nil, -1, err
	}
	if rec.TxPosition < 0 || rec.TxPosition >= len(block.Transactions) {
		return nil, nil, -1, os.ErrNotExist
	}
	return block.Transactions[rec.TxPosition], idx, rec.TxPosition, nil
}

func (c *Chain) AddressTxIDs(address string) ([]string, error) {
	idxStore, ok := c.store.(addressIndexStore)
	if !ok || !idxStore.AddressIndexEnabled() {
		return nil, os.ErrNotExist
	}
	return idxStore.AddressTxIDs(address)
}

func (c *Chain) AddressUTXOs(address string) ([]AddressIndexUTXO, error) {
	idxStore, ok := c.store.(addressIndexStore)
	if !ok || !idxStore.AddressIndexEnabled() {
		return nil, os.ErrNotExist
	}
	return idxStore.AddressUTXOs(address)
}

func (c *Chain) AddressBalance(address string) (int64, int64, error) {
	idxStore, ok := c.store.(addressIndexStore)
	if !ok || !idxStore.AddressIndexEnabled() {
		return 0, 0, os.ErrNotExist
	}
	return idxStore.AddressBalance(address)
}

func (c *Chain) AddressHistory(address string) ([]AddressHistoryEntry, error) {
	idxStore, ok := c.store.(addressIndexStore)
	if !ok || !idxStore.AddressIndexEnabled() {
		return nil, os.ErrNotExist
	}
	return idxStore.AddressHistory(address)
}

func (c *Chain) OrphanCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.orphanByHash)
}

func (c *Chain) DisconnectTip() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.disconnectTipLocked()
}

func (c *Chain) disconnectTipLocked() error {
	if c.tip == nil || c.tip.Hash == "" || c.tip.Height <= 0 {
		return fmt.Errorf("cannot disconnect tip at height <= 0")
	}
	idx := *c.tip
	block, _, err := c.store.LoadBlock(idx.Hash)
	if err != nil {
		return err
	}
	undo, err := c.store.LoadUndo(idx.Hash)
	if err != nil {
		return err
	}
	prevIdx, err := c.store.LoadIndexByHeight(idx.Height - 1)
	if err != nil {
		return err
	}
	if prevIdx.Hash != block.Header.PrevBlock.String() {
		return fmt.Errorf("previous index mismatch")
	}
	if err := c.store.DisconnectBlock(idx.Hash, prevIdx, *undo); err != nil {
		return err
	}
	c.tip = prevIdx
	return nil
}

type StorageHealth struct {
	OK                    bool   `json:"ok"`
	TipHeight             int32  `json:"tip_height"`
	TipHash               string `json:"tip_hash"`
	BestBlockReadable     bool   `json:"bestblock_readable"`
	HeightIndexReadable   bool   `json:"height_index_readable"`
	HeightIndexMatchesTip bool   `json:"height_index_matches_tip"`
	ChainworkReadable     bool   `json:"chainwork_readable"`
	ChainworkAtTip        string `json:"chainwork_at_tip,omitempty"`
	UTXOStatsReadable     bool   `json:"utxo_stats_readable"`
	Error                 string `json:"error,omitempty"`
}

type heightIndexRepairer interface {
	RepairHeightIndex() error
}

type fullIndexRepairer interface {
	RepairIndexes() error
}

func (c *Chain) StorageHealth() StorageHealth {
	c.mu.RLock()
	tip := c.tip
	c.mu.RUnlock()
	h := StorageHealth{OK: true, TipHeight: -1}
	if tip == nil {
		return h
	}
	h.TipHeight = tip.Height
	h.TipHash = tip.Hash
	if tip.Hash == "" {
		return h
	}
	if _, idx, err := c.store.LoadBlock(tip.Hash); err == nil && idx != nil {
		h.BestBlockReadable = true
	} else if err != nil {
		h.OK = false
		h.Error = err.Error()
	}
	if idx, err := c.store.LoadIndexByHeight(tip.Height); err == nil && idx != nil {
		h.HeightIndexReadable = true
		h.HeightIndexMatchesTip = idx.Hash == tip.Hash
		if work, ok := parseChainwork(idx.ChainWork); ok {
			h.ChainworkReadable = true
			h.ChainworkAtTip = work.Text(10)
		} else {
			total := big.NewInt(0)
			ok := true
			for height := int32(0); height <= tip.Height; height++ {
				hidx, err := c.store.LoadIndexByHeight(height)
				if err != nil {
					ok = false
					break
				}
				total = new(big.Int).Add(total, consensus.WorkForBits(hidx.Bits))
			}
			if ok {
				h.ChainworkReadable = true
				h.ChainworkAtTip = total.Text(10)
			}
		}
		if !h.HeightIndexMatchesTip {
			h.OK = false
			if h.Error == "" {
				h.Error = "height index does not match active tip"
			}
		}
	} else if err != nil {
		h.OK = false
		if h.Error == "" {
			h.Error = err.Error()
		}
	}
	if _, err := c.store.UTXOStats(); err == nil {
		h.UTXOStatsReadable = true
	} else {
		h.OK = false
		if h.Error == "" {
			h.Error = err.Error()
		}
	}
	if !h.BestBlockReadable || !h.HeightIndexReadable || !h.HeightIndexMatchesTip || !h.UTXOStatsReadable || !h.ChainworkReadable {
		h.OK = false
	}
	return h
}

func (c *Chain) ReindexActiveChain() (map[string]any, error) {
	c.mu.Lock()
	tip := c.tip

	if repairer, ok := c.store.(fullIndexRepairer); ok {
		if err := repairer.RepairIndexes(); err != nil {
			c.mu.Unlock()
			return nil, err
		}
	} else if repairer, ok := c.store.(heightIndexRepairer); ok {
		if err := repairer.RepairHeightIndex(); err != nil {
			c.mu.Unlock()
			return nil, err
		}
	}
	if err := c.rebuildActiveChainworkLocked(); err != nil {
		c.mu.Unlock()
		return nil, err
	}
	c.mu.Unlock()

	health := c.StorageHealth()
	result := map[string]any{
		"ok":              health.OK,
		"tip_height":      health.TipHeight,
		"tip_hash":        health.TipHash,
		"height_index_ok": health.HeightIndexReadable && health.HeightIndexMatchesTip,
		"chainwork_ok":    health.ChainworkReadable,
		"chainwork":       health.ChainworkAtTip,
		"note":            "active-chain indexes rebuilt from stored tip",
	}
	if tip != nil {
		result["active_tip_height_before"] = tip.Height
		result["active_tip_hash_before"] = tip.Hash
	}
	if !health.OK && health.Error != "" {
		result["warning"] = health.Error
	}
	return result, nil
}
