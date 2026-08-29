"use client";

import { useEffect, useState } from "react";
import SyncLoader from "./SyncLoader";

/**
 * The first-paint cover.
 *
 * It renders on the server as well, so the loader is already on screen and
 * turning in the very first frame rather than appearing once React hydrates.
 * It clears on `window.load`, with a floor of MIN_MS so a fast load reads as
 * a deliberate beat instead of a flicker.
 *
 * A `<noscript>` rule in the layout hides this outright, so the page is never
 * stuck behind a cover that has nothing to dismiss it.
 */

/** Shortest time the cover stays up, so it never flashes. */
const MIN_MS = 450;

export default function Splash() {
  const [done, setDone] = useState(false);

  useEffect(() => {
    const start = performance.now();

    const finish = () => {
      const remaining = Math.max(0, MIN_MS - (performance.now() - start));
      const timer = window.setTimeout(() => setDone(true), remaining);
      return () => window.clearTimeout(timer);
    };

    if (document.readyState === "complete") return finish();

    let cancelTimer: (() => void) | undefined;
    const onLoad = () => {
      cancelTimer = finish();
    };
    window.addEventListener("load", onLoad, { once: true });
    return () => {
      window.removeEventListener("load", onLoad);
      cancelTimer?.();
    };
  }, []);

  return (
    <div className="splash" data-done={done} aria-hidden={done}>
      <SyncLoader size={56} label="Loading SyncR" />
    </div>
  );
}
