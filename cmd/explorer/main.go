package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"log"
	"math"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"legacycoin/legacy-go/internal/amount"
	"legacycoin/legacy-go/internal/config"
	"legacycoin/legacy-go/internal/version"
)

var (
	rpcUser     string
	rpcPassword string
	rpcHost     string
	rpcPort     string
	listenAddr  string
	mainnet     bool

	nodeClient *http.Client
	cache      = &lruCache{data: make(map[string]*cacheEntry)}
)

type cacheEntry struct {
	data      any
	expiresAt time.Time
}

type lruCache struct {
	mu   sync.Mutex
	data map[string]*cacheEntry
	keys []string
}

func (c *lruCache) get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[key]
	if !ok || time.Now().After(e.expiresAt) {
		if ok {
			delete(c.data, key)
		}
		return nil, false
	}
	return e.data, true
}

func (c *lruCache) set(key string, val any, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.data[key] = &cacheEntry{data: val, expiresAt: time.Now().Add(ttl)}
	c.keys = append(c.keys, key)
	if len(c.keys) > 1000 {
		delete(c.data, c.keys[0])
		c.keys = c.keys[1:]
	}
}

func rpcCall(method string, params []any) (any, error) {
	body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": method, "params": params})
	req, err := http.NewRequest("POST", fmt.Sprintf("http://%s:%s", rpcHost, rpcPort), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(rpcUser, rpcPassword)
	req.Header.Set("Content-Type", "application/json")
	resp, err := nodeClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	var rpcResp struct {
		Result any            `json:"result"`
		Error  map[string]any `json:"error"`
	}
	if err := json.Unmarshal(b, &rpcResp); err != nil {
		return nil, fmt.Errorf("rpc decode: %w", err)
	}
	if rpcResp.Error != nil {
		return nil, fmt.Errorf("rpc error: %v", rpcResp.Error["message"])
	}
	return rpcResp.Result, nil
}

type pageData struct {
	Title    string
	Mainnet  bool
	CoinName string
	Version  string
	Content  template.HTML
	Error    string
}

var funcMap = template.FuncMap{
	"sub": func(a, b int) int { return a - b },
	"add": func(a, b int) int { return a + b },
}

var pageTmpl = template.Must(template.New("page").Funcs(funcMap).Parse(`<!DOCTYPE html>
<html lang="en"><head>
<meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}} - {{.CoinName}} Explorer</title>
<style>
*{margin:0;padding:0;box-sizing:border-box}
body{font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,sans-serif;background:#0b0e14;color:#e0e0e0;min-height:100vh}
.header{background:#151b24;border-bottom:1px solid #2a3548;padding:16px 24px;display:flex;align-items:center;justify-content:space-between;flex-wrap:wrap;gap:12px}
.header h1{font-size:20px;color:#ffd700}
.header h1 a{color:#ffd700;text-decoration:none}
.header .nav{display:flex;gap:12px}
.header .nav a{color:#8ab4f8;text-decoration:none;font-size:14px}
.header .nav a:hover{text-decoration:underline}
.search-form{display:flex;gap:8px}
.search-form input{background:#1e293b;border:1px solid #334155;color:#e0e0e0;padding:8px 12px;border-radius:6px;width:280px;font-size:14px}
.search-form button{background:#ffd700;color:#0b0e14;border:none;padding:8px 16px;border-radius:6px;font-weight:600;cursor:pointer;font-size:14px}
.container{max-width:1200px;margin:0 auto;padding:24px}
.card{background:#151b24;border:1px solid #2a3548;border-radius:10px;padding:20px;margin-bottom:20px}
.card h2{font-size:16px;color:#ffd700;margin-bottom:16px}
table{width:100%;border-collapse:collapse;font-size:14px}
th,td{text-align:left;padding:10px 8px;border-bottom:1px solid #2a3548}
th{color:#8ab4f8;font-weight:600;font-size:12px;text-transform:uppercase}
td{color:#c0c0c0;word-break:break-all}
td a{color:#8ab4f8;text-decoration:none}
td a:hover{text-decoration:underline}
.hash{font-family:'SF Mono','Fira Code',monospace;font-size:13px}
.value{color:#4ade80;font-weight:600}
.error{color:#ef4444;background:#1e0f12;border:1px solid #5c1a1a;padding:12px;border-radius:6px;margin-bottom:16px}
.block-link{color:#8ab4f8}
.age{color:#888;font-size:12px}
.footer{text-align:center;padding:24px;color:#555;font-size:12px}
.footer a{color:#8ab4f8;text-decoration:none}
.status-dot{display:inline-block;width:8px;height:8px;border-radius:50%;margin-right:6px}
.status-green{background:#4ade80}
.status-yellow{background:#facc15}
.status-red{background:#ef4444}
</style></head><body>
<div class="header">
<h1><a href="/">{{.CoinName}} Explorer</a></h1>
<div class="nav">
<a href="/">Home</a>
<a href="/api/latest">API</a>
</div>
<form class="search-form" action="/search" method="get">
<input type="text" name="q" placeholder="Block hash, txid, address..." autofocus>
<button type="submit">Search</button>
</form>
</div>
<div class="container">
{{if .Error}}<div class="error">{{.Error}}</div>{{end}}
<div class="card">{{.Content}}</div>
</div>
<div class="footer">{{.CoinName}} Explorer {{.Version}} | <a href="https://github.com/legacybtc/LegacyCore">GitHub</a></div>
</body></html>`))

func renderPage(w http.ResponseWriter, title string, content template.HTML, errMsg string) {
	p := pageData{
		Title:    title,
		Mainnet:  mainnet,
		CoinName: "Legacy Coin",
		Version:  version.CoreVersion,
		Content:  content,
		Error:    errMsg,
	}
	var buf bytes.Buffer
	if err := pageTmpl.Execute(&buf, p); err != nil {
		http.Error(w, "render error", 500)
		return
	}
	w.Write(buf.Bytes())
}

func homePage(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		searchOrDetail(w, r)
		return
	}
	info, err := getBlockchainInfo()
	if err != nil {
		renderPage(w, "Error", "", err.Error())
		return
	}
	bestHash, _ := info["bestblockhash"].(string)
	blocks := toFloat64(info["blocks"])
	var rows string
	if bestHash != "" {
		rows = listRecentBlocks(bestHash, int(blocks), 15)
	}
	ti := nodeInfo()
	statusClass := "status-green"
	if info["verificationprogress"] != nil {
		if p := toFloat64(info["verificationprogress"]); p < 0.999 {
			statusClass = "status-yellow"
		}
	}
	prog := 100.0
	if info["verificationprogress"] != nil {
		prog = toFloat64(info["verificationprogress"]) * 100
	}
	content := template.HTML(fmt.Sprintf(`
<h2>Network Status</h2>
<table>
<tr><th>Status</th><td><span class="status-dot %s"></span> %s</td></tr>
<tr><th>Height</th><td class="value">%d</td></tr>
<tr><th>Chain</th><td>%s</td></tr>
<tr><th>Sync Progress</th><td>%.1f%%</td></tr>
<tr><th>Difficulty</th><td>%s</td></tr>
<tr><th>Connections</th><td>%s</td></tr>
<tr><th>Mempool Txs</th><td>%s</td></tr>
<tr><th>Node Version</th><td>%s</td></tr>
</table>
<h2 style="margin-top:24px">Latest Blocks</h2>
<table><tr><th>Height</th><th>Age</th><th>Hash</th><th>Txs</th><th>Size</th></tr>%s</table>
`, statusClass, chainStatus(info), int(blocks), info["chain"], math.Min(prog, 100), formatDifficulty(info), ti, mempoolInfo(), version.CoreFull(), rows))
	renderPage(w, "Home", content, "")
}

