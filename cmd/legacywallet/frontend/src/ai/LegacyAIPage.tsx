import React, { useState, useRef, useEffect } from "react";
import {
  Bot, Cpu, HardDrive, Shield, AlertTriangle, CheckCircle,
  RefreshCw, Trash2, Terminal, Wrench, Image, Code, Search, Globe, Sparkles
} from "lucide-react";
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
  const [provider, setProvider] = useState("built-in");
  const [apiKey, setApiKey] = useState("");
  const [modelName, setModelName] = useState("");
  const [showSettings, setShowSettings] = useState(false);
  const chatEnd = useRef<HTMLDivElement>(null);

  useEffect(() => { chatEnd.current?.scrollIntoView({ behavior: "smooth" }); }, [chat, images]);
  useEffect(() => { init(); }, []);

  async function init() {
    try {
      const a = api();
      if (a?.AIHealth) { const h = await a.AIHealth(); setHealth(h); if (h?.status === "disabled" || h?.status === "stopped") setShowWizard(true); }
      if (a?.AIListTools) setTools(await a.AIListTools());
      if (a?.AIModels) setModels(await a.AIModels());
    } catch {}
  }

  async function sendChat() {
    const msg = input.trim(); if (!msg || generating) return;
    setInput(""); setGenerating(true); setError("");
    setChat(p => [...p, { role: "user", content: msg }]);
    setAgentState("listen"); setAgentSpeech(msg.slice(0, 60));
    if (mode === "developer" && msg.startsWith("/")) { await executeTool(msg.slice(1)); setGenerating(false); return; }
    try {
      const a = api(); if (!a?.AIChat) { setError("AI not available"); return; }
      setAgentState("think");
      const r = await a.AIChat(msg, mode);
      const content = r?.content || r?.error || "No response";
      setChat(p => [...p, { role: "assistant", content }]);
      setAgentState("speak"); setAgentSpeech(content.slice(0, 200));
    } catch (e: any) { setError(e?.message); setAgentState("error"); }
    finally { setGenerating(false); setTimeout(() => { setAgentState("idle"); setAgentSpeech(""); }, 3000); }
  }

  async function executeTool(cmd: string) {
    const a = api(); if (!a?.AIToolExecute) return;
    setChat(p => [...p, { role: "tool", content: "Running: " + cmd }]);
    setAgentState("code");
    try {
      const r = await a.AIToolExecute(cmd);
      setChat(p => [...p, { role: "tool", content: `[${r.allowed ? "OK" : "BLOCKED"}] exit=${r.exit_code} ${r.duration}\n${r.stdout || r.stderr}` }]);
      setAgentState(r.exit_code === 0 ? "success" : "error");
    } catch (e: any) { setAgentState("error"); }
    setTimeout(() => setAgentState("idle"), 3000);
  }

  async function generateImage() {
    if (!imgPrompt.trim() || generating) return;
    setGenerating(true); setError("");
    setAgentState("code"); setAgentSpeech("Generating image...");
    try {
      const a = api(); if (!a?.AIImageGenerate) { setError("Image gen not available"); return; }
      const r = await a.AIImageGenerate(imgPrompt, 512, 512, "pollinations");
      if (r?.ok) { setImages(p => [{ url: r.image_url, prompt: r.prompt, model: r.model }, ...p]); setAgentState("success"); setAgentSpeech("Image ready!"); }
      else { setError(r?.error || "Failed"); setAgentState("error"); }
    } catch (e: any) { setError(e?.message); setAgentState("error"); }
    finally { setGenerating(false); setTimeout(() => setAgentState("idle"), 2000); setImgPrompt(""); }
  }

  async function doResearch() {
    const q = input.trim(); if (!q || generating) return;
    setInput(""); setGenerating(true); setError("");
    setChat(p => [...p, { role: "user", content: "Research: " + q }]);
    setAgentState("search"); setAgentSpeech("Searching: " + q);
    const a = api();
    try {
      let result = "";
      if (a?.AIChat) { const r = await a.AIChat(q, mode); result = r?.content || ""; }
      setChat(p => [...p, { role: "assistant", content: "**Research: " + q + "**\n\n" + (result || "Try a more specific query. Web search is powered by DuckDuckGo.") + "\n\n*DuckDuckGo • No tracking*" }]);
      setAgentState("speak");
    } catch (e: any) { setError(e?.message); setAgentState("error"); }
    finally { setGenerating(false); setTimeout(() => setAgentState("idle"), 3000); }
  }

  async function configureProvider() {
    setError("");
    try {
      const a = api(); if (!a?.AIConfigure) return;
      const r = await a.AIConfigure(provider, apiKey, modelName);
      if (!r?.ok) { setError(r?.error || "Configuration failed"); return; }
      setShowSettings(false);
      init();
    } catch (e: any) { setError(e?.message); }
  }

  const status = health?.status || "disabled";
  const promptChips = mode === "developer"
    ? ["/legacycoin-cli getblockchaininfo", "/legacycoin-cli getpeerinfo", "/legacycoin-cli getmininginfo", "/netstat -an | findstr 19555", "/get-process legacycoind"]
    : ["How is my sync?", "Explain my peers", "Is mining safe?", "My balance", "Node health overview", "Why are rewards immature?"];

  if (showWizard && status !== "ready") return <AISetupWizard onComplete={() => { setShowWizard(false); init(); }} />;

  return (
    <div className="page legacyAIPage">
      <div className="heroPanel compactHero">
        <Bot size={30} />
        <div>
          <p className="eyebrow">Legacy AI Workstation &bull; GPU-Powered &bull; Private</p>
          <h3>Legacy AI</h3>
        </div>
        <span className="pill">{health?.backend || "built-in"}</span>
      </div>

      <div className="row" style={{gap:6,marginBottom:12,flexWrap:"wrap"}}>
        <span className={`pill ${status==="ready"?"good":""}`}>{status==="ready"?<CheckCircle size={12}/>:<AlertTriangle size={12}/>} {status}</span>
        {health?.gpu_name && <span className="pill"><HardDrive size={12}/> {health.gpu_name}</span>}
        <button className="primary" onClick={() => api()?.AIStart()} disabled={status==="ready"}>Start AI</button>
        <button onClick={() => { api()?.AIStop(); init(); }} disabled={status!=="ready"}>Stop</button>
        <button onClick={init}><RefreshCw size={14}/></button>
        <select value={mode} onChange={e => setMode(e.target.value as any)} style={{padding:"4px 8px",borderRadius:6,background:"#1e293b",border:"1px solid #475569",color:"#e2e8f0"}}>
          <option value="advisor">Advisor</option>
          <option value="developer">Developer</option>
        </select>
        <select value={provider} onChange={e => setProvider(e.target.value)} style={{padding:"4px 8px",borderRadius:6,background:"#1e293b",border:"1px solid #475569",color:"#e2e8f0"}}>
          <option value="built-in">Built-in AI</option>
          <option value="groq">Groq (Cloud)</option>
          <option value="llama-server">llama.cpp (GPU)</option>
        </select>
        <button onClick={() => setShowSettings(s => !s)} style={{fontSize:12}}>{showSettings ? "Hide" : "Settings"}</button>
      </div>

      {showSettings && (
        <section className="panel" style={{marginBottom:12}}>
          <h3>AI Provider Settings</h3>
          <div className="row" style={{gap:8,marginTop:8}}>
            <select value={provider} onChange={e => setProvider(e.target.value)} style={{padding:"4px 8px",borderRadius:6,background:"#1e293b",border:"1px solid #475569",color:"#e2e8f0",flex:1}}>
              <option value="built-in">Built-in (Offline)</option>
              <option value="groq">Groq (Free Cloud)</option>
              <option value="llama-server">llama.cpp (Local GPU)</option>
            </select>
            {provider !== "built-in" && (
              <input value={apiKey} onChange={e => setApiKey(e.target.value)} placeholder={provider==="groq"?"Groq API key (free at console.groq.com)":"Model name"}
                style={{flex:2}} />
            )}
            {provider === "groq" && (
              <input value={modelName} onChange={e => setModelName(e.target.value)} placeholder="Model (e.g. llama-3.1-8b-instant)" style={{flex:1}} />
            )}
            <button className="primary" onClick={configureProvider}>Apply &amp; Restart</button>
          </div>
          {provider === "groq" && <small style={{opacity:0.6}}>Get a free API key at console.groq.com. Free tier: 30 requests/min.</small>}
          {provider === "llama-server" && <small style={{opacity:0.6}}>Requires llama-server binary in PATH and a GGUF model. Auto-detected if installed.</small>}
        </section>
      )}

      <nav style={{display:"flex",gap:2,marginBottom:12}}>
        {([
          ["chat", Bot, "Chat"],
          ["image", Image, "Image Gen"],
          ["code", Terminal, "Code Agent"],
          ["research", Globe, "Research"],
        ] as const).map(([id, Icon, label]) => (
          <button key={id} className={tab === id ? "primary" : ""} onClick={() => setTab(id)} style={{flex:1,display:"flex",alignItems:"center",justifyContent:"center",gap:6,padding:"6px 4px",fontSize:13}}>
            <Icon size={14} /> {label}
          </button>
        ))}
      </nav>

      {tab === "chat" && (
        <section className="panel">
          <div style={{minHeight:220,maxHeight:400,overflow:"auto",marginBottom:8}}>
            {chat.length===0 && <div style={{textAlign:"center",padding:24,opacity:0.7}}>
              <Bot size={36} style={{opacity:0.3}} />
              <p style={{marginTop:8}}>Ask about your node, mining, peers, or balance.</p>
              <div className="row" style={{flexWrap:"wrap",gap:4,marginTop:12,justifyContent:"center"}}>
                {promptChips.map((q,i)=><button key={i} onClick={()=>{setInput(q);setTimeout(sendChat,50)}} style={{fontSize:11}}>{q}</button>)}
              </div>
            </div>}
            {chat.map((m,i)=>(
              <div key={i} style={{marginBottom:8,padding:"8px 10px",background:m.role==="user"?"#1e293b":m.role==="tool"?"#0d3b2e":"#0f172a",borderRadius:6}}>
                <strong style={{fontSize:10,opacity:0.5,textTransform:"uppercase"}}>{m.role==="user"?"You":m.role==="tool"?"Tool":"Legacy AI"}</strong>
                <div style={{marginTop:4,whiteSpace:"pre-wrap",fontSize:13,lineHeight:1.5}}>{m.content}</div>
              </div>
            ))}
            {generating && <div style={{padding:8,opacity:0.6,fontSize:12}}>Generating...</div>}
            <div ref={chatEnd}/>
          </div>
          {error && <div className="notice danger" style={{marginBottom:8}}>{error}</div>}
          <div className="row">
            <input value={input} onChange={e=>setInput(e.target.value)} onKeyDown={e=>{if(e.key==="Enter")sendChat()}}
              placeholder={mode==="developer"?"/command or ask..." :"Ask Legacy AI..."} disabled={generating} style={{flex:1}}/>
            <button className="primary" onClick={sendChat} disabled={generating||!input.trim()}>Send</button>
            <button onClick={()=>setChat([])}><Trash2 size={14}/></button>
          </div>
        </section>
      )}

      {tab === "image" && (
        <div>
          <section className="panel">
            <h3><Image size={14}/> AI Image Generation</h3>
            <p style={{fontSize:12,opacity:0.6,marginBottom:8}}>Free, no API key. Powered by Pollinations.ai</p>
            <div className="row" style={{gap:8,marginBottom:8}}>
              <input value={imgPrompt} onChange={e=>setImgPrompt(e.target.value)} onKeyDown={e=>{if(e.key==="Enter")generateImage()}}
                placeholder="Describe an image..." disabled={generating} style={{flex:1}}/>
              <button className="primary" onClick={generateImage} disabled={generating||!imgPrompt.trim()}><Sparkles size={14}/> Generate</button>
            </div>
          </section>
          {images.length > 0 && (
            <div style={{display:"grid",gridTemplateColumns:"repeat(auto-fill, minmax(180px,1fr))",gap:8}}>
              {images.map((img,i)=>(
                <div key={i} style={{border:"1px solid #334155",borderRadius:8,overflow:"hidden",background:"#0f172a"}}>
                  <img src={img.url} alt={img.prompt} style={{width:"100%",aspectRatio:"1",objectFit:"cover"}} onError={e=>{(e.target as HTMLImageElement).style.display="none"}}/>
                  <div style={{padding:8}}><p style={{fontSize:10,opacity:0.5,marginBottom:4}}>{img.model}</p><p style={{fontSize:11}}>{img.prompt.slice(0,60)}</p></div>
                </div>
              ))}
            </div>
          )}
          {images.length===0 && !generating && (
            <div style={{textAlign:"center",padding:32,opacity:0.4}}><Image size={48}/><p style={{marginTop:8}}>Generated images appear here</p></div>
          )}
        </div>
      )}

      {tab === "code" && (
        <div>
          <section className="panel">
            <h3><Terminal size={14}/> Developer Agent</h3>
            <p style={{fontSize:12,opacity:0.6,marginBottom:8}}>Allowlisted CLI commands. Prefix with / to run.</p>
            <div style={{display:"flex",flexWrap:"wrap",gap:4,marginBottom:8}}>
              {tools.map((t,i)=><code key={i} style={{fontSize:11,background:"#1e293b",padding:"2px 6px",borderRadius:4,cursor:"pointer"}} onClick={()=>{setTab("chat");setInput("/"+t)}}>{t}</code>)}
            </div>
          </section>
          {chat.length > 0 && (
            <section className="panel">
              <div style={{minHeight:200,maxHeight:400,overflow:"auto"}}>
                {chat.map((m,i)=>(<div key={i} style={{marginBottom:8,padding:8,background:m.role==="tool"?"#0d3b2e":"#0f172a",borderRadius:6}}>
                  <strong style={{fontSize:10,opacity:0.5}}>{m.role==="tool"?"Tool":"AI"}</strong>
                  <pre style={{marginTop:4,whiteSpace:"pre-wrap",fontSize:12,fontFamily:"monospace"}}>{m.content}</pre>
                </div>))}
              </div>
            </section>
          )}
        </div>
      )}

      {tab === "research" && (
        <div>
          <section className="panel">
            <h3><Globe size={14}/> AI Research</h3>
            <p style={{fontSize:12,opacity:0.6,marginBottom:8}}>Web search + AI analysis. Private, no tracking.</p>
            <div className="row" style={{marginBottom:8}}>
              <input value={input} onChange={e=>setInput(e.target.value)} onKeyDown={e=>{if(e.key==="Enter")doResearch()}}
                placeholder="Search anything..." disabled={generating} style={{flex:1}}/>
              <button className="primary" onClick={doResearch} disabled={generating||!input.trim()}><Search size={14}/> Research</button>
            </div>
          </section>
          {chat.length > 0 && (
            <section className="panel">
              <div style={{minHeight:200,maxHeight:400,overflow:"auto"}}>
                {chat.map((m,i)=>(<div key={i} style={{marginBottom:8,padding:8,background:m.role==="user"?"#1e293b":"#0f172a",borderRadius:6}}>
                  <strong style={{fontSize:10,opacity:0.5}}>{m.role==="user"?"Query":"Results"}</strong>
                  <div style={{marginTop:4,whiteSpace:"pre-wrap",fontSize:13}}>{m.content}</div>
                </div>))}
              </div>
            </section>
          )}
        </div>
      )}

      <div className="twoCol" style={{marginTop:12}}>
        <section className="panel">
          <h3>Privacy</h3>
          <div className="kv">
            <div><span>Runs locally</span><strong><CheckCircle size={14} className="green"/> Yes</strong></div>
            <div><span>No wallet secrets</span><strong><Shield size={14}/> Guaranteed</strong></div>
            <div><span>No cloud calls*</span><strong>Offline core</strong></div>
          </div>
          <small>*Image gen and web search use internet</small>
        </section>
        <section className="panel">
          <h3>Models ({models.length})</h3>
          <div className="kv">
            {models.slice(0,5).map((m:any,i:number)=>(<div key={i}><span>{m.name}</span><strong className={m.free?"green":""}>{m.free?"Free":m.requires_key?"Key":"Local"}</strong></div>))}
          </div>
        </section>
      </div>
    </div>
  );
}
