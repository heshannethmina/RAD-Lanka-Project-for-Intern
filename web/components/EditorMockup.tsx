const lines = [
  { indent: 0, tokens: [{ t: "func ", c: "text-[#818CF8]" }, { t: "twoSum", c: "text-[#5EEAD4]" }, { t: "(nums []int, target int) []int {", c: "text-ink-dim" }] },
  { indent: 1, tokens: [{ t: "seen ", c: "text-ink" }, { t: ":= ", c: "text-[#818CF8]" }, { t: "make(map[int]int)", c: "text-[#F5A97F]" }] },
  { indent: 1, tokens: [{ t: "for ", c: "text-[#818CF8]" }, { t: "i, n ", c: "text-ink" }, { t: ":= ", c: "text-[#818CF8]" }, { t: "range nums {", c: "text-ink-dim" }] },
  { indent: 2, tokens: [{ t: "if ", c: "text-[#818CF8]" }, { t: "j, ok ", c: "text-ink" }, { t: ":= ", c: "text-[#818CF8]" }, { t: "seen[target-n]; ok {", c: "text-ink-dim" }] },
  { indent: 3, tokens: [{ t: "return ", c: "text-[#818CF8]" }, { t: "[]int{j, i}", c: "text-[#5EEAD4]" }] },
  { indent: 2, tokens: [{ t: "}", c: "text-ink-dim" }] },
  { indent: 2, tokens: [{ t: "seen[n] ", c: "text-ink" }, { t: "= i", c: "text-ink-dim" }] },
  { indent: 1, tokens: [{ t: "}", c: "text-ink-dim" }] },
  { indent: 0, tokens: [{ t: "}", c: "text-ink-dim" }] },
];

export default function EditorMockup() {
  return (
    <div className="relative">
      {/* floating presence chip */}
      <div
        className="glass-bright absolute -right-4 -top-5 z-20 flex items-center gap-2 rounded-full px-3 py-1.5 shadow-[0_8px_30px_rgba(0,0,0,0.35)]"
        style={{ animation: "float-slow 7s ease-in-out infinite" }}
      >
        <span className="relative flex h-2 w-2">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[#5EEAD4] opacity-75" />
          <span className="relative inline-flex h-2 w-2 rounded-full bg-[#5EEAD4]" />
        </span>
        <span className="font-mono text-[11px] text-ink-dim">Nadeesha joined</span>
      </div>

      <div className="glass-bright overflow-hidden rounded-2xl shadow-[0_30px_80px_-20px_rgba(0,0,0,0.6)]">
        {/* window chrome */}
        <div className="flex items-center gap-2 border-b border-white/[0.06] bg-white/[0.02] px-4 py-3">
          <span className="h-2.5 w-2.5 rounded-full bg-[#FF6B6B]/70" />
          <span className="h-2.5 w-2.5 rounded-full bg-[#FFD166]/70" />
          <span className="h-2.5 w-2.5 rounded-full bg-[#5EEAD4]/70" />
          <span className="ml-3 font-mono text-[11px] text-ink-faint">
            interview — two-sum.go
          </span>
        </div>

        {/* code */}
        <div className="px-5 py-5 font-mono text-[12.5px] leading-[1.9] sm:text-[13.5px]">
          {lines.map((line, i) => (
            <div
              key={i}
              className="flex items-baseline gap-3 opacity-0"
              style={{
                animation: `fade-in 0.35s ease-out forwards`,
                animationDelay: `${0.15 + i * 0.12}s`,
              }}
            >
              <span className="w-5 shrink-0 text-right text-ink-faint select-none">
                {i + 1}
              </span>
              <span style={{ paddingLeft: `${line.indent * 1.1}em` }}>
                {line.tokens.map((tok, j) => (
                  <span key={j} className={tok.c}>
                    {tok.t}
                  </span>
                ))}
                {i === lines.length - 1 && (
                  <span
                    className="ml-0.5 inline-block h-[1.1em] w-[2px] translate-y-[2px] bg-[#5EEAD4] align-middle"
                    style={{ animation: "blink-cursor 1s step-end infinite" }}
                  />
                )}
              </span>
            </div>
          ))}
        </div>

        {/* run bar */}
        <div className="flex items-center justify-between border-t border-white/[0.06] bg-white/[0.02] px-5 py-3">
          <div className="flex items-center gap-2">
            <span className="h-1.5 w-1.5 rounded-full bg-[#5EEAD4]" />
            <span className="font-mono text-[11px] text-ink-dim">
              Go 1.23 · sandboxed
            </span>
          </div>
          <span className="rounded-md bg-gradient-to-br from-[var(--accent-a)] to-[var(--accent-b)] px-3 py-1 font-mono text-[11px] font-semibold text-[#0B0E14]">
            Run ▸
          </span>
        </div>
      </div>

      <style>{`
        @keyframes fade-in {
          from { opacity: 0; transform: translateY(4px); }
          to { opacity: 1; transform: translateY(0); }
        }
      `}</style>
    </div>
  );
}
