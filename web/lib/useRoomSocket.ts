"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export type ConnectionStatus = "connecting" | "open" | "closed";

/** Frames the server sends. Mirrors backend/internal/ws/message.go. */
export type Role = "interviewer" | "candidate";

/** Mirrors ws.ActivitySummary. Aggregates only — no keystroke stream. */
export type ActivitySummary = {
  away_count: number;
  away_ms: number;
  paste_count: number;
  /** True while the candidate is currently out of the tab. */
  away: boolean;
};

export type ActivityKind = "away" | "back" | "paste";

/** Mirrors ws.ActivityEvent. */
export type ActivityEvent = {
  kind: ActivityKind;
  /** Milliseconds since the epoch, stamped by the server, not the browser. */
  at: number;
  lines?: number;
  ms?: number;
  /** The pasted text, on paste events. Truncated server-side. */
  text?: string;
  truncated?: boolean;
};

/** Where somebody's mouse is, as a fraction of their viewport. */
export type Pointer = { x: number; y: number; role: Role };

type ServerMessage =
  | {
      type: "snapshot";
      text?: string;
      prompt?: string;
      role?: string;
      activity?: ActivitySummary;
      events?: ActivityEvent[];
    }
  | { type: "edit"; text?: string }
  | { type: "presence"; clients?: number }
  | { type: "result"; text?: string; failed?: boolean }
  | { type: "prompt"; prompt?: string }
  | {
      type: "activity";
      kind?: ActivityKind;
      lines?: number;
      ms?: number;
      activity?: ActivitySummary;
      event?: ActivityEvent;
    }
  | { type: "pointer"; x?: number; y?: number; role?: string };

const WS_BASE = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8080";

const MAX_BACKOFF_MS = 10_000;

type Handlers = {
  /**
   * Full document handed over on join. `send` is passed in so a first client
   * can seed an empty room without the caller having to reach back into this
   * hook's return value.
   */
  onSnapshot: (text: string, send: (text: string) => void) => void;
  /**
   * The interview question, and what this client is allowed to do with it.
   * Delivered with the snapshot because both are room state a joiner needs
   * before it can render anything correctly.
   */
  onJoin: (
    prompt: string,
    role: Role,
    activity: ActivitySummary | null,
    events: ActivityEvent[],
  ) => void;
  /**
   * The candidate left, returned, or pasted. Carries the running tally, which
   * the server owns — so a reload does not reset it and the client never has
   * to accumulate anything itself.
   */
  onActivity: (summary: ActivitySummary, event: ActivityEvent) => void;
  /** Somebody moved their mouse. Stale immediately; never stored. */
  onPointer: (pointer: Pointer) => void;
  /** The interviewer changed the question. */
  onPrompt: (prompt: string) => void;
  /** Full document after somebody else's edit. */
  onEdit: (text: string) => void;
  /**
   * Output from a run the *other* person started. The hub does not echo a
   * result to its author, who already rendered it locally.
   */
  onResult: (text: string, failed: boolean) => void;
};

/**
 * Opens the room's WebSocket and keeps it open.
 *
 * The connection is deliberately direct to the Go backend — it does not go
 * through Next.js, which only renders the shell.
 *
 * `token` is either the interviewer's session token or the candidate's invite
 * token; the server accepts both and works out which it was given. It goes in
 * the query string because browsers cannot set headers on a WebSocket
 * handshake. Passing null keeps the socket shut, which is what a page should
 * do while it is still working out whether the viewer is allowed in.
 *
 * A rejected handshake is indistinguishable from a network failure in the
 * browser — both surface as close code 1006 — so this hook does not try to
 * tell them apart. The room page checks access over REST first, and by the
 * time it opens a socket a failure really is the network.
 */
