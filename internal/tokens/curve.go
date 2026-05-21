package tokens

import (
	"fmt"
	"math/big"
)

const (
	DefaultCurveSupply    = "1000000000"
	DefaultVirtualLBTC    = "30"
	DefaultVirtualToken   = "1000000000"
	DefaultGraduationLBTC = "500"
	DefaultDeployFeeLBTC  = "0.02"
	DefaultTradeFeeBPS    = 100
	DefaultSlippageBPS    = 500
)

type CurveQuote struct {
	TokensOut          string `json:"tokens_out,omitempty"`
	LBTCOut            string `json:"lbtc_out,omitempty"`
	NetLBTCIn          string `json:"net_lbtc_in,omitempty"`
	TradeFeeLBTC       string `json:"trade_fee_lbtc,omitempty"`
	PriceBefore        string `json:"price_before,omitempty"`
	PriceAfter         string `json:"price_after,omitempty"`
	GraduationProgress string `json:"graduation_progress,omitempty"`
}

func QuoteBuy(virtualLBTC, tokenReserve, realLBTC, lbtcIn string, feeBPS int, graduationLBTC string) (CurveQuote, error) {
	x, err := parseRat(virtualLBTC)
	if err != nil {
		return CurveQuote{}, fmt.Errorf("bad virtual LBTC reserve: %w", err)
	}
	real, err := parseRat(realLBTC)
	if err != nil {
		return CurveQuote{}, fmt.Errorf("bad real LBTC reserve: %w", err)
	}
	y, err := parseRat(tokenReserve)
	if err != nil {
		return CurveQuote{}, fmt.Errorf("bad token reserve: %w", err)
	}
	in, err := parseRat(lbtcIn)
	if err != nil {
		return CurveQuote{}, fmt.Errorf("bad LBTC input: %w", err)
	}
	if in.Sign() <= 0 || y.Sign() <= 0 {
		return CurveQuote{}, fmt.Errorf("LBTC input and token reserve must be greater than zero")
	}
	if feeBPS < 0 || feeBPS > 1000 {
		return CurveQuote{}, fmt.Errorf("trade fee must be 0..1000 bps")
	}
	fee := new(big.Rat).Mul(in, big.NewRat(int64(feeBPS), 10000))
	netIn := new(big.Rat).Sub(in, fee)
	x.Add(x, real)
	k := new(big.Rat).Mul(x, y)
	newX := new(big.Rat).Add(x, netIn)
	newY := new(big.Rat).Quo(k, newX)
	out := new(big.Rat).Sub(y, newY)
	progress := "0"
	if grad, err := parseRat(graduationLBTC); err == nil && grad.Sign() > 0 {
		progress = ratString(new(big.Rat).Mul(new(big.Rat).Quo(new(big.Rat).Add(real, netIn), grad), big.NewRat(100, 1)))
	}
	return CurveQuote{
		TokensOut:          ratString(out),
		NetLBTCIn:          ratString(netIn),
		TradeFeeLBTC:       ratString(fee),
		PriceBefore:        ratString(new(big.Rat).Quo(x, y)),
		PriceAfter:         ratString(new(big.Rat).Quo(newX, newY)),
		GraduationProgress: progress,
	}, nil
}

func QuoteSell(virtualLBTC, tokenReserve, realLBTC, tokenIn string, feeBPS int) (CurveQuote, error) {
	x, err := parseRat(virtualLBTC)
	if err != nil {
		return CurveQuote{}, fmt.Errorf("bad virtual LBTC reserve: %w", err)
	}
	real, err := parseRat(realLBTC)
	if err != nil {
		return CurveQuote{}, fmt.Errorf("bad real LBTC reserve: %w", err)
	}
	y, err := parseRat(tokenReserve)
	if err != nil {
		return CurveQuote{}, fmt.Errorf("bad token reserve: %w", err)
	}
	in, err := parseRat(tokenIn)
	if err != nil {
		return CurveQuote{}, fmt.Errorf("bad token input: %w", err)
	}
	if in.Sign() <= 0 || y.Sign() <= 0 {
		return CurveQuote{}, fmt.Errorf("token input and token reserve must be greater than zero")
	}
	x.Add(x, real)
	k := new(big.Rat).Mul(x, y)
	newY := new(big.Rat).Add(y, in)
	newX := new(big.Rat).Quo(k, newY)
	grossOut := new(big.Rat).Sub(x, newX)
	fee := new(big.Rat).Mul(grossOut, big.NewRat(int64(feeBPS), 10000))
	netOut := new(big.Rat).Sub(grossOut, fee)
	if netOut.Cmp(real) > 0 {
		return CurveQuote{}, fmt.Errorf("sell quote exceeds real LBTC reserve; payout is not safely enforceable")
	}
	return CurveQuote{
		LBTCOut:      ratString(netOut),
		TradeFeeLBTC: ratString(fee),
		PriceBefore:  ratString(new(big.Rat).Quo(x, y)),
		PriceAfter:   ratString(new(big.Rat).Quo(newX, newY)),
	}, nil
}

func parseRat(v string) (*big.Rat, error) {
	r, ok := new(big.Rat).SetString(v)
	if !ok {
		return nil, fmt.Errorf("invalid decimal %q", v)
	}
	return r, nil
}

func ratString(v *big.Rat) string {
	return v.FloatString(8)
}