func chainStatus(info map[string]any) string {
	if info["initialblockdownload"] == true {
		return "Syncing (IBD)"
	}
	if prog, ok := info["verificationprogress"].(float64); ok && prog < 0.999 {
		return "Syncing"
	}
	return "Synchronized"
}

func formatDifficulty(info map[string]any) string {
	d := toFloat64(info["difficulty"])
	if d < 1000 {
		return fmt.Sprintf("%.2f", d)
	}
	return fmt.Sprintf("%.0f", d)
}

func toFloat64(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case uint32:
		return float64(n)
	case int32:
		return float64(n)
	}
	return 0
}

func nodeInfo() string {
	val, err := cache.GetWith("nodeinfo", 5*time.Second, func() (any, error) {
		return rpcCall("getnetworkinfo", nil)
	})
	if err != nil {
		return "offline"
	}
	m := val.(map[string]any)
	conns := m["connections"]
	return fmt.Sprintf("%v connections", conns)
}

func mempoolInfo() string {
	val, err := cache.GetWith("mempool", 5*time.Second, func() (any, error) {
		return rpcCall("getmempoolinfo", nil)
	})
	if err != nil {
		return "?"
	}
	m := val.(map[string]any)
	return fmt.Sprintf("%v txs", m["size"])
}

func (c *lruCache) GetWith(key string, ttl time.Duration, fn func() (any, error)) (any, error) {
	if v, ok := c.get(key); ok {
		return v, nil
	}
	v, err := fn()
	if err != nil {
		return nil, err
	}
	c.set(key, v, ttl)
	return v, nil
}

