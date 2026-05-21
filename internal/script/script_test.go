package script

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"

	"legacycoin/legacy-go/internal/address"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/pqc"
	"legacycoin/legacy-go/internal/wire"
)

func TestP2PKHVerifyInput(t *testing.T) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := priv.PubKey().SerializeCompressed()
	pkScript, err := PayToPubKeyHashScript(Hash160(pub))
	if err != nil {
		t.Fatal(err)
	}
	prevHash := chainhash.DoubleHashB([]byte("prev tx"))
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: 0},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1, PkScript: []byte{OP_0}}},
	}
	hash, err := SignatureHash(tx, 0, pkScript, SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	sig := btcecdsa.Sign(priv, hash[:]).Serialize()
	sigScript, err := SignatureScript(sig, pub)
	if err != nil {
		t.Fatal(err)
	}
	tx.TxIn[0].SignatureScript = sigScript
	if err := VerifyInput(tx, 0, pkScript); err != nil {
		t.Fatal(err)
	}
	tx.TxOut[0].Value = 2
	if err := VerifyInput(tx, 0, pkScript); err == nil {
		t.Fatal("expected signature failure after tampering")
	}
}

func TestHybridP2PKHVerifyInput(t *testing.T) {
	key, err := pqc.GenerateHybridKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := key.Public().Bytes()
	addrStr := address.NewHybridAddress(pub.SecpCompressed, pub.MLDSA65)
	hash20, err := address.DecodeHybridAddress(addrStr)
	if err != nil {
		t.Fatal(err)
	}
	pkScript, err := PayToHybridPubKeyHashScript(hash20)
	if err != nil {
		t.Fatal(err)
	}
	prevHash := chainhash.DoubleHashB([]byte("hybrid prev tx"))
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: 0},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1, PkScript: []byte{OP_0}}},
	}
	sighash, err := SignatureHash(tx, 0, pkScript, SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	hsig, err := key.Sign(sighash[:])
	if err != nil {
		t.Fatal(err)
	}
	var sigScript bytes.Buffer
	if err := writePushData(&sigScript, append(append([]byte{}, hsig.ECDSADER...), SigHashAll)); err != nil {
		t.Fatal(err)
	}
	if err := writePushData(&sigScript, hsig.MLDSA65); err != nil {
		t.Fatal(err)
	}
	if err := writePushData(&sigScript, pub.SecpCompressed); err != nil {
		t.Fatal(err)
	}
	if err := writePushData(&sigScript, pub.MLDSA65); err != nil {
		t.Fatal(err)
	}
	tx.TxIn[0].SignatureScript = sigScript.Bytes()
	if err := VerifyInput(tx, 0, pkScript); err != nil {
		t.Fatal(err)
	}
	tx.TxOut[0].Value = 2
	if err := VerifyInput(tx, 0, pkScript); err == nil {
		t.Fatal("expected hybrid signature failure after tampering")
	}
}

func TestP2PKVerifyInput(t *testing.T) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := priv.PubKey().SerializeCompressed()
	pkScript := append([]byte{byte(len(pub))}, pub...)
	pkScript = append(pkScript, OP_CHECKSIG)
	prevHash := chainhash.DoubleHashB([]byte("p2pk prev tx"))
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: 1},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1, PkScript: []byte{OP_0}}},
	}
	hash, err := SignatureHash(tx, 0, pkScript, SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	sig := append(btcecdsa.Sign(priv, hash[:]).Serialize(), SigHashAll)
	sigScript, err := pushOnly(sig)
	if err != nil {
		t.Fatal(err)
	}
	tx.TxIn[0].SignatureScript = sigScript
	if err := VerifyInput(tx, 0, pkScript); err != nil {
		t.Fatal(err)
	}
	tx.TxIn[0].Sequence = 1
	if err := VerifyInput(tx, 0, pkScript); err == nil {
		t.Fatal("expected signature failure after tampering")
	}
}

func TestP2SHTemplate(t *testing.T) {
	h := make([]byte, 20)
	h[0] = 0xaa
	pkScript, err := PayToScriptHashScript(h)
	if err != nil {
		t.Fatal(err)
	}
	if !IsPayToScriptHash(pkScript) {
		t.Fatal("expected p2sh template")
	}
}

