package script

//lint:file-ignore SA1019 required for Bitcoin P2PKH address compatibility

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"golang.org/x/crypto/ripemd160" // #nosec

	"legacycoin/legacy-go/internal/address"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/pqc"
	"legacycoin/legacy-go/internal/wire"
)

const (
	OP_0             byte = 0x00
	OP_PUSHDATA1     byte = 0x4c
	OP_PUSHDATA2     byte = 0x4d
	OP_DATA_20       byte = 0x14
	OP_IF            byte = 0x63
	OP_NOTIF         byte = 0x64
	OP_ELSE          byte = 0x67
	OP_ENDIF         byte = 0x68
	OP_DUP           byte = 0x76
	OP_HASH160       byte = 0xa9
	OP_EQUAL         byte = 0x87
	OP_EQUALVERIFY   byte = 0x88
	OP_CHECKSIG      byte = 0xac
	OP_CHECKMULTISIG byte = 0xae
	OP_1             byte = 0x51
	OP_2             byte = 0x52
	OP_16            byte = 0x60

	SigHashAll     byte = 0x01
	MaxTxSigOps    int  = 4_000
	MaxBlockSigOps int  = 20_000
)

var (
	ErrUnsupportedScript  = errors.New("unsupported script")
	ErrBadSignatureScript = errors.New("bad signature script")
	ErrBadSignature       = errors.New("bad signature")
	ErrPubKeyHashMismatch = errors.New("public key hash mismatch")
	ErrMalformedScript    = errors.New("malformed script")
	ErrScriptEval         = errors.New("script evaluation failed")
)

type CoverageStatus struct {
	Implemented []string
	Pending     []string
	Percent     int
}

func Coverage() CoverageStatus {
	implemented := []string{
		"consensus_locktime_sequence_finality",
		"p2pk",
		"p2pkh",
		"p2sh_redeem_p2pk",
		"p2sh_redeem_p2pkh",
		"multisig_checkmultisig",
		"hybrid_pqc_p2pkh",
		"pushdata1_pushdata2",
		"sigops_counting_limits",
		"broader_standardness_templates",
		"control_flow_structure_validation",
		"non-push script evaluation vm",
		"control-flow execution semantics",
	}
	pending := []string{}
	total := len(implemented) + len(pending)
	percent := 0
	if total > 0 {
		percent = len(implemented) * 100 / total
	}
	return CoverageStatus{
		Implemented: implemented,
		Pending:     pending,
		Percent:     percent,
	}
}

func Hash160(b []byte) []byte {
	sha := sha256.Sum256(b)
	ripemd := ripemd160.New() // #nosec
	_, _ = ripemd.Write(sha[:])
	return ripemd.Sum(nil)
}

func PayToPubKeyHashScript(pubKeyHash []byte) ([]byte, error) {
	if len(pubKeyHash) != 20 {
		return nil, fmt.Errorf("pubkey hash length %d", len(pubKeyHash))
	}
	return []byte{OP_DUP, OP_HASH160, OP_DATA_20,
		pubKeyHash[0], pubKeyHash[1], pubKeyHash[2], pubKeyHash[3], pubKeyHash[4],
		pubKeyHash[5], pubKeyHash[6], pubKeyHash[7], pubKeyHash[8], pubKeyHash[9],
		pubKeyHash[10], pubKeyHash[11], pubKeyHash[12], pubKeyHash[13], pubKeyHash[14],
		pubKeyHash[15], pubKeyHash[16], pubKeyHash[17], pubKeyHash[18], pubKeyHash[19],
		OP_EQUALVERIFY, OP_CHECKSIG}, nil
}

func IsPayToPubKeyHash(pkScript []byte) bool {
	return len(pkScript) == 25 &&
		pkScript[0] == OP_DUP &&
		pkScript[1] == OP_HASH160 &&
		pkScript[2] == OP_DATA_20 &&
		pkScript[23] == OP_EQUALVERIFY &&
		pkScript[24] == OP_CHECKSIG
}

