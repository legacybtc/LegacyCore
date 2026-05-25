package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"legacycoin/legacy-go/internal/amount"
	"legacycoin/legacy-go/internal/config"
)

type rpcReq struct {
	JSONRPC string `json:"jsonrpc"`
	ID      string `json:"id"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
}

func main() {
	if err := runCLI(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

type cliOptions struct {
	RPCUser       string
	RPCPassword   string
	RPCCookieFile string
	RPCConnect    string
	RPCPort       string
	RPCURL        string
	DataDir       string
}

func runCLI(argv []string, stdout io.Writer, stderr io.Writer) error {
	opts, rest, err := parseGlobalFlags(argv)
	if err != nil {
		return fmt.Errorf("cli error: %w", err)
	}
	if len(rest) < 1 || rest[0] == "help" || rest[0] == "--help" || rest[0] == "-h" {
	        printHelp()
	        return nil
	}
	// special client-only commands
	if len(rest) > 0 && rest[0] == "getbalance" {
	        return runGetBalance(opts, stdout)
	}
	method := rest[0]

	args := rest[1:]
	params, rpcMethod, err := buildParams(method, args)
	if err != nil {
		return fmt.Errorf("cli error: %w", err)
	}
	body, _ := json.Marshal(rpcReq{JSONRPC: "2.0", ID: "cli", Method: rpcMethod, Params: params})
	url := rpcURL(opts)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("rpc request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if err := applyRPCAuth(req, opts); err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("rpc error: %w", err)
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("RPC unauthorized. Check rpcuser/rpcpassword or .cookie file.")
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("RPC HTTP error %s: %s", resp.Status, strings.TrimSpace(string(out)))
	}
	var pretty any
	if json.Unmarshal(out, &pretty) == nil {
		b, _ := json.MarshalIndent(pretty, "", "  ")
		fmt.Fprintln(stdout, string(b))
		return nil
	}
	fmt.Fprintln(stdout, string(out))
	return nil
	}

func rpcCall(opts cliOptions, method string, params []any) ([]byte, error) {
	body, _ := json.Marshal(rpcReq{JSONRPC: "2.0", ID: "cli", Method: method, Params: params})
	url := rpcURL(opts)
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
	        return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if err := applyRPCAuth(req, opts); err != nil {
	        return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
	        return nil, err
	}
	defer resp.Body.Close()
	out, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
	        return out, fmt.Errorf("RPC HTTP error %s: %s", resp.Status, strings.TrimSpace(string(out)))
	}
	return out, nil
	}

	// runGetBalance aggregates mature and immature balances per address.
func runGetBalance(opts cliOptions, stdout io.Writer) error {
	// fetch addresses
	addOut, err := rpcCall(opts, "listaddresses", nil)
	if err != nil {
	        return fmt.Errorf("rpc listaddresses: %w", err)
	}
	var addrs []string
	if err := json.Unmarshal(addOut, &addrs); err != nil {
	        // try to parse standard RPC envelope
	        var env struct{ Result any `json:"result"` }
	        if json.Unmarshal(addOut, &env) == nil {
	                if res, ok := env.Result.([]any); ok {
	                        for _, v := range res {
	                                if s, ok := v.(string); ok {
	                                        addrs = append(addrs, s)
	                                }
	                        }
	                }
	        }
	}
	// fetch unspent
	unOut, err := rpcCall(opts, "listunspent", nil)
	if err != nil {
	        return fmt.Errorf("rpc listunspent: %w", err)
	}
	var utxos []map[string]any
	if err := json.Unmarshal(unOut, &utxos); err != nil {
	        var env struct{ Result any `json:"result"` }
	        if json.Unmarshal(unOut, &env) == nil {
	                if res, ok := env.Result.([]any); ok {
	                        for _, it := range res {
	                                if m, ok := it.(map[string]any); ok {
	                                        utxos = append(utxos, m)
	                                }
	                        }
	                }
	        }
	}

	// aggregate
	type Bal struct {
	        Mature   int64 `json:"mature_base_units"`
	        Immature int64 `json:"immature_base_units"`
	}
	balances := make(map[string]Bal)
	// initialize addresses
	for _, a := range addrs {
	        balances[a] = Bal{}
	}
	// coinbase maturity (best-effort): use 100
	const coinbaseMaturity = 100
	for _, u := range utxos {
	        addr, _ := u["address"].(string)
	        // value parsed as float64 from JSON numbers
	        var val int64
	        if f, ok := u["value"].(float64); ok {
	                val = int64(f)
	        } else if n, ok := u["value"].(int64); ok {
	                val = n
	        }
	        coinbase := false
	        if cb, ok := u["coinbase"].(bool); ok {
	                coinbase = cb
	        }
	        confs := int64(0)
	        if c, ok := u["confirmations"].(float64); ok {
	                confs = int64(c)
	        }
	        b := balances[addr]
	        if coinbase && confs > 0 && confs < coinbaseMaturity {
	                b.Immature += val
	        } else {
	                b.Mature += val
	        }
	        balances[addr] = b
	}

	// include addresses with zero balance
	outMap := make(map[string]map[string]any)
	for addr, b := range balances {
	        outMap[addr] = map[string]any{
	                "mature_base_units":   b.Mature,
	                "immature_base_units": b.Immature,
	                "mature_lbtc":         amount.FormatWithTicker(b.Mature),
	                "immature_lbtc":       amount.FormatWithTicker(b.Immature),
	                "total_base_units":    b.Mature + b.Immature,
	                "total_lbtc":          amount.FormatWithTicker(b.Mature + b.Immature),
	        }
	}
	b, _ := json.MarshalIndent(outMap, "", "  ")
	fmt.Fprintln(stdout, string(b))
	return nil
	}

func parseGlobalFlags(args []string) (cliOptions, []string, error) {

	var opts cliOptions
	rest := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "--" {
			if arg == "--" {
				rest = append(rest, args[i+1:]...)
			} else {
				rest = append(rest, args[i:]...)
			}
			break
		}
		key, val, hasEq := strings.Cut(arg, "=")
		needsValue := func() (string, error) {
			if hasEq {
				return val, nil
			}
			i++
			if i >= len(args) {
				return "", fmt.Errorf("%s requires value", key)
			}
			return args[i], nil
		}
		var err error
		switch key {
		case "-rpcuser", "--rpcuser":
			opts.RPCUser, err = needsValue()
		case "-rpcpassword", "--rpcpassword":
			opts.RPCPassword, err = needsValue()
		case "-rpccookiefile", "--rpccookiefile":
			opts.RPCCookieFile, err = needsValue()
		case "-rpcconnect", "--rpcconnect":
			opts.RPCConnect, err = needsValue()
		case "-rpcport", "--rpcport":
			opts.RPCPort, err = needsValue()
		case "-rpcurl", "--rpcurl":
			opts.RPCURL, err = needsValue()
		case "-datadir", "--datadir":
			opts.DataDir, err = needsValue()
		default:
			rest = append(rest, args[i:]...)
			return opts, rest, nil
		}
		if err != nil {
			return opts, nil, err
		}
	}
	return opts, rest, nil
}

func rpcURL(opts cliOptions) string {
	if opts.RPCURL != "" {
		return opts.RPCURL
	}
	if env := strings.TrimSpace(os.Getenv("LEGACYCOIN_RPC_URL")); env != "" {
		return env
	}
	host := opts.RPCConnect
	if host == "" {
		host = "127.0.0.1"
	}
	port := opts.RPCPort
	if port == "" {
		port = "19556"
	}
	return "http://" + host + ":" + port
}

func applyRPCAuth(req *http.Request, opts cliOptions) error {
	if opts.RPCUser != "" || opts.RPCPassword != "" {
		if opts.RPCUser == "" || opts.RPCPassword == "" {
			return fmt.Errorf("RPC auth requires both -rpcuser and -rpcpassword.")
		}
		req.SetBasicAuth(opts.RPCUser, opts.RPCPassword)
		return nil
	}
	cookiePath := opts.RPCCookieFile
	if cookiePath == "" {
		dataDir := opts.DataDir
		if dataDir == "" {
			dataDir = config.DefaultDataDir()
		}
		cookiePath = config.CookiePathForDataDir(dataDir)
	}
	auth, err := loadCookieFile(cookiePath)
	if err != nil {
		return fmt.Errorf("RPC cookie not found. Start legacycoind first or configure rpcuser/rpcpassword. Looked for: %s", cookiePath)
	}
	req.SetBasicAuth(auth.User, auth.Password)
	return nil
}

func loadCookieFile(path string) (config.RPCAuth, error) {
	if strings.HasPrefix(path, "~") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, strings.TrimPrefix(strings.TrimPrefix(path, "~"), string(os.PathSeparator)))
		}
	}
	data, err := os.ReadFile(os.ExpandEnv(path))
	if err != nil {
		return config.RPCAuth{}, err
	}
	parts := strings.SplitN(strings.TrimSpace(string(data)), ":", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return config.RPCAuth{}, fmt.Errorf("invalid rpc cookie")
	}
	return config.RPCAuth{User: parts[0], Password: parts[1], Enabled: true}, nil
}

func printHelp() {
	fmt.Println("legacycoin-cli <method> [params...]")
	fmt.Println("global flags:")
	fmt.Println("  -datadir=<path>        data directory containing .cookie")
	fmt.Println("  -rpcuser=<user>        explicit RPC username")
	fmt.Println("  -rpcpassword=<pass>    explicit RPC password")
	fmt.Println("  -rpccookiefile=<path>  explicit RPC cookie path")
	fmt.Println("  -rpcconnect=<host>     RPC host, default 127.0.0.1")
	fmt.Println("  -rpcport=<port>        RPC port, default 19556")
	fmt.Println("examples:")
	fmt.Println("  legacycoin-cli getblockcount")
	fmt.Println("  legacycoin-cli -datadir=/home/user/.legacycoin getnetworkinfo")
	fmt.Println("  legacycoin-cli -rpcuser=legacyrpc -rpcpassword=strong_password getnetworkinfo")
	fmt.Println("  legacycoin-cli getwalletsummary")
	fmt.Println("  legacycoin-cli setupwallet my-test-passphrase")
	fmt.Println("  legacycoin-cli generate <pubkey_hash_hex> 1")
	fmt.Println("  legacycoin-cli sendtoaddress <address> 1 --yes")
	fmt.Println("  legacycoin-cli sendtoaddress <address> 0.00000546 --yes")
	fmt.Println("  legacycoin-cli sendtoaddress <address> 100000000 --base-units --yes")
	fmt.Println("  legacycoin-cli getbalance")
	fmt.Println()
	fmt.Println("RPC commands (grouped, short descriptions):")
	fmt.Println("  Node: getinfo, gethealth, getreadiness, getselfcheck, getblockchaininfo, getchainparams, getbootstrapinfo")
	fmt.Println("  Network: getnetworkinfo, getpeerinfo, getconnectioncount, addnode, disconnectnode, getnetworkhashps, getchaintiming")
	fmt.Println("  Mining: getmininginfo, getminerstatus, startminer, stopminer, restartminer, benchmarkminer, autotuneminer, setminerthreads, configureminer, generate, getblocktemplate, submitblock")
	fmt.Println("  Blocks: getblockcount, getbestblockhash, getblockhash, getblock, gettxout, gettxoutsetinfo, getblocklocator, disconnecttip")
	fmt.Println("  Mempool: getrawmempool, getmempoolinfo, getmempoolentry, sendrawtransaction")
	fmt.Println("  Wallet: setupwallet, getminingaddress, setminingaddress, getwalletsummary, getwalletinfo, listaddresses, listunspent, listimmature, getbalance (CLI helper), backupwallet, dumpwallet")
	fmt.Println("  Keys: dumpprivkey, importprivkey")
	fmt.Println("  Transactions: sendtoaddress, sendtoaddressraw, sendfromaddress, sendfromaddressraw, sendrawtransaction")
	fmt.Println("  Tokens: sendtokendeploy, sendtokendeploycurve, sendtokentransfer, sendtokenburn, sendtokenbuy, sendtokensell")
	fmt.Println("  Utilities: tobaseunits, frombaseunits, encryptwallet, walletpassphrase, walletlock, sethdseed, stop, doctor")
	}
func buildParams(method string, args []string) ([]any, string, error) {
	switch method {
	case "walletpassphrase":
		if len(args) != 2 {
			return nil, method, fmt.Errorf("walletpassphrase expects <passphrase> <timeout_seconds>")
		}
		timeout, err := strconv.Atoi(args[1])
		if err != nil || timeout <= 0 {
			return nil, method, fmt.Errorf("walletpassphrase timeout must be a positive integer")
		}
		return []any{args[0], timeout}, method, nil
	case "sendtoaddress":
		return buildSendToAddress(method, args)
	case "sendfromaddress":
		return buildSendFromAddress(method, args)
	default:
		params := make([]any, 0, len(args))
		for _, arg := range args {
			params = append(params, parseParam(arg))
		}
		return params, method, nil
	}
}

func buildSendToAddress(method string, args []string) ([]any, string, error) {
	clean, yes, baseUnits := splitFlags(args)
	if len(clean) < 2 || len(clean) > 3 {
		return nil, method, fmt.Errorf("sendtoaddress expects <address> <amount_lbtc> [fee_lbtc] [--yes] [--base-units]")
	}
	addr := clean[0]
	amountText := clean[1]
	feeText := "0.00001000"
	if baseUnits {
		feeText = "1000"
	}
	if len(clean) == 3 {
		feeText = clean[2]
	}
	sendValue, feeValue, err := parseCLIAmounts(amountText, feeText, baseUnits)
	if err != nil {
		return nil, method, err
	}
	if !yes {
		if err := confirmSend(addr, sendValue, feeValue, baseUnits); err != nil {
			return nil, method, err
		}
	}
	rpcMethod := method
	if baseUnits {
		rpcMethod = method + "raw"
	}
	return []any{addr, amountText, feeText}, rpcMethod, nil
}

func buildSendFromAddress(method string, args []string) ([]any, string, error) {
	clean, yes, baseUnits := splitFlags(args)
	if len(clean) < 3 || len(clean) > 4 {
		return nil, method, fmt.Errorf("sendfromaddress expects <from> <to> <amount_lbtc> [fee_lbtc] [--yes] [--base-units]")
	}
	from := clean[0]
	to := clean[1]
	amountText := clean[2]
	feeText := "0.00001000"
	if baseUnits {
		feeText = "1000"
	}
	if len(clean) == 4 {
		feeText = clean[3]
	}
	sendValue, feeValue, err := parseCLIAmounts(amountText, feeText, baseUnits)
	if err != nil {
		return nil, method, err
	}
	if !yes {
		fmt.Printf("From:\n  %s\n", from)
		if err := confirmSend(to, sendValue, feeValue, baseUnits); err != nil {
			return nil, method, err
		}
	}
	rpcMethod := method
	if baseUnits {
		rpcMethod = method + "raw"
	}
	return []any{from, to, amountText, feeText}, rpcMethod, nil
}

func splitFlags(args []string) (clean []string, yes bool, baseUnits bool) {
	for _, a := range args {
		switch a {
		case "--yes", "-y":
			yes = true
		case "--base-units", "--raw-units":
			baseUnits = true
		default:
			clean = append(clean, a)
		}
	}
	return clean, yes, baseUnits
}

func parseCLIAmounts(sendText, feeText string, baseUnits bool) (int64, int64, error) {
	if baseUnits {
		sendValue, err := amount.ParseBaseUnits(sendText)
		if err != nil {
			return 0, 0, fmt.Errorf("bad base-unit amount: %w", err)
		}
		feeValue, err := amount.ParseBaseUnits(feeText)
		if err != nil {
			return 0, 0, fmt.Errorf("bad base-unit fee: %w", err)
		}
		return sendValue, feeValue, nil
	}
	sendValue, err := amount.ParseLBTC(sendText)
	if err != nil {
		return 0, 0, fmt.Errorf("bad LBTC amount: %w", err)
	}
	feeValue, err := amount.ParseLBTC(feeText)
	if err != nil {
		return 0, 0, fmt.Errorf("bad LBTC fee: %w", err)
	}
	return sendValue, feeValue, nil
}

func confirmSend(addr string, sendValue int64, feeValue int64, baseUnits bool) error {
	fmt.Println("You are about to send:")
	fmt.Println()
	fmt.Printf("  Amount: %s\n", amount.FormatWithTicker(sendValue))
	fmt.Printf("  Fee:    %s\n", amount.FormatWithTicker(feeValue))
	fmt.Printf("  Total:  %s\n", amount.FormatWithTicker(sendValue+feeValue))
	fmt.Printf("  Base units: amount=%d fee=%d total=%d\n", sendValue, feeValue, sendValue+feeValue)
	if baseUnits {
		fmt.Println("  Mode:   explicit base units")
	} else {
		fmt.Println("  Mode:   human LBTC amount")
	}
	fmt.Println()
	fmt.Println("To:")
	fmt.Printf("  %s\n\n", addr)
	fmt.Print("Type YES to broadcast: ")
	reader := bufio.NewReader(os.Stdin)
	line, _ := reader.ReadString('\n')
	if strings.TrimSpace(line) != "YES" {
		return fmt.Errorf("send cancelled")
	}
	return nil
}

func parseParam(s string) any {
	var v any
	if json.Unmarshal([]byte(s), &v) == nil {
		return v
	}
	return s
}
