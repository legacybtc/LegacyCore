package mempool

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	btcecdsa "github.com/btcsuite/btcd/btcec/v2/ecdsa"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/pqc"
	"legacycoin/legacy-go/internal/script"
	"legacycoin/legacy-go/internal/storage"
	"legacycoin/legacy-go/internal/wire"
)

func TestMeetsMinRelayFee(t *testing.T) {
	tests := []struct {
		name string
		fee  int64
		size int
		want bool
	}{
		{name: "zero size", fee: 1000, size: 0, want: false},
		{name: "one byte enough", fee: 1, size: 1, want: true},
		{name: "one kb exact", fee: 1000, size: 1000, want: true},
		{name: "one kb low", fee: 999, size: 1000, want: false},
		{name: "two kb rounded", fee: 2000, size: 1999, want: true},
		{name: "two kb rounded low", fee: 1998, size: 1999, want: false},
	}
	for _, tc := range tests {
		if got := MeetsMinRelayFee(tc.fee, tc.size); got != tc.want {
			t.Fatalf("%s: got=%v want=%v", tc.name, got, tc.want)
		}
	}
}

func TestCheckStandardness(t *testing.T) {
	pubHash := make([]byte, 20)
	pkScript, err := script.PayToPubKeyHashScript(pubHash)
	if err != nil {
		t.Fatal(err)
	}
	standard := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{1}, Index: 0},
			SignatureScript:  []byte{0x01, 0x01},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{
			Value:    1000,
			PkScript: pkScript,
		}},
	}
	raw, err := standard.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if err := checkStandardness(standard, len(raw)); err != nil {
		t.Fatalf("standard tx rejected: %v", err)
	}

	dust := *standard
	dust.TxOut = []wire.TxOut{{Value: DustThreshold - 1, PkScript: pkScript}}
	if err := checkStandardness(&dust, len(raw)); err == nil {
		t.Fatalf("dust tx accepted")
	}

	nonstd := *standard
	nonstd.TxOut = []wire.TxOut{{Value: 1000, PkScript: []byte{0x6a, 0x01, 0x01}}}
	if err := checkStandardness(&nonstd, len(raw)); err == nil {
		t.Fatalf("non-standard script accepted")
	}

	oversig := *standard
	oversig.TxIn = []wire.TxIn{{
		PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{1}, Index: 0},
		SignatureScript:  make([]byte, MaxStandardSigScript+1),
		Sequence:         0xffffffff,
	}}
	if err := checkStandardness(&oversig, len(raw)); err == nil {
		t.Fatalf("oversized sigscript accepted")
	}
}

func TestFeeRateAndLowestEntry(t *testing.T) {
	a := Entry{Fee: 1000, Size: 1000}
	b := Entry{Fee: 900, Size: 1000}
	c := Entry{Fee: 2000, Size: 1000}
	if feeRatePerKB(a.Fee, a.Size) != 1000 {
		t.Fatalf("unexpected fee rate")
	}
	lowID, low, ok := lowestFeeRateEntry(map[string]Entry{"a": a, "b": b, "c": c})
	if !ok {
		t.Fatalf("expected lowest entry")
	}
	if lowID != "b" || low.Fee != 900 {
		t.Fatalf("unexpected lowest entry: %s %+v", lowID, low)
	}
}

func TestOrphanBoundedPool(t *testing.T) {
	p := New()
	p.maxOrph = 2
	tx := func(n byte) *wire.MsgTx {
		return &wire.MsgTx{
			Version: 1,
			TxIn: []wire.TxIn{{
				PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{n}, Index: 0},
				SignatureScript:  []byte{0x01, 0x01},
				Sequence:         0xffffffff,
			}},
			TxOut: []wire.TxOut{{
				Value:    1000,
				PkScript: []byte{0x51},
			}},
		}
	}
	p.addOrphan(tx(1), "", []string{"a:0"})
	p.addOrphan(tx(2), "", []string{"a:1"})
	p.addOrphan(tx(3), "", []string{"a:2"})
	if got := p.OrphanCount(); got != 2 {
		t.Fatalf("orphan count=%d", got)
	}
}