func PayToHybridPubKeyHashScript(hybridHash20 []byte) ([]byte, error) {
	if len(hybridHash20) != 20 {
		return nil, fmt.Errorf("hybrid hash length %d", len(hybridHash20))
	}
	// Custom template: OP_0 OP_HASH160 <20-byte-hash> OP_EQUALVERIFY OP_CHECKSIG
	return []byte{OP_0, OP_HASH160, OP_DATA_20,
		hybridHash20[0], hybridHash20[1], hybridHash20[2], hybridHash20[3], hybridHash20[4],
		hybridHash20[5], hybridHash20[6], hybridHash20[7], hybridHash20[8], hybridHash20[9],
		hybridHash20[10], hybridHash20[11], hybridHash20[12], hybridHash20[13], hybridHash20[14],
		hybridHash20[15], hybridHash20[16], hybridHash20[17], hybridHash20[18], hybridHash20[19],
		OP_EQUALVERIFY, OP_CHECKSIG}, nil
}

func IsPayToHybridPubKeyHash(pkScript []byte) bool {
	return len(pkScript) == 25 &&
		pkScript[0] == OP_0 &&
		pkScript[1] == OP_HASH160 &&
		pkScript[2] == OP_DATA_20 &&
		pkScript[23] == OP_EQUALVERIFY &&
		pkScript[24] == OP_CHECKSIG
}

func PayToScriptHashScript(scriptHash []byte) ([]byte, error) {
	if len(scriptHash) != 20 {
		return nil, fmt.Errorf("script hash length %d", len(scriptHash))
	}
	return []byte{OP_HASH160, OP_DATA_20,
		scriptHash[0], scriptHash[1], scriptHash[2], scriptHash[3], scriptHash[4],
		scriptHash[5], scriptHash[6], scriptHash[7], scriptHash[8], scriptHash[9],
		scriptHash[10], scriptHash[11], scriptHash[12], scriptHash[13], scriptHash[14],
		scriptHash[15], scriptHash[16], scriptHash[17], scriptHash[18], scriptHash[19],
		OP_EQUAL}, nil
}

func IsPayToScriptHash(pkScript []byte) bool {
	return len(pkScript) == 23 &&
		pkScript[0] == OP_HASH160 &&
		pkScript[1] == OP_DATA_20 &&
		pkScript[22] == OP_EQUAL
}

func IsPayToPubKey(pkScript []byte) bool {
	if len(pkScript) != 35 && len(pkScript) != 67 {
		return false
	}
	keyLen := int(pkScript[0])
	return (keyLen == 33 || keyLen == 65) && len(pkScript) == keyLen+2 && pkScript[len(pkScript)-1] == OP_CHECKSIG
}

func IsPayToMultiSig(pkScript []byte) bool {
	_, _, ok := parseMultiSigScript(pkScript)
	return ok
}

func MultiSigPubKeys(pkScript []byte) ([][]byte, bool) {
	_, pubKeys, ok := parseMultiSigScript(pkScript)
	if !ok {
		return nil, false
	}
	return pubKeys, true
}

func MultiSigParams(pkScript []byte) (m int, n int, ok bool) {
	m, pubKeys, ok := parseMultiSigScript(pkScript)
	if !ok {
		return 0, 0, false
	}
	return m, len(pubKeys), true
}

func VerifyInput(tx *wire.MsgTx, inputIndex int, prevPkScript []byte) error {
	switch {
	case IsPayToHybridPubKeyHash(prevPkScript):
		return verifyHybridP2PKH(tx, inputIndex, prevPkScript)
	case IsPayToPubKeyHash(prevPkScript):
		return verifyP2PKH(tx, inputIndex, prevPkScript)
	case IsPayToPubKey(prevPkScript):
		return verifyP2PK(tx, inputIndex, prevPkScript)
	case IsPayToMultiSig(prevPkScript):
		return verifyMultiSig(tx, inputIndex, prevPkScript)
	case IsPayToScriptHash(prevPkScript):
		return verifyP2SH(tx, inputIndex, prevPkScript)
	default:
		return ErrUnsupportedScript
	}
}

