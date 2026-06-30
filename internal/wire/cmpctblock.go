package wire

import (
	"encoding/binary"
	"fmt"
	"io"

	"legacycoin/legacy-go/internal/chainhash"
)

const (
	CmpctBlockVersion  = 1
	ShortIDSize        = 6
	maxCmpctShortIDs   = 100_000
	maxCmpctTxs        = 100_000
	maxBlockTxnIndexes = 100_000
)

type PrefilledTransaction struct {
	Index uint64
	Tx    *MsgTx
}

type MsgCmpctBlock struct {
	Header       BlockHeader
	Nonce        uint64
	ShortIDs     [][ShortIDSize]byte
	PrefilledTxs []PrefilledTransaction
}

func (m *MsgCmpctBlock) Serialize(w io.Writer) error {
	if err := m.Header.Serialize(w); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, m.Nonce); err != nil {
		return err
	}
	if err := WriteVarInt(w, uint64(len(m.ShortIDs))); err != nil {
		return err
	}
	for _, sid := range m.ShortIDs {
		if _, err := w.Write(sid[:]); err != nil {
			return err
		}
	}
	if err := WriteVarInt(w, uint64(len(m.PrefilledTxs))); err != nil {
		return err
	}
	prevIndex := uint64(0)
	for _, pt := range m.PrefilledTxs {
		diff := pt.Index - prevIndex
		if err := WriteVarInt(w, diff); err != nil {
			return err
		}
		if err := pt.Tx.Serialize(w); err != nil {
			return err
		}
		prevIndex = pt.Index
	}
	return nil
}

func (m *MsgCmpctBlock) Deserialize(r io.Reader) error {
	header, err := ReadBlockHeader(r)
	if err != nil {
		return err
	}
	m.Header = header
	if err := binary.Read(r, binary.LittleEndian, &m.Nonce); err != nil {
		return err
	}
	shortCount, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if shortCount > maxCmpctShortIDs {
		return fmt.Errorf("cmpctblock short id count %d exceeds max %d", shortCount, maxCmpctShortIDs)
	}
	m.ShortIDs = make([][ShortIDSize]byte, shortCount)
	for i := uint64(0); i < shortCount; i++ {
		if _, err := io.ReadFull(r, m.ShortIDs[i][:]); err != nil {
			return err
		}
	}
	txCount, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if txCount > maxCmpctTxs {
		return fmt.Errorf("cmpctblock prefilled tx count %d exceeds max %d", txCount, maxCmpctTxs)
	}
	m.PrefilledTxs = make([]PrefilledTransaction, txCount)
	prevIndex := uint64(0)
	for i := uint64(0); i < txCount; i++ {
		diff, err := ReadVarInt(r)
		if err != nil {
			return err
		}
		prevIndex += diff
		m.PrefilledTxs[i].Index = prevIndex
		tx, err := ReadTx(r)
		if err != nil {
			return err
		}
		m.PrefilledTxs[i].Tx = tx
	}
	return nil
}

type MsgGetBlockTxn struct {
	BlockHash chainhash.Hash
	Indexes   []uint64
}

func (m *MsgGetBlockTxn) Serialize(w io.Writer) error {
	if _, err := w.Write(m.BlockHash[:]); err != nil {
		return err
	}
	if err := WriteVarInt(w, uint64(len(m.Indexes))); err != nil {
		return err
	}
	for _, idx := range m.Indexes {
		if err := WriteVarInt(w, idx); err != nil {
			return err
		}
	}
	return nil
}

func (m *MsgGetBlockTxn) Deserialize(r io.Reader) error {
	if _, err := io.ReadFull(r, m.BlockHash[:]); err != nil {
		return err
	}
	count, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if count > maxBlockTxnIndexes {
		return fmt.Errorf("getblocktxn index count %d exceeds max %d", count, maxBlockTxnIndexes)
	}
	m.Indexes = make([]uint64, count)
	for i := uint64(0); i < count; i++ {
		idx, err := ReadVarInt(r)
		if err != nil {
			return err
		}
		m.Indexes[i] = idx
	}
	return nil
}

type MsgBlockTxn struct {
	BlockHash    chainhash.Hash
	Transactions []*MsgTx
}

