"use client";

import { useEffect, useRef } from "react";
import type { ActivityKind } from "./useRoomSocket";

/**
 * Reports when the candidate leaves the tab and when they come back.
 *
 * What this is *not*: a way to stop someone opening another tab. No browser
 * API can do that — a page is not allowed to trap a user, deliberately — and
 * anyone determined can use a phone, a second monitor, or another machine. So
 * every event here is a signal that somebody stepped away, never proof of
 * anything, and the UI must not present it as more.
 *
 * Only the candidate's client calls this. An interviewer switching tabs is
 * their own business, and the server drops activity frames from them anyway.
 *
 * Two browser events cover the cases, and they overlap:
 *
 *   visibilitychange  the tab was backgrounded, or the screen locked
 *   blur              the window lost focus while still visible — which is
 *                     what a second monitor looks like
 *
 * A single switch often fires both, so "away" is only reported on the leading
 * edge. The server also guards against a duplicate; this keeps the traffic
 * down rather than relying on that.
 */

/**
 * Ignore absences shorter than this.
 *
 * A notification stealing focus for a moment, a click on the browser chrome,
 * or an OS alert are not somebody leaving the interview. Without this floor
 * the interviewer's panel fills with noise and stops being worth reading —
 * which is worse than not having it at all.
 */
const MIN_AWAY_MS = 1500;

export function useActivityReporter(
  enabled: boolean,
  report: (
    kind: ActivityKind,
    extra?: { ms?: number; lines?: number; text?: string },
  ) => void,
) {
  // Held in a ref so the effect does not re-subscribe when the callback
  // identity changes on a re-render.
  const reportRef = useRef(report);
  useEffect(() => {
    reportRef.current = report;
  });

  useEffect(() => {
    if (!enabled) return;

    // null when present, otherwise the timestamp they left.
    let awaySince: number | null = null;
    let pending: ReturnType<typeof setTimeout> | null = null;

    const leave = () => {
      if (awaySince !== null) return; // Already away; the other event fired.
      const at = Date.now();
      // Wait out the floor before reporting, so a momentary blur never
      // reaches the interviewer at all.
      pending = setTimeout(() => {
        awaySince = at;
        pending = null;
        reportRef.current("away");
      }, MIN_AWAY_MS);
    };

    const returned = () => {
      if (pending) {
        // Back before the floor elapsed — this was a blip, not an absence.
        clearTimeout(pending);
        pending = null;
        return;
      }
      if (awaySince === null) return;
      const ms = Date.now() - awaySince;
      awaySince = null;
      reportRef.current("back", { ms });
    };

    const onVisibility = () => {
      if (document.visibilityState === "hidden") leave();
      else returned();
    };

    document.addEventListener("visibilitychange", onVisibility);
    window.addEventListener("blur", leave);
    window.addEventListener("focus", returned);

    return () => {
      if (pending) clearTimeout(pending);
      document.removeEventListener("visibilitychange", onVisibility);
      window.removeEventListener("blur", leave);
      window.removeEventListener("focus", returned);
    };
  }, [enabled]);
}

/** Turns milliseconds into something an interviewer can read at a glance. */
export function formatAway(ms: number): string {
  if (ms < 1000) return "0s";
  const total = Math.round(ms / 1000);
  const mins = Math.floor(total / 60);
  const secs = total % 60;
  return mins > 0 ? `${mins}m ${secs}s` : `${secs}s`;
}