func verifyHybridP2PKH(tx *wire.MsgTx, inputIndex int, prevPkScript []byte) error {
	if inputIndex < 0 || inputIndex >= len(tx.TxIn) {
		return ErrBadSignatureScript
	}
	pushes, err := parsePushes(tx.TxIn[inputIndex].SignatureScript)
	if err != nil || len(pushes) != 4 {
		return ErrBadSignatureScript
	}
	ecdsaWithHashType := pushes[0]
	pqSig := pushes[1]
	secpPub := pushes[2]
	pqPub := pushes[3]
	if len(ecdsaWithHashType) < 2 || ecdsaWithHashType[len(ecdsaWithHashType)-1] != SigHashAll {
		return ErrBadSignature
	}
	addrStr := address.NewHybridAddress(secpPub, pqPub)
	hash20, err := address.DecodeHybridAddress(addrStr)
	if err != nil {
		return err
	}
	if !bytes.Equal(hash20, prevPkScript[3:23]) {
		return ErrPubKeyHashMismatch
	}
	hash, err := SignatureHash(tx, inputIndex, prevPkScript, SigHashAll)
	if err != nil {
		return err
	}
	pub, err := pqc.HybridPublicKeyFromBytes(pqc.HybridPublicBytes{
		SecpCompressed: secpPub,
		MLDSA65:        pqPub,
	})
	if err != nil {
		return err
	}
	sig := pqc.HybridSignature{
		ECDSADER: ecdsaWithHashType[:len(ecdsaWithHashType)-1],
		MLDSA65:  pqSig,
	}
	if !pub.Verify(hash[:], sig) {
		return ErrBadSignature
	}
	return nil
}

func verifyP2PKH(tx *wire.MsgTx, inputIndex int, prevPkScript []byte) error {
	if inputIndex < 0 || inputIndex >= len(tx.TxIn) {
		return ErrBadSignatureScript
	}
	sig, pubKeyBytes, err := parseP2PKHScriptSig(tx.TxIn[inputIndex].SignatureScript)
	if err != nil {
		return err
	}
	if len(sig) < 2 || sig[len(sig)-1] != SigHashAll {
		return ErrBadSignature
	}
	pubKeyHash := prevPkScript[3:23]
	if !bytes.Equal(Hash160(pubKeyBytes), pubKeyHash) {
		return ErrPubKeyHashMismatch
	}
	pubKey, err := btcec.ParsePubKey(pubKeyBytes)
	if err != nil {
		return err
	}
	parsedSig, err := btcecdsa.ParseDERSignature(sig[:len(sig)-1])
	if err != nil {
		return err
	}
	hash, err := SignatureHash(tx, inputIndex, prevPkScript, SigHashAll)
	if err != nil {
		return err
	}
	if !parsedSig.Verify(hash[:], pubKey) {
		return ErrBadSignature
	}
	return nil
}

func verifyP2PK(tx *wire.MsgTx, inputIndex int, prevPkScript []byte) error {
	if inputIndex < 0 || inputIndex >= len(tx.TxIn) {
		return ErrBadSignatureScript
	}
	sig, err := parseP2PKScriptSig(tx.TxIn[inputIndex].SignatureScript)
	if err != nil {
		return err
	}
	if len(sig) < 2 || sig[len(sig)-1] != SigHashAll {
		return ErrBadSignature
	}
	keyLen := int(prevPkScript[0])
	pubKey, err := btcec.ParsePubKey(prevPkScript[1 : 1+keyLen])
	if err != nil {
		return err
	}
	parsedSig, err := btcecdsa.ParseDERSignature(sig[:len(sig)-1])
	if err != nil {
		return err
	}
	hash, err := SignatureHash(tx, inputIndex, prevPkScript, SigHashAll)
	if err != nil {
		return err
	}
	if !parsedSig.Verify(hash[:], pubKey) {
		return ErrBadSignature
	}
	return nil
}