func listRecentBlocks(bestHash string, bestHeight int, count int) string {
	var rows string
	hash := bestHash
	for i := 0; i < count && hash != ""; i++ {
		blk, err := getBlockCached(hash)
		if err != nil {
			break
		}
		height := int(toFloat64(blk["height"]))
		txs := blk["tx"]
		txCount := 0
		switch t := txs.(type) {
		case []any:
			txCount = len(t)
		case float64:
			txCount = int(t)
		}
		timeStr := ""
		if t, ok := blk["time"].(float64); ok {
			timeStr = timeAgo(time.Unix(int64(t), 0))
		}
		size := int(toFloat64(blk["size"]))
		rows += fmt.Sprintf(`<tr><td class="value">%d</td><td class="age">%s</td><td class="hash"><a href="/block/%s">%s</a></td><td>%d</td><td>%s</td></tr>`,
			height, timeStr, hash, shorten(hash, 16), txCount, formatSize(size))
		hash, _ = blk["previousblockhash"].(string)
	}
	return rows
}

func shorten(s string, n int) string {
	if len(s) <= n*2+3 {
		return s
	}
	return s[:n] + "..." + s[len(s)-n:]
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func formatSize(b int) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	return fmt.Sprintf("%.1f KB", float64(b)/1024)
}

func getBlockCached(hash string) (map[string]any, error) {
	val, err := cache.GetWith("blk_"+hash, 30*time.Second, func() (any, error) {
		return getBlock(hash)
	})
	if err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

func getBlock(hash string) (map[string]any, error) {
	val, err := rpcCall("getblock", []any{hash})
	if err != nil {
		return nil, err
	}
	m := val.(map[string]any)
	if m == nil {
		return nil, fmt.Errorf("block not found")
	}
	return m, nil
}

func getBlockchainInfo() (map[string]any, error) {
	val, err := cache.GetWith("blockchaininfo", 5*time.Second, func() (any, error) {
		return rpcCall("getblockchaininfo", nil)
	})
	if err != nil {
		return nil, err
	}
	return val.(map[string]any), nil
}

func searchOrDetail(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		homePage(w, r)
		return
	}
	parts := strings.SplitN(path, "/", 2)
	switch parts[0] {
	case "block":
		if len(parts) > 1 {
			blockDetail(w, parts[1])
		} else {
			renderPage(w, "Error", "", "Missing block hash")
		}
	case "tx":
		if len(parts) > 1 {
			txDetail(w, parts[1])
		} else {
			renderPage(w, "Error", "", "Missing transaction id")
		}
	case "address":
		if len(parts) > 1 {
			addressDetail(w, parts[1])
		} else {
			renderPage(w, "Error", "", "Missing address")
		}
	case "search":
		q := r.URL.Query().Get("q")
		search(w, q)
	case "api":
		apiEndpoint(w, r)
	default:
		search(w, path)
	}
}

