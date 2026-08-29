/**
 * Decorative editor for the hero. Code is rendered as coloured bars rather
 * than fake source: it reads as code at a glance without inviting anyone to
 * squint at nonsense.
 */

type Bar = { w: string; c: string };

const LINES: { indent: number; bars: Bar[] }[] = [
  { indent: 0, bars: [{ w: "14%", c: "bg-[var(--code-key)]" }, { w: "26%", c: "bg-[var(--code-fn)]" }, { w: "12%", c: "bg-white/25" }] },
  { indent: 1, bars: [{ w: "18%", c: "bg-white/30" }, { w: "10%", c: "bg-[var(--code-num)]" }] },
  { indent: 1, bars: [{ w: "12%", c: "bg-[var(--code-key)]" }, { w: "22%", c: "bg-white/25" }, { w: "16%", c: "bg-[var(--code-str)]" }] },
  { indent: 2, bars: [{ w: "20%", c: "bg-white/25" }, { w: "14%", c: "bg-[var(--code-num)]" }] },
  { indent: 2, bars: [{ w: "26%", c: "bg-[var(--code-fn)]" }, { w: "10%", c: "bg-white/20" }] },
  { indent: 1, bars: [{ w: "8%", c: "bg-white/20" }] },
  { indent: 1, bars: [{ w: "16%", c: "bg-[var(--code-key)]" }, { w: "30%", c: "bg-white/25" }] },
  { indent: 0, bars: [{ w: "6%", c: "bg-white/20" }] },
  { indent: 0, bars: [{ w: "22%", c: "bg-[var(--code-fn)]" }, { w: "18%", c: "bg-[var(--code-str)]" }] },
];

function Cursor({
  label,
  color,
  className,
}: {
  label: string;
  color: string;
  className: string;
}) {
  return (
    <div className={`absolute z-20 flex items-start ${className}`}>
      <span className="h-4 w-[2px] rounded-full" style={{ background: color }} />
      <span
        className="ml-1 rounded-md px-1.5 py-0.5 text-[9px] font-semibold text-white shadow-lg"
        style={{ background: color }}
      >
        {label}
      </span>
    </div>
  );
}

export default function EditorMockup() {
  return (
    <div className="relative">
      <div
        className="glass-bright relative overflow-hidden rounded-2xl"
        style={{ animation: "float-slow 8s ease-in-out infinite" }}
      >
        {/* window chrome */}
        <div className="flex items-center gap-3 border-b border-white/[0.07] bg-white/[0.03] px-4 py-2.5">
          <div className="flex gap-1.5">
            <span className="h-2.5 w-2.5 rounded-full bg-[#FF6058]" />
            <span className="h-2.5 w-2.5 rounded-full bg-[#FFBD2E]" />
            <span className="h-2.5 w-2.5 rounded-full bg-[#28CA42]" />
          </div>
          <span className="inset rounded-md px-2 py-0.5 font-mono text-[10px] text-ink-dim">
            main.py
          </span>
        </div>

        {/* Code body. Frosted rather than near-black: in the hero this panel
            sits over the violet bloom and should read as a pane of glass
            catching the light, not as a dark editor cut out of the page. */}
        <div className="relative bg-gradient-to-b from-white/[0.15] to-white/[0.05] px-4 py-5">
          {LINES.map((line, i) => (
            <div key={i} className="flex items-center gap-3 py-[5px]">
              <span className="w-4 shrink-0 text-right font-mono text-[9px] text-ink-faint">
                {i + 1}
              </span>
              <div
                className="flex flex-1 items-center gap-1.5"
                style={{ paddingLeft: `${line.indent * 14}px` }}
              >
                {line.bars.map((bar, j) => (
                  <span
                    key={j}
                    className={`h-[7px] rounded-full ${bar.c}`}
                    style={{ width: bar.w }}
                  />
                ))}
              </div>
            </div>
          ))}

          <Cursor label="Interviewer" color="var(--accent-a)" className="left-[38%] top-[52px]" />
          <Cursor label="Candidate" color="var(--accent-b)" className="left-[26%] top-[128px]" />
        </div>
      </div>

      {/* floating presence chip */}
      <div className="glass-bright absolute -right-3 -top-4 z-30 flex items-center gap-2 rounded-full px-3 py-1.5 shadow-[0_10px_30px_rgba(0,0,0,0.45)]">
        <span className="relative flex h-2 w-2">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-[#34D399] opacity-75" />
          <span className="relative inline-flex h-2 w-2 rounded-full bg-[#34D399]" />
        </span>
        <span className="font-mono text-[10px] text-ink-dim">2 in room</span>
      </div>
    </div>
  );
}
