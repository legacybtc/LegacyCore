import React, { useState, useRef, useEffect } from "react";
import { Bot, Cpu, HardDrive, Shield, Zap, AlertTriangle, CheckCircle, XCircle, RefreshCw, Trash2, Download } from "lucide-react";

function api() { return window.go?.main?.App; }

export function LegacyAIPage({ snap }: { snap?: any }) {
  const [input, setInput] = useState("");
  const [chat, setChat] = useState<{ role: string; content: string }[]>([]);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState("");
  const [health, setHealth] = useState<any>({});
  const chatEnd = useRef<HTMLDivElement>(null);

  useEffect(() => { chatEnd.current?.scrollIntoView({ behavior: "smooth" }); }, [chat]);
  useEffect(() => { refreshHealth(); }, []);

  async function refreshHealth() {
    try { const a = api(); if (a?.AIHealth) setHealth(await a.AIHealth()); } catch {}
  }

  async function send() {
    const msg = input.trim(); if (!msg || generating) return;
    setInput(""); setGenerating(true); setError("");
    setChat(p => [...p, { role: "user", content: msg }]);
    try {
      const a = api(); if (!a?.AIChat) { setError("AI unavailable"); return; }
      const r = await a.AIChat(msg);
      setChat(p => [...p, { role: "assistant", content: r?.content || r?.error || "No response" }]);
    } catch (e: any) { setError(e?.message); }
    finally { setGenerating(false); }
  }

  const status = health?.status || "disabled";
  const prompts = ["Why is mining paused?", "Is my node synchronized?", "Explain degraded-safe mining.", "Explain my immature rewards.", "Is RPC healthy?", "Is storage healthy?"];

  return (
    <div className="page legacyAIPage">
      <div className="heroPanel compactHero">
        <Bot size={34} />
        <div>
          <p className="eyebrow">Legacy AI Companion • Local • Private • Advisory</p>
          <h3>Legacy AI</h3>
          <p>Fully local AI assistant. No cloud. No secrets shared.</p>
        </div>
        <span className="pill good">Experimental Preview</span>
      </div>

      <div className="row" style={{gap:8,marginBottom:12}}>
        <span className={`pill ${status==="ready"?"good":status==="error"?"danger":""}`}>
          {status==="ready"?<CheckCircle size={14}/>:<AlertTriangle size={14}/>} {status}
        </span>
        {health?.backend && <span className="pill"><Cpu size={14}/> {health.backend}</span>}
        {health?.gpu_name && <span className="pill"><HardDrive size={14}/> {health.gpu_name}</span>}
        {health?.pid > 0 && <span className="pill">PID: {health.pid}</span>}
        <button className="primary" onClick={() => api()?.AIStart()} disabled={status==="ready"}>Start AI</button>
        <button onClick={() => api()?.AIStop()} disabled={status!=="ready"}>Stop</button>
        <button onClick={refreshHealth}><RefreshCw size={14}/> Refresh</button>
      </div>

      <div className="twoCol">
        <section className="panel">
          <h3>Privacy</h3>
          <div className="kv">
            <div><span>Runs locally</span><strong><CheckCircle size={14} className="green"/> Yes</strong></div>
            <div><span>No wallet secrets</span><strong><Shield size={14}/> Guaranteed</strong></div>
            <div><span>No cloud calls</span><strong>Offline</strong></div>
            <div><span>Advisory only</span><strong>Read-only</strong></div>
          </div>
        </section>
        <section className="panel">
          <h3>Status</h3>
          <div className="kv">
            <div><span>Provider</span><strong>{health?.backend || "not started"}</strong></div>
            {health?.model_name && <div><span>Model</span><strong>{health.model_name}</strong></div>}
            {health?.uptime && <div><span>Uptime</span><strong>{health.uptime}</strong></div>}
            {health?.last_error && <div><span>Last error</span><strong className="red">{health.last_error}</strong></div>}
          </div>
        </section>
      </div>

      <section className="panel">
        <h3>Chat</h3>
        <div className="chatBox" style={{minHeight:200,maxHeight:400,overflow:"auto",marginBottom:8}}>
          {chat.length===0 && !generating && (
            <div style={{textAlign:"center",padding:24}}>
              <Bot size={48} style={{opacity:0.3}} />
              <p style={{marginTop:8}}>Ask me about your wallet, node, mining, or peers.</p>
              <div style={{display:"flex",flexWrap:"wrap",gap:6,marginTop:12}}>
                {prompts.map((q,i) => <button key={i} onClick={() => { setInput(q); setTimeout(send, 50); }} style={{fontSize:12}}>{q}</button>)}
              </div>
            </div>
          )}
          {chat.map((m,i) => (
            <div key={i} style={{marginBottom:8,padding:8,background:m.role==="user"?"#1e293b":"#0f172a",borderRadius:6}}>
              <strong style={{fontSize:11,opacity:0.6}}>{m.role==="user"?"You":"Legacy AI"}</strong>
              <p style={{margin:"4px 0 0",whiteSpace:"pre-wrap"}}>{m.content}</p>
            </div>
          ))}
          {generating && <div style={{padding:8,opacity:0.6}}>Generating...</div>}
          <div ref={chatEnd} />
        </div>
        {error && <div className="notice danger" style={{marginBottom:8}}>{error}</div>}
        <div className="row">
          <input value={input} onChange={e => setInput(e.target.value)} onKeyDown={e => { if (e.key==="Enter") send(); }} placeholder="Ask Legacy AI..." disabled={generating} style={{flex:1}} />
          <button className="primary" onClick={send} disabled={generating||!input.trim()}>Send</button>
          <button onClick={() => setChat([])}><Trash2 size={14}/></button>
        </div>
      </section>
    </div>
  );
}
