# Runtime Provider Evaluation

## Option A — llama.cpp / llama-server

| Criterion | Assessment |
|-----------|------------|
| **NVIDIA CUDA** | Full support via GGML_CUDA=1 build |
| **AMD HIP/ROCm** | Supported via GGML_HIPBLAS=1 |
| **Vulkan** | Supported via GGML_VULKAN=1 |
| **CPU fallback** | Native, always available |
| **Partial GPU offloading** | --n-gpu-layers N |
| **GGUF models** | Native, all quantizations |
| **OpenAI-compatible API** | Built-in /chat/completions endpoint |
| **Process lifecycle** | External process, managed by wallet |
| **Windows packaging** | Pre-built binaries or build from source |
| **Linux packaging** | Same |
| **License** | MIT |
| **Model licenses** | Vary (Llama, Mistral, Gemma, etc.) |
| **Recommended for POC** | Yes — mature, widely deployed |

## Option B — Ollama

| Criterion | Assessment |
|-----------|------------|
| **NVIDIA CUDA** | Yes, auto-detected |
| **AMD HIP/ROCm** | Yes (Ollama for Linux + ROCm) |
| **Vulkan** | Not directly — uses llama.cpp underneath |
| **CPU fallback** | Yes |
| **Partial GPU offloading** | Auto-managed, limited user control |
| **GGUF models** | Yes |
| **OpenAI-compatible API** | Built-in /api/chat endpoint |
| **Process lifecycle** | Separate daemon, system service |
| **Windows packaging** | Native installer |
| **Linux packaging** | curl | sh install |
| **License** | MIT |
| **Dependency** | Requires Ollama to be installed separately |
| **Recommended for POC** | Yes — easier for users who already have Ollama |

## Selected Strategy

**Both, via provider abstraction.**

1. **Milestone 1 (current)**: MockProvider for deterministic testing + LlamaProvider for local llama-server users
2. **Milestone 2**: OllamaProvider adapter for existing Ollama users
3. **Milestone 3**: Bundled llama-server sidecar for zero-dependency experience

The abstract `AIProvider` interface allows all three without changing the wallet UI.