func TestP2SHVerifyInputWithP2PKHRedeem(t *testing.T) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := priv.PubKey().SerializeCompressed()
	redeem, err := PayToPubKeyHashScript(Hash160(pub))
	if err != nil {
		t.Fatal(err)
	}
	prevPkScript, err := PayToScriptHashScript(Hash160(redeem))
	if err != nil {
		t.Fatal(err)
	}
	prevHash := chainhash.DoubleHashB([]byte("p2sh prev"))
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: 0},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1, PkScript: []byte{OP_0}}},
	}
	hash, err := SignatureHash(tx, 0, redeem, SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	sig := append(btcecdsa.Sign(priv, hash[:]).Serialize(), SigHashAll)
	var sigScript []byte
	sigScript = append(sigScript, byte(len(sig)))
	sigScript = append(sigScript, sig...)
	sigScript = append(sigScript, byte(len(pub)))
	sigScript = append(sigScript, pub...)
	sigScript = append(sigScript, byte(len(redeem)))
	sigScript = append(sigScript, redeem...)
	tx.TxIn[0].SignatureScript = sigScript
	if err := VerifyInput(tx, 0, prevPkScript); err != nil {
		t.Fatal(err)
	}
	tx.TxOut[0].Value = 2
	if err := VerifyInput(tx, 0, prevPkScript); err == nil {
		t.Fatal("expected signature failure after tampering")
	}
}

func TestP2SHRejectsRedeemHashMismatch(t *testing.T) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := priv.PubKey().SerializeCompressed()
	redeem, err := PayToPubKeyHashScript(Hash160(pub))
	if err != nil {
		t.Fatal(err)
	}
	otherRedeem, err := PayToPubKeyHashScript(Hash160([]byte("other pubkey bytes........")))
	if err == nil && len(otherRedeem) == 25 {
		_ = otherRedeem
	}
	prevPkScript, err := PayToScriptHashScript(Hash160([]byte("not redeem hash bytes!"))[:20])
	if err != nil {
		t.Fatal(err)
	}
	prevHash := chainhash.DoubleHashB([]byte("p2sh mismatch"))
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: 0},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1, PkScript: []byte{OP_0}}},
	}
	hash, err := SignatureHash(tx, 0, redeem, SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	sig := append(btcecdsa.Sign(priv, hash[:]).Serialize(), SigHashAll)
	var sigScript []byte
	sigScript = append(sigScript, byte(len(sig)))
	sigScript = append(sigScript, sig...)
	sigScript = append(sigScript, byte(len(pub)))
	sigScript = append(sigScript, pub...)
	sigScript = append(sigScript, byte(len(redeem)))
	sigScript = append(sigScript, redeem...)
	tx.TxIn[0].SignatureScript = sigScript
	if err := VerifyInput(tx, 0, prevPkScript); err == nil {
		t.Fatal("expected redeem hash mismatch failure")
	}
}

func TestP2SHRejectsUnsupportedRedeemScript(t *testing.T) {
	redeem := []byte{OP_0}
	prevPkScript, err := PayToScriptHashScript(Hash160(redeem))
	if err != nil {
		t.Fatal(err)
	}
	prevHash := chainhash.DoubleHashB([]byte("p2sh unsupported"))
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: 0},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1, PkScript: []byte{OP_0}}},
	}
	sigScript := []byte{byte(len(redeem))}
	sigScript = append(sigScript, redeem...)
	tx.TxIn[0].SignatureScript = sigScript
	if err := VerifyInput(tx, 0, prevPkScript); err == nil {
		t.Fatal("expected unsupported redeem script failure")
	}
}

