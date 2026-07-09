package address

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"math/big"
)

const alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

var alphabetIndex = func() map[byte]int {
	m := make(map[byte]int, len(alphabet))
	for i := 0; i < len(alphabet); i++ {
		m[alphabet[i]] = i
	}
	return m
}()

var (
	ErrBadBase58Char = errors.New("bad base58 character")
	ErrBadChecksum   = errors.New("bad address checksum")
)

func EncodeBase58Check(version byte, payload []byte) string {
	data := make([]byte, 1+len(payload)+4)
	data[0] = version
	copy(data[1:], payload)
	sum := checksum(data[:1+len(payload)])
	copy(data[1+len(payload):], sum[:])
	return encodeBase58(data)
}

func DecodeBase58Check(s string) (byte, []byte, error) {
	data, err := decodeBase58(s)
	if err != nil {
		return 0, nil, err
	}
	if len(data) < 5 {
		return 0, nil, ErrBadChecksum
	}
	body := data[:len(data)-4]
	got := data[len(data)-4:]
	want := checksum(body)
	if !bytes.Equal(got, want[:]) {
		return 0, nil, ErrBadChecksum
	}
	payload := make([]byte, len(body)-1)
	copy(payload, body[1:])
	return body[0], payload, nil
}

func checksum(data []byte) [4]byte {
	first := sha256.Sum256(data)
	second := sha256.Sum256(first[:])
	var out [4]byte
	copy(out[:], second[:4])
	return out
}

func encodeBase58(data []byte) string {
	x := new(big.Int).SetBytes(data)
	base := big.NewInt(58)
	zero := big.NewInt(0)
	mod := new(big.Int)
	out := make([]byte, 0)
	for x.Cmp(zero) > 0 {
		x.DivMod(x, base, mod)
		out = append(out, alphabet[mod.Int64()])
	}
	for _, b := range data {
		if b != 0 {
			break
		}
		out = append(out, alphabet[0])
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

func decodeBase58(s string) ([]byte, error) {
	x := big.NewInt(0)
	base := big.NewInt(58)
	for i := 0; i < len(s); i++ {
		v, ok := alphabetIndex[s[i]]
		if !ok {
			return nil, ErrBadBase58Char
		}
		x.Mul(x, base)
		x.Add(x, big.NewInt(int64(v)))
	}
	out := x.Bytes()
	leading := 0
	for leading < len(s) && s[leading] == alphabet[0] {
		leading++
	}
	if leading > 0 {
		out = append(bytes.Repeat([]byte{0}, leading), out...)
	}
	return out, nil
}
