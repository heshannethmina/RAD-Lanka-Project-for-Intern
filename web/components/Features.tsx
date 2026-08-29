import type { ReactNode } from "react";

/* --- illustrations ------------------------------------------------------ */

function SyncArt() {
  return (
    <svg viewBox="0 0 200 110" className="h-full w-full" aria-hidden="true">
      <rect x="26" y="18" width="120" height="74" rx="8" fill="rgba(255,255,255,0.07)" stroke="rgba(255,255,255,0.13)" />
      <rect x="38" y="32" width="70" height="6" rx="3" fill="var(--accent-a)" opacity="0.75" />
      <rect x="38" y="46" width="94" height="6" rx="3" fill="rgba(255,255,255,0.26)" />
      <rect x="38" y="60" width="54" height="6" rx="3" fill="rgba(255,255,255,0.26)" />
      <rect x="38" y="74" width="80" height="6" rx="3" fill="var(--accent-b)" opacity="0.6" />
      <path d="M104 58 L104 82 L111 75 L116 86 L121 83 L116 73 L125 73 Z" fill="#fff" stroke="rgba(0,0,0,0.35)" strokeWidth="1.5" />
      <path d="M150 40 L150 64 L157 57 L162 68 L167 65 L162 55 L171 55 Z" fill="var(--accent-a)" stroke="rgba(0,0,0,0.3)" strokeWidth="1.5" />
    </svg>
  );
}

function ExecArt() {
  return (
    <svg viewBox="0 0 200 110" className="h-full w-full" aria-hidden="true">
      <rect x="44" y="16" width="112" height="78" rx="10" fill="rgba(6,9,24,0.75)" stroke="rgba(255,255,255,0.14)" />
      <circle cx="58" cy="30" r="3" fill="rgba(255,255,255,0.35)" />
      <circle cx="68" cy="30" r="3" fill="rgba(255,255,255,0.25)" />
      <circle cx="78" cy="30" r="3" fill="rgba(255,255,255,0.18)" />
      <path d="M88 48 L76 62 L88 76" fill="none" stroke="var(--accent-a)" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M112 48 L124 62 L112 76" fill="none" stroke="var(--accent-b)" strokeWidth="4" strokeLinecap="round" strokeLinejoin="round" />
      <path d="M104 44 L96 80" stroke="rgba(255,255,255,0.5)" strokeWidth="4" strokeLinecap="round" />
    </svg>
  );
}

function JoinArt() {
  return (
    <svg viewBox="0 0 200 110" className="h-full w-full" aria-hidden="true">
      <circle cx="66" cy="42" r="15" fill="rgba(255,255,255,0.22)" />
      <path d="M42 88 a24 24 0 0 1 48 0 z" fill="rgba(255,255,255,0.22)" />
      <rect x="104" y="46" width="34" height="16" rx="8" fill="none" stroke="var(--accent-a)" strokeWidth="3.5" />
      <rect x="126" y="46" width="34" height="16" rx="8" fill="none" stroke="var(--accent-b)" strokeWidth="3.5" />
      <path d="M120 54 h24" stroke="rgba(255,255,255,0.55)" strokeWidth="3.5" strokeLinecap="round" />
      <path d="M134 68 L134 92 L141 85 L146 96 L151 93 L146 83 L155 83 Z" fill="#fff" stroke="rgba(0,0,0,0.35)" strokeWidth="1.5" />
    </svg>
  );
}

function TeamArt() {
  return (
    <svg viewBox="0 0 200 110" className="h-full w-full" aria-hidden="true">
      <circle cx="72" cy="40" r="19" fill="none" stroke="var(--accent-a)" strokeWidth="3.5" />
      <path d="M72 30 v20 M67 35 h10 M67 45 h10" stroke="var(--accent-a)" strokeWidth="3" strokeLinecap="round" />
      <rect x="108" y="26" width="26" height="18" rx="4" fill="rgba(255,255,255,0.22)" />
      <rect x="94" y="66" width="22" height="16" rx="4" fill="rgba(255,255,255,0.16)" />
      <rect x="126" y="66" width="22" height="16" rx="4" fill="rgba(255,255,255,0.16)" />
      <path d="M121 44 v10 M105 54 h32 M105 54 v12 M137 54 v12" stroke="rgba(255,255,255,0.3)" strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}

/* --- content ------------------------------------------------------------ */

const features: {
  title: string;
  tag: string;
  desc: string;
  art: ReactNode;
}[] = [
  {
    title: "Real-time sync",
    tag: "Collab",
    desc: "No lag between collaborators",
    art: <SyncArt />,
  },
  {
    title: "Real code execution",
    tag: "Code",
    desc: "Sandboxed, not simulated",
    art: <ExecArt />,
  },
  {
    title: "No setup for candidates",
    tag: "Easy",
    desc: "Join via link, no install or account",
    art: <JoinArt />,
  },
  {
    title: "Pricing built for small teams",
    tag: "Teams",
    desc: "Transparent plans, no enterprise complexity",
    art: <TeamArt />,
  },
];

export default function Features() {
  return (
    <section id="product" className="px-6 py-20">
      <div className="mx-auto max-w-6xl">
        <span className="chip inline-flex items-center rounded-full px-3 py-1 text-[11px] tracking-wide">
          Features
        </span>
        <h2 className="mt-4 font-display text-3xl font-semibold tracking-tight sm:text-[2.5rem]">
          Platform Capabilities
        </h2>

        <div className="mt-10 grid gap-5 sm:grid-cols-2">
          {features.map((f) => (
            <div
              key={f.title}
              className="glass glass-hover lift flex flex-col rounded-2xl p-6"
            >
              <div className="flex items-start justify-between gap-4">
                <h3 className="max-w-[70%] font-display text-lg font-semibold leading-snug tracking-tight text-ink">
                  {f.title}
                </h3>
                <span className="chip shrink-0 rounded-full px-2.5 py-1 font-mono text-[10px]">
                  {f.tag}
                </span>
              </div>

              <p className="mt-2 text-[13px] leading-relaxed text-ink-dim">
                {f.desc}
              </p>

              <div className="mt-5 h-[120px] w-full overflow-hidden rounded-xl border border-white/[0.06] bg-gradient-to-b from-white/[0.06] to-white/[0.015]">
                {f.art}
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
