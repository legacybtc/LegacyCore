import React, { useState, useRef, useEffect, useCallback, useMemo } from "react";

interface AgentPosition { x: number; y: number }

export type AgentState =
  | "hidden" | "idle" | "wake" | "listen" | "think" | "speak"
  | "code" | "search" | "success" | "warning" | "error" | "sleep";

interface LegacyAgentProps {
  state?: AgentState;
  speechText?: string;
  onDoubleClick?: () => void;
  onHide?: () => void;
  reducedMotion?: boolean;
  size?: number;
}

const defaultPos = { x: window.innerWidth - 80, y: window.innerHeight - 120 };

function loadPos(): AgentPosition {
  try {
    const raw = localStorage.getItem("legacy-ai-agent-pos");
    if (raw) return JSON.parse(raw);
  } catch {}
  return defaultPos;
}

function savePos(p: AgentPosition) {
  try { localStorage.setItem("legacy-ai-agent-pos", JSON.stringify(p)); } catch {}
}

export function LegacyAgent({ state = "idle", speechText, onDoubleClick, onHide, reducedMotion = false, size = 52 }: LegacyAgentProps) {
  const [pos, setPos] = useState<AgentPosition>(loadPos);
  const [dragging, setDragging] = useState(false);
  const [hidden, setHidden] = useState(false);
  const [showBubble, setShowBubble] = useState(false);
  const dragOffset = useRef({ x: 0, y: 0 });
  const agentRef = useRef<HTMLDivElement>(null);
  const posRef = useRef(pos);
  posRef.current = pos;

  useEffect(() => { savePos(pos); }, [pos]);

  const clamp = useCallback((x: number, y: number) => ({
    x: Math.max(0, Math.min(x, window.innerWidth - size)),
    y: Math.max(0, Math.min(y, window.innerHeight - size - 40)),
  }), [size]);

  const handleMouseDown = useCallback((e: React.MouseEvent) => {
    e.preventDefault();
    setDragging(true);
    dragOffset.current = { x: e.clientX - posRef.current.x, y: e.clientY - posRef.current.y };
    setShowBubble(false);
  }, []);

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (dragging) setPos(clamp(e.clientX - dragOffset.current.x, e.clientY - dragOffset.current.y));
  }, [dragging, clamp]);

  const handleMouseUp = useCallback(() => { setDragging(false); }, []);

  useEffect(() => {
    if (dragging) {
      window.addEventListener("mousemove", handleMouseMove);
      window.addEventListener("mouseup", handleMouseUp);
      return () => {
        window.removeEventListener("mousemove", handleMouseMove);
        window.removeEventListener("mouseup", handleMouseUp);
      };
    }
  }, [dragging, handleMouseMove, handleMouseUp]);

  const handleClick = useCallback(() => {
    if (!dragging) setShowBubble(b => !b);
  }, [dragging]);

  const handleDblClick = useCallback(() => {
    if (onDoubleClick) { onDoubleClick(); setShowBubble(false); }
  }, [onDoubleClick]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === "Escape" || e.key === "Enter") setShowBubble(false);
  }, []);

  if (hidden) {
    return (
      <button
        onClick={() => setHidden(false)}
        style={{
          position: "fixed", bottom: 16, right: 16, zIndex: 9999,
          width: 36, height: 36, borderRadius: "50%",
          background: "#1e293b", border: "1px solid #475569",
          color: "#60a5fa", cursor: "pointer", display: "flex",
          alignItems: "center", justifyContent: "center", fontSize: 16,
        }}
        title="Show Legacy AI Agent"
      >
        &#9672;
      </button>
    );
  }

  const expression = useMemo(() => {
    switch (state) {
      case "wake": case "listen": return "idle";
      case "think": case "search": return "thinking";
      case "speak": return "happy";
      case "code": return "thinking";
      case "success": return "happy";
      case "warning": return "alert";
      case "error": return "alert";
      case "sleep": return "sleeping";
      default: return "idle";
    }
  }, [state]);

  const floatAnim = reducedMotion ? "none" : "float 3s ease-in-out infinite";
  const draggable = dragging ? undefined : "auto";

  return (
    <>
      <style>{`
        @keyframes float {
          0%, 100% { transform: translateY(0); }
          50% { transform: translateY(-4px); }
        }
        @keyframes bubbleIn {
          from { opacity: 0; transform: translateY(4px); }
          to { opacity: 1; transform: translateY(0); }
        }
        .lagent-bubble {
          animation: bubbleIn 0.2s ease-out;
        }
        .lagent-no-select {
          user-select: none;
          -webkit-user-select: none;
        }
      `}</style>
      <div
        ref={agentRef}
        className="lagent-no-select"
        tabIndex={0}
        onKeyDown={handleKeyDown}
        style={{
          position: "fixed", left: pos.x, top: pos.y, zIndex: 9998,
          width: size, height: size, cursor: dragging ? "grabbing" : "grab",
          transition: dragging ? "none" : "left 0.3s ease, top 0.3s ease",
        }}
      >
        {/* Speech bubble */}
        {showBubble && speechText && (
          <div
            className="lagent-bubble"
            style={{
              position: "absolute", bottom: size + 10, left: -80, width: 220,
              maxHeight: 120, overflow: "auto",
              background: "#1e293b", border: "1px solid #475569",
              borderRadius: 10, padding: "8px 10px", fontSize: 12,
              color: "#e2e8f0", whiteSpace: "pre-wrap",
              boxShadow: "0 4px 12px rgba(0,0,0,0.4)",
            }}
          >
            {speechText}
            <button
              onClick={(e) => { e.stopPropagation(); setShowBubble(false); }}
              style={{
                position: "absolute", top: 2, right: 4,
                background: "none", border: "none", color: "#94a3b8",
                cursor: "pointer", fontSize: 12, padding: 0,
              }}
            >&times;</button>
            {/* Tail */}
            <div style={{
              position: "absolute", bottom: -6, left: 80,
              width: 0, height: 0,
              borderLeft: "6px solid transparent",
              borderRight: "6px solid transparent",
              borderTop: "6px solid #475569",
            }} />
          </div>
        )}

        {/* Agent body */}
        <div
          onMouseDown={handleMouseDown}
          onClick={handleClick}
          onDoubleClick={handleDblClick}
          onContextMenu={(e) => { e.preventDefault(); onHide?.(); }}
          style={{ animation: floatAnim }}
        >
          <AgentBody expression={expression} size={size} state={state} />
        </div>

        {/* State label */}
        {speechText && !hidden && (
          <div style={{
            position: "absolute", top: -16, left: "50%", transform: "translateX(-50%)",
            fontSize: 9, opacity: 0.6, whiteSpace: "nowrap", color: "#60a5fa",
          }}>
            {stateLabel(state)}
          </div>
        )}
      </div>
    </>
  );
}