func TestP2SHVerifyInputWithP2PKRedeem(t *testing.T) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub := priv.PubKey().SerializeCompressed()
	redeem := append([]byte{byte(len(pub))}, pub...)
	redeem = append(redeem, OP_CHECKSIG)
	prevPkScript, err := PayToScriptHashScript(Hash160(redeem))
	if err != nil {
		t.Fatal(err)
	}
	prevHash := chainhash.DoubleHashB([]byte("p2sh p2pk prev"))
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: 0},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1, PkScript: []byte{OP_0}}},
	}
	hash, err := SignatureHash(tx, 0, redeem, SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	sig := append(btcecdsa.Sign(priv, hash[:]).Serialize(), SigHashAll)
	var sigScript []byte
	sigScript = append(sigScript, byte(len(sig)))
	sigScript = append(sigScript, sig...)
	sigScript = append(sigScript, byte(len(redeem)))
	sigScript = append(sigScript, redeem...)
	tx.TxIn[0].SignatureScript = sigScript
	if err := VerifyInput(tx, 0, prevPkScript); err != nil {
		t.Fatal(err)
	}
	tx.TxOut[0].Value = 3
	if err := VerifyInput(tx, 0, prevPkScript); err == nil {
		t.Fatal("expected signature failure after tampering")
	}
}

func TestP2SHRejectsMalformedPushScriptSig(t *testing.T) {
	redeem := []byte{OP_0}
	prevPkScript, err := PayToScriptHashScript(Hash160(redeem))
	if err != nil {
		t.Fatal(err)
	}
	prevHash := chainhash.DoubleHashB([]byte("p2sh malformed push"))
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: 0},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1, PkScript: []byte{OP_0}}},
	}
	// Declares 5-byte push but only includes 2 bytes.
	tx.TxIn[0].SignatureScript = []byte{0x05, 0xaa, 0xbb}
	if err := VerifyInput(tx, 0, prevPkScript); err == nil {
		t.Fatal("expected malformed push failure")
	}
}

func TestPushData1RoundTrip(t *testing.T) {
	payload := make([]byte, 120)
	for i := range payload {
		payload[i] = byte(i)
	}
	var buf bytes.Buffer
	if err := writePushData(&buf, payload); err != nil {
		t.Fatal(err)
	}
	if got := buf.Bytes(); len(got) < 2 || got[0] != OP_PUSHDATA1 || got[1] != byte(len(payload)) {
		t.Fatalf("unexpected pushdata1 prefix")
	}
	decoded, err := readPushData(bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(payload) {
		t.Fatalf("decoded len=%d", len(decoded))
	}
	for i := range decoded {
		if decoded[i] != payload[i] {
			t.Fatalf("payload mismatch at %d", i)
		}
	}
}

func TestPushData2RoundTrip(t *testing.T) {
	payload := make([]byte, 600)
	for i := range payload {
		payload[i] = byte(i)
	}
	var buf bytes.Buffer
	if err := writePushData(&buf, payload); err != nil {
		t.Fatal(err)
	}
	got := buf.Bytes()
	if len(got) < 3 || got[0] != OP_PUSHDATA2 {
		t.Fatalf("expected OP_PUSHDATA2 prefix")
	}
	decoded, err := readPushData(bytes.NewReader(got))
	if err != nil {
		t.Fatal(err)
	}
	if len(decoded) != len(payload) {
		t.Fatalf("decoded len=%d want=%d", len(decoded), len(payload))
	}
}

func TestCountSigOps(t *testing.T) {
	script := []byte{OP_DUP, OP_CHECKSIG, OP_CHECKSIG, OP_CHECKMULTISIG}
	if got := CountSigOps(script); got != 22 {
		t.Fatalf("sigops=%d want=22", got)
	}
}

func TestSigOpsForP2SHSpend(t *testing.T) {
	redeem := []byte{0x21}
	redeem = append(redeem, bytes.Repeat([]byte{0x02}, 33)...)
	redeem = append(redeem, OP_CHECKSIG)
	prev, err := PayToScriptHashScript(Hash160(redeem))
	if err != nil {
		t.Fatal(err)
	}
	sig := []byte{0x30, 0x01, 0x00, SigHashAll}
	var sigScript []byte
	sigScript = append(sigScript, byte(len(sig)))
	sigScript = append(sigScript, sig...)
	sigScript = append(sigScript, byte(len(redeem)))
	sigScript = append(sigScript, redeem...)
	ops, err := SigOpsForSpend(sigScript, prev)
	if err != nil {
		t.Fatal(err)
	}
	if ops != 1 {
		t.Fatalf("sigops=%d want=1", ops)
	}
}

func TestBareMultiSigVerifyInput(t *testing.T) {
	priv1, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	priv2, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	pub1 := priv1.PubKey().SerializeCompressed()
	pub2 := priv2.PubKey().SerializeCompressed()
	// 2-of-2 multisig.
	redeem := []byte{OP_2}
	redeem = append(redeem, byte(len(pub1)))
	redeem = append(redeem, pub1...)
	redeem = append(redeem, byte(len(pub2)))
	redeem = append(redeem, pub2...)
	redeem = append(redeem, OP_2, OP_CHECKMULTISIG)
	if !IsPayToMultiSig(redeem) {
		t.Fatal("expected bare multisig template")
	}
	prevHash := chainhash.DoubleHashB([]byte("ms prev"))
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: 0},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1, PkScript: []byte{OP_0}}},
	}
	h, err := SignatureHash(tx, 0, redeem, SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	s1 := append(btcecdsa.Sign(priv1, h[:]).Serialize(), SigHashAll)
	s2 := append(btcecdsa.Sign(priv2, h[:]).Serialize(), SigHashAll)
	var sigScript []byte
	sigScript = append(sigScript, OP_0) // dummy
	sigScript = append(sigScript, byte(len(s1)))
	sigScript = append(sigScript, s1...)
	sigScript = append(sigScript, byte(len(s2)))
	sigScript = append(sigScript, s2...)
	tx.TxIn[0].SignatureScript = sigScript
	if err := VerifyInput(tx, 0, redeem); err != nil {
		t.Fatal(err)
	}
}

