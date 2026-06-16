import React, { useState, useRef, useEffect } from "react";
import { Bot, Cpu, HardDrive, Shield, Zap, AlertTriangle, CheckCircle, RefreshCw, Trash2, Terminal, Wrench } from "lucide-react";
import { LegacyMascot } from "./LegacyMascot";
import { AISetupWizard } from "./AISetupWizard";
import type { AgentState } from "./LegacyAgent";

function api() { return window.go?.main?.App; }

interface LegacyAIPageProps {
  snap?: any;
  agentState: AgentState;
  setAgentState: (s: AgentState) => void;
  agentSpeech: string;
  setAgentSpeech: (s: string) => void;
}

export function LegacyAIPage({ snap, setAgentState, setAgentSpeech }: LegacyAIPageProps) {
  const [input, setInput] = useState("");
  const [chat, setChat] = useState<{ role: string; content: string }[]>([]);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState("");
  const [health, setHealth] = useState<any>({});
  const [mode, setMode] = useState<"advisor" | "developer">("advisor");
  const [showWizard, setShowWizard] = useState(false);
  const [tools, setTools] = useState<string[]>([]);
  const chatEnd = useRef<HTMLDivElement>(null);

  useEffect(() => { chatEnd.current?.scrollIntoView({ behavior: "smooth" }); }, [chat]);
  useEffect(() => { refreshHealth(); }, []);

  async function refreshHealth() {
    try {
      const a = api();
      if (a?.AIHealth) {
        const h = await a.AIHealth();
        setHealth(h);
        if (h?.status === "disabled" || h?.status === "stopped") setShowWizard(true);
      }
      if (a?.AIListTools) setTools(await a.AIListTools());
    } catch {}
  }

  async function send() {
    const msg = input.trim(); if (!msg || generating) return;
    setInput(""); setGenerating(true); setError("");
    setChat(p => [...p, { role: "user", content: msg }]);
    setAgentState("listen");
    setAgentSpeech(`"${msg.slice(0, 60)}${msg.length > 60 ? "..." : ""}"`);

    if (mode === "developer" && msg.startsWith("/")) {
      setAgentSpeech(`Running: ${msg.slice(1, 50)}`);
      await executeTool(msg.slice(1));
      setGenerating(false);
      return;
    }

    try {
      const a = api(); if (!a?.AIChat) { setError("AI unavailable"); return; }
      setAgentState("think");
      setAgentSpeech("Analyzing your wallet data...");
      const r = await a.AIChat(msg, mode);
      const content = r?.content || r?.error || "No response";
      setChat(p => [...p, { role: "assistant", content }]);
      setAgentState("speak");
      setAgentSpeech(content.slice(0, 200) + (content.length > 200 ? "..." : ""));
    } catch (e: any) {
      setError(e?.message);
      setAgentState("error");
      setAgentSpeech(e?.message || "AI request failed");
    }
    finally {
      setGenerating(false);
      setTimeout(() => { setAgentState("idle"); setAgentSpeech(""); }, 3000);
    }
  }

  async function executeTool(cmd: string) {
    const a = api();
    if (!a?.AIToolExecute) { setChat(p => [...p, { role: "system", content: "Tool execution not available" }]); return; }
    setChat(p => [...p, { role: "tool", content: `Running: ${cmd}` }]);
    setAgentState("code");
    try {
      const r = await a.AIToolExecute(cmd);
      const output = r.stdout || r.stderr || "(no output)";
      const truncated = r.truncated ? "\n[output truncated]" : "";
      const info = `[${r.allowed ? "ALLOWED" : "BLOCKED"}] ${r.duration} exit=${r.exit_code}`;
      setChat(p => [...p, { role: "tool", content: `${info}\n${output}${truncated}` }]);
      setAgentState(r.exit_code === 0 && r.allowed ? "success" : "error");
      setAgentSpeech(r.allowed ? `Exit ${r.exit_code} in ${r.duration}` : "Blocked or failed");
    } catch (e: any) {
      setChat(p => [...p, { role: "system", content: `Tool error: ${e?.message}` }]);
      setAgentState("error");
      setAgentSpeech(e?.message || "Tool execution failed");
    }
    setTimeout(() => { setAgentState("idle"); setAgentSpeech(""); }, 3000);
  }

  function mascotExpression() {
    if (generating) return "thinking";
    if (health?.status === "error") return "alert";
    if (health?.status === "ready") return "happy";
    if (health?.status === "disabled" || health?.status === "stopped") return "sleeping";
    return "idle";
  }

  const status = health?.status || "disabled";

  if (showWizard && status !== "ready") {
    return <AISetupWizard onComplete={() => { setShowWizard(false); refreshHealth(); }} />;
  }

  const prompts = mode === "developer"
    ? ["/legacycoin-cli getblockchaininfo", "/legacycoin-cli getpeerinfo", "/legacycoin-cli getmininginfo", "/get-process legacycoind", "/netstat -an | findstr 19555"]
    : ["Why is mining paused?", "Is my node synchronized?", "Explain degraded-safe mining.", "Explain my immature rewards.", "Is RPC healthy?", "Is storage healthy?"];

  const devTools = mode === "developer" ? tools : [];

  return (
    <div className="page legacyAIPage">
      <div className="heroPanel compactHero" style={{position:"relative"}}>
        <div style={{marginRight:12}}>
          <LegacyMascot expression={mascotExpression()} size={48} />
        </div>
        <div>
          <p className="eyebrow">Legacy AI Companion • Local • Private</p>
          <h3>Legacy AI <span style={{fontSize:12,opacity:0.6,marginLeft:8}}>{mode === "developer" ? "dev" : ""}</span></h3>
          <p>Fully local AI assistant. No cloud. No secrets shared.</p>
        </div>
        <span className="pill">v1.0.6</span>
      </div>

      <div className="row" style={{gap:8,marginBottom:12,flexWrap:"wrap"}}>
        <span className={`pill ${status==="ready"?"good":status==="error"?"danger":""}`}>
          {status==="ready"?<CheckCircle size={14}/>:<AlertTriangle size={14}/>} {status}
        </span>
        {health?.backend && <span className="pill"><Cpu size={14}/> {health.backend}</span>}
        {health?.gpu_name && <span className="pill"><HardDrive size={14}/> {health.gpu_name}</span>}
        {health?.pid > 0 && <span className="pill">PID: {health.pid}</span>}
        <button className="primary" onClick={() => api()?.AIStart()} disabled={status==="ready"}>Start AI</button>
        <button onClick={() => { api()?.AIStop(); refreshHealth(); }} disabled={status!=="ready"}>Stop</button>
        <button onClick={refreshHealth}><RefreshCw size={14}/> Refresh</button>
        <select value={mode} onChange={e => setMode(e.target.value as "advisor"|"developer")} style={{padding:"4px 8px",borderRadius:6}}>
          <option value="advisor">Advisor</option>
          <option value="developer">Developer</option>
        </select>
      </div>

      <div className="twoCol">
        <section className="panel">
          <h3>Privacy</h3>
          <div className="kv">
            <div><span>Runs locally</span><strong><CheckCircle size={14} className="green"/> Yes</strong></div>
            <div><span>No wallet secrets</span><strong><Shield size={14}/> Guaranteed</strong></div>
            <div><span>No cloud calls</span><strong>Offline</strong></div>
            <div><span>{mode === "developer" ? "Tool sandbox" : "Advisory only"}</span><strong>Read-only</strong></div>
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

      {mode === "developer" && devTools.length > 0 && (
        <section className="panel">
          <h3><Wrench size={14}/> Allowlisted Tools ({devTools.length})</h3>
          <div style={{display:"flex",flexWrap:"wrap",gap:4,marginTop:8,opacity:0.7}}>
            {devTools.map((t, i) => (
              <code key={i} style={{fontSize:11,background:"#1e293b",padding:"2px 6px",borderRadius:4,cursor:"pointer"}}
                onClick={() => { setInput("/" + t); }}>{t}</code>
            ))}
          </div>
        </section>
      )}

      <section className="panel">
        <h3>{mode === "developer" ? <Terminal size={14}/> : <Bot size={14}/>} {mode === "developer" ? "Developer Agent" : "Chat"}</h3>
        {mode === "developer" && (
          <p style={{fontSize:11,opacity:0.6,marginBottom:8}}>
            Prefix commands with "/" to execute allowlisted tools. Results appear in chat.
          </p>
        )}
        <div className="chatBox" style={{minHeight:200,maxHeight:400,overflow:"auto",marginBottom:8}}>
          {chat.length===0 && !generating && (
            <div style={{textAlign:"center",padding:24}}>
              <LegacyMascot expression="idle" size={48} />
              <p style={{marginTop:8}}>
                {mode === "developer" ? "Developer mode: run allowlisted CLI commands." : "Ask me about your wallet, node, mining, or peers."}
              </p>
              <div style={{display:"flex",flexWrap:"wrap",gap:6,marginTop:12,justifyContent:"center"}}>
                {prompts.map((q,i) => <button key={i} onClick={() => { setInput(q); setTimeout(send, 50); }} style={{fontSize:12}}>{q}</button>)}
              </div>
            </div>
          )}
          {chat.map((m,i) => {
            const isTool = m.role === "tool";
            const isUser = m.role === "user";
            const bg = isTool ? "#0d3b2e" : isUser ? "#1e293b" : "#0f172a";
            const label = isTool ? "Tool" : m.role === "user" ? "You" : "Legacy AI";
            return (
              <div key={i} style={{marginBottom:8,padding:8,background:bg,borderRadius:6}}>
                <strong style={{fontSize:11,opacity:0.6}}>{label}</strong>
                <pre style={{margin:"4px 0 0",whiteSpace:"pre-wrap",fontSize:12,fontFamily:"inherit"}}>{m.content}</pre>
              </div>
            );
          })}
          {generating && (
            <div style={{padding:8,opacity:0.6,display:"flex",alignItems:"center",gap:8}}>
              <LegacyMascot expression="thinking" size={20} bounce={false} />
              {mode === "developer" ? "Executing..." : "Generating..."}
            </div>
          )}
          <div ref={chatEnd} />
        </div>
        {error && <div className="notice danger" style={{marginBottom:8}}>{error}</div>}
        <div className="row">
          <input
            value={input} onChange={e => setInput(e.target.value)}
            onKeyDown={e => { if (e.key==="Enter") send(); }}
            placeholder={mode === "developer" ? "/command or ask Legacy AI..." : "Ask Legacy AI..."}
            disabled={generating} style={{flex:1}}
          />
          <button className="primary" onClick={send} disabled={generating||!input.trim()}>Send</button>
          <button onClick={() => setChat([])}><Trash2 size={14}/></button>
        </div>
      </section>
    </div>
  );
}
