package tokens

import "testing"

func TestValidateV03Operations(t *testing.T) {
	deploy := Normalize(Operation{
		Protocol: Magic, Op: "DEPLOY_CURVE", Name: "Legacy Dog", Ticker: "LDOG",
		Creator: "Lcreator", Supply: DefaultCurveSupply, Decimals: 0,
		VirtualLBTC: DefaultVirtualLBTC, VirtualToken: DefaultVirtualToken,
		GraduationLBTC: DefaultGraduationLBTC, TradeFeeBPS: DefaultTradeFeeBPS,
	}, "")
	if err := Validate(deploy); err != nil {
		t.Fatalf("deploy curve should validate: %v", err)
	}
	buy := Normalize(Operation{Protocol: Magic, Op: "BUY", TokenID: "txid", From: "Lbuyer", LBTCAmount: "1"}, "")
	if err := Validate(buy); err != nil {
		t.Fatalf("buy should validate: %v", err)
	}
	sell := Normalize(Operation{Protocol: Magic, Op: "SELL", TokenID: "txid", From: "Lseller", Amount: "100"}, "")
	if err := Validate(sell); err != nil {
		t.Fatalf("sell schema should validate: %v", err)
	}
}

func TestDeployAliasNormalizesToSimple(t *testing.T) {
	op := Normalize(Operation{Protocol: Magic, Op: "DEPLOY", Name: "Legacy Cat", Ticker: "LCAT", Creator: "Lcreator", Supply: "1000"}, "")
	if op.Op != "DEPLOY_SIMPLE" {
		t.Fatalf("DEPLOY alias normalized to %q", op.Op)
	}
	if err := Validate(op); err != nil {
		t.Fatalf("deploy alias should validate: %v", err)
	}
}

func TestCurveBuyQuote(t *testing.T) {
	q, err := QuoteBuy(DefaultVirtualLBTC, DefaultVirtualToken, "0", "1", DefaultTradeFeeBPS, DefaultGraduationLBTC)
	if err != nil {
		t.Fatalf("quote buy: %v", err)
	}
	if q.TokensOut == "" || q.NetLBTCIn != "0.99000000" || q.TradeFeeLBTC != "0.01000000" {
		t.Fatalf("unexpected quote: %+v", q)
	}
}

func TestCurveSellQuoteRequiresRealReserve(t *testing.T) {
	_, err := QuoteSell(DefaultVirtualLBTC, DefaultVirtualToken, "0", "1000", DefaultTradeFeeBPS)
	if err == nil {
		t.Fatalf("sell without real reserve should fail")
	}
}