func TestP2SHMultiSigVerifyInput(t *testing.T) {
	priv1, _ := btcec.NewPrivateKey()
	priv2, _ := btcec.NewPrivateKey()
	pub1 := priv1.PubKey().SerializeCompressed()
	pub2 := priv2.PubKey().SerializeCompressed()
	redeem := []byte{OP_2}
	redeem = append(redeem, byte(len(pub1)))
	redeem = append(redeem, pub1...)
	redeem = append(redeem, byte(len(pub2)))
	redeem = append(redeem, pub2...)
	redeem = append(redeem, OP_2, OP_CHECKMULTISIG)
	prevPkScript, err := PayToScriptHashScript(Hash160(redeem))
	if err != nil {
		t.Fatal(err)
	}
	prevHash := chainhash.DoubleHashB([]byte("p2sh ms prev"))
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: prevHash, Index: 0},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1, PkScript: []byte{OP_0}}},
	}
	h, err := SignatureHash(tx, 0, redeem, SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	s1 := append(btcecdsa.Sign(priv1, h[:]).Serialize(), SigHashAll)
	s2 := append(btcecdsa.Sign(priv2, h[:]).Serialize(), SigHashAll)
	var sigScript []byte
	sigScript = append(sigScript, OP_0)
	sigScript = append(sigScript, byte(len(s1)))
	sigScript = append(sigScript, s1...)
	sigScript = append(sigScript, byte(len(s2)))
	sigScript = append(sigScript, s2...)
	sigScript = append(sigScript, byte(len(redeem)))
	sigScript = append(sigScript, redeem...)
	tx.TxIn[0].SignatureScript = sigScript
	if err := VerifyInput(tx, 0, prevPkScript); err != nil {
		t.Fatal(err)
	}
}

func pushOnly(data []byte) ([]byte, error) {
	out := make([]byte, 0, len(data)+1)
	out = append(out, byte(len(data)))
	out = append(out, data...)
	return out, nil
}

