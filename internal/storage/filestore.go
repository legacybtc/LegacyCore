package storage

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/fsutil"
	"legacycoin/legacy-go/internal/wire"
)

type FileStore struct {
	dir string
}

type journalOp string

const (
	journalConnect    journalOp = "connect"
	journalDisconnect journalOp = "disconnect"
)

type storeJournal struct {
	Op           journalOp              `json:"op"`
	Index        blockchain.BlockIndex  `json:"index,omitempty"`
	PrevTip      *blockchain.BlockIndex `json:"prev_tip,omitempty"`
	Adds         []blockchain.UTXOEntry `json:"adds,omitempty"`
	Spends       []string               `json:"spends,omitempty"`
	SpentEntries []blockchain.UTXOEntry `json:"spent_entries,omitempty"`
	Undo         *blockchain.UndoData   `json:"undo,omitempty"`
}

func NewFileStore(dir string) *FileStore {
	return &FileStore{dir: dir}
}

func (s *FileStore) tipPath() string {
	return filepath.Join(s.dir, "chainstate.json")
}

func (s *FileStore) journalPath() string {
	return filepath.Join(s.dir, "chainstate.journal.json")
}

func (s *FileStore) blocksDir() string {
	return filepath.Join(s.dir, "blocks")
}

func (s *FileStore) indexDir() string {
	return filepath.Join(s.dir, "index")
}

func (s *FileStore) utxoDir() string {
	return filepath.Join(s.dir, "utxo")
}

func (s *FileStore) undoDir() string {
	return filepath.Join(s.dir, "undo")
}

func (s *FileStore) blockPath(hash string) string {
	return filepath.Join(s.blocksDir(), hash+".blk")
}

func (s *FileStore) hashIndexPath(hash string) string {
	return filepath.Join(s.indexDir(), "hash", hash+".json")
}

func (s *FileStore) heightIndexPath(height int32) string {
	return filepath.Join(s.indexDir(), "height", strconv.FormatInt(int64(height), 10)+".json")
}

func (s *FileStore) utxoPath(key string) string {
	return filepath.Join(s.utxoDir(), strings.ReplaceAll(key, ":", "_")+".json")
}

func (s *FileStore) undoPath(hash string) string {
	return filepath.Join(s.undoDir(), hash+".json")
}

func (s *FileStore) LoadTip() (*blockchain.BlockIndex, error) {
	if err := s.recoverJournal(); err != nil {
		return nil, err
	}
	b, err := os.ReadFile(s.tipPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var tip blockchain.BlockIndex
	if err := json.Unmarshal(b, &tip); err != nil {
		return nil, err
	}
	return &tip, nil
}

func (s *FileStore) SaveTip(tip blockchain.BlockIndex) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(tip, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.tipPath(), b, 0600)
}

