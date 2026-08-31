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
/**
 * Where a candidate's invite lives once it has been taken out of the URL.
 *
 * Per tab and per room: sessionStorage dies with the tab, which is the right
 * lifetime for a link that admits someone to one interview, and keying by room
 * stops two rooms open in the same tab from overwriting each other.
 */
function inviteKey(roomId: string): string {
  return `syncr.invite.${roomId}`;
}

/**
 * Reads the invite once, from the URL if it is there and from this tab's store
 * if it is not, and remembers it either way.
 *
 * The remembering is what makes a reload work. The token is deliberately
 * stripped from the address bar below — it should not sit in a screenshot, a
 * screen share or browser history — but that also means the URL cannot be the
 * only copy, or pressing F5 mid-interview locks the candidate out of their own
 * interview.
 */
function readInvite(roomId: string, fromURL: string | null): string | null {
  if (typeof window === "undefined") return fromURL;
  if (fromURL) {
    try {
      window.sessionStorage.setItem(inviteKey(roomId), fromURL);
    } catch {
      // Private mode and "block site data" throw rather than return null.
      // Losing the copy only costs a reload, so carry on with the URL value.
    }
    return fromURL;
  }
  try {
    return window.sessionStorage.getItem(inviteKey(roomId));
  } catch {
    return null;
  }
}

function forgetInvite(roomId: string): void {
  if (typeof window === "undefined") return;
  try {
    window.sessionStorage.removeItem(inviteKey(roomId));
  } catch {
    // Nothing to do; a stale entry only ever produces the same refusal again.
  }
}

export default function RoomGate({ roomId }: { roomId: string }) {
  const params = useSearchParams();

  // Latched, not read live from the router on every render.
  //
  // Stripping ?t= from the address bar calls history.replaceState, and Next
  // feeds that straight back into useSearchParams. Read live, this value went
  // null the instant the URL was cleaned: the effect re-ran, its cleanup
  // aborted the join request already in flight, and the second pass fell
  // through to the interviewer branch — telling a candidate holding a
  // perfectly good link to sign in. Reading it once breaks that loop.
  const [invite] = useState(() => readInvite(roomId, params.get("t")));

  const [state, setState] = useState<
    | { status: "checking" }
    | { status: "ready"; token: string; room: Room | null }
    | { status: "denied"; message: string; canSignIn: boolean }
  >({ status: "checking" });

  useEffect(() => {
    const controller = new AbortController();

    // Take the token out of the address bar now that it is safely latched
    // above, so it does not end up in a screenshot, a screen share or the
    // browser history.
    if (typeof window !== "undefined" && params.get("t")) {
      const clean = new URL(window.location.href);
      clean.searchParams.delete("t");
      window.history.replaceState({}, "", clean.toString());
    }

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
          // A link that has been rotated or revoked is never coming back, so
          // drop the remembered copy rather than replaying the same refusal on
          // every reload for the rest of the tab's life.
          if (cause instanceof ApiError) forgetInvite(roomId);
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
  }, [roomId, invite, params]);

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

        {/*
          A dead end here is how someone gets stuck. Offer whatever actually
          helps: an interviewer who is still signed in wants their list back,
          one whose session lapsed wants to sign in, and a candidate whose link
          expired can only go and ask for a new one — so they get the honest
          answer rather than a button that leads nowhere useful.
        */}
        <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
          {state.canSignIn &&
            (getToken() ? (
              <Link href="/dashboard" className="btn-primary h-10 px-5 text-sm">
                Back to interviews
              </Link>
            ) : (
              <Link href="/login" className="btn-primary h-10 px-5 text-sm">
                Sign in
              </Link>
            ))}
          <Link href="/" className="btn-secondary h-10 px-5 text-sm">
            Go home
          </Link>
        </div>
      </main>
    );
  }

  return (
    <RoomEditor
      roomId={roomId}
      token={state.token}
      title={state.room?.title}
      initialLanguage={state.room?.language ?? "python"}
    />
  );
}
