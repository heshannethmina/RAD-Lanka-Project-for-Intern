"use client";

import { useEffect, useState } from "react";

/**
 * How long the interview has left.
 *
 * Driven from an absolute deadline rather than a countdown the server ticks
 * down: the two clients need not agree with the server about the current time,
 * only about when the interview ends, and a dropped connection cannot leave
 * the timer stuck at whatever it last heard.
 *
 * The server is the authority regardless. This is the visible half — the hub
 * stops accepting edits on its own schedule whatever this shows.
 */

/** Below these thresholds the display gets louder. */
const WARN_MS = 5 * 60 * 1000;
const URGENT_MS = 60 * 1000;

function format(ms: number): string {
  const total = Math.max(0, Math.floor(ms / 1000));
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return `${mins}:${String(secs).padStart(2, "0")}`;
}

export default function Countdown({ endsAt }: { endsAt: number | null }) {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (endsAt === null) return;
    // A second is enough: nothing here needs to be smoother than the digits
    // that are changing.
    const id = setInterval(() => setNow(Date.now()), 1000);
    return () => clearInterval(id);
  }, [endsAt]);

  // An unmetered plan has no deadline and so nothing to show. Rendering
  // "unlimited" would be noise in a bar that is already busy.
  if (endsAt === null) return null;

  const left = endsAt - now;
  const urgent = left <= URGENT_MS;
  const warn = left <= WARN_MS;

  return (
    <span
      title="Time remaining in this interview"
      aria-live={urgent ? "assertive" : "off"}
      className={[
        "inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1 font-mono text-[13px] tabular-nums transition-colors",
        left <= 0
          ? "border-[#F1AEAE] bg-[#FDF3F3] text-[#B42318]"
          : urgent
            ? "border-[#F1AEAE] bg-[#FDF3F3] text-[#B42318]"
            : warn
              ? "border-[#F5C4A0] bg-[#FFF7ED] text-[#9A5B1E]"
              : "border-line bg-white text-ink-muted",
      ].join(" ")}
    >
      <svg viewBox="0 0 16 16" className="h-3.5 w-3.5" aria-hidden="true">
        <circle cx="8" cy="8" r="6.25" fill="none" stroke="currentColor" strokeWidth="1.4" />
        <path d="M8 4.5V8l2.5 1.5" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
      </svg>
      {left <= 0 ? "Time up" : format(left)}
    </span>
  );
}