func TestValidateScriptStructure(t *testing.T) {
	// Valid: IF <push> ELSE <push> ENDIF
	valid := []byte{OP_IF, 0x01, 0x01, OP_ELSE, 0x01, 0x00, OP_ENDIF}
	if err := ValidateScriptStructure(valid); err != nil {
		t.Fatalf("valid control-flow script rejected: %v", err)
	}

	// Invalid: ELSE without IF
	if err := ValidateScriptStructure([]byte{OP_ELSE}); err == nil {
		t.Fatalf("expected ELSE-without-IF to fail")
	}

	// Invalid: unclosed IF
	if err := ValidateScriptStructure([]byte{OP_IF, 0x01, 0x01}); err == nil {
		t.Fatalf("expected unclosed IF to fail")
	}

	// Invalid push length overrun
	if err := ValidateScriptStructure([]byte{0x02, 0x01}); err == nil {
		t.Fatalf("expected bad push overrun to fail")
	}
}

func TestEvalPushScriptControlFlow(t *testing.T) {
	// IF true -> take first branch, skip ELSE branch.
	program := []byte{
		0x01, 0x01, // push true
		OP_IF,
		0x01, 0xaa, // then push aa
		OP_ELSE,
		0x01, 0xbb, // else push bb
		OP_ENDIF,
	}
	stack, err := EvalPushScript(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(stack) != 1 || len(stack[0]) != 1 || stack[0][0] != 0xaa {
		t.Fatalf("unexpected stack result: %#v", stack)
	}
}

func TestEvalPushScriptNotIfBranch(t *testing.T) {
	program := []byte{
		OP_0,     // false
		OP_NOTIF, // true branch because NOTIF
		0x01, 0x11,
		OP_ELSE,
		0x01, 0x22,
		OP_ENDIF,
	}
	stack, err := EvalPushScript(program, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(stack) != 1 || stack[0][0] != 0x11 {
		t.Fatalf("unexpected stack result: %#v", stack)
	}
}

func TestEvalPushScriptEqualVerify(t *testing.T) {
	okScript := []byte{
		0x01, 0x44,
		OP_DUP,
		OP_EQUALVERIFY,
	}
	stack, err := EvalPushScript(okScript, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(stack) != 0 {
		t.Fatalf("expected empty stack after equalverify, got %#v", stack)
	}

	failScript := []byte{
		0x01, 0x44,
		0x01, 0x55,
		OP_EQUALVERIFY,
	}
	if _, err := EvalPushScript(failScript, nil); err == nil {
		t.Fatal("expected equalverify failure")
	}
}

func TestEvalPushScriptCheckSigHook(t *testing.T) {
	program := []byte{
		0x02, 0x30, 0x01, // mock sig+hashtype
		0x02, 0x02, 0x03, // mock compressed pubkey prefix bytes
		OP_CHECKSIG,
	}
	stack, err := EvalPushScriptWithCheckSig(program, nil, func(sigWithHashType, pubKey []byte) bool {
		return len(sigWithHashType) == 2 && len(pubKey) == 2 && pubKey[0] == 0x02
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stack) != 1 || len(stack[0]) != 1 || stack[0][0] != 1 {
		t.Fatalf("expected true checksig result, got %#v", stack)
	}

	stack, err = EvalPushScriptWithCheckSig(program, nil, func(sigWithHashType, pubKey []byte) bool {
		return false
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stack) != 1 || len(stack[0]) != 0 {
		t.Fatalf("expected false checksig result, got %#v", stack)
	}
}

func TestEvalPushScriptCheckMultiSigHook(t *testing.T) {
	// Stack form before OP_CHECKMULTISIG:
	// dummy, sig1, sig2, m(2), pub1, pub2, n(2)
	program := []byte{
		OP_0,
		0x01, 0xa1,
		0x01, 0xa2,
		OP_2,
		0x01, 0xb1,
		0x01, 0xb2,
		OP_2,
		OP_CHECKMULTISIG,
	}
	stack, err := EvalPushScriptWithHooks(program, nil, nil, func(sigs [][]byte, pubKeys [][]byte) bool {
		return len(sigs) == 2 && len(pubKeys) == 2 && sigs[0][0] == 0xa1 && pubKeys[1][0] == 0xb2
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(stack) != 1 || len(stack[0]) != 1 || stack[0][0] != 1 {
		t.Fatalf("expected true checkmultisig result, got %#v", stack)
	}
}