func TestDeleteOrphanClearsRefs(t *testing.T) {
	p := New()
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{9}, Index: 0},
			SignatureScript:  []byte{0x01, 0x01},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1000, PkScript: []byte{0x51}}},
	}
	depKey := blockchain.OutPointKey(tx.TxIn[0].PreviousOutPoint.Hash.String(), tx.TxIn[0].PreviousOutPoint.Index)
	p.addOrphan(tx, "", []string{depKey})
	if p.OrphanCount() != 1 {
		t.Fatalf("expected orphan")
	}
	txHash, _ := tx.TxHash()
	p.mu.Lock()
	p.deleteOrphanLocked(txHash.String())
	p.mu.Unlock()
	if p.OrphanCount() != 0 {
		t.Fatalf("orphan not deleted")
	}
	p.mu.RLock()
	_, ok := p.orphRef[depKey]
	p.mu.RUnlock()
	if ok {
		t.Fatalf("orphan ref not cleared")
	}
}

func TestOrphanDependencyIndexing(t *testing.T) {
	p := New()
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{4}, Index: 1},
			SignatureScript:  []byte{0x01, 0x01},
			Sequence:         0xffffffff,
		}},
		TxOut: []wire.TxOut{{Value: 1000, PkScript: []byte{0x51}}},
	}
	depKey := blockchain.OutPointKey(tx.TxIn[0].PreviousOutPoint.Hash.String(), tx.TxIn[0].PreviousOutPoint.Index)
	p.addOrphan(tx, "", []string{depKey})
	p.mu.RLock()
	waiters := p.orphRef[depKey]
	p.mu.RUnlock()
	if len(waiters) != 1 {
		t.Fatalf("expected 1 orphan waiter, got %d", len(waiters))
	}
}

func TestLowestEvictionCandidatePrefersLeaf(t *testing.T) {
	entries := map[string]Entry{
		"parent": {Fee: 500, Size: 500},
		"leaf":   {Fee: 700, Size: 700},
	}
	childs := map[string]map[string]struct{}{
		"parent": {"child": struct{}{}},
	}
	id, _, ok := lowestEvictionCandidate(entries, childs)
	if !ok {
		t.Fatalf("expected candidate")
	}
	if id != "leaf" {
		t.Fatalf("expected leaf candidate, got %s", id)
	}
}

func TestIsCompressedP2PKHPubKey(t *testing.T) {
	// [sig push=2 bytes][sig bytes][pub push=33 bytes][pub bytes]
	okScript := append([]byte{0x02, 0x30, 0x01, 0x21, 0x02}, make([]byte, 32)...)
	got, err := isCompressedP2PKHPubKey(okScript)
	if err != nil || !got {
		t.Fatalf("expected compressed pubkey accepted: got=%v err=%v", got, err)
	}

	// Uncompressed 65-byte key.
	badScript := append([]byte{0x02, 0x30, 0x01, 0x41, 0x04}, make([]byte, 64)...)
	got, err = isCompressedP2PKHPubKey(badScript)
	if err != nil {
		t.Fatalf("unexpected parse err: %v", err)
	}
	if got {
		t.Fatalf("expected uncompressed pubkey rejected")
	}
}

func TestHasLowSSignature(t *testing.T) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	msg := make([]byte, 32)
	sig := btcecdsa.Sign(priv, msg).Serialize() // canonical low-S
	sigScript := append([]byte{byte(len(sig) + 1)}, sig...)
	sigScript = append(sigScript, byte(script.SigHashAll))
	ok, err := hasLowSSignature(sigScript)
	if err != nil || !ok {
		t.Fatalf("expected low-S signature accepted: ok=%v err=%v", ok, err)
	}

	// Malformed DER in first push should fail policy parsing.
	bad := []byte{0x02, 0x30, 0x00}
	ok, err = hasLowSSignature(bad)
	if err == nil || ok {
		t.Fatalf("expected malformed signature rejected")
	}
}

func TestCheckP2SHPolicyP2PKH(t *testing.T) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	msg := make([]byte, 32)
	sig := append(btcecdsa.Sign(priv, msg).Serialize(), byte(script.SigHashAll))
	pub := priv.PubKey().SerializeCompressed()
	redeem, err := script.PayToPubKeyHashScript(script.Hash160(pub))
	if err != nil {
		t.Fatal(err)
	}
	sigPush, err := encodePushData(sig)
	if err != nil {
		t.Fatal(err)
	}
	pubPush, err := encodePushData(pub)
	if err != nil {
		t.Fatal(err)
	}
	redeemPush, err := encodePushData(redeem)
	if err != nil {
		t.Fatal(err)
	}
	sigScript := append(append(sigPush, pubPush...), redeemPush...)
	if err := checkP2SHPolicy(sigScript); err != nil {
		t.Fatalf("expected standard p2sh-p2pkh policy pass: %v", err)
	}

	pubUncompressed := priv.PubKey().SerializeUncompressed()
	pubPushBad, err := encodePushData(pubUncompressed)
	if err != nil {
		t.Fatal(err)
	}
	badSigScript := append(append(sigPush, pubPushBad...), redeemPush...)
	if err := checkP2SHPolicy(badSigScript); err == nil {
		t.Fatalf("expected uncompressed pubkey rejection for p2sh-p2pkh")
	}
}

