package tokens

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"legacycoin/legacy-go/internal/script"
)

const (
	Magic       = "LTK1"
	ChunkData   = 14
	MaxChunks   = 64
	MaxJSONSize = ChunkData * MaxChunks
	MaxName     = 40
	MaxTicker   = 12
	MaxDesc     = 160
	MaxURL      = 180
)

var (
	nameRE   = regexp.MustCompile(`^[A-Za-z0-9 ._\-]{1,40}$`)
	tickerRE = regexp.MustCompile(`^[A-Z0-9]{2,12}$`)
)

type Operation struct {
	Protocol       string `json:"p"`
	Op             string `json:"op"`
	TokenID        string `json:"id,omitempty"`
	Name           string `json:"name,omitempty"`
	Ticker         string `json:"tick,omitempty"`
	Description    string `json:"desc,omitempty"`
	ImageURL       string `json:"img,omitempty"`
	Website        string `json:"web,omitempty"`
	XURL           string `json:"x,omitempty"`
	TelegramURL    string `json:"tg,omitempty"`
	DiscordURL     string `json:"discord,omitempty"`
	Creator        string `json:"creator,omitempty"`
	From           string `json:"from,omitempty"`
	To             string `json:"to,omitempty"`
	Supply         string `json:"supply,omitempty"`
	Amount         string `json:"amt,omitempty"`
	LBTCAmount     string `json:"lbtc,omitempty"`
	Decimals       int    `json:"dec,omitempty"`
	Mode           string `json:"mode,omitempty"`
	VirtualLBTC    string `json:"v_lbtc,omitempty"`
	VirtualToken   string `json:"v_token,omitempty"`
	GraduationLBTC string `json:"grad_lbtc,omitempty"`
	DeployFeeLBTC  string `json:"deploy_fee,omitempty"`
	TradeFeeBPS    int    `json:"trade_fee_bps,omitempty"`
	FeeAddress     string `json:"fee_addr,omitempty"`
}

func Normalize(op Operation, opName string) Operation {
	op.Protocol = Magic
	name := strings.ToUpper(strings.TrimSpace(opName))
	if name == "" {
		name = strings.ToUpper(strings.TrimSpace(op.Op))
	}
	op.Op = name
	if op.Op == "DEPLOY" {
		op.Op = "DEPLOY_SIMPLE"
	}
	op.Ticker = strings.ToUpper(strings.TrimSpace(op.Ticker))
	op.Name = strings.TrimSpace(op.Name)
	op.Description = strings.TrimSpace(op.Description)
	op.Mode = strings.ToLower(strings.TrimSpace(op.Mode))
	return op
}

func Validate(op Operation) error {
	if op.Protocol != Magic {
		return fmt.Errorf("bad token protocol")
	}
	switch strings.ToUpper(op.Op) {
	case "DEPLOY_SIMPLE":
		if err := validateDeploy(op); err != nil {
			return err
		}
	case "DEPLOY_CURVE":
		if err := validateDeploy(op); err != nil {
			return err
		}
		if op.VirtualLBTC == "" || op.VirtualToken == "" || op.GraduationLBTC == "" {
			return fmt.Errorf("curve deploy requires virtual LBTC reserve, virtual token reserve, and graduation threshold")
		}
		if op.TradeFeeBPS < 0 || op.TradeFeeBPS > 1000 {
			return fmt.Errorf("trade fee must be 0..1000 bps")
		}
	case "TRANSFER":
		if op.TokenID == "" || op.From == "" || op.To == "" || op.Amount == "" {
			return fmt.Errorf("transfer requires id, from, to, amount")
		}
	case "BURN":
		if op.TokenID == "" || op.From == "" || op.Amount == "" {
			return fmt.Errorf("burn requires id, from, amount")
		}
	case "BUY":
		if op.TokenID == "" || op.From == "" || op.LBTCAmount == "" {
			return fmt.Errorf("buy requires id, from, and lbtc amount")
		}
	case "SELL":
		if op.TokenID == "" || op.From == "" || op.Amount == "" {
			return fmt.Errorf("sell requires id, from, and token amount")
		}
	default:
		return fmt.Errorf("unsupported token op %q", op.Op)
	}
	return nil
}

func validateDeploy(op Operation) error {
	if !nameRE.MatchString(op.Name) {
		return fmt.Errorf("token name must be 1..%d characters using letters, numbers, spaces, dot, dash, or underscore", MaxName)
	}
	if !tickerRE.MatchString(op.Ticker) {
		return fmt.Errorf("ticker must be 2..%d uppercase letters or numbers", MaxTicker)
	}
	if len(op.Description) > MaxDesc {
		return fmt.Errorf("description is too long: max %d characters", MaxDesc)
	}
	if op.Decimals < 0 || op.Decimals > 8 {
		return fmt.Errorf("decimals must be 0..8")
	}
	if op.Creator == "" || op.Supply == "" {
		return fmt.Errorf("deploy requires creator and supply")
	}
	for label, url := range map[string]string{"image URL": op.ImageURL, "website": op.Website, "X/Twitter": op.XURL, "Telegram": op.TelegramURL, "Discord": op.DiscordURL} {
		if err := validateURL(label, url); err != nil {
			return err
		}
	}
	return nil
}

func MarkerPayloads(op Operation) ([][20]byte, []byte, error) {
	raw, err := json.Marshal(op)
	if err != nil {
		return nil, nil, err
	}
	if len(raw) > MaxJSONSize {
		return nil, nil, fmt.Errorf("token metadata too large: %d > %d", len(raw), MaxJSONSize)
	}
	total := (len(raw) + ChunkData - 1) / ChunkData
	if total == 0 {
		total = 1
	}
	if total > MaxChunks {
		return nil, nil, fmt.Errorf("too many token chunks")
	}
	out := make([][20]byte, total)
	for i := 0; i < total; i++ {
		copy(out[i][0:4], []byte(Magic))
		out[i][4] = byte(total)
		out[i][5] = byte(i)
		start := i * ChunkData
		end := start + ChunkData
		if end > len(raw) {
			end = len(raw)
		}
		copy(out[i][6:], raw[start:end])
	}
	return out, raw, nil
}

func MarkerScriptHexes(op Operation) ([]string, []byte, error) {
	chunks, raw, err := MarkerPayloads(op)
	if err != nil {
		return nil, nil, err
	}
	out := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		pk, err := script.PayToPubKeyHashScript(chunk[:])
		if err != nil {
			return nil, nil, err
		}
		out = append(out, hex.EncodeToString(pk))
	}
	return out, raw, nil
}

func validateURL(label, v string) error {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	if len(v) > MaxURL {
		return fmt.Errorf("%s is too long: max %d characters", label, MaxURL)
	}
	if !strings.HasPrefix(v, "https://") && !strings.HasPrefix(v, "http://") {
		return fmt.Errorf("%s must start with http:// or https://", label)
	}
	return nil
}