func blockDetail(w http.ResponseWriter, hash string) {
	blk, err := getBlockCached(hash)
	if err != nil {
		// try as height
		val, err2 := rpcCall("getblockhash", []any{hash})
		if err2 == nil {
			h, _ := val.(string)
			if h != "" {
				blockDetail(w, h)
				return
			}
		}
		renderPage(w, "Block Not Found", "",
			fmt.Sprintf("Block %s not found. Check the hash or height and try again.", hash))
		return
	}
	height := int(toFloat64(blk["height"]))
	blkHash, _ := blk["hash"].(string)
	prevHash, _ := blk["previousblockhash"].(string)
	nextHash, _ := blk["nextblockhash"].(string)
	txIDs := getTxIDs(blk)
	confirmations := int(toFloat64(blk["confirmations"]))
	blkTime := ""
	if t, ok := blk["time"].(float64); ok {
		blkTime = time.Unix(int64(t), 0).Format("2006-01-02 15:04:05 UTC")
	}
	bits, _ := blk["bits"].(string)
	nonce := int64(toFloat64(blk["nonce"]))
	size := int(toFloat64(blk["size"]))
	weight := int(toFloat64(blk["weight"]))
	versionHex := ""
	if v, ok := blk["versionHex"]; ok {
		versionHex, _ = v.(string)
	} else {
		versionHex = fmt.Sprintf("%x", int(toFloat64(blk["version"])))
	}
	var txRows string
	for i, tx := range txIDs {
		cls := ""
		if i == 0 {
			cls = " (coinbase)"
		}
		txRows += fmt.Sprintf(`<tr><td>%d</td><td class="hash"><a href="/tx/%s">%s</a>%s</td></tr>`, i, tx, tx, cls)
	}
	navLinks := fmt.Sprintf(`<a href="/block/%s">← Previous</a>`, prevHash)
	if nextHash != "" {
		navLinks += fmt.Sprintf(` | <a href="/block/%s">Next →</a>`, nextHash)
	}
	content := template.HTML(fmt.Sprintf(`
<div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
<h2>Block #%d</h2><span style="font-size:13px">%s</span>
</div>
<table>
<tr><th>Hash</th><td class="hash">%s</td></tr>
<tr><th>Height</th><td class="value">%d</td></tr>
<tr><th>Confirmations</th><td>%d</td></tr>
<tr><th>Timestamp</th><td>%s</td></tr>
<tr><th>Transactions</th><td>%d</td></tr>
<tr><th>Size</th><td>%s</td></tr>
<tr><th>Weight</th><td>%s</td></tr>
<tr><th>Version</th><td>%s</td></tr>
<tr><th>Bits</th><td class="hash">%s</td></tr>
<tr><th>Nonce</th><td>%d</td></tr>
<tr><th>Previous Block</th><td class="hash"><a href="/block/%s">%s</a></td></tr>
</table>
%s
<h2 style="margin-top:24px">Transactions</h2>
<table><tr><th>#</th><th>TxID</th></tr>%s</table>
`, height, navLinks, blkHash, height, confirmations, blkTime, len(txIDs), formatSize(size), formatSize(weight), versionHex, bits, nonce, prevHash, prevHash, nextLinkOrEmpty(nextHash), txRows))
	renderPage(w, fmt.Sprintf("Block #%d", height), content, "")
}

func nextLinkOrEmpty(hash string) string {
	if hash == "" {
		return ""
	}
	return fmt.Sprintf(`<tr><th>Next Block</th><td class="hash"><a href="/block/%s">%s</a></td></tr>`, hash, hash)
}

func getTxIDs(blk map[string]any) []string {
	txs, ok := blk["tx"].([]any)
	if !ok {
		if cnt, ok2 := blk["nTx"].(float64); ok2 {
			return make([]string, int(cnt))
		}
		return nil
	}
	ids := make([]string, len(txs))
	for i, tx := range txs {
		switch t := tx.(type) {
		case string:
			ids[i] = t
		case map[string]any:
			ids[i], _ = t["txid"].(string)
		}
	}
	return ids
}

