package wire

import (
	"encoding/binary"
	"fmt"
	"io"
)

func ReadVarInt(r io.Reader) (uint64, error) {
	var prefix [1]byte
	if _, err := io.ReadFull(r, prefix[:]); err != nil {
		return 0, err
	}
	switch prefix[0] {
	case 0xff:
		var v uint64
		err := binary.Read(r, binary.LittleEndian, &v)
		return v, err
	case 0xfe:
		var v uint32
		err := binary.Read(r, binary.LittleEndian, &v)
		return uint64(v), err
	case 0xfd:
		var v uint16
		err := binary.Read(r, binary.LittleEndian, &v)
		return uint64(v), err
	default:
		return uint64(prefix[0]), nil
	}
}

func WriteVarInt(w io.Writer, val uint64) error {
	switch {
	case val < 0xfd:
		_, err := w.Write([]byte{byte(val)})
		return err
	case val <= 0xffff:
		if _, err := w.Write([]byte{0xfd}); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, uint16(val))
	case val <= 0xffffffff:
		if _, err := w.Write([]byte{0xfe}); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, uint32(val))
	default:
		if _, err := w.Write([]byte{0xff}); err != nil {
			return err
		}
		return binary.Write(w, binary.LittleEndian, val)
	}
}

func ReadVarBytes(r io.Reader, max uint64, fieldName string) ([]byte, error) {
	n, err := ReadVarInt(r)
	if err != nil {
		return nil, err
	}
	if n > max {
		return nil, fmt.Errorf("%s length %d exceeds max %d", fieldName, n, max)
	}
	b := make([]byte, n)
	_, err = io.ReadFull(r, b)
	return b, err
}

func WriteVarBytes(w io.Writer, b []byte) error {
	if err := WriteVarInt(w, uint64(len(b))); err != nil {
		return err
	}
	_, err := w.Write(b)
	return err
}
