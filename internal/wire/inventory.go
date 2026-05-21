package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"

	"legacycoin/legacy-go/internal/chainhash"
)

const (
	InvTypeError uint32 = iota
	InvTypeTx
	InvTypeBlock

	MaxInvPerMessage       = 50_000
	MaxBlockLocatorHashes  = 500
	MaxHeadersPerMessage   = 2_000
	MaxGetBlocksStopHashes = 1
)

type InvVect struct {
	Type uint32
	Hash chainhash.Hash
}

func WriteInvPayload(w io.Writer, inv []InvVect) error {
	if len(inv) > MaxInvPerMessage {
		return fmt.Errorf("inventory count %d exceeds max %d", len(inv), MaxInvPerMessage)
	}
	if err := WriteVarInt(w, uint64(len(inv))); err != nil {
		return err
	}
	for _, v := range inv {
		if err := binary.Write(w, binary.LittleEndian, v.Type); err != nil {
			return err
		}
		if _, err := w.Write(v.Hash[:]); err != nil {
			return err
		}
	}
	return nil
}

func ReadInvPayload(r io.Reader) ([]InvVect, error) {
	count, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if count > MaxInvPerMessage {
		return nil, fmt.Errorf("inventory count %d exceeds max %d", count, MaxInvPerMessage)
	}
	inv := make([]InvVect, count)
	for i := range inv {
		if err := binary.Read(r, binary.LittleEndian, &inv[i].Type); err != nil {
			return nil, err
		}
		if _, err := io.ReadFull(r, inv[i].Hash[:]); err != nil {
			return nil, err
		}
	}
	return inv, nil
}

func InvPayload(inv []InvVect) ([]byte, error) {
	var buf bytes.Buffer
	if err := WriteInvPayload(&buf, inv); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

type GetBlocks struct {
	Version int32
	Locator []chainhash.Hash
	Stop    chainhash.Hash
}

func (g GetBlocks) Bytes() ([]byte, error) {
	if len(g.Locator) > MaxBlockLocatorHashes {
		return nil, fmt.Errorf("locator count %d exceeds max %d", len(g.Locator), MaxBlockLocatorHashes)
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, g.Version); err != nil {
		return nil, err
	}
	if err := WriteVarInt(&buf, uint64(len(g.Locator))); err != nil {
		return nil, err
	}
	for _, h := range g.Locator {
		if _, err := buf.Write(h[:]); err != nil {
			return nil, err
		}
	}
	if _, err := buf.Write(g.Stop[:]); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func ReadGetBlocks(r io.Reader) (GetBlocks, error) {
	var g GetBlocks
	if err := binary.Read(r, binary.LittleEndian, &g.Version); err != nil {
		return g, err
	}
	count, err := ReadVarInt(r)
	if err != nil {
		return g, err
	}
	if count > MaxBlockLocatorHashes {
		return g, fmt.Errorf("locator count %d exceeds max %d", count, MaxBlockLocatorHashes)
	}
	g.Locator = make([]chainhash.Hash, count)
	for i := range g.Locator {
		if _, err := io.ReadFull(r, g.Locator[i][:]); err != nil {
			return g, err
		}
	}
	if _, err := io.ReadFull(r, g.Stop[:]); err != nil {
		return g, err
	}
	return g, nil
}

func HeadersPayload(headers []BlockHeader) ([]byte, error) {
	if len(headers) > MaxHeadersPerMessage {
		return nil, fmt.Errorf("header count %d exceeds max %d", len(headers), MaxHeadersPerMessage)
	}
	var buf bytes.Buffer
	if err := WriteVarInt(&buf, uint64(len(headers))); err != nil {
		return nil, err
	}
	for _, header := range headers {
		if err := header.Serialize(&buf); err != nil {
			return nil, err
		}
		if err := WriteVarInt(&buf, 0); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func ReadHeadersPayload(r io.Reader) ([]BlockHeader, error) {
	count, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if count > MaxHeadersPerMessage {
		return nil, fmt.Errorf("header count %d exceeds max %d", count, MaxHeadersPerMessage)
	}
	headers := make([]BlockHeader, count)
	for i := range headers {
		header, err := ReadBlockHeader(r)
		if err != nil {
			return nil, err
		}
		txCount, err := ReadVarInt(r)
		if err != nil {
			return nil, err
		}
		if txCount != 0 {
			return nil, fmt.Errorf("headers message item has tx count %d", txCount)
		}
		headers[i] = header
	}
	return headers, nil
}