func txDetail(w http.ResponseWriter, txid string) {
	val, err := rpcCall("getrawtransaction", []any{txid, true})
	if err != nil {
		renderPage(w, "Transaction Not Found", "",
			fmt.Sprintf("Transaction %s not found. Enable txindex=1 and wait for reindex.", txid))
		return
	}
	tx := val.(map[string]any)
	txid, _ = tx["txid"].(string)
	size := int(toFloat64(tx["size"]))
	vsize := int(toFloat64(tx["vsize"]))
	version := int(toFloat64(tx["version"]))
	locktime := int(toFloat64(tx["locktime"]))
	blockHash, _ := tx["blockhash"].(string)
	confirmations := int(toFloat64(tx["confirmations"]))
	timeStr := ""
	if t, ok := tx["time"].(float64); ok {
		timeStr = time.Unix(int64(t), 0).Format("2006-01-02 15:04:05 UTC")
	}
	var totalIn, totalOut int64
	var vinRows, voutRows string
	vins, _ := tx["vin"].([]any)
	for i, vin := range vins {
		v := vin.(map[string]any)
		if coinbase, ok := v["coinbase"]; ok {
			vinRows += fmt.Sprintf(`<tr><td>%d</td><td colspan="3">Coinbase: %s</td></tr>`, i, coinbase)
			continue
		}
		utxID, _ := v["txid"].(string)
		vout := int(toFloat64(v["vout"]))
		addr, amt := lookupPrevOut(utxID, vout)
		totalIn += amt
		vinRows += fmt.Sprintf(`<tr><td>%d</td><td class="hash"><a href="/tx/%s">%s:%d</a></td><td>%s</td><td class="value">%s</td></tr>`,
			i, utxID, shorten(utxID, 10), vout, addr, formatLBTC(amt))
	}
	vouts, _ := tx["vout"].([]any)
	for _, vout := range vouts {
		v := vout.(map[string]any)
		n := int(toFloat64(v["n"]))
		val := int64(toFloat64(v["value"]))
		totalOut += val
		script := v["scriptPubKey"].(map[string]any)
		addrs, _ := script["addresses"].([]any)
		addrStr := ""
		if len(addrs) > 0 {
			addrStr = fmt.Sprintf(`<a href="/address/%s">%s</a>`, addrs[0], addrs[0])
		}
		typeStr, _ := script["type"].(string)
		voutRows += fmt.Sprintf(`<tr><td>%d</td><td class="hash">%s</td><td>%s</td><td class="value">%s</td></tr>`,
			n, addrStr, typeStr, formatLBTC(val))
	}
	fee := totalIn - totalOut
	feeStr := formatLBTC(fee)
	if fee < 0 {
		feeStr = "0"
	}
	content := template.HTML(fmt.Sprintf(`
<h2>Transaction</h2>
<table>
<tr><th>TxID</th><td class="hash">%s</td></tr>
<tr><th>Size / VSize</th><td>%s / %s</td></tr>
<tr><th>Version</th><td>%d</td></tr>
<tr><th>Locktime</th><td>%d</td></tr>
<tr><th>Block</th><td class="hash"><a href="/block/%s">%s</a></td></tr>
<tr><th>Confirmations</th><td>%d</td></tr>
<tr><th>Timestamp</th><td>%s</td></tr>
<tr><th>Fee</th><td class="value">%s</td></tr>
</table>
<h2 style="margin-top:24px">Inputs (%d)</h2>
<table><tr><th>#</th><th>Source</th><th>Address</th><th>Amount</th></tr>%s</table>
<h2 style="margin-top:24px">Outputs (%d)</h2>
<table><tr><th>#</th><th>Address</th><th>Type</th><th>Amount</th></tr>%s</table>
<h2 style="margin-top:24px">Raw Transaction</h2>
<pre style="background:#1e293b;padding:12px;border-radius:6px;font-size:12px;overflow-x:auto;white-space:pre-wrap;word-break:break-all;color:#8ab4f8">%s</pre>
`, txid, formatSize(size), formatSize(vsize), version, locktime, blockHash, blockHash, confirmations, timeStr, feeStr,
		len(vins), vinRows, len(vouts), voutRows, prettyJSON(tx)))
	renderPage(w, fmt.Sprintf("Tx %s", shorten(txid, 12)), content, "")
}