func verifyP2SH(tx *wire.MsgTx, inputIndex int, prevPkScript []byte) error {
	if inputIndex < 0 || inputIndex >= len(tx.TxIn) {
		return ErrBadSignatureScript
	}
	pushes, err := parsePushes(tx.TxIn[inputIndex].SignatureScript)
	if err != nil || len(pushes) < 1 {
		return ErrBadSignatureScript
	}
	redeem := pushes[len(pushes)-1]
	if !bytes.Equal(Hash160(redeem), prevPkScript[2:22]) {
		return ErrBadSignature
	}
	args := pushes[:len(pushes)-1]
	sigScript, err := encodePushes(args)
	if err != nil {
		return err
	}
	tmp := *tx
	tmp.TxIn = append([]wire.TxIn(nil), tx.TxIn...)
	tmp.TxIn[inputIndex] = tx.TxIn[inputIndex]
	tmp.TxIn[inputIndex].SignatureScript = sigScript
	switch {
	case IsPayToPubKeyHash(redeem):
		return verifyP2PKH(&tmp, inputIndex, redeem)
	case IsPayToPubKey(redeem):
		return verifyP2PK(&tmp, inputIndex, redeem)
	case IsPayToMultiSig(redeem):
		return verifyMultiSig(&tmp, inputIndex, redeem)
	default:
		return ErrUnsupportedScript
	}
}

func verifyMultiSig(tx *wire.MsgTx, inputIndex int, prevPkScript []byte) error {
	if inputIndex < 0 || inputIndex >= len(tx.TxIn) {
		return ErrBadSignatureScript
	}
	m, pubKeys, ok := parseMultiSigScript(prevPkScript)
	if !ok || m < 1 || len(pubKeys) < m {
		return ErrUnsupportedScript
	}
	pushes, err := parsePushes(tx.TxIn[inputIndex].SignatureScript)
	if err != nil || len(pushes) < 1 {
		return ErrBadSignatureScript
	}
	// CHECKMULTISIG bug compatibility: leading dummy push (typically OP_0).
	sigItems := pushes
	if len(sigItems) > 0 && len(sigItems[0]) == 0 {
		sigItems = sigItems[1:]
	}
	if len(sigItems) < m || len(sigItems) > len(pubKeys) {
		return ErrBadSignature
	}
	sigs := make([][]byte, 0, len(sigItems))
	for _, s := range sigItems {
		if len(s) < 2 || s[len(s)-1] != SigHashAll {
			return ErrBadSignature
		}
		sigs = append(sigs, s[:len(s)-1])
	}
	hash, err := SignatureHash(tx, inputIndex, prevPkScript, SigHashAll)
	if err != nil {
		return err
	}
	// Match signatures to pubkeys in order.
	pubIdx := 0
	for _, sigDER := range sigs {
		sig, err := btcecdsa.ParseDERSignature(sigDER)
		if err != nil {
			return err
		}
		matched := false
		for pubIdx < len(pubKeys) {
			pub, err := btcec.ParsePubKey(pubKeys[pubIdx])
			pubIdx++
			if err != nil {
				continue
			}
			if sig.Verify(hash[:], pub) {
				matched = true
				break
			}
		}
		if !matched {
			return ErrBadSignature
		}
	}
	return nil
}

