package wire

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const MaxAddrPerMessage = 1_000

type NetAddress struct {
	Timestamp uint32
	Services  uint64
	IP        net.IP
	Port      uint16
}

func WriteAddrPayload(w io.Writer, addrs []NetAddress) error {
	if len(addrs) > MaxAddrPerMessage {
		return fmt.Errorf("address count %d exceeds max %d", len(addrs), MaxAddrPerMessage)
	}
	if err := WriteVarInt(w, uint64(len(addrs))); err != nil {
		return err
	}
	for _, addr := range addrs {
		ip := addr.IP.To16()
		if ip == nil {
			return fmt.Errorf("invalid peer address ip %q", addr.IP.String())
		}
		if err := binary.Write(w, binary.LittleEndian, addr.Timestamp); err != nil {
			return err
		}
		if err := binary.Write(w, binary.LittleEndian, addr.Services); err != nil {
			return err
		}
		if _, err := w.Write(ip); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, addr.Port); err != nil {
			return err
		}
	}
	return nil
}

func ReadAddrPayload(r io.Reader) ([]NetAddress, error) {
	count, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if count > MaxAddrPerMessage {
		return nil, fmt.Errorf("address count %d exceeds max %d", count, MaxAddrPerMessage)
	}
	addrs := make([]NetAddress, count)
	for i := range addrs {
		if err := binary.Read(r, binary.LittleEndian, &addrs[i].Timestamp); err != nil {
			return nil, err
		}
		if err := binary.Read(r, binary.LittleEndian, &addrs[i].Services); err != nil {
			return nil, err
		}
		var ip [16]byte
		if _, err := io.ReadFull(r, ip[:]); err != nil {
			return nil, err
		}
		addrs[i].IP = net.IP(ip[:])
		if err := binary.Read(r, binary.BigEndian, &addrs[i].Port); err != nil {
			return nil, err
		}
	}
	return addrs, nil
}

func AddrPayload(addrs []NetAddress) ([]byte, error) {
	var buf bytes.Buffer
	if err := WriteAddrPayload(&buf, addrs); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
