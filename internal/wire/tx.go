package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"legacycoin/legacy-go/internal/chainhash"
)

const (
	MaxTxInPerMessage  = 100_000
	MaxTxOutPerMessage = 100_000
	MaxScriptSize      = 10_000
)

type OutPoint struct {
	Hash  chainhash.Hash
	Index uint32
}

type TxIn struct {
	PreviousOutPoint OutPoint
	SignatureScript  []byte
	Sequence         uint32
}

type TxOut struct {
	Value    int64
	PkScript []byte
}

type MsgTx struct {
	Version  int32
	TxIn     []TxIn
	TxOut    []TxOut
	LockTime uint32
}

func (tx *MsgTx) Serialize(w io.Writer) error {
	if err := binary.Write(w, binary.LittleEndian, tx.Version); err != nil {
		return err
	}
	if err := WriteVarInt(w, uint64(len(tx.TxIn))); err != nil {
		return err
	}
	for _, in := range tx.TxIn {
		if _, err := w.Write(in.PreviousOutPoint.Hash[:]); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, in.PreviousOutPoint.Index); err != nil {
			return err
		}
		if err := WriteVarBytes(w, in.SignatureScript); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, in.Sequence); err != nil {
			return err
		}
	}
	if err := WriteVarInt(w, uint64(len(tx.TxOut))); err != nil {
		return err
	}
	for _, out := range tx.TxOut {
		if err := binary.Write(w, binary.LittleEndian, out.Value); err != nil {
			return err
		}
		if err := WriteVarBytes(w, out.PkScript); err != nil {
			return err
		}
	}
	return binary.Write(w, binary.LittleEndian, tx.LockTime)
}

func ReadTx(r io.Reader) (*MsgTx, error) {
	tx := &MsgTx{}
	if err := binary.Read(r, binary.LittleEndian, &tx.Version); err != nil {
		return nil, err
	}
	inCount, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if inCount > MaxTxInPerMessage {
		return nil, fmt.Errorf("tx input count %d exceeds max %d", inCount, MaxTxInPerMessage)
	}
	tx.TxIn = make([]TxIn, inCount)
	for i := range tx.TxIn {
		if _, err := io.ReadFull(r, tx.TxIn[i].PreviousOutPoint.Hash[:]); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &tx.TxIn[i].PreviousOutPoint.Index); err != nil {
			return nil, err
		}
		sig, err := ReadVarBytes(r, MaxScriptSize, "signature script")
		if err != nil {
			return nil, err
		}
		tx.TxIn[i].SignatureScript = sig
		if err := binary.Read(r, binary.LittleEndian, &tx.TxIn[i].Sequence); err != nil {
			return nil, err
		}
	}
	outCount, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if outCount > MaxTxOutPerMessage {
		return nil, fmt.Errorf("tx output count %d exceeds max %d", outCount, MaxTxOutPerMessage)
	}
	tx.TxOut = make([]TxOut, outCount)
	for i := range tx.TxOut {
		if err := binary.Read(r, binary.LittleEndian, &tx.TxOut[i].Value); err != nil {
			return nil, err
		}
		pk, err := ReadVarBytes(r, MaxScriptSize, "public key script")
		if err != nil {
			return nil, err
		}
		tx.TxOut[i].PkScript = pk
	}
	if err := binary.Read(r, binary.LittleEndian, &tx.LockTime); err != nil {
		return nil, err
	}
	return tx, nil
}

func (tx *MsgTx) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	if err := tx.Serialize(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func (tx *MsgTx) TxHash() (chainhash.Hash, error) {
	b, err := tx.Bytes()
	if err != nil {
		return chainhash.Hash{}, err
	}
	return chainhash.DoubleHashB(b), nil
}
