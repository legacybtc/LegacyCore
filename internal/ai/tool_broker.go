package ai

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

type ToolBroker struct {
	allowlist map[string]bool
}

func NewToolBroker() *ToolBroker {
	return &ToolBroker{allowlist: defaultAllowlist()}
}

func defaultAllowlist() map[string]bool {
	cmds := map[string]bool{}
	safe := []string{
		"legacycoin-cli getblockchaininfo",
		"legacycoin-cli getmininginfo",
		"legacycoin-cli getpeerinfo",
		"legacycoin-cli getnetworkinfo",
		"legacycoin-cli getmempoolinfo",
		"legacycoin-cli getwalletinfo",
		"legacycoin-cli listtransactions",
		"legacycoin-cli getblock",
		"legacycoin-cli getrawmempool",
		"legacycoin-cli uptime",
		"get-process legacywallet",
		"get-process legacycoind",
		"netstat -an | findstr 19555",
		"netstat -an | findstr 19556",
	}
	for _, c := range safe {
		cmds[c] = true
	}
	return cmds
}

type ToolResult struct {
	Command   string `json:"command"`
	Stdout    string `json:"stdout"`
	Stderr    string `json:"stderr"`
	ExitCode  int    `json:"exit_code"`
	Duration  string `json:"duration"`
	Allowed   bool   `json:"allowed"`
	Truncated bool   `json:"truncated,omitempty"`
}

const maxToolOutput = 2048

func (tb *ToolBroker) Execute(ctx context.Context, cmdLine string) ToolResult {
	start := time.Now()
	r := ToolResult{Command: cmdLine, Allowed: false}

	if !tb.allowlist[strings.TrimSpace(cmdLine)] {
		r.Stderr = fmt.Sprintf("command not in allowlist: %s", cmdLine)
		r.ExitCode = -1
		r.Duration = time.Since(start).Round(time.Millisecond).String()
		return r
	}

	r.Allowed = true
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var args []string
	fields := strings.Fields(cmdLine)
	exe := fields[0]
	if len(fields) > 1 {
		args = fields[1:]
	}

	cmd := exec.CommandContext(ctx, exe, args...)
	out, err := cmd.CombinedOutput()

	if err != nil {
		if ctx.Err() != nil {
			r.Stderr = "timed out"
			r.ExitCode = -1
		} else {
			r.Stderr = err.Error()
			r.ExitCode = cmd.ProcessState.ExitCode()
		}
	}

	r.Stdout = string(out)
	if len(r.Stdout) > maxToolOutput {
		r.Stdout = r.Stdout[:maxToolOutput]
		r.Truncated = true
	}
	if len(r.Stderr) > maxToolOutput {
		r.Stderr = r.Stderr[:maxToolOutput]
		r.Truncated = true
	}
	r.Duration = time.Since(start).Round(time.Millisecond).String()
	if r.ExitCode == 0 && err != nil {
		r.ExitCode = 1
	}
	return r
}

func (tb *ToolBroker) ListAllowlist() []string {
	out := make([]string, 0, len(tb.allowlist))
	for c := range tb.allowlist {
		out = append(out, c)
	}
	return out
}

func (tb *ToolBroker) AddCommand(cmd string) error {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" { return fmt.Errorf("empty command") }
	if strings.Contains(trimmed, ";") || strings.Contains(trimmed, "&&") || strings.Contains(trimmed, "|") {
		return fmt.Errorf("command chaining not allowed")
	}
	dangerous := []string{"rm ", "del ", "shutdown", "reboot", "dd ", "mkfs", "format ", ">", "wget ", "curl "}
	for _, d := range dangerous {
		if strings.Contains(strings.ToLower(trimmed), d) {
			return fmt.Errorf("dangerous command rejected: contains %q", d)
		}
	}
	tb.allowlist[trimmed] = true
	return nil
}
