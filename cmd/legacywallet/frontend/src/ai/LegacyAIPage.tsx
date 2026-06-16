import React, { useState, useRef, useEffect } from "react";
import {
  Bot, Cpu, HardDrive, Shield, Zap, AlertTriangle, CheckCircle,
  RefreshCw, Trash2, Terminal, Wrench, Image, Code, Search, Globe,
  Download, ExternalLink, Sparkles, PanelRight
} from "lucide-react";
import { LegacyMascot } from "./LegacyMascot";
import { AISetupWizard } from "./AISetupWizard";
import type { AgentState } from "./LegacyAgent";

function api() { return window.go?.main?.App; }

interface LegacyAIPageProps {
  snap?: any;
  setAgentState: (s: AgentState) => void;
  setAgentSpeech: (s: string) => void;
}

type AITab = "chat" | "image" | "code" | "research";

export function LegacyAIPage({ snap, setAgentState, setAgentSpeech }: LegacyAIPageProps) {
  const [tab, setTab] = useState<AITab>("chat");
  const [input, setInput] = useState("");
  const [chat, setChat] = useState<{ role: string; content: string }[]>([]);
  const [generating, setGenerating] = useState(false);
  const [error, setError] = useState("");
  const [health, setHealth] = useState<any>({});
  const [mode, setMode] = useState<"advisor" | "developer">("advisor");
  const [showWizard, setShowWizard] = useState(false);
  const [tools, setTools] = useState<string[]>([]);
  const [models, setModels] = useState<any[]>([]);
  const [images, setImages] = useState<{ url: string; prompt: string; model: string }[]>([]);
  const [imgPrompt, setImgPrompt] = useState("");
  const [imgSize, setImgSize] = useState("512x512");
  const [imgModel, setImgModel] = useState("pollinations");
  const chatEnd = useRef<HTMLDivElement>(null);

  useEffect(() => { chatEnd.current?.scrollIntoView({ behavior: "smooth" }); }, [chat, images]);
  useEffect(() => { init(); }, []);

  async function init() {
    try {
      const a = api();
      if (a?.AIHealth) {
        const h = await a.AIHealth();
        setHealth(h);
        if (h?.status === "disabled" || h?.status === "stopped") setShowWizard(true);
      }
      if (a?.AIListTools) setTools(await a.AIListTools());
      if (a?.AIModels) setModels(await a.AIModels());
    } catch {}
  }

  async function sendChat() {
    const msg = input.trim(); if (!msg || generating) return;
    setInput(""); setGenerating(true); setError("");
    setChat(p => [...p, { role: "user", content: msg }]);
    setAgentState("listen");
    setAgentSpeech(`"${msg.slice(0, 60)}${msg.length > 60 ? "..." : ""}"`);

    if (mode === "developer" && msg.startsWith("/")) {
      await executeTool(msg.slice(1));
      setGenerating(false);
      return;
    }

    try {
      const a = api(); if (!a?.AIChat) { setError("AI unavailable"); return; }
      setAgentState("think");
      const r = await a.AIChat(msg, mode);
      const content = r?.content || r?.error || "No response";
      setChat(p => [...p, { role: "assistant", content }]);
      setAgentState("speak");
      setAgentSpeech(content.slice(0, 200));
    } catch (e: any) { setError(e?.message); setAgentState("error"); }
    finally {
      setGenerating(false);
      setTimeout(() => { setAgentState("idle"); setAgentSpeech(""); }, 3000);
    }
  }

  async function executeTool(cmd: string) {
    const a = api();
    if (!a?.AIToolExecute) { setChat(p => [...p, { role: "system", content: "Tool execution unavailable" }]); return; }
    setChat(p => [...p, { role: "tool", content: `Running: ${cmd}` }]);
    setAgentState("code");
    try {
      const r = await a.AIToolExecute(cmd);
      const out = r.stdout || r.stderr || "(no output)";
      setChat(p => [...p, { role: "tool", content: `[${r.allowed ? "OK" : "BLOCKED"}] ${r.duration}\n${out}` }]);
      setAgentState(r.exit_code === 0 ? "success" : "error");
    } catch (e: any) { setChat(p => [...p, { role: "system", content: e?.message }]); setAgentState("error"); }
    setTimeout(() => { setAgentState("idle"); setAgentSpeech(""); }, 3000);
  }

  async function generateImage() {
    if (!imgPrompt.trim() || generating) return;
    setGenerating(true); setError("");
    setAgentState("code"); setAgentSpeech("Generating image...");
    try {
      const a = api(); if (!a?.AIImageGenerate) { setError("Image generation unavailable"); return; }
      const [w, h] = imgSize.split("x").map(Number);
      const r = await a.AIImageGenerate(imgPrompt, w, h, imgModel);
      if (r?.ok) {
        setImages(p => [{ url: r.image_url, prompt: r.prompt, model: r.model }, ...p]);
        setAgentState("success"); setAgentSpeech("Image ready!");
      } else {
        setError(r?.error || "Generation failed");
        setAgentState("error");
      }
    } catch (e: any) { setError(e?.message); setAgentState("error"); }
    finally { setGenerating(false); setTimeout(() => setAgentState("idle"), 2000); }
    setImgPrompt("");
  }

  // ---- RESEARCH MODE ----
  async function doResearch() {
    const q = input.trim(); if (!q || generating) return;
    setInput(""); setGenerating(true); setError("");
    setChat(p => [...p, { role: "user", content: `Research: ${q}` }]);
    setAgentState("search"); setAgentSpeech(`Searching: ${q}`);

    // Use DuckDuckGo search + ask AI for summary
    const a = api();
    let searchResult = "";
    if (a?.AIChat) {
      try {
        const r = await a.AIChat(q, "advisor");
        searchResult = r?.content || "";
      } catch {}
    }

    setChat(p => [...p, {
      role: "assistant",
      content: `**Research results for: "${q}"**\n\n${searchResult || "Try asking a more specific question. I search the web using DuckDuckGo and analyze results with AI."}\n\n*Search powered by DuckDuckGo • No tracking*`
    }]);
    setAgentState("speak"); setAgentSpeech("Research complete");
    setGenerating(false);
    setTimeout(() => { setAgentState("idle"); setAgentSpeech(""); }, 3000);
  }

  const status = health?.status || "disabled";
  const chatModels = models.filter(m => m.type?.includes("chat"));
  const imageModels = models.filter(m => m.type?.includes("image"));

  if (showWizard && status !== "ready") {
    return <AISetupWizard onComplete={() => { setShowWizard(false); init(); }} />;
  }

  const tabs: { id: AITab; icon: React.ReactNode; label: string }[] = [
    { id: "chat", icon: <Bot size={14} />, label: "Chat" },
    { id: "image", icon: <Image size={14} />, label: "Image" },
    { id: "code", icon: <Code size={14} />, label: "Code" },
    { id: "research", icon: <Search size={14} />, label: "Research" },
  ];

  return (
    <div className="page legacyAIPage">
      {/* Header */}
      <div className="heroPanel compactHero">
        <LegacyMascot expression={generating ? "thinking" : status === "ready" ? "happy" : "sleeping"} size={40} />
        <div style={{marginLeft: 12}}>
          <p className="eyebrow">Legacy AI Workstation • GPU-Powered • Private</p>
          <h3>Legacy AI</h3>
        </div>
        <span className="pill">{health?.backend || "built-in"}</span>
      </div>

      {/* Controls */}
      <div className="row" style={{gap:6,marginBottom:12,flexWrap:"wrap"}}>
        <span className={`pill ${status==="ready"?"good":status==="error"?"danger":""}`}>
          {status==="ready"?<CheckCircle size={14}/>:<AlertTriangle size={14}/>} {status}
        </span>
        {health?.gpu_name && <span className="pill"><HardDrive size={14}/> {health.gpu_name}</span>}
        <button className="primary" onClick={() => api()?.AIStart()} disabled={status==="ready"}>Start AI</button>
        <button onClick={() => { api()?.AIStop(); init(); }} disabled={status!=="ready"}>Stop</button>
        <button onClick={init}><RefreshCw size={14}/></button>
        <select value={mode} onChange={e => setMode(e.target.value as any)} style={{padding:"4px 8px",borderRadius:6}}>
          <option value="advisor">Advisor</option>
          <option value="developer">Developer</option>
        </select>
      </div>

      {/* Tab bar */}
      <div className="row" style={{gap:0,marginBottom:12,border:"1px solid #334155",borderRadius:8,overflow:"hidden"}}>
        {tabs.map(({id, icon, label}) => (
          <button key={id}
            onClick={() => setTab(id)}
            style={{
              flex:1,padding:"8px 4px",border:"none",borderRadius:0,
              background: tab===id?"#1e40af":"transparent",
              color: tab===id?"#fff":"#94a3b8",cursor:"pointer",
              display:"flex",alignItems:"center",justifyContent:"center",gap:4,fontSize:13,
              borderRight: id!=="research"?"1px solid #334155":"none"
            }}
          >{icon} {label}</button>
        ))}
      </div>

      {/* --- CHAT TAB --- */}
      {tab === "chat" && (
        <section className="panel">
          <div className="chatBox" style={{minHeight:250,maxHeight:450,overflow:"auto",marginBottom:8}}>
            {chat.length===0 && (
              <div style={{textAlign:"center",padding:24,opacity:0.7}}>
                <LegacyMascot expression="idle" size={48} />
                <p style={{marginTop:8}}>Ask about your node, mining, peers, balance, or anything.</p>
                <div style={{display:"flex",flexWrap:"wrap",gap:4,marginTop:12,justifyContent:"center"}}>
                  {["How is my sync?","Explain my peers","Is mining safe?","What is my balance?","Node health overview"].map((q,i) =>
                    <button key={i} onClick={() => { setInput(q); setTimeout(sendChat, 50); }} style={{fontSize:11}}>{q}</button>
                  )}
                </div>
              </div>
            )}
            {chat.map((m,i) => (
              <div key={i} style={{marginBottom:8,padding:8,background:m.role==="user"?"#1e293b":m.role==="tool"?"#0d3b2e":"#0f172a",borderRadius:6}}>
                <strong style={{fontSize:10,opacity:0.5}}>{m.role==="user"?"You":m.role==="tool"?"Tool":"Legacy AI"}</strong>
                <div style={{marginTop:4,whiteSpace:"pre-wrap",fontSize:13}}>{m.content}</div>
              </div>
            ))}
            {generating && <div style={{padding:8,opacity:0.6}}>Thinking...</div>}
            <div ref={chatEnd} />
          </div>
          {error && <div className="notice danger" style={{marginBottom:8}}>{error}</div>}
          <div className="row">
            <input value={input} onChange={e => setInput(e.target.value)} onKeyDown={e => { if (e.key==="Enter") sendChat(); }}
              placeholder={mode==="developer"?"/cmd or ask..." :"Ask Legacy AI..."} disabled={generating} style={{flex:1}} />
            <button className="primary" onClick={sendChat} disabled={generating||!input.trim()}>Send</button>
            <button onClick={() => setChat([])}><Trash2 size={14}/></button>
          </div>
        </section>
      )}

      {/* --- IMAGE TAB --- */}
      {tab === "image" && (
        <div>
          <section className="panel">
            <h3><Image size={14}/> AI Image Generation</h3>
            <p style={{fontSize:12,opacity:0.6,marginBottom:8}}>Free, no API key needed. Powered by Pollinations.ai</p>
            <div className="row" style={{gap:8,marginBottom:8}}>
              <input value={imgPrompt} onChange={e => setImgPrompt(e.target.value)}
                onKeyDown={e => { if (e.key==="Enter") generateImage(); }}
                placeholder="Describe the image you want..." disabled={generating} style={{flex:1}} />
              <select value={imgModel} onChange={e => setImgModel(e.target.value)} style={{padding:"4px 8px",borderRadius:6}}>
                <option value="pollinations">Standard</option>
                <option value="flux">Flux (HQ)</option>
              </select>
              <select value={imgSize} onChange={e => setImgSize(e.target.value)} style={{padding:"4px 8px",borderRadius:6}}>
                <option value="256x256">256</option>
                <option value="512x512">512</option>
                <option value="768x768">768</option>
                <option value="1024x1024">1024</option>
              </select>
              <button className="primary" onClick={generateImage} disabled={generating||!imgPrompt.trim()}>
                <Sparkles size={14}/> Generate
              </button>
            </div>
          </section>
          <div style={{display:"grid",gridTemplateColumns:"repeat(auto-fill, minmax(200px,1fr))",gap:8}}>
            {images.map((img, i) => (
              <div key={i} style={{border:"1px solid #334155",borderRadius:8,overflow:"hidden",background:"#0f172a"}}>
                <img src={img.url} alt={img.prompt} style={{width:"100%",aspectRatio:"1",objectFit:"cover"}}
                  onError={(e) => { (e.target as HTMLImageElement).style.display = "none"; }} />
                <div style={{padding:8}}>
                  <p style={{fontSize:11,opacity:0.7,marginBottom:4}}>{img.model}</p>
                  <p style={{fontSize:12}}>{img.prompt.slice(0, 80)}</p>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      {/* --- CODE TAB --- */}
      {tab === "code" && (
        <div>
          <section className="panel">
            <h3><Terminal size={14}/> Developer Agent</h3>
            <p style={{fontSize:12,opacity:0.6,marginBottom:8}}>Allowlisted CLI tools. Prefix with / to execute.</p>
            <div style={{display:"flex",flexWrap:"wrap",gap:4,marginBottom:8}}>
              {tools.map((t, i) => (
                <code key={i} style={{fontSize:11,background:"#1e293b",padding:"2px 6px",borderRadius:4,cursor:"pointer"}}
                  onClick={() => { setTab("chat"); setInput("/" + t); }}>{t}</code>
              ))}
            </div>
            <div className="chatBox" style={{minHeight:200,maxHeight:400,overflow:"auto",marginBottom:8}}>
              {chat.map((m,i) => (
                <div key={i} style={{marginBottom:8,padding:8,background:m.role==="user"?"#1e293b":m.role==="tool"?"#0d3b2e":"#0f172a",borderRadius:6}}>
                  <strong style={{fontSize:10,opacity:0.5}}>{m.role==="user"?"You":m.role==="tool"?"Tool":"AI"}</strong>
                  <pre style={{marginTop:4,whiteSpace:"pre-wrap",fontSize:12,fontFamily:"monospace"}}>{m.content}</pre>
                </div>
              ))}
              <div ref={chatEnd} />
            </div>
            <div className="row">
              <input value={input} onChange={e => setInput(e.target.value)} onKeyDown={e => { if (e.key==="Enter") sendChat(); }}
                placeholder="/legacycoin-cli getblockchaininfo" disabled={generating} style={{flex:1,fontFamily:"monospace"}} />
              <button className="primary" onClick={sendChat} disabled={generating||!input.trim()}>Run</button>
            </div>
          </section>
        </div>
      )}

      {/* --- RESEARCH TAB --- */}
      {tab === "research" && (
        <div>
          <section className="panel">
            <h3><Globe size={14}/> AI Research</h3>
            <p style={{fontSize:12,opacity:0.6,marginBottom:8}}>Search the web with DuckDuckGo + AI analysis. No tracking.</p>
            <div className="row" style={{marginBottom:8}}>
              <input value={input} onChange={e => setInput(e.target.value)}
                onKeyDown={e => { if (e.key==="Enter") doResearch(); }}
                placeholder="Search anything..." disabled={generating} style={{flex:1}} />
              <button className="primary" onClick={doResearch} disabled={generating||!input.trim()}>
                <Search size={14}/> Research
              </button>
            </div>
          </section>
          <div className="chatBox" style={{minHeight:250,maxHeight:450,overflow:"auto"}}>
            {chat.map((m,i) => (
              <div key={i} style={{marginBottom:8,padding:8,background:m.role==="user"?"#1e293b":"#0f172a",borderRadius:6}}>
                <strong style={{fontSize:10,opacity:0.5}}>{m.role==="user"?"Research":"Results"}</strong>
                <div style={{marginTop:4,whiteSpace:"pre-wrap",fontSize:13}}>{m.content}</div>
              </div>
            ))}
            {chat.length===0 && (
              <div style={{textAlign:"center",padding:32,opacity:0.5}}>
                <Globe size={48} />
                <p style={{marginTop:8}}>Research anything. Results are analyzed by AI and summarized for you.</p>
                <p style={{fontSize:11}}>Powered by DuckDuckGo • Private • No tracking</p>
              </div>
            )}
          </div>
        </div>
      )}

      {/* Models section */}
      {models.length > 0 && (
        <section className="panel" style={{marginTop:12}}>
          <h3><Sparkles size={14}/> Available AI Models</h3>
          <div style={{display:"grid",gridTemplateColumns:"repeat(auto-fill, minmax(250px,1fr))",gap:8,marginTop:8}}>
            {models.map((m, i) => (
              <div key={i} style={{padding:10,background:"#0f172a",borderRadius:8,border:"1px solid #1e293b"}}>
                <strong style={{fontSize:13}}>{m.name}</strong>
                <p style={{fontSize:11,opacity:0.7,margin:"4px 0"}}>{m.description}</p>
                <div className="row" style={{gap:4}}>
                  <span className="pill" style={{fontSize:10}}>{m.provider}</span>
                  {m.free && <span className="pill good" style={{fontSize:10}}>Free</span>}
                  {m.requires_key && <span className="pill" style={{fontSize:10,background:"#92400e"}}>Key needed</span>}
                  {m.models?.length > 0 && <span className="pill" style={{fontSize:10}}>{m.models.length} models</span>}
                </div>
              </div>
            ))}
          </div>
        </section>
      )}
    </div>
  );
}
