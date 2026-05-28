package rpc

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"legacycoin/legacy-go/internal/address"
	"legacycoin/legacy-go/internal/chaincfg"
)

func TestJSONRPCSingleRequestMethodNotFound(t *testing.T) {
	s := &Server{}
	body := `{"jsonrpc":"1.0","id":"t1","method":"doesnotexist","params":[]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	s.handle(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32601 {
		t.Fatalf("expected method-not-found error, got %+v", resp.Error)
	}
}

func TestJSONRPCBatchRequest(t *testing.T) {
	s := &Server{}
	body := `[
		{"jsonrpc":"1.0","id":"1","method":"doesnotexist","params":[]},
		{"jsonrpc":"1.0","id":"2","method":"also_missing","params":[]}
	]`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	s.handle(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var responses []response
	if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
		t.Fatalf("decode batch response: %v", err)
	}
	if len(responses) != 2 {
		t.Fatalf("batch response len = %d, want 2", len(responses))
	}
	for _, r := range responses {
		if r.Error == nil || r.Error.Code != -32601 {
			t.Fatalf("expected method-not-found for response %+v", r)
		}
	}
}

func TestJSONRPCInvalidParamsSubmitBlockNoArgs(t *testing.T) {
	s := &Server{}
	body := `{"jsonrpc":"1.0","id":"probe","method":"submitblock","params":[]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	s.handle(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error == nil || resp.Error.Code != -32602 {
		t.Fatalf("expected invalid params, got %+v", resp.Error)
	}
}

func TestParseSendManyOutputs(t *testing.T) {
	addr := address.EncodeBase58Check(chaincfg.PublicKeyHashVersion, bytes.Repeat([]byte{0x11}, 20))
	out, err := parseSendManyOutputs(json.RawMessage(`{"`+addr+`": 1.5}`), false)
	if err != nil {
		t.Fatalf("parseSendManyOutputs failed: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected one output, got %d", len(out))
	}
}

func TestJSONRPCHelpMethod(t *testing.T) {
	s := &Server{}
	body := `{"jsonrpc":"1.0","id":"help","method":"help","params":[]}`
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", bytes.NewBufferString(body))
	s.handle(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var resp response
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Error != nil {
		t.Fatalf("unexpected help error: %+v", resp.Error)
	}
	out, ok := resp.Result.(map[string]any)
	if !ok {
		t.Fatalf("help result type = %T, want map[string]any", resp.Result)
	}
	methods, ok := out["methods"].([]any)
	if !ok || len(methods) == 0 {
		t.Fatalf("help methods missing or empty: %#v", out["methods"])
	}
}

func TestRPCHelpEntriesIncludeAddressHistoryAndPassphraseChange(t *testing.T) {
	hasHistory := false
	hasPassphraseChange := false
	for _, entry := range rpcHelpEntries {
		if entry.Method == "getaddresshistory" {
			hasHistory = true
		}
		if entry.Method == "walletpassphrasechange" {
			hasPassphraseChange = true
		}
	}
	if !hasHistory {
		t.Fatalf("rpc help table missing getaddresshistory")
	}
	if !hasPassphraseChange {
		t.Fatalf("rpc help table missing walletpassphrasechange")
	}
}
