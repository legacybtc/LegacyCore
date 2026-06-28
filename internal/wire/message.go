package wire

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"

	"legacycoin/legacy-go/internal/chainhash"
)

const (
	CommandVersion    = "version"
	CommandVerAck     = "verack"
	CommandPing       = "ping"
	CommandPong       = "pong"
	CommandBlock      = "block"
	CommandTx         = "tx"
	CommandInv        = "inv"
	CommandGetData    = "getdata"
	CommandAddr       = "addr"
	CommandGetAddr    = "getaddr"
	CommandGetBlocks  = "getblocks"
	CommandGetHeaders = "getheaders"
	CommandHeaders    = "headers"
	CommandReject     = "reject"

	MaxMessagePayload = 32 * 1024 * 1024
)

var (
	ErrBadMessageMagic    = errors.New("bad message magic")
	ErrBadMessageChecksum = errors.New("bad message checksum")
)

type Message struct {
	Command string
	Payload []byte
}

func ReadMessage(r io.Reader, magic [4]byte) (Message, error) {
	var header [24]byte
	if _, err := io.ReadFull(r, header[:]); err != nil {
		return Message{}, err
	}
	if !bytes.Equal(header[:4], magic[:]) {
		return Message{}, ErrBadMessageMagic
	}
	command := strings.TrimRight(string(header[4:16]), "\x00")
	payloadLen := binary.LittleEndian.Uint32(header[16:20])
	if payloadLen > MaxMessagePayload {
		return Message{}, fmt.Errorf("message payload %d exceeds max %d", payloadLen, MaxMessagePayload)
	}
	payload := make([]byte, payloadLen)
	if _, err := io.ReadFull(r, payload); err != nil {
		return Message{}, err
	}
	checksum := chainhash.DoubleHashB(payload)
	if !bytes.Equal(header[20:24], checksum[:4]) {
		return Message{}, ErrBadMessageChecksum
	}
	return Message{Command: command, Payload: payload}, nil
}

func WriteMessage(w io.Writer, magic [4]byte, command string, payload []byte) error {
	if len(command) > 12 {
		return fmt.Errorf("command %q exceeds 12 bytes", command)
	}
	if len(payload) > MaxMessagePayload {
		return fmt.Errorf("message payload %d exceeds max %d", len(payload), MaxMessagePayload)
	}
	var header [24]byte
	copy(header[:4], magic[:])
	copy(header[4:16], []byte(command))
	binary.LittleEndian.PutUint32(header[16:20], uint32(len(payload)))
	checksum := chainhash.DoubleHashB(payload)
	copy(header[20:24], checksum[:4])
	if _, err := w.Write(header[:]); err != nil {
		return err
	}
	if len(payload) == 0 {
		return nil
	}
	_, err := w.Write(payload)
	return err
}
