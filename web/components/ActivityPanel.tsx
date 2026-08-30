"use client";

import { useState } from "react";
import { formatAway } from "@/lib/useActivityReporter";
import type { ActivityEvent, ActivitySummary } from "@/lib/useRoomSocket";

/**
 * The interviewer's view of what the candidate did.
 *
 * A timeline rather than a counter, because "pasted twice" does not tell you
 * whether it was a test case or a finished solution — the content is the
 * whole point, and a count is only useful as a way in to it.
 *
 * Worded as observation, never accusation. Switching tabs is not cheating;
 * plenty of interviewers expect a candidate to read documentation. This
 * reports what happened and leaves the judgement to the person running the
 * interview, which is also the only honest framing given the detection can be
 * defeated by anyone with a phone.
 */

function clockTime(at: number): string {
  return new Date(at).toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function PasteEvent({ event }: { event: ActivityEvent }) {
  const [open, setOpen] = useState(false);
  const text = event.text ?? "";
  const lines = event.lines ?? 0;

  return (
    <div className="min-w-0">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="flex w-full items-center gap-1.5 text-left text-[12px] text-ink hover:text-accent"
        aria-expanded={open}
      >
        <svg
          viewBox="0 0 16 16"
          className={`h-3 w-3 shrink-0 transition-transform ${open ? "rotate-90" : ""}`}
          aria-hidden="true"
        >
          <path
            d="M6 4l4 4-4 4"
            fill="none"
            stroke="currentColor"
            strokeWidth="1.6"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </svg>
        <span className="font-medium">
          Pasted {lines > 0 ? `${lines} line${lines === 1 ? "" : "s"}` : "code"}
        </span>
      </button>

      {open && (
        <div className="mt-1.5 overflow-hidden rounded-md border border-line bg-white">
          <pre className="max-h-48 overflow-auto whitespace-pre-wrap p-2 font-mono text-[11px] leading-relaxed text-ink">
            {text || "(empty)"}
          </pre>
          {event.truncated && (
            // Say so explicitly, or a fragment gets read as the whole paste.
            <p className="border-t border-line px-2 py-1 text-[10px] text-ink-muted">
              Truncated — only the first part was captured.
            </p>
          )}
        </div>
      )}
    </div>
  );
}

export default function ActivityPanel({
  summary,
  events,
}: {
  summary: ActivitySummary | null;
  events: ActivityEvent[];
}) {
  // Newest first: the thing that just happened is what an interviewer is
  // looking for, and scrolling to the bottom mid-interview is friction.
  const ordered = [...events].reverse();

  return (
    <div className="flex h-full min-h-0 flex-col">
      {summary && (
        <div className="flex shrink-0 flex-wrap items-center gap-x-4 gap-y-1 border-b border-line pb-2 text-[12px]">
          <span className="flex items-center gap-1.5">
            <span
              className={`h-1.5 w-1.5 rounded-full ${
                summary.away ? "bg-[#D97706]" : "bg-[#16A34A]"
              }`}
            />
            <span className={summary.away ? "font-medium text-[#9A5B1E]" : "text-ink-muted"}>
              {summary.away ? "Away from the tab now" : "In the tab"}
            </span>
          </span>
          <span className="text-ink-muted">
            Left {summary.away_count}×
            {summary.away_ms > 0 && ` · ${formatAway(summary.away_ms)} total`}
          </span>
          <span className="text-ink-muted">Pasted {summary.paste_count}×</span>
        </div>
      )}

      <div className="min-h-0 flex-1 overflow-auto pt-2">
        {ordered.length === 0 ? (
          <p className="py-6 text-center text-[12px] text-ink-muted">
            Nothing yet. Tab switches and pastes appear here.
          </p>
        ) : (
          <ul className="space-y-2.5">
            {ordered.map((event, i) => (
              <li key={`${event.at}-${i}`} className="flex gap-2.5">
                <span className="shrink-0 pt-px font-mono text-[11px] text-ink-muted">
                  {clockTime(event.at)}
                </span>
                <div className="min-w-0 flex-1">
                  {event.kind === "paste" ? (
                    <PasteEvent event={event} />
                  ) : event.kind === "away" ? (
                    <span className="text-[12px] text-ink">Left the tab</span>
                  ) : (
                    <span className="text-[12px] text-ink-muted">
                      Came back
                      {event.ms ? ` after ${formatAway(event.ms)}` : ""}
                    </span>
                  )}
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>

      <p className="shrink-0 border-t border-line pt-2 text-[11px] leading-relaxed text-ink-muted">
        A signal, not proof. A browser cannot prevent tab switching, and this
        does not see other devices — plenty of candidates read documentation.
      </p>
    </div>
  );
}
