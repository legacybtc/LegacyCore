import React, { useState, useEffect } from "react";
import { Bot, Cpu, HardDrive, Download, Zap, Shield, ArrowRight, CheckCircle, AlertTriangle, RefreshCw } from "lucide-react";

function api() { return window.go?.main?.App; }

interface GPUInfo {
  vendor: string; name: string; vram_mb: number;
  cuda: boolean; rocm: boolean; vulkan: boolean;
  recommended: string; fallback_reason: string;
  available_backends: string[];
}

export function AISetupWizard({ onComplete }: { onComplete: () => void }) {
  const [step, setStep] = useState(0);
  const [gpu, setGpu] = useState<GPUInfo | null>(null);
  const [detecting, setDetecting] = useState(false);
  const [starting, setStarting] = useState(false);
  const [error, setError] = useState("");
  const [health, setHealth] = useState<any>(null);

  useEffect(() => {
    if (step === 1 && !gpu) detectGPU();
  }, [step]);

  async function detectGPU() {
    setDetecting(true); setError("");
    try {
      const a = api();
      if (a?.AIDetectGPU) {
        const result = await a.AIDetectGPU();
        setGpu(result as unknown as GPUInfo);
      }
    } catch (e: any) { setError(e?.message || "Detection failed"); }
    finally { setDetecting(false); }
  }

  async function startAI() {
    setStarting(true); setError("");
    try {
      const a = api();
      if (a?.AIStart) {
        const result = await a.AIStart();
        if (!result?.ok) { setError(result?.error || "Start failed"); setStarting(false); return; }
      }
      await new Promise(r => setTimeout(r, 1000));
      if (a?.AIHealth) {
        const h = await a.AIHealth();
        setHealth(h);
      }
      onComplete();
    } catch (e: any) { setError(e?.message || "Start failed"); }
    finally { setStarting(false); }
  }

  const steps = [
    {
      title: "Welcome to Legacy AI Companion",
      description: "A fully local, private AI assistant that runs on your machine. No cloud. No secrets shared. Advisory only — it cannot spend coins or control your node.",
      icon: <Bot size={48} />,
      action: () => setStep(1),
      actionLabel: "Get Started",
      extra: (
        <div style={{marginTop:12}}>
          <button className="primary" onClick={() => setStep(1)}>
            <ArrowRight size={14} /> Get Started
          </button>
        </div>
      ),
    },
    {
      title: "GPU Detection",
      description: gpu
        ? gpu.vendor !== "none"
          ? `Detected: ${gpu.name} (${gpu.vram_mb} MB VRAM)\nRecommended backend: ${gpu.recommended}\nAvailable: ${gpu.available_backends.join(", ")}`
          : "No GPU detected. AI will run on CPU (slower)."
        : "Scanning for GPU hardware...",
      icon: gpu?.vendor !== "none" ? <HardDrive size={48} /> : <Cpu size={48} />,
      extra: (
        <div style={{marginTop:12}}>
          <button onClick={detectGPU} disabled={detecting}>
            <RefreshCw size={14} style={{animation: detecting ? "spin 1s linear infinite" : "none"}} />
            {detecting ? " Detecting..." : " Re-scan"}
          </button>
          <button className="primary" onClick={() => setStep(2)} style={{marginLeft:8}} disabled={!gpu}>
            <ArrowRight size={14} /> Next
          </button>
        </div>
      ),
    },
    {
      title: "Start AI Service",
      description: health
        ? `AI is running!\nBackend: ${health.backend}\nStatus: ${health.status}`
        : "Launch the AI service. Uses the mock provider by default — functional responses without a real model. Add your own GGUF model later for real intelligence.",
      icon: <Zap size={48} />,
      extra: (
        <div style={{marginTop:12}}>
          <button className="primary" onClick={startAI} disabled={starting || !!health}>
            {starting ? "Starting..." : health ? "Running" : "Start AI"}
          </button>
          {health && <span className="pill good" style={{marginLeft:8}}><CheckCircle size={14}/> Active</span>}
        </div>
      ),
    },
    {
      title: "Ready",
      description: "Legacy AI Companion is ready. Open the AI tab to chat with it about your node, mining, peers, rewards, and more.\n\nPrivacy guarantees:\n• All processing is local\n• Wallet secrets are never shared\n• No cloud API calls\n• Read-only advisory mode",
      icon: <Shield size={48} />,
      extra: (
        <div style={{marginTop:12}}>
          <button className="primary" onClick={onComplete}>Open AI Chat</button>
        </div>
      ),
    },
  ];

  const current = steps[step];

  return (
    <div className="page" style={{maxWidth:560,margin:"0 auto",padding:"24px 16px"}}>
      <div style={{textAlign:"center",marginBottom:24}}>
        {current.icon}
        <h2 style={{marginTop:16,marginBottom:8}}>{current.title}</h2>
        <p style={{whiteSpace:"pre-wrap",opacity:0.75}}>{current.description}</p>
      </div>

      {error && <div className="notice danger" style={{marginBottom:12}}>{error}</div>}

      <div style={{display:"flex",justifyContent:"space-between",alignItems:"center",marginTop:16}}>
        <div style={{display:"flex",gap:4}}>
          {steps.map((_, i) => (
            <div key={i} style={{width:24,height:4,borderRadius:2,background:i<=step?"var(--accent,#60a5fa)":"#334155"}} />
          ))}
        </div>
        {current.extra}
      </div>
    </div>
  );
}