func (s *FileStore) SaveBlock(block *wire.MsgBlock, idx blockchain.BlockIndex, adds []blockchain.UTXOEntry, spends []string, spentEntries []blockchain.UTXOEntry) error {
	blockBytes, err := block.Bytes()
	if err != nil {
		return err
	}
	if err := s.ensureDirs(); err != nil {
		return err
	}

	journal := storeJournal{
		Op:           journalConnect,
		Index:        idx,
		Adds:         adds,
		Spends:       spends,
		SpentEntries: spentEntries,
	}
	if err := s.writeJournal(journal); err != nil {
		return err
	}

	if err := fsutil.WriteFileAtomic(s.blockPath(idx.Hash), blockBytes, 0600); err != nil {
		return err
	}
	indexBytes, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(s.hashIndexPath(idx.Hash), indexBytes, 0600); err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(s.heightIndexPath(idx.Height), indexBytes, 0600); err != nil {
		return err
	}
	for _, key := range spends {
		if err := os.Remove(s.utxoPath(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for _, entry := range adds {
		if err := s.writeUTXO(entry); err != nil {
			return err
		}
	}
	undo := blockchain.UndoData{AddedKeys: make([]string, 0, len(adds)), Spent: spentEntries}
	for _, add := range adds {
		undo.AddedKeys = append(undo.AddedKeys, add.Key)
	}
	undoBytes, err := json.MarshalIndent(undo, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(s.undoPath(idx.Hash), undoBytes, 0600); err != nil {
		return err
	}
	if err := s.SaveTip(idx); err != nil {
		return err
	}
	return s.clearJournal()
}

func (s *FileStore) LoadUndo(hash string) (*blockchain.UndoData, error) {
	b, err := os.ReadFile(s.undoPath(hash))
	if err != nil {
		return nil, err
	}
	var undo blockchain.UndoData
	if err := json.Unmarshal(b, &undo); err != nil {
		return nil, err
	}
	return &undo, nil
}

func (s *FileStore) DisconnectBlock(hash string, prevTip *blockchain.BlockIndex, undo blockchain.UndoData) error {
	if prevTip == nil {
		return errors.New("nil previous tip")
	}
	if err := s.ensureDirs(); err != nil {
		return err
	}
	journal := storeJournal{
		Op:      journalDisconnect,
		Index:   blockchain.BlockIndex{Hash: hash, Height: prevTip.Height + 1},
		PrevTip: prevTip,
		Undo:    &undo,
	}
	if err := s.writeJournal(journal); err != nil {
		return err
	}
	if err := s.applyDisconnect(hash, prevTip, undo); err != nil {
		return err
	}
	return s.clearJournal()
}

func (s *FileStore) LoadBlock(hash string) (*wire.MsgBlock, *blockchain.BlockIndex, error) {
	idx, err := s.loadIndex(s.hashIndexPath(hash))
	if err != nil {
		return nil, nil, err
	}
	blockBytes, err := os.ReadFile(s.blockPath(hash))
	if err != nil {
		return nil, nil, err
	}
	block, err := wire.ReadBlock(bytes.NewReader(blockBytes))
	if err != nil {
		return nil, nil, err
	}
	return block, idx, nil
}

func (s *FileStore) LoadIndexByHeight(height int32) (*blockchain.BlockIndex, error) {
	idx, err := s.loadIndex(s.heightIndexPath(height))
	if err == nil {
		return idx, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	return s.repairHeightIndexFromTip(height)
}

func (s *FileStore) LoadUTXO(key string) (*blockchain.UTXOEntry, error) {
	b, err := os.ReadFile(s.utxoPath(key))
	if err != nil {
		return nil, err
	}
	var entry blockchain.UTXOEntry
	if err := json.Unmarshal(b, &entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

func (s *FileStore) UTXOStats() (blockchain.UTXOStats, error) {
	var stats blockchain.UTXOStats
	utxos, err := s.ListUTXO()
	if err != nil {
		return stats, err
	}
	for _, utxo := range utxos {
		stats.Count++
		stats.Total += utxo.Value
	}
	return stats, nil
}

func (s *FileStore) ListUTXO() ([]blockchain.UTXOEntry, error) {
	entries, err := os.ReadDir(s.utxoDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	out := make([]blockchain.UTXOEntry, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.utxoDir(), entry.Name()))
		if err != nil {
			return nil, err
		}
		var utxo blockchain.UTXOEntry
		if err := json.Unmarshal(b, &utxo); err != nil {
			return nil, err
		}
		out = append(out, utxo)
	}
	return out, nil
}

func (s *FileStore) ensureDirs() error {
	for _, dir := range []string{
		s.dir,
		s.blocksDir(),
		filepath.Dir(s.hashIndexPath("placeholder")),
		filepath.Dir(s.heightIndexPath(0)),
		s.utxoDir(),
		s.undoDir(),
	} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			return err
		}
	}
	return nil
}

func (s *FileStore) writeUTXO(entry blockchain.UTXOEntry) error {
	b, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.utxoPath(entry.Key), b, 0600)
}

func (s *FileStore) writeJournal(j storeJournal) error {
	if err := os.MkdirAll(s.dir, 0700); err != nil {
		return err
	}
	b, err := json.MarshalIndent(j, "", "  ")
	if err != nil {
		return err
	}
	return fsutil.WriteFileAtomic(s.journalPath(), b, 0600)
}

func (s *FileStore) clearJournal() error {
	if err := os.Remove(s.journalPath()); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *FileStore) recoverJournal() error {
	b, err := os.ReadFile(s.journalPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var j storeJournal
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	switch j.Op {
	case journalConnect:
		return s.rollbackConnect(j)
	case journalDisconnect:
		if j.PrevTip == nil || j.Undo == nil {
			return errors.New("invalid disconnect journal")
		}
		if err := s.applyDisconnect(j.Index.Hash, j.PrevTip, *j.Undo); err != nil {
			return err
		}
		return s.clearJournal()
	default:
		return errors.New("unknown chainstate journal operation")
	}
}

func (s *FileStore) rollbackConnect(j storeJournal) error {
	// If the tip already points to the journaled block, the operation committed;
	// only the stale journal needs to be removed.
	if tip, err := s.readTipNoRecover(); err == nil && tip != nil && tip.Hash == j.Index.Hash {
		return s.clearJournal()
	}
	for _, entry := range j.Adds {
		if err := os.Remove(s.utxoPath(entry.Key)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for _, entry := range j.SpentEntries {
		if err := s.writeUTXO(entry); err != nil {
			return err
		}
	}
	for _, path := range []string{
		s.blockPath(j.Index.Hash),
		s.hashIndexPath(j.Index.Hash),
		s.heightIndexPath(j.Index.Height),
		s.undoPath(j.Index.Hash),
	} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return s.clearJournal()
}

func (s *FileStore) applyDisconnect(hash string, prevTip *blockchain.BlockIndex, undo blockchain.UndoData) error {
	for _, key := range undo.AddedKeys {
		if err := os.Remove(s.utxoPath(key)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	for _, entry := range undo.Spent {
		if err := s.writeUTXO(entry); err != nil {
			return err
		}
	}
	// Keep block bytes, hash index, and undo records so disconnected blocks
	// remain available as side-chain history and can be reconsidered later.
	if err := os.Remove(s.heightIndexPath(prevTip.Height + 1)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return s.SaveTip(*prevTip)
}

func (s *FileStore) readTipNoRecover() (*blockchain.BlockIndex, error) {
	b, err := os.ReadFile(s.tipPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}
	var tip blockchain.BlockIndex
	if err := json.Unmarshal(b, &tip); err != nil {
		return nil, err
	}
	return &tip, nil
}

func (s *FileStore) repairHeightIndexFromTip(height int32) (*blockchain.BlockIndex, error) {
	if height < 0 {
		return nil, os.ErrNotExist
	}
	tip, err := s.readTipNoRecover()
	if err != nil {
		return nil, err
	}
	if tip == nil || tip.Height < height || tip.Hash == "" {
		return nil, os.ErrNotExist
	}

	curHash := tip.Hash
	for {
		block, idx, err := s.LoadBlock(curHash)
		if err != nil {
			return nil, err
		}
		idxBytes, err := json.MarshalIndent(idx, "", "  ")
		if err != nil {
			return nil, err
		}
		// Recreate active-chain height index entries as we walk back from tip.
		if err := fsutil.WriteFileAtomic(s.heightIndexPath(idx.Height), idxBytes, 0600); err != nil {
			return nil, err
		}
		if idx.Height == height {
			return idx, nil
		}
		if idx.Height == 0 {
			break
		}
		curHash = block.Header.PrevBlock.String()
	}
	return nil, os.ErrNotExist
}

func (s *FileStore) RepairHeightIndex() error {
	tip, err := s.readTipNoRecover()
	if err != nil {
		return err
	}
	if tip == nil || tip.Hash == "" {
		return nil
	}
	_, err = s.repairHeightIndexFromTip(0)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (s *FileStore) loadIndex(path string) (*blockchain.BlockIndex, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var idx blockchain.BlockIndex
	if err := json.Unmarshal(b, &idx); err != nil {
		return nil, err
	}
	return &idx, nil
}
