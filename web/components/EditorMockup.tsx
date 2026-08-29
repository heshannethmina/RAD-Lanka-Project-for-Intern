import type { ReactNode } from "react";

/**
 * A still of the product for the hero: window chrome, one file open, real
 * Python, and a Run button.
 *
 * The code is the same problem the demo room ships with, so the marketing
 * image and the actual product agree with each other.
 */

/* Token helpers — small enough to read inline, which keeps the code sample
   below looking like code rather than like markup. */
const K = ({ children }: { children: ReactNode }) => (
  <span style={{ color: "var(--code-key)" }}>{children}</span>
);
const F = ({ children }: { children: ReactNode }) => (
  <span style={{ color: "var(--code-fn)" }}>{children}</span>
);
const N = ({ children }: { children: ReactNode }) => (
  <span style={{ color: "var(--code-num)" }}>{children}</span>
);

const LINES: ReactNode[] = [
  <>
    <K>def</K> <F>max_element</F>(values):
  </>,
  <>
    {"    "}
    <K>if</K> <K>not</K> values:
  </>,
  <>
    {"        "}
    <K>return</K> <K>None</K>
  </>,
  <>{""}</>,
  <>
    {"    "}largest = values[<N>0</N>]
  </>,
  <>
    {"    "}
    <K>for</K> value <K>in</K> values[<N>1</N>:]:
  </>,
  <>
    {"        "}
    <K>if</K> value &gt; largest:
  </>,
  <>{"            largest = value"}</>,
  <>
    {"    "}
    <K>return</K> largest
  </>,
  <>{""}</>,
  <>
    <F>print</F>(<F>max_element</F>([<N>10</N>, <N>5</N>, <N>22</N>, <N>11</N>
    ]))
  </>,
];

function PlayIcon() {
  return (
    <svg viewBox="0 0 12 12" className="h-3 w-3" aria-hidden="true">
      <path d="M3.5 2.5v7l6-3.5z" fill="currentColor" />
    </svg>
  );
}

export default function EditorMockup() {
  return (
    <div className="panel overflow-hidden rounded-xl">
      {/* Window chrome */}
      <div className="flex items-center gap-3 border-b border-line bg-bg-subtle px-4 py-3">
        <div className="flex gap-1.5" aria-hidden="true">
          <span className="h-2.5 w-2.5 rounded-full bg-[#F0645C]" />
          <span className="h-2.5 w-2.5 rounded-full bg-[#F5BD4F]" />
          <span className="h-2.5 w-2.5 rounded-full bg-[#5FC454]" />
        </div>

        <span className="ml-1 rounded-md border border-line bg-white px-2.5 py-1 font-mono text-[11px] text-ink">
          main.py
        </span>

        <span className="ml-auto inline-flex items-center gap-1.5 rounded-full bg-accent px-3 py-1.5 text-[11px] font-medium text-white">
          <PlayIcon />
          Run
        </span>
      </div>

      {/* Code */}
      <div className="overflow-x-auto bg-white px-4 py-4">
        <pre className="font-mono text-[12.5px] leading-[1.75] text-[var(--code-plain)]">
          {LINES.map((line, i) => (
            <div key={i} className="flex">
              <span className="mr-4 w-5 shrink-0 select-none text-right text-[var(--code-comment)]">
                {i + 1}
              </span>
              <span className="whitespace-pre">{line}</span>
            </div>
          ))}
        </pre>
      </div>

      {/* Result of the run above — the point of the product, in one line. */}
      <div className="flex items-center gap-2 border-t border-line bg-bg-subtle px-4 py-2.5 font-mono text-[11.5px]">
        <span className="text-ink-muted">output</span>
        <span className="text-ink">22</span>
        <span className="ml-auto text-ink-muted">0.04s</span>
      </div>
    </div>
  );
}
