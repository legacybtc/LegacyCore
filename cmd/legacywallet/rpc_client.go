package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"legacycoin/legacy-go/internal/config"
)

type RPCClient struct {
	url    string
	auth   string
	client *http.Client
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      string          `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type rpcResponse struct {
	ID     string           `json:"id"`
	Result json.RawMessage  `json:"result"`
	Error  *rpcError        `json:"error"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *rpcError) Error() string {
	return fmt.Sprintf("RPC error %d: %s", e.Code, e.Message)
}

// NewRPCClient creates a new RPC client for connecting to an external RPC server.
// It loads the RPC cookie from the default data directory (~/.legacycoin) if no user/password is provided.
func NewRPCClient(url, user, password string) (*RPCClient, error) {
	c := &RPCClient{
		url:    url,
		client: &http.Client{Timeout: 30 * time.Second},
	}

	// If no user/password provided, try to load from cookie
	if user == "" || password == "" {
		cookieAuth, err := config.LoadRPCCookie()
		if err != nil {
			return nil, fmt.Errorf("failed to load RPC cookie: %w", err)
		}
		if cookieAuth.Enabled {
			user = cookieAuth.User
			password = cookieAuth.Password
		}
	}

	if user != "" && password != "" {
		c.auth = base64.StdEncoding.EncodeToString([]byte(user + ":" + password))
	}
	return c, nil
}

func (c *RPCClient) Call(method string, params []any) (json.RawMessage, error) {
	p, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	reqBody := rpcRequest{
		JSONRPC: "1.0",
		ID:      "legacy-wallet",
		Method:  method,
		Params:  p,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	httpReq, err := http.NewRequest("POST", c.url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.auth != "" {
		httpReq.Header.Set("Authorization", "Basic "+c.auth)
	}

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("RPC call %s: %w", method, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("RPC call %s: read body: %w", method, err)
	}

	var rpcResp rpcResponse
	if err := json.Unmarshal(respBody, &rpcResp); err != nil {
		return nil, fmt.Errorf("RPC call %s: parse response: %w", method, err)
	}
	if rpcResp.Error != nil {
		return nil, rpcResp.Error
	}
	return rpcResp.Result, nil
}

// CallMethod is a convenience method that unmarshals the result into a pointer
func (c *RPCClient) CallMethod(method string, params []any, result any) error {
	raw, err := c.Call(method, params)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, result)
}
