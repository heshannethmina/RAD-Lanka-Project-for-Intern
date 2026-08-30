"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import RoomEditor from "./RoomEditor";
import Logo from "./Logo";
import SyncLoader from "./SyncLoader";
import { api, ApiError, getToken, type Room } from "@/lib/api";

/**
 * Works out who is at the door, and with what.
 *
 * Two ways in, and the query string decides which:
 *
 *   ?t=<invite>  a candidate following a shared link. No account.
 *   (nothing)    an interviewer, using the session token in localStorage.
 *
 * Access is resolved over REST before any socket opens. That is not
 * belt-and-braces: a refused WebSocket handshake reaches the browser as close
 * code 1006, identical to a dropped connection, so a socket alone cannot tell
 * "you are not allowed in" from "the wifi died" — and the reconnect loop would
 * hammer away at a room the viewer will never be admitted to. The server still
 * authorises the socket itself; this is about being able to say why.
 */
export default function RoomGate({ roomId }: { roomId: string }) {
  const params = useSearchParams();
  const invite = params.get("t");

  const [state, setState] = useState<
    | { status: "checking" }
    | { status: "ready"; token: string; room: Room | null }
    | { status: "denied"; message: string; canSignIn: boolean }
  >({ status: "checking" });

  useEffect(() => {
    const controller = new AbortController();

    async function resolve() {
      // Candidate first: an invite link should work even for someone who
      // happens to also have an interviewer session in this browser, because
      // the link is what they were sent.
      if (invite) {
        try {
          const room = await api.joinRoom(roomId, invite, controller.signal);
          setState({ status: "ready", token: invite, room });
        } catch (cause) {
          if (cause instanceof DOMException && cause.name === "AbortError") return;
          setState({
            status: "denied",
            message:
              cause instanceof ApiError
                ? cause.message
                : "Could not open this room.",
            canSignIn: false,
          });
        }
        return;
      }

      // Interviewer.
      const session = getToken();
      if (!session) {
        setState({
          status: "denied",
          message: "Sign in to open this room, or use the link you were sent.",
          canSignIn: true,
        });
        return;
      }

      try {
        // Doubles as the ownership check: the server answers 404 for a room
        // that is not yours, so a stranger with a valid session gets the same
        // answer as for a room that does not exist.
        const room = await api.getRoom(roomId, controller.signal);
        setState({ status: "ready", token: session, room });
      } catch (cause) {
        if (cause instanceof DOMException && cause.name === "AbortError") return;
        const unauthorized = cause instanceof ApiError && cause.status === 401;
        setState({
          status: "denied",
          message: unauthorized
            ? "Your session has expired."
            : cause instanceof ApiError
              ? cause.message
              : "Could not open this room.",
          canSignIn: true,
        });
      }
    }

    void resolve();
    return () => controller.abort();
  }, [roomId, invite]);

  if (state.status === "checking") {
    return (
      <main className="flex min-h-screen items-center justify-center">
        <SyncLoader />
      </main>
    );
  }

  if (state.status === "denied") {
    return (
      <main className="flex min-h-screen flex-col items-center justify-center px-6 text-center">
        <Link href="/" className="mb-10 rounded-sm">
          <Logo className="h-[26px] w-auto" priority />
        </Link>
        <h1 className="text-xl font-semibold tracking-tight text-ink">
          This room is not open to you
        </h1>
        <p className="mt-2 max-w-sm text-sm text-ink-muted">{state.message}</p>
        {state.canSignIn && (
          <Link href="/login" className="btn-primary mt-6 h-10 px-5 text-sm">
            Sign in
          </Link>
        )}
      </main>
    );
  }

  return (
    <RoomEditor
      roomId={roomId}
      token={state.token}
      initialLanguage={state.room?.language ?? "python"}
    />
  );
}
