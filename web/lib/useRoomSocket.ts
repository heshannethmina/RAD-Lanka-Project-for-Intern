"use client";

import { useCallback, useEffect, useRef, useState } from "react";

export type ConnectionStatus = "connecting" | "open" | "closed";

/** Frames the server sends. Mirrors backend/internal/ws/message.go. */
type ServerMessage =
  | { type: "snapshot"; text?: string }
  | { type: "edit"; text?: string }
  | { type: "presence"; clients?: number };

const WS_BASE = process.env.NEXT_PUBLIC_WS_URL ?? "ws://localhost:8080";

const MAX_BACKOFF_MS = 10_000;

type Handlers = {
  /**
   * Full document handed over on join. `send` is passed in so a first client
   * can seed an empty room without the caller having to reach back into this
   * hook's return value.
   */
  onSnapshot: (text: string, send: (text: string) => void) => void;
  /** Full document after somebody else's edit. */
  onEdit: (text: string) => void;
};

/**
 * Opens the room's WebSocket and keeps it open.
 *
 * The connection is deliberately direct to the Go backend — it does not go
 * through Next.js, which only renders the shell.
 */
export function useRoomSocket(roomId: string, handlers: Handlers) {
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

  useEffect(() => {
    let disposed = false;
    let retry: ReturnType<typeof setTimeout> | undefined;
    let attempt = 0;

    function connect() {
      if (disposed) return;

      setStatus("connecting");
      const ws = new WebSocket(`${WS_BASE}/ws/${encodeURIComponent(roomId)}`);
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
            handlersRef.current.onSnapshot(msg.text ?? "", sendEdit);
            break;
          case "edit":
            handlersRef.current.onEdit(msg.text ?? "");
            break;
          case "presence":
            setPeers(msg.clients ?? 1);
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
  }, [roomId, sendEdit]);

  return { status, peers, sendEdit };
}
