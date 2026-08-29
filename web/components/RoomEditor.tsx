"use client";

import { useState } from "react";
import Editor from "@monaco-editor/react";

const STARTER = `func twoSum(nums []int, target int) []int {
	seen := make(map[int]int)
	for i, n := range nums {
		if j, ok := seen[target-n]; ok {
			return []int{j, i}
		}
		seen[n] = i
	}
	return nil
}
`;

const LANGUAGES = [
  { id: "go", label: "Go" },
  { id: "python", label: "Python" },
  { id: "javascript", label: "JavaScript" },
];

export default function RoomEditor({ roomId }: { roomId: string }) {
  const [language, setLanguage] = useState("go");
  const [output, setOutput] = useState<string | null>(null);
  const [running, setRunning] = useState(false);

  function handleRun() {
    setRunning(true);
    setOutput(null);
    // Placeholder for Judge0-backed execution via the Go backend.
    setTimeout(() => {
      setOutput("[0 1]\n\n✓ ran in 42ms · exit 0");
      setRunning(false);
    }, 700);
  }

  return (
    <div className="flex h-screen flex-col bg-[#0B0E14]">
      {/* top bar */}
      <div className="flex items-center justify-between border-b border-white/[0.07] bg-white/[0.02] px-5 py-3">
        <div className="flex items-center gap-3">
          <span className="flex h-6 w-6 items-center justify-center rounded bg-gradient-to-br from-[var(--accent-a)] to-[var(--accent-b)] font-mono text-[10px] font-bold text-[#0B0E14]">
            P
          </span>
          <span className="font-mono text-[12px] text-ink-dim">
            room / <span className="text-ink">{roomId}</span>
          </span>
        </div>

        <div className="flex items-center gap-3">
          <div className="flex -space-x-2">
            <span className="flex h-7 w-7 items-center justify-center rounded-full border-2 border-[#0B0E14] bg-gradient-to-br from-[var(--accent-a)] to-[var(--accent-b)] font-mono text-[10px] font-bold text-[#0B0E14]">
              H
            </span>
            <span className="relative flex h-7 w-7 items-center justify-center rounded-full border-2 border-[#0B0E14] bg-[#2A3142] font-mono text-[10px] font-semibold text-ink-dim">
              N
              <span className="absolute -bottom-0.5 -right-0.5 h-2 w-2 rounded-full border-2 border-[#0B0E14] bg-[#5EEAD4]" />
            </span>
          </div>
          <select
            value={language}
            onChange={(e) => setLanguage(e.target.value)}
            className="glass rounded-lg px-3 py-1.5 font-mono text-xs text-ink outline-none"
          >
            {LANGUAGES.map((l) => (
              <option key={l.id} value={l.id} className="bg-[#10141D]">
                {l.label}
              </option>
            ))}
          </select>
          <button
            onClick={handleRun}
            disabled={running}
            className="rounded-lg bg-gradient-to-br from-[var(--accent-a)] to-[var(--accent-b)] px-4 py-1.5 font-mono text-xs font-semibold text-[#0B0E14] transition hover:brightness-110 disabled:opacity-60"
          >
            {running ? "Running…" : "Run ▸"}
          </button>
        </div>
      </div>

      {/* editor + output */}
      <div className="flex flex-1 flex-col overflow-hidden lg:flex-row">
        <div className="flex-1 overflow-hidden">
          <Editor
            height="100%"
            language={language}
            defaultValue={STARTER}
            theme="vs-dark"
            options={{
              fontFamily: "var(--font-mono)",
              fontSize: 13.5,
              minimap: { enabled: false },
              padding: { top: 20 },
              scrollBeyondLastLine: false,
            }}
          />
        </div>

        <div className="flex w-full flex-col border-t border-white/[0.07] bg-white/[0.015] lg:w-[340px] lg:border-l lg:border-t-0">
          <div className="border-b border-white/[0.07] px-5 py-3">
            <span className="font-mono text-[11px] tracking-wide text-ink-faint">
              OUTPUT
            </span>
          </div>
          <div className="flex-1 overflow-auto px-5 py-4 font-mono text-[12.5px] leading-relaxed">
            {output ? (
              <pre className="whitespace-pre-wrap text-[#5EEAD4]">{output}</pre>
            ) : (
              <span className="text-ink-faint">
                Run the code to see output here.
              </span>
            )}
          </div>

          <div className="border-t border-white/[0.07] px-5 py-4">
            <span className="font-mono text-[11px] tracking-wide text-ink-faint">
              PROMPT
            </span>
            <p className="mt-2 text-sm leading-relaxed text-ink-dim">
              Given an array of integers <code className="text-ink">nums</code> and
              an integer <code className="text-ink">target</code>, return indices
              of the two numbers that add up to <code className="text-ink">target</code>.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