func SignatureHash(tx *wire.MsgTx, inputIndex int, prevPkScript []byte, hashType byte) (chainhash.Hash, error) {
	if inputIndex < 0 || inputIndex >= len(tx.TxIn) {
		return chainhash.Hash{}, ErrBadSignatureScript
	}
	var buf bytes.Buffer
	if err := binary.Write(&buf, binary.LittleEndian, tx.Version); err != nil {
		return chainhash.Hash{}, err
	}
	if err := wire.WriteVarInt(&buf, uint64(len(tx.TxIn))); err != nil {
		return chainhash.Hash{}, err
	}
	for i, in := range tx.TxIn {
		if _, err := buf.Write(in.PreviousOutPoint.Hash[:]); err != nil {
			return chainhash.Hash{}, err
		}
		if err := binary.Write(&buf, binary.LittleEndian, in.PreviousOutPoint.Index); err != nil {
			return chainhash.Hash{}, err
		}
		script := []byte(nil)
		if i == inputIndex {
			script = prevPkScript
		}
		if err := wire.WriteVarBytes(&buf, script); err != nil {
			return chainhash.Hash{}, err
		}
		if err := binary.Write(&buf, binary.LittleEndian, in.Sequence); err != nil {
			return chainhash.Hash{}, err
		}
	}
	if err := wire.WriteVarInt(&buf, uint64(len(tx.TxOut))); err != nil {
		return chainhash.Hash{}, err
	}
	for _, out := range tx.TxOut {
		if err := binary.Write(&buf, binary.LittleEndian, out.Value); err != nil {
			return chainhash.Hash{}, err
		}
		if err := wire.WriteVarBytes(&buf, out.PkScript); err != nil {
			return chainhash.Hash{}, err
		}
	}
	if err := binary.Write(&buf, binary.LittleEndian, tx.LockTime); err != nil {
		return chainhash.Hash{}, err
	}
	if err := binary.Write(&buf, binary.LittleEndian, uint32(hashType)); err != nil {
		return chainhash.Hash{}, err
	}
	return chainhash.DoubleHashB(buf.Bytes()), nil
}

