package amount

import (
	"fmt"
	"strconv"
	"strings"

	"legacycoin/legacy-go/internal/chaincfg"
)

// ParseLBTC converts a human LBTC amount string (for example "1", "0.5",
// "0.00000546") into base units. It is deliberately strict: no scientific
// notation, no commas, no negative values, and at most 8 decimal places.
func ParseLBTC(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty amount")
	}
	s = strings.TrimPrefix(s, "+")
	if strings.HasPrefix(s, "-") {
		return 0, fmt.Errorf("amount must be positive")
	}
	if strings.ContainsAny(s, "eE,") {
		return 0, fmt.Errorf("amount must be a plain decimal LBTC value")
	}
	parts := strings.Split(s, ".")
	if len(parts) > 2 {
		return 0, fmt.Errorf("bad decimal amount")
	}
	wholePart := parts[0]
	fracPart := ""
	if len(parts) == 2 {
		fracPart = parts[1]
	}
	if wholePart == "" {
		wholePart = "0"
	}
	if fracPart == "" && len(parts) == 2 {
		fracPart = "0"
	}
	if len(fracPart) > 8 {
		return 0, fmt.Errorf("too many decimals: LBTC supports 8 decimal places")
	}
	for _, r := range wholePart + fracPart {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("amount must contain only digits and one decimal point")
		}
	}
	whole, err := strconv.ParseInt(wholePart, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("bad whole amount: %w", err)
	}
	fracPadded := fracPart + strings.Repeat("0", 8-len(fracPart))
	frac := int64(0)
	if fracPadded != "" {
		frac, err = strconv.ParseInt(fracPadded, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("bad fractional amount: %w", err)
		}
	}
	if whole > chaincfg.MaxMoney/chaincfg.Coin {
		return 0, fmt.Errorf("amount out of range")
	}
	value := whole*chaincfg.Coin + frac
	if value <= 0 {
		return 0, fmt.Errorf("amount must be greater than 0")
	}
	if !chaincfg.MoneyRange(value) {
		return 0, fmt.Errorf("amount out of range")
	}
	return value, nil
}

// ParseBaseUnits parses an explicit raw/base-unit amount.
func ParseBaseUnits(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" || strings.HasPrefix(s, "-") || strings.ContainsAny(s, ".eE,+") {
		return 0, fmt.Errorf("base-unit amount must be a positive integer")
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0, err
	}
	if v <= 0 || !chaincfg.MoneyRange(v) {
		return 0, fmt.Errorf("base-unit amount out of range")
	}
	return v, nil
}

// FormatLBTC returns a fixed 8-decimal LBTC string without ticker.
func FormatLBTC(v int64) string {
	sign := ""
	if v < 0 {
		sign = "-"
		v = -v
	}
	whole := v / chaincfg.Coin
	frac := v % chaincfg.Coin
	return fmt.Sprintf("%s%d.%08d", sign, whole, frac)
}

func FormatWithTicker(v int64) string { return FormatLBTC(v) + " LBTC" }