func TestCheckP2SHPolicyRejectsUnknownRedeem(t *testing.T) {
	sig := []byte{0x30, 0x01, 0x00, byte(script.SigHashAll)}
	sigPush, err := encodePushData(sig)
	if err != nil {
		t.Fatal(err)
	}
	redeemPush, err := encodePushData([]byte{script.OP_0})
	if err != nil {
		t.Fatal(err)
	}
	sigScript := append([]byte{}, sigPush...)
	sigScript = append(sigScript, redeemPush...)
	if err := checkP2SHPolicy(sigScript); err == nil {
		t.Fatalf("expected non-standard redeem rejection")
	}
}

func TestParsePushOnlyScriptRejectsOpCodes(t *testing.T) {
	// OP_DUP is not a push opcode, so push-only parser must fail.
	if _, err := parsePushOnlyScript([]byte{script.OP_DUP}); err == nil {
		t.Fatalf("expected parse failure")
	}
}

func TestEncodePushDataRoundTrip(t *testing.T) {
	payload := bytes.Repeat([]byte{0x42}, 120)
	enc, err := encodePushData(payload)
	if err != nil {
		t.Fatal(err)
	}
	got, err := readScriptPushData(bytes.NewReader(enc))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(payload) {
		t.Fatalf("unexpected len: got=%d want=%d", len(got), len(payload))
	}
}

func TestEncodePushData2RoundTrip(t *testing.T) {
	payload := bytes.Repeat([]byte{0x24}, 600)
	enc, err := encodePushData(payload)
	if err != nil {
		t.Fatal(err)
	}
	got, err := readScriptPushData(bytes.NewReader(enc))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(payload) {
		t.Fatalf("unexpected len: got=%d want=%d", len(got), len(payload))
	}
}

func TestCheckMultiSigArgsLowS(t *testing.T) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	msg := make([]byte, 32)
	sig := append(btcecdsa.Sign(priv, msg).Serialize(), byte(script.SigHashAll))
	if err := checkMultiSigArgsLowS([][]byte{{}, sig}); err != nil {
		t.Fatalf("expected low-S multisig args accepted: %v", err)
	}
	if err := checkMultiSigArgsLowS([][]byte{{}}); err == nil {
		t.Fatalf("expected missing multisig sigs rejected")
	}
}

func TestCheckMultiSigPubKeysCompressed(t *testing.T) {
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
	ms := []byte{script.OP_2}
	ms = append(ms, byte(len(pub1)))
	ms = append(ms, pub1...)
	ms = append(ms, byte(len(pub2)))
	ms = append(ms, pub2...)
	ms = append(ms, script.OP_2, script.OP_CHECKMULTISIG)
	if err := checkMultiSigPubKeysCompressed(ms); err != nil {
		t.Fatalf("expected compressed multisig accepted: %v", err)
	}
	pub2u := priv2.PubKey().SerializeUncompressed()
	msBad := []byte{script.OP_2}
	msBad = append(msBad, byte(len(pub1)))
	msBad = append(msBad, pub1...)
	msBad = append(msBad, byte(len(pub2u)))
	msBad = append(msBad, pub2u...)
	msBad = append(msBad, script.OP_2, script.OP_CHECKMULTISIG)
	if err := checkMultiSigPubKeysCompressed(msBad); err == nil {
		t.Fatalf("expected uncompressed multisig pubkey rejected")
	}
}

