package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGlobalFlags(t *testing.T) {
	opts, rest, err := parseGlobalFlags([]string{"-datadir=/tmp/legacy", "-rpcuser", "alice", "-rpcpassword=secret", "-rpcconnect=127.0.0.1", "-rpcport=19556", "getnetworkinfo"})
	if err != nil {
		t.Fatalf("parse flags: %v", err)
	}
	if opts.DataDir != "/tmp/legacy" || opts.RPCUser != "alice" || opts.RPCPassword != "secret" || opts.RPCConnect != "127.0.0.1" || opts.RPCPort != "19556" {
		t.Fatalf("opts=%+v", opts)
	}
	if len(rest) != 1 || rest[0] != "getnetworkinfo" {
		t.Fatalf("rest=%v", rest)
	}
}

func TestApplyRPCAuthReadsCookieFromDataDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cookie"), []byte("__cookie__:secret\n"), 0o600); err != nil {
		t.Fatalf("write cookie: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:19556", nil)
	if err := applyRPCAuth(req, cliOptions{DataDir: dir}); err != nil {
		t.Fatalf("apply auth: %v", err)
	}
	user, pass, ok := req.BasicAuth()
	if !ok || user != "__cookie__" || pass != "secret" {
		t.Fatalf("basic auth user=%q pass=%q ok=%t", user, pass, ok)
	}
}

func TestApplyRPCAuthMissingCookieMessage(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:19556", nil)
	err := applyRPCAuth(req, cliOptions{DataDir: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "RPC cookie not found") {
		t.Fatalf("missing cookie err=%v", err)
	}
}

func TestRunCLIUnauthorizedMessage(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".cookie"), []byte("__cookie__:secret\n"), 0o600); err != nil {
		t.Fatalf("write cookie: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()
	err := runCLI([]string{"-rpcurl=" + srv.URL, "-datadir=" + dir, "getnetworkinfo"}, &bytes.Buffer{}, &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "RPC unauthorized") {
		t.Fatalf("unauthorized err=%v", err)
	}
}

func TestRunCLIExplicitAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "legacyrpc" || pass != "strong_password" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"result":{"ok":true},"error":null,"id":"cli"}`))
	}))
	defer srv.Close()
	var out bytes.Buffer
	if err := runCLI([]string{"-rpcurl=" + srv.URL, "-rpcuser=legacyrpc", "-rpcpassword=strong_password", "getnetworkinfo"}, &out, &bytes.Buffer{}); err != nil {
		t.Fatalf("run cli: %v", err)
	}
	if !strings.Contains(out.String(), `"ok": true`) {
		t.Fatalf("out=%s", out.String())
	}
}

func TestBuildParamsSendManyEscapedJSON(t *testing.T) {
	params, method, err := buildParams("sendmany", []string{
		"",
		"{\\\"LfP4QmiYvhgv5JBTqm6iiTadHQg41xve3F\\\":0.0001,\\\"LhyDoWS7RscYSj6G3jz8x7fy8Fq9e6taRB\\\":0.0002}",
	})
	if err != nil {
		t.Fatalf("buildParams sendmany: %v", err)
	}
	if method != "sendmany" {
		t.Fatalf("method=%s", method)
	}
	if len(params) != 2 {
		t.Fatalf("params len=%d", len(params))
	}
	obj, ok := params[1].(map[string]any)
	if !ok {
		t.Fatalf("outputs type=%T", params[1])
	}
	if len(obj) != 2 {
		t.Fatalf("outputs=%v", obj)
	}
}

func TestBuildParamsSendManyRaw(t *testing.T) {
	params, method, err := buildParams("sendmanyraw", []string{
		"",
		`{"LfP4QmiYvhgv5JBTqm6iiTadHQg41xve3F":10000}`,
	})
	if err != nil {
		t.Fatalf("buildParams sendmanyraw: %v", err)
	}
	if method != "sendmanyraw" {
		t.Fatalf("method=%s", method)
	}
	if len(params) != 2 {
		t.Fatalf("params len=%d", len(params))
	}
	encoded, err := json.Marshal(params[1])
	if err != nil {
		t.Fatalf("marshal outputs: %v", err)
	}
	if !strings.Contains(string(encoded), "10000") {
		t.Fatalf("unexpected outputs: %s", string(encoded))
	}
}

func TestParseSendManyOutputsArgBareKeys(t *testing.T) {
	out, err := parseSendManyOutputsArg("{LfP4QmiYvhgv5JBTqm6iiTadHQg41xve3F:0.0001,LhyDoWS7RscYSj6G3jz8x7fy8Fq9e6taRB:0.0002}")
	if err != nil {
		t.Fatalf("parseSendManyOutputsArg: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("outputs=%v", out)
	}
}

func TestBuildParamsSendManySplitFragments(t *testing.T) {
	params, method, err := buildParams("sendmany", []string{
		"",
		"{LfP4QmiYvhgv5JBTqm6iiTadHQg41xve3F:0.0001",
		"LhyDoWS7RscYSj6G3jz8x7fy8Fq9e6taRB:0.0002}",
	})
	if err != nil {
		t.Fatalf("buildParams sendmany split fragments: %v", err)
	}
	if method != "sendmany" {
		t.Fatalf("method=%s", method)
	}
	if len(params) != 2 {
		t.Fatalf("params len=%d", len(params))
	}
	out, ok := params[1].(map[string]any)
	if !ok {
		t.Fatalf("outputs type=%T", params[1])
	}
	if len(out) != 2 {
		t.Fatalf("outputs=%v", out)
	}
}

func TestBuildParamsSendManyWithoutAccountArg(t *testing.T) {
	params, method, err := buildParams("sendmany", []string{
		`{"LfP4QmiYvhgv5JBTqm6iiTadHQg41xve3F":0.0001}`,
	})
	if err != nil {
		t.Fatalf("buildParams sendmany no-account: %v", err)
	}
	if method != "sendmany" {
		t.Fatalf("method=%s", method)
	}
	if len(params) != 2 {
		t.Fatalf("params len=%d", len(params))
	}
	if account, ok := params[0].(string); !ok || account != "" {
		t.Fatalf("account=%v", params[0])
	}
}

func TestBuildParamsWalletPassphraseChange(t *testing.T) {
	params, method, err := buildParams("walletpassphrasechange", []string{"old-pass", "new-pass"})
	if err != nil {
		t.Fatalf("buildParams walletpassphrasechange: %v", err)
	}
	if method != "walletpassphrasechange" {
		t.Fatalf("method=%s", method)
	}
	if len(params) != 2 {
		t.Fatalf("params len=%d", len(params))
	}
	if params[0] != "old-pass" || params[1] != "new-pass" {
		t.Fatalf("params=%v", params)
	}
}