func lookupPrevOut(txid string, vout int) (addr string, amt int64) {
	val, err := rpcCall("gettxout", []any{txid, vout})
	if err != nil || val == nil {
		return "unknown", 0
	}
	m := val.(map[string]any)
	amt = int64(toFloat64(m["value"]))
	if script, ok := m["scriptPubKey"].(map[string]any); ok {
		if addrs, ok := script["addresses"].([]any); ok && len(addrs) > 0 {
			addr, _ = addrs[0].(string)
		}
	}
	return
}

func formatLBTC(n int64) string {
	return amount.FormatLBTC(n)
}

func addressDetail(w http.ResponseWriter, addr string) {
	val, err := cache.GetWith("addr_"+addr, 30*time.Second, func() (any, error) {
		return rpcCall("getaddressbalance", []any{addr})
	})
	if err != nil {
		renderPage(w, "Address Not Found", "",
			fmt.Sprintf("Could not fetch data for address %s. Enable addressindex=1.", addr))
		return
	}
	bal := val.(map[string]any)
	confirmed := int64(toFloat64(bal["confirmed"]))
	total := int64(toFloat64(bal["total"]))
	utxoVal, _ := cache.GetWith("utxo_"+addr, 30*time.Second, func() (any, error) {
		return rpcCall("getaddressutxos", []any{addr})
	})
	utxoCount := 0
	if utxoVal != nil {
		if utxos, ok := utxoVal.([]any); ok {
			utxoCount = len(utxos)
		}
	}
	txVal, _ := cache.GetWith("hist_"+addr, 60*time.Second, func() (any, error) {
		return rpcCall("getaddresstxids", []any{addr})
	})
	txCount := 0
	if txVal != nil {
		if txs, ok := txVal.([]any); ok {
			txCount = len(txs)
		}
	}
	content := template.HTML(fmt.Sprintf(`
<h2>Address</h2>
<table>
<tr><th>Address</th><td class="hash">%s</td></tr>
<tr><th>Confirmed Balance</th><td class="value">%s</td></tr>
<tr><th>Total Balance</th><td class="value">%s</td></tr>
<tr><th>UTXOs</th><td>%d</td></tr>
<tr><th>Transactions</th><td>%d</td></tr>
</table>
<p style="margin-top:16px;color:#888;font-size:13px">Note: Requires addressindex=1 on the node for full data.</p>
`, addr, formatLBTC(confirmed), formatLBTC(total), utxoCount, txCount))
	renderPage(w, fmt.Sprintf("Address %s", shorten(addr, 12)), content, "")
}

func search(w http.ResponseWriter, q string) {
	if q == "" {
		homePage(w, nil)
		return
	}
	q = strings.TrimSpace(q)
	if len(q) == 64 {
		_, err := hex.DecodeString(q)
		if err == nil {
			// try as block hash first, then txid
			blk, err := getBlockCached(q)
			if err == nil && blk != nil {
				blockDetail(w, q)
				return
			}
			txDetail(w, q)
			return
		}
	}
	if (strings.HasPrefix(q, "L") || strings.HasPrefix(q, "lhyb")) && len(q) >= 26 {
		addressDetail(w, q)
		return
	}
	// try as block height
	var height int
	if _, err := fmt.Sscanf(q, "%d", &height); err == nil && height >= 0 {
		val, err := rpcCall("getblockhash", []any{height})
		if err == nil {
			if h, ok := val.(string); ok && h != "" {
				blockDetail(w, h)
				return
			}
		}
	}
	renderPage(w, "Search", "",
		fmt.Sprintf("No results found for %q. Try a block hash, txid, address, or block height.", q))
}

func prettyJSON(v any) string {
	b, _ := json.MarshalIndent(v, "", "  ")
	return string(b)
}

func apiEndpoint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"coin":     "Legacy Coin",
		"ticker":   "LBTC",
		"version":  version.CoreVersion,
		"explorer": "v1.0.21",
		"endpoints": map[string]string{
			"GET /api/latest":         "Latest blocks summary",
			"GET /api/block/{hash}":   "Block details",
			"GET /api/tx/{txid}":      "Transaction details",
			"GET /api/address/{addr}": "Address details",
		},
	})
}