func TestCheckStandardBareMultiSig(t *testing.T) {
	priv1, _ := btcec.NewPrivateKey()
	priv2, _ := btcec.NewPrivateKey()
	pub1 := priv1.PubKey().SerializeCompressed()
	pub2 := priv2.PubKey().SerializeCompressed()
	ms := []byte{script.OP_2}
	ms = append(ms, byte(len(pub1)))
	ms = append(ms, pub1...)
	ms = append(ms, byte(len(pub2)))
	ms = append(ms, pub2...)
	ms = append(ms, script.OP_2, script.OP_CHECKMULTISIG)
	if err := checkStandardBareMultiSig(ms); err != nil {
		t.Fatalf("expected standard 2-of-2 multisig accepted: %v", err)
	}

	// 1-of-4 should be rejected by conservative standard policy.
	priv3, _ := btcec.NewPrivateKey()
	priv4, _ := btcec.NewPrivateKey()
	pub3 := priv3.PubKey().SerializeCompressed()
	pub4 := priv4.PubKey().SerializeCompressed()
	ms4 := []byte{script.OP_1}
	for _, p := range [][]byte{pub1, pub2, pub3, pub4} {
		ms4 = append(ms4, byte(len(p)))
		ms4 = append(ms4, p...)
	}
	ms4 = append(ms4, 0x54 /* OP_4 */, script.OP_CHECKMULTISIG)
	if err := checkStandardBareMultiSig(ms4); err == nil {
		t.Fatalf("expected non-standard 1-of-4 multisig rejected")
	}
}

func TestCheckP2SHPolicyRejectsLargeRedeemMultiSig(t *testing.T) {
	priv1, _ := btcec.NewPrivateKey()
	priv2, _ := btcec.NewPrivateKey()
	priv3, _ := btcec.NewPrivateKey()
	priv4, _ := btcec.NewPrivateKey()
	pubs := [][]byte{
		priv1.PubKey().SerializeCompressed(),
		priv2.PubKey().SerializeCompressed(),
		priv3.PubKey().SerializeCompressed(),
		priv4.PubKey().SerializeCompressed(),
	}
	redeem := []byte{script.OP_1}
	for _, p := range pubs {
		redeem = append(redeem, byte(len(p)))
		redeem = append(redeem, p...)
	}
	redeem = append(redeem, 0x54 /* OP_4 */, script.OP_CHECKMULTISIG)
	msg := make([]byte, 32)
	sig := append(btcecdsa.Sign(priv1, msg).Serialize(), byte(script.SigHashAll))
	sigPush, err := encodePushData(sig)
	if err != nil {
		t.Fatal(err)
	}
	redeemPush, err := encodePushData(redeem)
	if err != nil {
		t.Fatal(err)
	}
	sigScript := append([]byte{script.OP_0}, sigPush...)
	sigScript = append(sigScript, redeemPush...)
	if err := checkP2SHPolicy(sigScript); err == nil {
		t.Fatalf("expected p2sh redeem multisig n>3 rejected")
	}
}

func TestCheckHybridInputPolicy(t *testing.T) {
	priv, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	msg := make([]byte, 32)
	ecdsaSig := append(btcecdsa.Sign(priv, msg).Serialize(), byte(script.SigHashAll))
	ecdsaPush, err := encodePushData(ecdsaSig)
	if err != nil {
		t.Fatal(err)
	}
	pqSigPush, err := encodePushData(bytes.Repeat([]byte{0x11}, pqc.MLDSASignatureSize))
	if err != nil {
		t.Fatal(err)
	}
	secpPubPush, err := encodePushData(append([]byte{0x02}, bytes.Repeat([]byte{0x22}, 32)...))
	if err != nil {
		t.Fatal(err)
	}
	pqPubPush, err := encodePushData(bytes.Repeat([]byte{0x33}, pqc.MLDSAPublicKeySize))
	if err != nil {
		t.Fatal(err)
	}
	good := append(append(append(ecdsaPush, pqSigPush...), secpPubPush...), pqPubPush...)
	if err := checkHybridInputPolicy(good); err != nil {
		t.Fatalf("expected valid hybrid policy pass: %v", err)
	}

	badPQSigPush, err := encodePushData(bytes.Repeat([]byte{0x44}, pqc.MLDSASignatureSize-1))
	if err != nil {
		t.Fatal(err)
	}
	badSig := append(append(append(ecdsaPush, badPQSigPush...), secpPubPush...), pqPubPush...)
	if err := checkHybridInputPolicy(badSig); err == nil {
		t.Fatalf("expected bad pq sig size rejection")
	}

	badPQPubPush, err := encodePushData(bytes.Repeat([]byte{0x55}, pqc.MLDSAPublicKeySize-1))
	if err != nil {
		t.Fatal(err)
	}
	badPub := append(append(append(ecdsaPush, pqSigPush...), secpPubPush...), badPQPubPush...)
	if err := checkHybridInputPolicy(badPub); err == nil {
		t.Fatalf("expected bad pq pubkey size rejection")
	}
}

