package node

import (
	"strings"
	"testing"
)

func TestRepairPrettyLogArtifactsEmojiMode(t *testing.T) {
	in := "рџЏ“ PING в†’ 176.229.49.108:19555"
	out := repairPrettyLogArtifacts(in, true)
	if !strings.Contains(out, "📡 PING →") {
		t.Fatalf("unexpected repaired pretty line: %q", out)
	}
}

func TestRepairPrettyLogArtifactsPlainMode(t *testing.T) {
	in := "рџџў PONG в†ђ 176.229.49.108:19555 | latency 31ms"
	out := repairPrettyLogArtifacts(in, false)
	if strings.Contains(out, "🏓") {
		t.Fatalf("plain mode should strip emoji, got: %q", out)
	}
	if !strings.Contains(out, "PONG ←") {
		t.Fatalf("plain mode should keep readable text, got: %q", out)
	}
}

func TestPrettyLineRPCLabelReadable(t *testing.T) {
	out := repairPrettyLogArtifacts(prettyLine("rpc auth enabled", true), true)
	if !strings.Contains(out, "🔐 [RPC]") {
		t.Fatalf("expected readable RPC marker, got: %q", out)
	}
}
