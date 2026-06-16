import React, { useMemo } from "react";

interface MascotProps {
  expression?: "idle" | "happy" | "thinking" | "alert" | "sleeping";
  size?: number;
  bounce?: boolean;
}

export function LegacyMascot({ expression = "idle", size = 48, bounce = true }: MascotProps) {
  const eyes = useMemo(() => {
    switch (expression) {
      case "happy": return { l: ">" as const, r: "<" as const, offset: 0 };
      case "thinking": return { l: "⌒" as const, r: "⌒" as const, offset: -2 };
      case "alert": return { l: "O" as const, r: "O" as const, offset: 0 };
      case "sleeping": return { l: "—" as const, r: "—" as const, offset: 3 };
      default: return { l: "◉" as const, r: "◉" as const, offset: 0 };
    }
  }, [expression]);

  const eyebrow = expression === "alert" ? "⌃" : expression === "thinking" ? "~" : "";

  const scale = size / 48;

  return (
    <div
      style={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        position: "relative",
        width: size,
        height: size,
        animation: bounce ? "mascotBounce 2s ease-in-out infinite" : "none",
      }}
    >
      <style>{`
        @keyframes mascotBounce {
          0%, 100% { transform: translateY(0); }
          30% { transform: translateY(-6px); }
          50% { transform: translateY(0); }
          70% { transform: translateY(-3px); }
        }
        @keyframes mascotThinking {
          0%, 100% { transform: rotate(0deg); }
          25% { transform: rotate(3deg); }
          75% { transform: rotate(-3deg); }
        }
        @keyframes mascotPulse {
          0%, 100% { opacity: 1; }
          50% { opacity: 0.6; }
        }
      `}</style>
      <svg
        width={size}
        height={size}
        viewBox="0 0 48 48"
        style={{ display: "block" }}
      >
        {/* Body - rounded rectangle */}
        <rect
          x="8" y="8" width="32" height="32" rx="8"
          fill="#1e293b"
          stroke="#475569"
          strokeWidth="2"
        />

        {/* Inner circuit pattern */}
        <circle cx="24" cy="8" r="3" fill="#60a5fa" />
        <circle cx="24" cy="40" r="3" fill="#60a5fa" />
        <circle cx="8" cy="24" r="2" fill="#60a5fa" />
        <circle cx="40" cy="24" r="2" fill="#60a5fa" />

        {/* Face */}
        {expression === "sleeping" && (
          <text x="24" y="31" textAnchor="middle" fontSize="5" fill="#60a5fa" fontFamily="monospace">Z</text>
        )}

        {/* Eyes */}
        {eyebrow && (
          <text x="17" y="17" textAnchor="middle" fontSize="5" fill="#60a5fa" fontFamily="monospace">{eyebrow}</text>
        )}
        {eyebrow && (
          <text x="31" y="17" textAnchor="middle" fontSize="5" fill="#60a5fa" fontFamily="monospace">{eyebrow}</text>
        )}
        <text x="17" y={24 + eyes.offset} textAnchor="middle" fontSize="6" fill="#60a5fa" fontFamily="monospace">{eyes.l}</text>
        <text x="31" y={24 + eyes.offset} textAnchor="middle" fontSize="6" fill="#60a5fa" fontFamily="monospace">{eyes.r}</text>

        {/* Mouth */}
        <text
          x="24" y="35"
          textAnchor="middle"
          fontSize={expression === "happy" ? 8 : 5}
          fill="#60a5fa"
          fontFamily="monospace"
        >
          {expression === "happy" ? "▽" : expression === "alert" ? "□" : expression === "thinking" ? "~~" : "—"}
        </text>

        {/* Decimal slash */}
        <line x1="25" y1="16" x2="23" y2="19" stroke="#60a5fa" strokeWidth="0.8" opacity="0.5" />
      </svg>
    </div>
  );
}
