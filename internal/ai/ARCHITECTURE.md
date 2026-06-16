# Legacy AI Assistant — Architecture

## Principles

- Fully offline after model download
- Read-only by default — never signs transactions or manages keys
- Independent sidecar process — a crash must not affect Legacy Core
- No third-party cloud dependency during chat
- Abstract provider interface — backend can be replaced without UI changes

## Architecture

```
LegacyWallet.exe (Wails/Go + React)
  |  localhost:19570 + random session token
  v
LegacyAI sidecar (Go process or external llama-server)
  |  local inference runtime
  v
GGUF quantized model
  |  GPU (CUDA/HIP/Vulkan) or CPU fallback
```

## Provider Abstraction

The `AIProvider` interface decouples the wallet from any specific runtime:

- MockProvider: deterministic, no model required, safe for CI
- LlamaProvider: connects to llama-server (OpenAI-compatible API)
- OllamaProvider: connects to locally installed Ollama

## Security Boundaries

- Sidecar binds only to 127.0.0.1
- Random session token per wallet start (never logged)
- Sanitized wallet snapshot — no addresses, keys, seeds, passwords, txids
- No RPC credentials passed to AI
- All tool calls executed by wallet bridge, not AI
- AI responses are advisory only — any write action requires wallet UI confirmation

## Data Flow

1. Wallet builds sanitized snapshot (SanitizedSnapshot)
2. Wallet sends snapshot + user message to sidecar
3. Sidecar runs inference locally
4. Response streamed back to wallet
5. Conversation history stored in LegacyCoin/ai/conversations/ (optional)

## Recommended Models

- Low-end (CPU only): 1B-3B Q4_K_M GGUF (~1-2 GB)
- Mid-range (4-6 GB VRAM): 3B-7B Q4_K_M GGUF (~2-4 GB)
- Advanced (8+ GB VRAM): 7B+ Q4_K_M or Q5_K_M GGUF (4+ GB)
