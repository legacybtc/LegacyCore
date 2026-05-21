package chaincfg

import "testing"

func TestBlockSubsidy(t *testing.T) {
	tests := []struct {
		height int32
		want   int64
	}{
		{0, 50 * Coin},
		{209999, 50 * Coin},
		{210000, 25 * Coin},
		{420000, 12_50000000},
	}
	for _, tc := range tests {
		got := BlockSubsidy(tc.height)
		if got != tc.want {
			t.Fatalf("height %d subsidy=%d want=%d", tc.height, got, tc.want)
		}
	}
}

func TestMainNetParams(t *testing.T) {
	if MainNet.DefaultPort != 19555 {
		t.Fatalf("p2p port=%d", MainNet.DefaultPort)
	}
	if MainNet.RPCPort != 19556 {
		t.Fatalf("rpc port=%d", MainNet.RPCPort)
	}
	if MainNet.YespowerPers != "LegacyCoinPoW" {
		t.Fatalf("yespower pers=%q", MainNet.YespowerPers)
	}
	if MainNet.ChainID != "legacy-mainnet-1.0.0-rc2-5b4c78e4" {
		t.Fatalf("chain id=%q", MainNet.ChainID)
	}
	if MainNet.MessageStart != [4]byte{0xa4, 0xac, 0xc6, 0x4d} {
		t.Fatalf("message start=%x", MainNet.MessageStart)
	}
	if MainNet.GenesisTime != 1779235200 || MainNet.GenesisNonce != 3 {
		t.Fatalf("genesis time/nonce=%d/%d", MainNet.GenesisTime, MainNet.GenesisNonce)
	}
	if MainNet.GenesisHash != "5b4c78e4556afcd51acf7b9eb2e387fbea2d1414e6042d80d38e6256987154f5" {
		t.Fatalf("genesis hash=%q", MainNet.GenesisHash)
	}
}