function stateLabel(s: AgentState): string {
  switch (s) {
    case "listen": return "Listening...";
    case "think": return "Thinking...";
    case "speak": return "Speaking";
    case "code": return "Coding...";
    case "search": return "Searching...";
    case "success": return "Success!";
    case "warning": return "Approval needed";
    case "error": return "Error";
    case "sleep": return "Sleeping";
    case "wake": return "Ready";
    default: return "";
  }
}

function AgentBody({ expression, size, state }: { expression: string; size: number; state: AgentState }) {
  const eyes = useMemo(() => {
    switch (expression) {
      case "happy": return { l: ">", r: "<" };
      case "thinking": return { l: "\u2312", r: "\u2312" };
      case "alert": return { l: "O", r: "O" };
      case "sleeping": return { l: "\u2014", r: "\u2014" };
      default: return { l: "\u25C9", r: "\u25C9" };
    }
  }, [expression]);

  const pulseAnim = state === "think" ? "pulse 1s ease-in-out infinite" : "none";
  const scale = size / 52;

  return (
    <>
      <style>{`@keyframes pulse { 0%,100%{opacity:1} 50%{opacity:0.5} }`}</style>
      <svg width={size} height={size} viewBox="0 0 52 52">
        {/* Energy ring */}
        <circle cx={26} cy={26} r={24} fill="none" stroke="#60a5fa" strokeWidth={1.5} opacity={0.3}
          style={{ animation: pulseAnim }} />
        <circle cx={26} cy={26} r={22} fill="none" stroke="#fbbf24" strokeWidth={0.8} opacity={0.2} />
        {/* Body */}
        <circle cx={26} cy={26} r={19} fill="#1e293b" stroke="#475569" strokeWidth={1.5} />
        {/* LBTC symbol */}
        <text x={26} y={22} textAnchor="middle" fontSize={7} fill="#fbbf24" fontFamily="monospace" fontWeight="bold">LB</text>
        {/* Eyes */}
        <text x={19} y={32} textAnchor="middle" fontSize={7} fill="#60a5fa" fontFamily="monospace">{eyes.l}</text>
        <text x={33} y={32} textAnchor="middle" fontSize={7} fill="#60a5fa" fontFamily="monospace">{eyes.r}</text>
        {/* Mouth */}
        <text x={26} y={40} textAnchor="middle" fontSize={5} fill="#60a5fa" fontFamily="monospace">
          {expression === "happy" ? "\u25BD" : expression === "alert" ? "\u25A1" : "\u2014"}
        </text>
        {/* Mechanical arms */}
        <line x1={7} y1={28} x2={14} y2={24} stroke="#60a5fa" strokeWidth={1} opacity={0.4} />
        <line x1={45} y1={28} x2={38} y2={24} stroke="#60a5fa" strokeWidth={1} opacity={0.4} />
      </svg>
    </>
  );
}