func (m *MsgBlockTxn) Serialize(w io.Writer) error {
	if _, err := w.Write(m.BlockHash[:]); err != nil {
		return err
	}
	if err := WriteVarInt(w, uint64(len(m.Transactions))); err != nil {
		return err
	}
	for _, tx := range m.Transactions {
		if err := tx.Serialize(w); err != nil {
			return err
		}
	}
	return nil
}

func (m *MsgBlockTxn) Deserialize(r io.Reader) error {
	if _, err := io.ReadFull(r, m.BlockHash[:]); err != nil {
		return err
	}
	count, err := ReadVarInt(r)
	if err != nil {
		return err
	}
	if count > maxCmpctTxs {
		return fmt.Errorf("blocktxn tx count %d exceeds max %d", count, maxCmpctTxs)
	}
	m.Transactions = make([]*MsgTx, count)
	for i := uint64(0); i < count; i++ {
		tx, err := ReadTx(r)
		if err != nil {
			return err
		}
		m.Transactions[i] = tx
	}
	return nil
}

type MsgSendCmpct struct {
	Announce bool
	Version  uint64
}

func (m *MsgSendCmpct) Serialize(w io.Writer) error {
	flags := byte(0)
	if m.Announce {
		flags |= 1
	}
	if _, err := w.Write([]byte{flags}); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, m.Version)
}

func (m *MsgSendCmpct) Deserialize(r io.Reader) error {
	var flags [1]byte
	if _, err := io.ReadFull(r, flags[:]); err != nil {
		return err
	}
	m.Announce = flags[0]&1 != 0
	return binary.Read(r, binary.LittleEndian, &m.Version)
}

func ShortID(key [16]byte, txid *chainhash.Hash) [ShortIDSize]byte {
	var sid [ShortIDSize]byte
	siphash := siphash24(key[:], txid[:])
	for i := 0; i < ShortIDSize; i++ {
		sid[i] = byte(siphash >> (8 * i))
	}
	return sid
}

func siphash24(key []byte, data []byte) uint64 {
	k0 := binary.LittleEndian.Uint64(key[0:8])
	k1 := binary.LittleEndian.Uint64(key[8:16])

	v0 := k0 ^ 0x736f6d6570736575
	v1 := k1 ^ 0x646f72616e646f6d
	v2 := k0 ^ 0x6c7967656e657261
	v3 := k1 ^ 0x7465646279746573

	blocks := len(data) / 8
	for i := 0; i < blocks; i++ {
		m := binary.LittleEndian.Uint64(data[i*8:])
		v3 ^= m
		for j := 0; j < 2; j++ {
			v0 += v1
			v1 = rotl64(v1, 13)
			v1 ^= v0
			v0 = rotl64(v0, 32)
			v2 += v3
			v3 = rotl64(v3, 16)
			v3 ^= v2
			v0 += v3
			v3 = rotl64(v3, 21)
			v3 ^= v0
			v2 += v1
			v1 = rotl64(v1, 17)
			v1 ^= v2
			v2 = rotl64(v2, 32)
		}
		v0 ^= m
	}

	tail := data[len(data)/8*8:]
	var last [8]byte
	copy(last[:], tail)
	last[7] = byte(len(data) % 256)

	v3 ^= binary.LittleEndian.Uint64(last[:])
	for j := 0; j < 2; j++ {
		v0 += v1
		v1 = rotl64(v1, 13)
		v1 ^= v0
		v0 = rotl64(v0, 32)
		v2 += v3
		v3 = rotl64(v3, 16)
		v3 ^= v2
		v0 += v3
		v3 = rotl64(v3, 21)
		v3 ^= v0
		v2 += v1
		v1 = rotl64(v1, 17)
		v1 ^= v2
		v2 = rotl64(v2, 32)
	}
	v0 ^= 0xff
	v2 ^= 0xff
	for j := 0; j < 4; j++ {
		v0 += v1
		v1 = rotl64(v1, 13)
		v1 ^= v0
		v0 = rotl64(v0, 32)
		v2 += v3
		v3 = rotl64(v3, 16)
		v3 ^= v2
		v0 += v3
		v3 = rotl64(v3, 21)
		v3 ^= v0
		v2 += v1
		v1 = rotl64(v1, 17)
		v1 ^= v2
		v2 = rotl64(v2, 32)
	}

	return v0 ^ v1 ^ v2 ^ v3
}

func rotl64(x uint64, b uint) uint64 {
	return (x << b) | (x >> (64 - b))
}