func SignatureScript(sigDER []byte, pubKeyCompressed []byte) ([]byte, error) {
	sig := append(append([]byte{}, sigDER...), SigHashAll)
	var buf bytes.Buffer
	if err := writePushData(&buf, sig); err != nil {
		return nil, err
	}
	if err := writePushData(&buf, pubKeyCompressed); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func HybridSignatureScript(sig pqc.HybridSignature, pub pqc.HybridPublicBytes) ([]byte, error) {
	ecdsaWithHashType := append(append([]byte{}, sig.ECDSADER...), SigHashAll)
	var buf bytes.Buffer
	if err := writePushData(&buf, ecdsaWithHashType); err != nil {
		return nil, err
	}
	if err := writePushData(&buf, sig.MLDSA65); err != nil {
		return nil, err
	}
	if err := writePushData(&buf, pub.SecpCompressed); err != nil {
		return nil, err
	}
	if err := writePushData(&buf, pub.MLDSA65); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func parseP2PKHScriptSig(sigScript []byte) ([]byte, []byte, error) {
	r := bytes.NewReader(sigScript)
	sig, err := readPushData(r)
	if err != nil {
		return nil, nil, err
	}
	pubKey, err := readPushData(r)
	if err != nil {
		return nil, nil, err
	}
	if r.Len() != 0 {
		return nil, nil, ErrBadSignatureScript
	}
	return sig, pubKey, nil
}

func parseP2PKScriptSig(sigScript []byte) ([]byte, error) {
	r := bytes.NewReader(sigScript)
	sig, err := readPushData(r)
	if err != nil {
		return nil, err
	}
	if r.Len() != 0 {
		return nil, ErrBadSignatureScript
	}
	return sig, nil
}

func parsePushes(sigScript []byte) ([][]byte, error) {
	r := bytes.NewReader(sigScript)
	var pushes [][]byte
	for r.Len() > 0 {
		d, err := readPushData(r)
		if err != nil {
			return nil, err
		}
		pushes = append(pushes, d)
	}
	return pushes, nil
}

func encodePushes(pushes [][]byte) ([]byte, error) {
	var buf bytes.Buffer
	for _, p := range pushes {
		if err := writePushData(&buf, p); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func writePushData(w io.Writer, data []byte) error {
	if len(data) > wire.MaxScriptSize {
		return ErrUnsupportedScript
	}
	if len(data) <= 75 {
		if _, err := w.Write([]byte{byte(len(data))}); err != nil {
			return err
		}
	} else {
		if len(data) <= 255 {
			if _, err := w.Write([]byte{OP_PUSHDATA1, byte(len(data))}); err != nil {
				return err
			}
		} else if len(data) <= 65535 {
			if _, err := w.Write([]byte{OP_PUSHDATA2, byte(len(data)), byte(len(data) >> 8)}); err != nil {
				return err
			}
		} else {
			return ErrUnsupportedScript
		}
	}
	_, err := w.Write(data)
	return err
}

func readPushData(r *bytes.Reader) ([]byte, error) {
	op, err := r.ReadByte()
	if err != nil {
		return nil, ErrBadSignatureScript
	}
	var size int
	switch {
	case op == OP_0:
		return []byte{}, nil
	case op >= 1 && op <= 75:
		size = int(op)
	case op == OP_PUSHDATA1:
		n, err := r.ReadByte()
		if err != nil {
			return nil, ErrBadSignatureScript
		}
		size = int(n)
	case op == OP_PUSHDATA2:
		lo, err := r.ReadByte()
		if err != nil {
			return nil, ErrBadSignatureScript
		}
		hi, err := r.ReadByte()
		if err != nil {
			return nil, ErrBadSignatureScript
		}
		size = int(lo) | (int(hi) << 8)
	default:
		return nil, ErrBadSignatureScript
	}
	if size > wire.MaxScriptSize {
		return nil, ErrBadSignatureScript
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return nil, ErrBadSignatureScript
	}
	return data, nil
}

func CountSigOps(pkScript []byte) int {
	n := 0
	for _, op := range pkScript {
		switch op {
		case OP_CHECKSIG:
			n++
		case OP_CHECKMULTISIG:
			// Legacy static accounting for CHECKMULTISIG.
			n += 20
		}
	}
	return n
}

func ValidateScriptStructure(program []byte) error {
	depth := 0
	pc := 0
	for pc < len(program) {
		op := program[pc]
		pc++
		switch {
		case op == OP_0:
			continue
		case op >= 1 && op <= 75:
			if pc+int(op) > len(program) {
				return ErrMalformedScript
			}
			pc += int(op)
		case op == OP_PUSHDATA1:
			if pc+1 > len(program) {
				return ErrMalformedScript
			}
			size := int(program[pc])
			pc++
			if pc+size > len(program) {
				return ErrMalformedScript
			}
			pc += size
		case op == OP_PUSHDATA2:
			if pc+2 > len(program) {
				return ErrMalformedScript
			}
			size := int(program[pc]) | int(program[pc+1])<<8
			pc += 2
			if pc+size > len(program) {
				return ErrMalformedScript
			}
			pc += size
		case op == OP_IF || op == OP_NOTIF:
			depth++
		case op == OP_ELSE:
			if depth == 0 {
				return ErrMalformedScript
			}
		case op == OP_ENDIF:
			if depth == 0 {
				return ErrMalformedScript
			}
			depth--
		default:
			continue
		}
	}
	if depth != 0 {
		return ErrMalformedScript
	}
	return nil
}

// EvalPushScript executes a conservative subset of script semantics used by the
// policy path: pushdata handling, stack primitives, hashing/equality checks, and
// IF/NOTIF/ELSE/ENDIF control flow.
func EvalPushScript(program []byte, initialStack [][]byte) ([][]byte, error) {
	return EvalPushScriptWithHooks(program, initialStack, nil, nil)
}

func EvalPushScriptWithCheckSig(program []byte, initialStack [][]byte, verifyCheckSig func(sigWithHashType, pubKey []byte) bool) ([][]byte, error) {
	return EvalPushScriptWithHooks(program, initialStack, verifyCheckSig, nil)
}

func EvalPushScriptWithHooks(
	program []byte,
	initialStack [][]byte,
	verifyCheckSig func(sigWithHashType, pubKey []byte) bool,
	verifyCheckMultiSig func(sigs [][]byte, pubKeys [][]byte) bool,
) ([][]byte, error) {
	if err := ValidateScriptStructure(program); err != nil {
		return nil, err
	}
	stack := make([][]byte, len(initialStack))
	for i := range initialStack {
		stack[i] = append([]byte(nil), initialStack[i]...)
	}
	conds := make([]bool, 0)
	pc := 0
	for pc < len(program) {
		op := program[pc]
		pc++
		active := true
		for _, c := range conds {
			if !c {
				active = false
				break
			}
		}
		switch {
		case op == OP_0:
			if active {
				stack = append(stack, []byte{})
			}
		case op >= OP_1 && op <= OP_16:
			if active {
				stack = append(stack, []byte{op - OP_1 + 1})
			}
		case op >= 1 && op <= 75:
			if pc+int(op) > len(program) {
				return nil, ErrMalformedScript
			}
			data := program[pc : pc+int(op)]
			pc += int(op)
			if active {
				stack = append(stack, append([]byte(nil), data...))
			}
		case op == OP_PUSHDATA1:
			if pc+1 > len(program) {
				return nil, ErrMalformedScript
			}
			size := int(program[pc])
			pc++
			if pc+size > len(program) {
				return nil, ErrMalformedScript
			}
			data := program[pc : pc+size]
			pc += size
			if active {
				stack = append(stack, append([]byte(nil), data...))
			}
		case op == OP_PUSHDATA2:
			if pc+2 > len(program) {
				return nil, ErrMalformedScript
			}
			size := int(program[pc]) | int(program[pc+1])<<8
			pc += 2
			if pc+size > len(program) {
				return nil, ErrMalformedScript
			}
			data := program[pc : pc+size]
			pc += size
			if active {
				stack = append(stack, append([]byte(nil), data...))
			}
		case op == OP_IF || op == OP_NOTIF:
			cond := false
			if active {
				if len(stack) < 1 {
					return nil, ErrScriptEval
				}
				top := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				cond = isTruthy(top)
				if op == OP_NOTIF {
					cond = !cond
				}
			}
			conds = append(conds, cond)
		case op == OP_ELSE:
			if len(conds) < 1 {
				return nil, ErrMalformedScript
			}
			parentActive := true
			for i := 0; i < len(conds)-1; i++ {
				if !conds[i] {
					parentActive = false
					break
				}
			}
			if parentActive {
				conds[len(conds)-1] = !conds[len(conds)-1]
			}
		case op == OP_ENDIF:
			if len(conds) < 1 {
				return nil, ErrMalformedScript
			}
			conds = conds[:len(conds)-1]
		default:
			if !active {
				continue
			}
			switch op {
			case OP_DUP:
				if len(stack) < 1 {
					return nil, ErrScriptEval
				}
				stack = append(stack, append([]byte(nil), stack[len(stack)-1]...))
			case OP_HASH160:
				if len(stack) < 1 {
					return nil, ErrScriptEval
				}
				top := stack[len(stack)-1]
				stack[len(stack)-1] = Hash160(top)
			case OP_EQUAL:
				if len(stack) < 2 {
					return nil, ErrScriptEval
				}
				a := stack[len(stack)-2]
				b := stack[len(stack)-1]
				stack = stack[:len(stack)-2]
				if bytes.Equal(a, b) {
					stack = append(stack, []byte{1})
				} else {
					stack = append(stack, []byte{})
				}
			case OP_EQUALVERIFY:
				if len(stack) < 2 {
					return nil, ErrScriptEval
				}
				a := stack[len(stack)-2]
				b := stack[len(stack)-1]
				stack = stack[:len(stack)-2]
				if !bytes.Equal(a, b) {
					return nil, ErrScriptEval
				}
			case OP_CHECKSIG:
				if len(stack) < 2 {
					return nil, ErrScriptEval
				}
				pubKey := stack[len(stack)-1]
				sigWithHashType := stack[len(stack)-2]
				stack = stack[:len(stack)-2]
				ok := false
				if verifyCheckSig != nil {
					ok = verifyCheckSig(sigWithHashType, pubKey)
				}
				if ok {
					stack = append(stack, []byte{1})
				} else {
					stack = append(stack, []byte{})
				}
			case OP_CHECKMULTISIG:
				// Legacy CHECKMULTISIG consumes a leading dummy item from stack.
				if len(stack) < 1 {
					return nil, ErrScriptEval
				}
				n, ok := decodeStackSmallInt(stack[len(stack)-1])
				if !ok || n < 0 {
					return nil, ErrScriptEval
				}
				stack = stack[:len(stack)-1]
				if len(stack) < n+1 {
					return nil, ErrScriptEval
				}
				pubKeys := make([][]byte, n)
				for i := n - 1; i >= 0; i-- {
					pubKeys[i] = append([]byte(nil), stack[len(stack)-1]...)
					stack = stack[:len(stack)-1]
				}
				m, ok := decodeStackSmallInt(stack[len(stack)-1])
				if !ok || m < 0 {
					return nil, ErrScriptEval
				}
				stack = stack[:len(stack)-1]
				if m > n || len(stack) < m+1 {
					return nil, ErrScriptEval
				}
				sigs := make([][]byte, m)
				for i := m - 1; i >= 0; i-- {
					sigs[i] = append([]byte(nil), stack[len(stack)-1]...)
					stack = stack[:len(stack)-1]
				}
				// CHECKMULTISIG bug compatibility dummy value.
				stack = stack[:len(stack)-1]
				pass := false
				if verifyCheckMultiSig != nil {
					pass = verifyCheckMultiSig(sigs, pubKeys)
				}
				if pass {
					stack = append(stack, []byte{1})
				} else {
					stack = append(stack, []byte{})
				}
			default:
				return nil, ErrUnsupportedScript
			}
		}
	}
	return stack, nil
}

func isTruthy(v []byte) bool {
	for _, b := range v {
		if b != 0 {
			return true
		}
	}
	return false
}

func decodeStackSmallInt(v []byte) (int, bool) {
	if len(v) == 0 {
		return 0, true
	}
	if len(v) != 1 {
		return 0, false
	}
	if v[0] > 16 {
		return 0, false
	}
	return int(v[0]), true
}

func SigOpsForSpend(sigScript []byte, prevPkScript []byte) (int, error) {
	sigOps := CountSigOps(prevPkScript)
	if !IsPayToScriptHash(prevPkScript) {
		return sigOps, nil
	}
	pushes, err := parsePushes(sigScript)
	if err != nil || len(pushes) < 1 {
		return 0, ErrBadSignatureScript
	}
	redeem := pushes[len(pushes)-1]
	if !bytes.Equal(Hash160(redeem), prevPkScript[2:22]) {
		return 0, ErrBadSignature
	}
	sigOps += CountSigOps(redeem)
	return sigOps, nil
}

func parseMultiSigScript(pkScript []byte) (int, [][]byte, bool) {
	if len(pkScript) < 1+1+1 {
		return 0, nil, false
	}
	m, ok := decodeSmallInt(pkScript[0])
	if !ok || m < 1 {
		return 0, nil, false
	}
	i := 1
	pubKeys := make([][]byte, 0)
	for i < len(pkScript)-2 {
		l := int(pkScript[i])
		if l != 33 && l != 65 {
			break
		}
		i++
		if i+l > len(pkScript)-2 {
			return 0, nil, false
		}
		pubKeys = append(pubKeys, append([]byte(nil), pkScript[i:i+l]...))
		i += l
	}
	n, ok := decodeSmallInt(pkScript[i])
	if !ok || n < 1 || n > 16 {
		return 0, nil, false
	}
	i++
	if i >= len(pkScript) || pkScript[i] != OP_CHECKMULTISIG || i != len(pkScript)-1 {
		return 0, nil, false
	}
	if len(pubKeys) != n || m > n {
		return 0, nil, false
	}
	return m, pubKeys, true
}

func decodeSmallInt(op byte) (int, bool) {
	if op < OP_1 || op > OP_16 {
		return 0, false
	}
	return int(op - OP_1 + 1), true
}
