package rpc

import (
	"encoding/hex"
	"encoding/json"
	"math"
	"testing"
	"time"

	"legacycoin/legacy-go/internal/blockchain"
	"legacycoin/legacy-go/internal/chaincfg"
	"legacycoin/legacy-go/internal/chainhash"
	"legacycoin/legacy-go/internal/script"
	"legacycoin/legacy-go/internal/storage"
	"legacycoin/legacy-go/internal/wire"
)

type rpcProposalHasher struct{}

func (rpcProposalHasher) HashHeader(h wire.BlockHeader) (chainhash.Hash, error) {
	var out chainhash.Hash
	out[0] = 0x00
	out[1] = byte(h.Nonce)
	out[2] = byte(h.Nonce >> 8)
	out[3] = byte(h.Nonce >> 16)
	out[4] = byte(h.Nonce >> 24)
	out[5] = byte(h.Timestamp)
	out[6] = byte(h.Timestamp >> 8)
	out[7] = byte(h.Timestamp >> 16)
	out[8] = byte(h.Timestamp >> 24)
	return out, nil
}

func TestValidateBlockProposalDiagnosticRejectsOverpayWithoutMutation(t *testing.T) {
	chain, err := blockchain.New(chaincfg.MainNet, rpcProposalHasher{}, storage.NewFileStore(t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	var zero chainhash.Hash
	genesisLike, err := rpcProposalBlock(zero, 0, 801)
	if err != nil {
		t.Fatal(err)
	}
	if err := chain.ProcessBlock(genesisLike); err != nil {
		t.Fatal(err)
	}
	genesisHash, err := chain.BlockHash(genesisLike)
	if err != nil {
		t.Fatal(err)
	}
	tipBefore := chain.Tip()
	overpay, err := rpcProposalBlock(genesisHash, 1, 802)
	if err != nil {
		t.Fatal(err)
	}
	overpay.Transactions[0].TxOut[0].Value = chaincfg.BlockSubsidy(1) + 1
	rpcRefreshMerkleRoot(t, overpay)
	overpayHash, err := chain.BlockHash(overpay)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := overpay.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	params, err := json.Marshal([]string{hex.EncodeToString(raw)})
	if err != nil {
		t.Fatal(err)
	}

	s := &Server{chain: chain}
	result, rpcErr := s.submitBlockDiagnostic(json.RawMessage(params), false)
	if rpcErr != nil {
		t.Fatalf("submitBlockDiagnostic rpc error: %v", rpcErr)
	}
	out, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result type=%T", result)
	}
	if out["accepted"] != false || out["would_accept"] != false {
		t.Fatalf("overpay proposal should not be accepted: %+v", out)
	}
	if out["reject_code"] != "bad-cb-amount" {
		t.Fatalf("reject_code=%v want bad-cb-amount", out["reject_code"])
	}
	if out["reject_category"] != "bad-coinbase" {
		t.Fatalf("reject_category=%v want bad-coinbase", out["reject_category"])
	}
	processResult, ok := out["processblock_result"].(blockchain.BlockProcessResult)
	if !ok {
		t.Fatalf("processblock_result type=%T", out["processblock_result"])
	}
	if processResult.Status != blockchain.BlockStatusRejected {
		t.Fatalf("process status=%q want %q", processResult.Status, blockchain.BlockStatusRejected)
	}
	if chain.HasBlock(overpayHash.String()) {
		t.Fatalf("validateblockproposal stored rejected block")
	}
	if tip := chain.Tip(); tip.Hash != tipBefore.Hash || tip.Height != tipBefore.Height {
		t.Fatalf("tip changed after validateblockproposal: got=%+v want=%+v", tip, tipBefore)
	}
}

func rpcProposalBlock(prev chainhash.Hash, height int32, nonce uint32) (*wire.MsgBlock, error) {
	coinbase, err := rpcProposalCoinbaseTx(height)
	if err != nil {
		return nil, err
	}
	block := &wire.MsgBlock{
		Header: wire.BlockHeader{
			Version:   1,
			PrevBlock: prev,
			Timestamp: uint32(time.Now().UTC().Unix()-100_000) + nonce,
			Bits:      rpcProposalBitsForHeight(height),
			Nonce:     nonce,
		},
		Transactions: []*wire.MsgTx{coinbase},
	}
	root, err := block.BuildMerkleRoot()
	if err != nil {
		return nil, err
	}
	block.Header.MerkleRoot = root
	return block, nil
}

func rpcProposalBitsForHeight(height int32) uint32 {
	if height <= 0 {
		return chaincfg.MainNet.GenesisBits
	}
	return chaincfg.MainNet.PostGenesisBits
}

func rpcProposalCoinbaseTx(height int32) (*wire.MsgTx, error) {
	pubHash := make([]byte, 20)
	pubHash[0] = 0x24
	pkScript, err := script.PayToPubKeyHashScript(pubHash)
	if err != nil {
		return nil, err
	}
	heightBytes := []byte{byte(height), byte(height >> 8), byte(height >> 16), byte(height >> 24)}
	sigScript := append([]byte{byte(len(heightBytes))}, heightBytes...)
	sigScript = append(sigScript, []byte("/Legacy-GO-RPCProposalTest/")...)
	return &wire.MsgTx{
		Version: 1,
		TxIn: []wire.TxIn{{
			PreviousOutPoint: wire.OutPoint{Hash: chainhash.Hash{}, Index: math.MaxUint32},
			SignatureScript:  sigScript,
			Sequence:         math.MaxUint32,
		}},
		TxOut: []wire.TxOut{{
			Value:    chaincfg.BlockSubsidy(height),
			PkScript: pkScript,
		}},
	}, nil
}

func rpcRefreshMerkleRoot(t *testing.T, block *wire.MsgBlock) {
	t.Helper()
	root, err := block.BuildMerkleRoot()
	if err != nil {
		t.Fatal(err)
	}
	block.Header.MerkleRoot = root
}
