"use client";

import { useEffect, useRef } from "react";
import type { Pointer, Role } from "@/lib/useRoomSocket";

/**
 * Shows the other person's mouse pointer over the room.
 *
 * "This line here" is most of what gets said in a technical interview, and
 * without a shared pointer it has to be typed out instead.
 *
 * Off by default and toggleable, because a cursor drifting across the screen
 * while somebody is trying to think is a genuine distraction. It earns its
 * place when two people are looking at the same code, not the rest of the time.
 */

/**
 * How often the local position is published, in milliseconds.
 *
 * Mouse moves fire far faster than anyone can see. At 60 events a second this
 * would be more traffic than the document itself, and every frame passes
 * through the room's single hub goroutine — so throttling is about not
 * flooding the room, not about saving bytes.
 */
const SEND_INTERVAL_MS = 60;

/*
 * A pointer that stops moving fades out after five seconds — see
 * .pointer-ghost in globals.css. Done in CSS rather than with a timer and a
 * piece of state: the animation restarts because the element's key changes
 * with the coordinates, so a moving cursor stays lit and a parked one fades,
 * with no re-render and no effect involved.
 */

const COLORS: Record<Role, string> = {
  interviewer: "#005DED",
  candidate: "#0F766E",
};

const LABELS: Record<Role, string> = {
  interviewer: "Interviewer",
  candidate: "Candidate",
};

/** Publishes the local pointer while enabled. */
export function usePointerBroadcast(
  enabled: boolean,
  send: (x: number, y: number) => void,
) {
  const sendRef = useRef(send);
  useEffect(() => {
    sendRef.current = send;
  });

  useEffect(() => {
    if (!enabled) return;

    let last = 0;
    const onMove = (e: PointerEvent) => {
      const now = performance.now();
      if (now - last < SEND_INTERVAL_MS) return;
      last = now;
      // Fractions of the viewport, so the position means the same thing on a
      // laptop and an external monitor.
      sendRef.current(
        e.clientX / window.innerWidth,
        e.clientY / window.innerHeight,
      );
    };

    window.addEventListener("pointermove", onMove);
    return () => window.removeEventListener("pointermove", onMove);
  }, [enabled]);
}

/**
 * Draws the remote pointer.
 *
 * pointer-events-none throughout: this floats over the editor, and a layer
 * that swallowed clicks would make the room unusable — which is exactly the
 * kind of bug that only shows up once somebody tries to type.
 */
export default function PointerLayer({
  pointer,
  enabled,
}: {
  pointer: Pointer | null;
  enabled: boolean;
}) {
  if (!enabled || !pointer) return null;

  const color = COLORS[pointer.role];

  return (
    <div
      // Changing the key remounts the node, which restarts the fade. A cursor
      // that is moving therefore stays visible; one that stops fades away.
      key={`${pointer.x},${pointer.y}`}
      aria-hidden="true"
      className="pointer-ghost pointer-events-none fixed z-40"
      style={{
        left: `${pointer.x * 100}%`,
        top: `${pointer.y * 100}%`,
        // The hotspot is the tip of the arrow, so the label hangs below-right
        // the way a real cursor's tooltip does.
        transform: "translate(-2px, -2px)",
      }}
    >
      <svg viewBox="0 0 16 16" className="h-4 w-4 drop-shadow-sm">
        <path
          d="M3 2l9 5.5-4 1-1.8 4z"
          fill={color}
          stroke="#fff"
          strokeWidth="1.2"
          strokeLinejoin="round"
        />
      </svg>
      <span
        className="ml-3 -mt-1 inline-block rounded px-1.5 py-0.5 text-[10px] font-medium text-white"
        style={{ background: color }}
      >
        {LABELS[pointer.role]}
      </span>
    </div>
  );
}