export function useRoomSocket(
  roomId: string,
  token: string | null,
  handlers: Handlers,
) {
  const [status, setStatus] = useState<ConnectionStatus>("connecting");
  const [peers, setPeers] = useState(1);
  const socketRef = useRef<WebSocket | null>(null);

  // Hold the callbacks in a ref so that a re-render with fresh closures does
  // not tear the socket down and reconnect.
  const handlersRef = useRef(handlers);
  useEffect(() => {
    handlersRef.current = handlers;
  });

  /** Publishes the whole document. The hub serialises these, so no OT. */
  const sendEdit = useCallback((text: string) => {
    const ws = socketRef.current;
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "edit", text }));
    }
  }, []);

  /**
   * Shares the output of a run with the rest of the room.
   *
   * Dropped silently when the socket is not open: a run whose result cannot
   * be shared still succeeded for the person who pressed Run, and failing
   * loudly here would report a problem they cannot act on.
   */
  /**
   * Publishes the interview question. The server ignores this from a
   * candidate, so the UI hiding the editor is a convenience, not the control.
   */
  const sendPrompt = useCallback((prompt: string) => {
    const ws = socketRef.current;
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "prompt", prompt }));
    }
  }, []);

  /**
   * Reports one observed event. Sent by the candidate's client only; the
   * server ignores these from an interviewer.
   */
  const sendActivity = useCallback(
    (kind: ActivityKind, extra?: { ms?: number; lines?: number }) => {
      const ws = socketRef.current;
      if (ws?.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify({ type: "activity", kind, ...extra }));
      }
    },
    [],
  );

  /**
   * Publishes the local mouse position as a fraction of the viewport.
   *
   * Fractions rather than pixels because the two people have different window
   * sizes, and a pixel position would land somewhere else on the other screen.
   */
  const sendPointer = useCallback((x: number, y: number) => {
    const ws = socketRef.current;
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "pointer", x, y }));
    }
  }, []);

  const sendResult = useCallback((text: string, failed: boolean) => {
    const ws = socketRef.current;
    if (ws?.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ type: "result", text, failed }));
    }
  }, []);

  useEffect(() => {
    // No token yet: the caller is still resolving access. Stay closed rather
    // than opening a socket that would only be refused.
    //
    // Nothing to set here — "connecting" is already the initial state, and
    // connect() sets it again on every attempt. Assigning it in this branch
    // would only be an extra render saying what the state already says.
    if (!token) return;

    let disposed = false;
    let retry: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;

    function connect() {
      if (disposed) return;

      setStatus("connecting");
      const url =
        `${WS_BASE}/ws/${encodeURIComponent(roomId)}` +
        `?token=${encodeURIComponent(token!)}`;
      const ws = new WebSocket(url);
      socketRef.current = ws;

      ws.onopen = () => {
        attempt = 0;
        setStatus("open");
      };

      ws.onmessage = (event) => {
        let msg: ServerMessage;
        try {
          msg = JSON.parse(event.data as string);
        } catch {
          return; // Ignore anything we cannot parse rather than dying.
        }

        switch (msg.type) {
          case "snapshot":
            // Role before document: the caller may render differently for a
            // candidate, and doing it in this order avoids a flash of the
            // interviewer's view.
            handlersRef.current.onJoin(
              msg.prompt ?? "",
              msg.role === "candidate" ? "candidate" : "interviewer",
              msg.activity ?? null,
              msg.events ?? [],
            );
            handlersRef.current.onSnapshot(msg.text ?? "", sendEdit);
            break;
          case "edit":
            handlersRef.current.onEdit(msg.text ?? "");
            break;
          case "presence":
            setPeers(msg.clients ?? 1);
            break;
          case "result":
            handlersRef.current.onResult(msg.text ?? "", msg.failed ?? false);
            break;
          case "prompt":
            handlersRef.current.onPrompt(msg.prompt ?? "");
            break;
          case "activity":
            if (msg.activity && msg.event) {
              handlersRef.current.onActivity(msg.activity, msg.event);
            }
            break;
          case "pointer":
            handlersRef.current.onPointer({
              x: msg.x ?? 0,
              y: msg.y ?? 0,
              role: msg.role === "interviewer" ? "interviewer" : "candidate",
            });
            break;
        }
      };

      ws.onerror = () => ws.close();

      ws.onclose = () => {
        if (disposed) return;
        setStatus("closed");
        // Back off, so a restarting backend does not get hammered.
        attempt += 1;
        const wait = Math.min(500 * 2 ** (attempt - 1), MAX_BACKOFF_MS);
        retry = setTimeout(connect, wait);
      };
    }

    connect();

    return () => {
      disposed = true;
      if (retry) clearTimeout(retry);
      socketRef.current?.close();
      socketRef.current = null;
    };
  }, [roomId, token, sendEdit]);

  return {
    status,
    peers,
    sendEdit,
    sendResult,
    sendPrompt,
    sendActivity,
    sendPointer,
  };
}