func TestAncestorDepthCalculation(t *testing.T) {
	p := New()
	// root <- mid <- leaf
	p.entries["root"] = Entry{TxID: "root"}
	p.entries["mid"] = Entry{TxID: "mid"}
	p.entries["leaf"] = Entry{TxID: "leaf"}
	p.parents["mid"] = map[string]struct{}{"root": {}}
	p.parents["leaf"] = map[string]struct{}{"mid": {}}

	depth := p.ancestorDepthLocked("leaf", make(map[string]int))
	if depth != 2 {
		t.Fatalf("depth=%d want=2", depth)
	}
}

func TestCheckAncestorDepthLimit(t *testing.T) {
	p := New()
	// Build a linear chain of MaxAncestorDepth ancestors ending at tx "tail".
	prev := "gen"
	p.entries[prev] = Entry{TxID: prev}
	for i := 0; i < MaxAncestorDepth; i++ {
		cur := fmt.Sprintf("t%d", i)
		p.entries[cur] = Entry{TxID: cur}
		p.parents[cur] = map[string]struct{}{prev: {}}
		prev = cur
	}
	// Candidate spends output from the deepest existing mempool tx => depth=MaxAncestorDepth+1 (reject).
	if err := p.checkAncestorDepthForParentsLocked([]string{prev}); err == nil {
		t.Fatalf("expected ancestor depth rejection")
	}
}

func TestMaxAncestorDepthObserved(t *testing.T) {
	p := New()
	// root <- mid <- leaf
	p.entries["root"] = Entry{TxID: "root"}
	p.entries["mid"] = Entry{TxID: "mid"}
	p.entries["leaf"] = Entry{TxID: "leaf"}
	p.parents["mid"] = map[string]struct{}{"root": {}}
	p.parents["leaf"] = map[string]struct{}{"mid": {}}
	if got := p.MaxAncestorDepthObserved(); got != 2 {
		t.Fatalf("max ancestor depth=%d want=2", got)
	}
}

func TestSignalsOptInRBF(t *testing.T) {
	tx := &wire.MsgTx{
		TxIn: []wire.TxIn{{Sequence: 0xffffffff}},
	}
	if signalsOptInRBF(tx) {
		t.Fatalf("final sequence should not signal rbf")
	}
	tx.TxIn[0].Sequence = 0xfffffffd
	if !signalsOptInRBF(tx) {
		t.Fatalf("non-final sequence should signal rbf")
	}
}

func TestCheckReplacementPolicyDisabledForV4(t *testing.T) {
	conflictTx := &wire.MsgTx{
		TxIn:  []wire.TxIn{{Sequence: 0xfffffffd}},
		TxOut: []wire.TxOut{{Value: 1000, PkScript: []byte{script.OP_1}}},
	}
	conflict := Entry{Tx: conflictTx, TxID: "conflict", Fee: 1000, Size: 300}
	candidateTx := &wire.MsgTx{
		TxIn:  []wire.TxIn{{Sequence: 0xfffffffd}},
		TxOut: []wire.TxOut{{Value: 900, PkScript: []byte{script.OP_1}}},
	}
	candidate := Entry{Tx: candidateTx, TxID: "candidate", Fee: 1400, Size: 300}
	if err := checkReplacementPolicy(candidateTx, candidate, []Entry{conflict}, map[string]map[string]struct{}{}); err == nil || !strings.Contains(err.Error(), "RBF replacement is disabled") {
		t.Fatalf("expected disabled RBF rejection, got %v", err)
	}
}

func TestValidSignedTxAcceptedAndRemovedAfterBlock(t *testing.T) {
	chain, spend := matureSpendFixture(t)
	pool := New()
	entry, err := pool.Add(chain, spend.validTx)
	if err != nil {
		t.Fatalf("valid tx rejected: %v", err)
	}
	if pool.Count() != 1 {
		t.Fatalf("mempool count=%d want 1", pool.Count())
	}
	txHash, _ := spend.validTx.TxHash()
	if entry.TxID != txHash.String() {
		t.Fatalf("entry txid=%s want %s", entry.TxID, txHash.String())
	}
	pool.RemoveForBlock(&wire.MsgBlock{Transactions: []*wire.MsgTx{spend.validTx}})
	if pool.Count() != 0 {
		t.Fatalf("mempool not cleared after block inclusion")
	}
}

