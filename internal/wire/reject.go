package wire

import (
	"io"
)

const (
	RejectMalformed       = 0x01
	RejectInvalid         = 0x10
	RejectObsolete        = 0x11
	RejectDuplicate       = 0x12
	RejectNonstandard     = 0x40
	RejectDust            = 0x41
	RejectInsufficientFee = 0x42
	RejectCheckpoint      = 0x43
)

type Reject struct {
	Cmd    string
	Code   uint8
	Reason string
	Hash   [32]byte
}

func NewReject(cmd string, code uint8, reason string) *Reject {
	return &Reject{
		Cmd:    cmd,
		Code:   code,
		Reason: reason,
	}
}

func NewRejectWithHash(cmd string, code uint8, reason string, hash [32]byte) *Reject {
	return &Reject{
		Cmd:    cmd,
		Code:   code,
		Reason: reason,
		Hash:   hash,
	}
}

func (r *Reject) Bytes() ([]byte, error) {
	hasHash := false
	for _, b := range r.Hash {
		if b != 0 {
			hasHash = true
			break
		}
	}
	buf := make([]byte, 0, 200)
	buf = append(buf, byte(len(r.Cmd)))
	buf = append(buf, []byte(r.Cmd)...)
	buf = append(buf, r.Code)
	buf = append(buf, byte(len(r.Reason)))
	buf = append(buf, []byte(r.Reason)...)
	if hasHash {
		buf = append(buf, r.Hash[:]...)
	}
	return buf, nil
}

func ReadReject(r io.Reader) (*Reject, error) {
	result := &Reject{}
	var cmdLen [1]byte
	if _, err := io.ReadFull(r, cmdLen[:]); err != nil {
		return nil, err
	}
	cmd := make([]byte, cmdLen[0])
	if _, err := io.ReadFull(r, cmd); err != nil {
		return nil, err
	}
	result.Cmd = string(cmd)
	if _, err := io.ReadFull(r, cmdLen[:]); err != nil {
		return nil, err
	}
	result.Code = cmdLen[0]
	if _, err := io.ReadFull(r, cmdLen[:]); err != nil {
		return nil, err
	}
	reason := make([]byte, cmdLen[0])
	if _, err := io.ReadFull(r, reason); err != nil {
		return nil, err
	}
	result.Reason = string(reason)
	// Hash is optional per BIP 61 (omitted for non-block/tx rejects).
	// Attempt to read the 32-byte hash, but it may not be present.
	var hashBuf [32]byte
	if n, err := io.ReadFull(r, hashBuf[:]); err == nil && n == 32 {
		result.Hash = hashBuf
	}
	return result, nil
}
