import React, { useState, useRef, useEffect, useCallback } from "react";
import { Shield, Bot, Cpu, HardDrive, Zap, AlertTriangle, CheckCircle, XCircle, RefreshCw, StopCircle, Trash2 } from "lucide-react";

function api() { return window.go?.main?.App; }

type AIChatMsg = { role: "user" | "assistant"; content: string; ts: number };

export function LegacyAIPage({ snap }: { snap?: any }) {
  const [status, setStatus] = useState("disabled");
  const [chatMessages, setChatMessages] = useState<AIChatMsg[]>([]);
  const [input, setInput] = useState("");
  const [generating, setGenerating] = useState(false);
  const [health, setHealth] = useState<any>(null);
  const [error, setError] = useState("");
  const chatEnd = useRef<HTMLDivElement>(null);

  useEffect(() => { chatEnd.current?.scrollIntoView({ behavior: "smooth" }); }, [chatMessages]);

  const app = api();

  const refreshHealth = useCallback(async () => {
    try {
      if (!app?.AIHealth) { setStatus("disabled"); return; }
      const h = await app.AIHealth();
      setHealth(h);
      setStatus(h?.status || "disabled");
    } catch {
      setStatus("disabled");
    }
  }, [app]);

  useEffect(() => { refreshHealth(); }, [refreshHealth]);

  async function sendMessage() {
    const msg = input.trim();
    if (!msg || generating) return;
    setInput("");
    setGenerating(true);
    setError("");
    const userMsg: AIChatMsg = { role: "user", content: msg, ts: Date.now() };
    setChatMessages(prev => [...prev, userMsg]);
    try {
      if (!app?.AIChat) { setError("AI backend not available"); setGenerating(false); return; }
      const response = await app.AIChat(msg);
      const aiMsg: AIChatMsg = { role: "assistant", content: response?.content || response || "No response.", ts: Date.now() };
      setChatMessages(prev => [...prev, aiMsg]);
    } catch (e: any) {
      setError(String(e?.message || e));
    } finally {
      setGenerating(false);
    }
  }

  function clearChat() { setChatMessages([]); setError(""); }

  const statusIcon = () => {
    switch (status) {
      case "ready": return <CheckCircle className="green" />;
      case "error": return <XCircle className="red" />;
      case "generating": return <Zap className="yellow" />;
      case "loading_model": return <RefreshCw className="spinning" />;
      case "starting": return <RefreshCw className="spinning" />;
      default: return <AlertTriangle />;
    }
  };

  return (
    <div className="page legacyAIPage">
      <div className="heroPanel compactHero">
        <Bot size={34} />
        <div>
          <p className="eyebrow">EXPERIMENTAL LOCAL AI — READ-ONLY</p>
          <h3>Legacy AI Assistant</h3>
          <p>Local GPU-powered assistant. No wallet secrets are shared. Fully offline.</p>
        </div>
      </div>

      <section className="panel">
        <div className="row">
          <span className={`pill ${status === "ready" ? "good" : status === "error" ? "danger" : ""}`}>
            {statusIcon()} {status || "disabled"}
          </span>
          {health?.backend && <span className="pill">Backend: {health.backend}</span>}
          {health?.gpu_name && <span className="pill"><Cpu size={14} /> {health.gpu_name}</span>}
          {health?.model_loaded && <span className="pill"><HardDrive size={14} /> Model loaded</span>}
        </div>
      </section>

      <div className="twoCol">
        <section className="panel">
          <h3>Privacy</h3>
          <div className="kv">
            <div><span>Runs locally</span><strong><CheckCircle size={14} className="green" /> Yes</strong></div>
            <div><span>No wallet secrets shared</span><strong><CheckCircle size={14} className="green" /> Yes</strong></div>
            <div><span>No cloud API calls</span><strong><CheckCircle size={14} className="green" /> Offline</strong></div>
            <div><span>Read-only by design</span><strong><Shield size={14} /> Advisory only</strong></div>
          </div>
        </section>

        <section className="panel">
          <h3>Diagnostics</h3>
          <div className="kv">
            <div><span>Status</span><strong>{status}</strong></div>
            {health?.pid > 0 && <div><span>Process ID</span><strong>{health.pid}</strong></div>}
            {health?.uptime && <div><span>Uptime</span><strong>{health.uptime}</strong></div>}
            {health?.model_name && <div><span>Model</span><strong>{health.model_name}</strong></div>}
            {health?.ram_mb > 0 && <div><span>RAM</span><strong>{health.ram_mb} MB</strong></div>}
            {health?.vram_mb > 0 && <div><span>VRAM</span><strong>{health.vram_mb} MB</strong></div>}
            {health?.last_error && <div><span>Last error</span><strong className="red">{health.last_error}</strong></div>}
          </div>
        </section>
      </div>

      <section className="panel">
        <h3>Chat</h3>
        <div className="chatBox">
          {chatMessages.length === 0 && !generating && (
            <div className="chatEmpty">
              <Bot size={48} className="muted" />
              <p>Ask me about your wallet status, node health, sync, peers, mining safety, RPC condition, storage diagnostics, or immature rewards.</p>
              <div className="examplePrompts">
                {["Why is my miner paused?", "Is my node synchronized?", "Explain my immature rewards.", "How many peers are connected?", "Is my RPC healthy?", "Check my wallet balance.", "Are there any safety warnings?"].map((q, i) => (
                  <button key={i} onClick={() => { setInput(q); setTimeout(() => sendMessage(), 50); }}>{q}</button>
                ))}
              </div>
            </div>
          )}
          {chatMessages.map((m, i) => (
            <div key={i} className={`chatMsg ${m.role}`}>
              <strong>{m.role === "user" ? "You" : "Legacy AI"}</strong>
              <p>{m.content}</p>
            </div>
          ))}
          {generating && <div className="chatMsg assistant"><strong>Legacy AI</strong><p><span className="typing">Generating...</span></p></div>}
          <div ref={chatEnd} />
        </div>
        {error && <div className="notice danger"><AlertTriangle size={16} /> {error}</div>}
        <div className="row">
          <input
            value={input}
            onChange={e => setInput(e.target.value)}
            onKeyDown={e => { if (e.key === "Enter") sendMessage(); }}
            placeholder="Ask Legacy AI..."
            disabled={generating}
            className="chatInput"
          />
          <button className="primary" onClick={sendMessage} disabled={generating || !input.trim()}>Send</button>
          {generating && <button onClick={() => {}}><StopCircle size={14} /> Stop</button>}
          <button onClick={clearChat} disabled={chatMessages.length === 0}><Trash2 size={14} /> Clear</button>
        </div>
      </section>
    </div>
  );
}