func TestDoubleSpendRejected(t *testing.T) {
	chain, spend := matureSpendFixture(t)
	pool := New()
	if _, err := pool.Add(chain, spend.validTx); err != nil {
		t.Fatalf("valid tx rejected: %v", err)
	}
	if _, err := pool.Add(chain, spend.doubleSpendTx); err == nil || !strings.Contains(err.Error(), "input already spent") {
		t.Fatalf("expected double-spend/RBF rejection, got %v", err)
	}
}

func TestInvalidSignatureRejected(t *testing.T) {
	chain, spend := matureSpendFixture(t)
	pool := New()
	if _, err := pool.Add(chain, spend.badSigTx); err == nil {
		t.Fatalf("expected invalid signature rejection")
	}
}

type matureSpend struct {
	validTx       *wire.MsgTx
	doubleSpendTx *wire.MsgTx
	badSigTx      *wire.MsgTx
}

func matureSpendFixture(t *testing.T) (*blockchain.Chain, matureSpend) {
	t.Helper()
	store := storage.NewFileStore(t.TempDir())
	fundingKey, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	fundingScript, err := script.PayToPubKeyHashScript(script.Hash160(fundingKey.PubKey().SerializeCompressed()))
	if err != nil {
		t.Fatal(err)
	}
	fundingHash := chainhash.DoubleHashB([]byte("mature mempool funding output"))
	block := &wire.MsgBlock{Header: wire.BlockHeader{Version: 1, Timestamp: 1, Bits: 1, Nonce: 1}}
	blockHash, err := block.Header.Hash()
	if err != nil {
		t.Fatal(err)
	}
	idx := blockchain.BlockIndex{Height: int32(chaincfg.CoinbaseMaturity), Hash: blockHash.String(), Time: 1, Bits: 1, Nonce: 1}
	utxo := blockchain.UTXOEntry{
		Key:      blockchain.OutPointKey(fundingHash.String(), 0),
		TxID:     fundingHash.String(),
		Vout:     0,
		Value:    100_000,
		PkScript: hex.EncodeToString(fundingScript),
		Height:   0,
		Coinbase: true,
	}
	if err := store.SaveBlock(block, idx, []blockchain.UTXOEntry{utxo}, nil, nil); err != nil {
		t.Fatal(err)
	}
	chain, err := blockchain.New(chaincfg.MainNet, mempoolTestHasher{}, store)
	if err != nil {
		t.Fatal(err)
	}
	valid := signedSpendTx(t, fundingKey, fundingHash, fundingScript, 99_000, 1)
	doubleSpend := signedSpendTx(t, fundingKey, fundingHash, fundingScript, 98_500, 2)
	badSig := signedSpendTx(t, fundingKey, fundingHash, fundingScript, 99_000, 3)
	badSig.TxIn[0].SignatureScript[len(badSig.TxIn[0].SignatureScript)-1] ^= 0x01
	return chain, matureSpend{validTx: valid, doubleSpendTx: doubleSpend, badSigTx: badSig}
}

type mempoolTestHasher struct{}

func (mempoolTestHasher) HashHeader(header wire.BlockHeader) (chainhash.Hash, error) {
	b, err := header.Bytes()
	if err != nil {
		return chainhash.Hash{}, err
	}
	return chainhash.DoubleHashB(b), nil
}

func signedSpendTx(t *testing.T, key *btcec.PrivateKey, fundingHash chainhash.Hash, prevScript []byte, value int64, tag uint32) *wire.MsgTx {
	t.Helper()
	dest, err := btcec.NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}
	destScript, err := script.PayToPubKeyHashScript(script.Hash160(dest.PubKey().SerializeCompressed()))
	if err != nil {
		t.Fatal(err)
	}
	tx := &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: fundingHash, Index: 0},
			Sequence:         math.MaxUint32 - tag,
		}},
		TxOut: []wire.TxOut{{Value: value, PkScript: destScript}},
	}
	sighash, err := script.SignatureHash(tx, 0, prevScript, script.SigHashAll)
	if err != nil {
		t.Fatal(err)
	}
	sig := btcecdsa.Sign(key, sighash[:]).Serialize()
	sigScript, err := script.SignatureScript(sig, key.PubKey().SerializeCompressed())
	if err != nil {
		t.Fatal(err)
	}
	tx.TxIn[0].SignatureScript = sigScript
	return tx
}