func apiLatest(w http.ResponseWriter, r *http.Request) {
	info, err := getBlockchainInfo()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	bestHash, _ := info["bestblockhash"].(string)
	blocks := toFloat64(info["blocks"])
	type blockSummary struct {
		Height int    `json:"height"`
		Hash   string `json:"hash"`
		Txs    int    `json:"txs"`
		Time   int64  `json:"time"`
		Size   int    `json:"size"`
	}
	var list []blockSummary
	hash := bestHash
	for i := 0; i < 15 && hash != ""; i++ {
		blk, err := getBlockCached(hash)
		if err != nil {
			break
		}
		txIDs := getTxIDs(blk)
		t := int64(toFloat64(blk["time"]))
		s := int(toFloat64(blk["size"]))
		list = append(list, blockSummary{
			Height: int(toFloat64(blk["height"])),
			Hash:   hash,
			Txs:    len(txIDs),
			Time:   t,
			Size:   s,
		})
		hash, _ = blk["previousblockhash"].(string)
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"height": int(blocks),
		"blocks": list,
	})
}

func main() {
	flag.StringVar(&rpcUser, "rpcuser", "", "RPC username")
	flag.StringVar(&rpcPassword, "rpcpassword", "", "RPC password")
	flag.StringVar(&rpcHost, "rpchost", "127.0.0.1", "RPC host")
	flag.StringVar(&rpcPort, "rpcport", "19556", "RPC port")
	flag.StringVar(&listenAddr, "listen", ":8081", "Explorer listen address")
	flag.BoolVar(&mainnet, "mainnet", true, "Mainnet mode")
	flag.Parse()

	rpcUser = firstNonEmpty(rpcUser, os.Getenv("LEGACY_RPC_USER"))
	rpcPassword = firstNonEmpty(rpcPassword, os.Getenv("LEGACY_RPC_PASS"))

	if rpcUser == "" || rpcPassword == "" {
		dataDir := config.DefaultDataDir()
		cookiePath := filepath.Join(dataDir, ".cookie")
		if cookie, err := os.ReadFile(cookiePath); err == nil {
			parts := strings.SplitN(strings.TrimSpace(string(cookie)), ":", 2)
			if len(parts) == 2 {
				rpcUser = parts[0]
				rpcPassword = parts[1]
			}
		}
	}
	if rpcUser == "" || rpcPassword == "" {
		log.Fatal("RPC credentials required. Set -rpcuser/-rpcpassword, LEGACY_RPC_USER/LEGACY_RPC_PASS, or run legacycoind first.")
	}

	nodeClient = &http.Client{Timeout: 10 * time.Second}

	go pollBlocks()

	mux := http.NewServeMux()
	mux.HandleFunc("/", homePage)
	mux.HandleFunc("/events", sseHandler)
	mux.HandleFunc("/api/latest", apiLatest)
	mux.HandleFunc("/api/block/", func(w http.ResponseWriter, r *http.Request) {
		hash := strings.TrimPrefix(r.URL.Path, "/api/block/")
		blk, err := getBlockCached(hash)
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(blk)
	})
	mux.HandleFunc("/api/tx/", func(w http.ResponseWriter, r *http.Request) {
		txid := strings.TrimPrefix(r.URL.Path, "/api/tx/")
		tx, err := rpcCall("getrawtransaction", []any{txid, true})
		if err != nil {
			http.Error(w, err.Error(), 404)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(tx)
	})

	server := &http.Server{
		Addr:         listenAddr,
		Handler:      securityHeaders(mux),
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		server.Close()
	}()

	log.Printf("Legacy Coin Explorer starting on http://%s", listenAddr)
	log.Printf("Connecting to node at %s:%s", rpcHost, rpcPort)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func init() {
	// Override rpcPort default from flag if env var set
	if p := os.Getenv("LEGACY_RPC_PORT"); p != "" {
		rpcPort = p
	}
	if h := os.Getenv("LEGACY_RPC_HOST"); h != "" {
		rpcHost = h
	}
}
