Legacy Wallet 1.0.6-rc1 AI Preview
=====================================
Experimental — Not for Release
Local AI — Advisory Only

This package adds an experimental Legacy AI assistant
to the Legacy Wallet. The AI runs fully locally on your
computer using GPU acceleration where available.

The standard Legacy Core daemon and CLI are unchanged from
the 1.0.6-rc1 stability release. No AI code runs on the
daemon. The AI feature is only in the wallet desktop app.

Quick Start:
1. Run legacycoind.exe to start the node (or use existing)
2. Run LegacyWallet.exe to open the wallet
3. Click "Legacy AI" in the sidebar
4. The mock provider works without any model download
5. For real AI: install llama-server and a GGUF model

Security:
- AI is read-only and advisory only
- No private keys, seeds, passwords, or RPC credentials
  are ever sent to the AI
- AI cannot send transactions, sign, or control mining
- All inference runs locally, offline by default

See AI-PREVIEW-GUIDE.txt for llama-server setup.

Build: feature/local-ai-assistant branch
Stability base: main at feb07aa
