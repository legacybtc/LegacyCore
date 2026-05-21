package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"legacycoin/legacy-go/internal/chainhash"
)

const MaxBlockTransactions = 1_000_000

type BlockHeader struct {
	Version    int32
	PrevBlock  chainhash.Hash
	MerkleRoot chainhash.Hash
	Timestamp  uint32
	Bits       uint32
	Nonce      uint32
}

type MsgBlock struct {
	Header       BlockHeader
	Transactions []*MsgTx
}

func (h *BlockHeader) Serialize(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, h.Version); err != nil {
		return err
	}
	if _, err := w.Write(h.PrevBlock[:]); err != nil {
		return err
	}
	if _, err := w.Write(h.MerkleRoot[:]); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, h.Timestamp); err != nil {
		return err
	}
	if err := binary.Write(w, binary.LittleEndian, h.Bits); err != nil {
		return err
	}
	return binary.Write(w, binary.LittleEndian, h.Nonce)
}

func ReadBlockHeader(r io.Reader) (BlockHeader, error) {
	var h BlockHeader
	if err := binary.Read(r, binary.LittleEndian, &h.Version); err != nil {
		return h, err
	}
	if _, err := io.ReadFull(r, h.PrevBlock[:]); err != nil {
		return h, err
	}
	if _, err := io.ReadFull(r, h.MerkleRoot[:]); err != nil {
		return h, err
	}
	if err := binary.Read(r, binary.LittleEndian, &h.Timestamp); err != nil {
		return h, err
	}
	if err := binary.Read(r, binary.LittleEndian, &h.Bits); err != nil {
		return h, err
	}
	if err := binary.Read(r, binary.LittleEndian, &h.Nonce); err != nil {
		return h, err
	}
	return h, nil
}

func (h *BlockHeader) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := h.Serialize(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (h *BlockHeader) Hash() (chainhash.Hash, error) {
	b, err := h.Bytes()
	if err != nil {
		return chainhash.Hash{}, err
	}
	return chainhash.DoubleHashB(b), nil
}

func (b *MsgBlock) Serialize(w io.Writer) error {
	if err := b.Header.Serialize(w); err != nil {
		return err
	}
	if err := WriteVarInt(w, uint64(len(b.Transactions))); err != nil {
		return err
	}
	for _, tx := range b.Transactions {
		if err := tx.Serialize(w); err != nil {
			return err
		}
	}
	return nil
}

func ReadBlock(r io.Reader) (*MsgBlock, error) {
	header, err := ReadBlockHeader(r)
	if err != nil {
		return nil, err
	}
	txCount, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if txCount > MaxBlockTransactions {
		return nil, fmt.Errorf("block tx count %d exceeds max %d", txCount, MaxBlockTransactions)
	}
	block := &MsgBlock{Header: header, Transactions: make([]*MsgTx, txCount)}
	for i := range block.Transactions {
		tx, err := ReadTx(r)
		if err != nil {
			return nil, err
		}
		block.Transactions[i] = tx
	}
	return block, nil
}

func (b *MsgBlock) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := b.Serialize(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (b *MsgBlock) BuildMerkleRoot() (chainhash.Hash, error) {
	if len(b.Transactions) == 0 {
		return chainhash.Hash{}, nil
	}
	hashes := make([]chainhash.Hash, len(b.Transactions))
	for i, tx := range b.Transactions {
		h, err := tx.TxHash()
		if err != nil {
			return chainhash.Hash{}, err
		}
		hashes[i] = h
	}
	for len(hashes) > 1 {
		if len(hashes)%2 == 1 {
			hashes = append(hashes, hashes[len(hashes)-1])
		}
		next := make([]chainhash.Hash, 0, len(hashes)/2)
		for i := 0; i < len(hashes); i += 2 {
			var pair [64]byte
			copy(pair[:32], hashes[i][:])
			copy(pair[32:], hashes[i+1][:])
			next = append(next, chainhash.DoubleHashB(pair[:]))
		}
		hashes = next
	}
	return hashes[0], nil
}
