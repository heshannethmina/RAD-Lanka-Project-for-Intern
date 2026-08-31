"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import Logo from "./Logo";
import PromoRedeem from "./PromoRedeem";
import { useAuth } from "@/lib/useAuth";
import {
  api,
  ApiError,
  inviteURL,
  isUnauthorized,
  type Room,
} from "@/lib/api";

/** Must stay in step with judge0.languageIDs on the server. */
const LANGUAGES = [
  { id: "python", label: "Python" },
  { id: "go", label: "Go" },
  { id: "javascript", label: "JavaScript" },
];

function formatDate(iso: string): string {
  const d = new Date(iso);
  return d.toLocaleDateString(undefined, {
    day: "numeric",
    month: "short",
    year: d.getFullYear() === new Date().getFullYear() ? undefined : "numeric",
  });
}

export default function Dashboard() {
  const { status, user, signOut, setUser } = useAuth("/login");
  const usage = status === "signedIn" ? user.usage : null;

  const [rooms, setRooms] = useState<Room[] | null>(null);
  const [error, setError] = useState<string | null>(null);

  /**
   * Invite tokens revealed during this page's lifetime.
   *
   * The server returns a token only from create and rotate, because it stores
   * just the hash. So a reload loses them, and the only way back to a link is
   * to rotate — which is why the row's action says "New link" rather than
   * "Copy link" once the token is gone. That is the intended trade, not a bug.
   */
  const [invites, setInvites] = useState<Record<string, string>>({});
  const [copied, setCopied] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);

  const [title, setTitle] = useState("");
  const [language, setLanguage] = useState("python");
  // 0 means "whatever the plan allows", which is the right default: somebody
  // on Free asking for an hour would only have it clamped anyway.
  const [minutes, setMinutes] = useState(0);
  const [creating, setCreating] = useState(false);

  const load = useCallback(async (signal?: AbortSignal) => {
    try {
      setRooms(await api.listRooms(signal));
      setError(null);
    } catch (cause) {
      if (cause instanceof DOMException && cause.name === "AbortError") return;
      // useAuth already redirects on a dead session; do not also show an error
      // for it, or the page flashes a failure on the way out.
      if (isUnauthorized(cause)) return;
      setError(cause instanceof ApiError ? cause.message : "Could not load your rooms.");
    }
  }, []);

  useEffect(() => {
    if (status !== "signedIn") return;
    const controller = new AbortController();
    // Queued rather than called straight from the effect body: load() resolves
    // synchronously on a cached response, and setting state during the effect
    // is the cascade React warns about. A microtask defers it past commit
    // without any user-visible delay.
    queueMicrotask(() => {
      if (!controller.signal.aborted) void load(controller.signal);
    });
    return () => controller.abort();
  }, [status, load]);

  async function onCreate(event: React.FormEvent) {
    event.preventDefault();
    if (creating) return;

    setCreating(true);
    setError(null);
    try {
      const room = await api.createRoom({
        title: title.trim(),
        language,
        durationMinutes: minutes,
      });
      setInvites((prev) => ({ ...prev, [room.id]: room.invite_token }));
      // Prepend rather than refetch: the listing is newest-first, so this is
      // where the server would have put it anyway.
      setRooms((prev) => [room, ...(prev ?? [])]);
      setTitle("");
      await copyLink(room.id, room.invite_token);
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Could not create the room.");
    } finally {
      setCreating(false);
    }
  }

  async function copyLink(roomId: string, token: string) {
    const url = inviteURL(roomId, token);
    try {
      await navigator.clipboard.writeText(url);
      setCopied(roomId);
      window.setTimeout(() => setCopied((c) => (c === roomId ? null : c)), 2000);
    } catch {
      // Clipboard access can be denied, and over plain http on anything but
      // localhost it is unavailable outright. Falling back to a prompt is ugly
      // but it still gets the link into someone's hands, which is the point.
      window.prompt("Copy this link and send it to your candidate:", url);
    }
  }

  async function onRotate(roomId: string) {
    setBusy(roomId);
    setError(null);
    try {
      const room = await api.rotateInvite(roomId);
      setInvites((prev) => ({ ...prev, [roomId]: room.invite_token }));
      await copyLink(roomId, room.invite_token);
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Could not create a link.");
    } finally {
      setBusy(null);
    }
  }

  async function onClose(roomId: string) {
    if (!window.confirm("End this interview? The candidate's link stops working.")) {
      return;
    }
    setBusy(roomId);
    setError(null);
    try {
      await api.closeRoom(roomId);
      setRooms((prev) =>
        (prev ?? []).map((r) =>
          r.id === roomId ? { ...r, open: false, closed_at: new Date().toISOString() } : r,
        ),
      );
      // The link is dead now, so stop offering to copy it.
      setInvites((prev) => {
        const next = { ...prev };
        delete next[roomId];
        return next;
      });
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Could not close the room.");
    } finally {
      setBusy(null);
    }
  }

  // Render nothing while the token is being checked. Showing the signed-out
  // view first and then swapping is the flash this avoids; useAuth handles the
  // redirect itself.
  if (status !== "signedIn") return null;

  return (
    <div className="min-h-screen">
      <header className="border-b border-line">
        <div className="mx-auto flex h-16 max-w-5xl items-center justify-between gap-6 px-6">
          <Link href="/" className="shrink-0 rounded-sm">
            <Logo className="h-[24px] w-auto" priority />
          </Link>
          <div className="flex items-center gap-4">
            {/* Only shown to an admin, but the server is the boundary: the
                admin routes answer 404 for anybody else regardless. */}
            {user.is_admin && (
              <Link href="/admin" className="nav-link text-sm">
                Admin
              </Link>
            )}
            <span className="hidden text-sm text-ink-muted sm:inline">{user.email}</span>
            <button
              type="button"
              onClick={() => void signOut()}
              className="btn-secondary h-9 px-4 text-sm"
            >
              Sign out
            </button>
          </div>
        </div>
      </header>

      <main className="mx-auto max-w-5xl px-6 py-12">
        <h1 className="text-2xl font-semibold tracking-tight text-ink">Interviews</h1>
        <p className="mt-1.5 text-sm text-ink-muted">
          Create a room, then send the link to your candidate. They join without
          an account.
        </p>

        {usage && (
          <div className="card mt-6 flex flex-wrap items-center justify-between gap-x-6 gap-y-2 p-4 text-[13px]">
            <div className="flex flex-wrap items-center gap-x-6 gap-y-1">
              <span className="font-medium text-ink">{usage.plan_label} plan</span>
              <span className="text-ink-muted">
                {usage.unlimited_interviews
                  ? `${usage.interviews_used} interviews run`
                  : `${usage.interviews_used} of ${usage.interviews_included} interviews used`}
                {/* "for life" vs "this month" is the whole difference between
                    a trial and an allowance, and somebody deciding whether to
                    upgrade needs to know which they are looking at. */}
                {!usage.unlimited_interviews &&
                  (usage.lifetime ? " — for life" : " this month")}
              </span>
              {usage.max_minutes > 0 && (
                <span className="text-ink-muted">
                  Up to {usage.max_minutes} min each
                </span>
              )}
            </div>
            {!usage.unlimited_interviews &&
              usage.interviews_used >= usage.interviews_included && (
                <Link href="/#pricing" className="font-medium text-accent hover:underline">
                  Upgrade for more
                </Link>
              )}
            {/* Full width, so the redeem form has room to open without
                squeezing the figures above it onto extra lines. */}
            <div className="w-full">
              <PromoRedeem usage={usage} onRedeemed={setUser} />
            </div>
          </div>
        )}

        <form
          onSubmit={onCreate}
          className="card mt-8 flex flex-col gap-3 p-4 sm:flex-row sm:items-end"
        >
          <div className="flex-1">
            <label htmlFor="title" className="field-label">
              What is this interview for?
            </label>
            <input
              id="title"
              className="field"
              value={title}
              onChange={(e) => setTitle(e.target.value)}
              placeholder="Backend screen — Priya"
              maxLength={200}
              disabled={creating}
            />
          </div>
          <div className="sm:w-36">
            <label htmlFor="minutes" className="field-label">
              Length
            </label>
            <select
              id="minutes"
              className="field"
              value={minutes}
              onChange={(e) => setMinutes(Number(e.target.value))}
              disabled={creating}
            >
              {/* Zero first, because letting the plan decide is the right
                  answer for most people and needs no explanation. */}
              <option value={0}>Plan default</option>
              {[10, 15, 30, 45, 60].map((m) => (
                <option key={m} value={m}>
                  {m} min
                </option>
              ))}
            </select>
          </div>

          <div className="sm:w-36">
            <label htmlFor="language" className="field-label">
              Language
            </label>
            <select
              id="language"
              className="field"
              value={language}
              onChange={(e) => setLanguage(e.target.value)}
              disabled={creating}
            >
              {LANGUAGES.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.label}
                </option>
              ))}
            </select>
          </div>
          <button
            type="submit"
            className="btn-primary h-11 px-5 text-sm sm:w-auto"
            disabled={creating}
          >
            {creating ? "Creating…" : "New interview"}
          </button>
        </form>

        {error && (
          <p className="form-error mt-4" role="alert" aria-live="polite">
            {error}
          </p>
        )}

        <div className="mt-8">
          {rooms === null ? (
            <p className="py-12 text-center text-sm text-ink-muted">Loading…</p>
          ) : rooms.length === 0 ? (
            <div className="card p-12 text-center">
              <p className="text-sm text-ink-body">No interviews yet.</p>
              <p className="mt-1 text-sm text-ink-muted">
                Create one above and the candidate link is copied for you.
              </p>
            </div>
          ) : (
            <ul className="divide-y divide-line overflow-hidden rounded-xl border border-line">
              {rooms.map((room) => {
                const invite = invites[room.id];
                const isBusy = busy === room.id;
                return (
                  <li
                    key={room.id}
                    className="flex flex-col gap-3 bg-white p-4 sm:flex-row sm:items-center sm:justify-between"
                  >
                    <div className="min-w-0">
                      <div className="flex items-center gap-2">
                        <span className="truncate font-medium text-ink">
                          {room.title || "Untitled interview"}
                        </span>
                        {!room.open && (
                          <span className="shrink-0 rounded-full bg-bg-sunken px-2 py-0.5 text-xs text-ink-muted">
                            Ended
                          </span>
                        )}
                      </div>
                      <p className="mt-0.5 truncate text-xs text-ink-muted">
                        <span className="font-mono">{room.id}</span>
                        {" · "}
                        {LANGUAGES.find((l) => l.id === room.language)?.label ??
                          room.language}
                        {" · "}
                        {room.duration_minutes} min
                        {" · "}
                        {formatDate(room.created_at)}
                      </p>
                    </div>

                    <div className="flex shrink-0 flex-wrap items-center gap-2">
                      {room.open &&
                        (invite ? (
                          <button
                            type="button"
                            onClick={() => void copyLink(room.id, invite)}
                            className="btn-secondary h-9 px-3 text-sm"
                          >
                            {copied === room.id ? "Copied" : "Copy link"}
                          </button>
                        ) : (
                          <button
                            type="button"
                            onClick={() => void onRotate(room.id)}
                            disabled={isBusy}
                            className="btn-secondary h-9 px-3 text-sm"
                            title="The previous link stops working"
                          >
                            {isBusy ? "Working…" : "New link"}
                          </button>
                        ))}
                      <Link
                        href={`/room/${encodeURIComponent(room.id)}`}
                        className="btn-secondary h-9 px-3 text-sm"
                      >
                        Open
                      </Link>
                      {room.open && (
                        <button
                          type="button"
                          onClick={() => void onClose(room.id)}
                          disabled={isBusy}
                          className="btn-secondary h-9 px-3 text-sm"
                        >
                          End
                        </button>
                      )}
                    </div>
                  </li>
                );
              })}
            </ul>
          )}
        </div>
      </main>
    </div>
  );
}
