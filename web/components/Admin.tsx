"use client";

import { useCallback, useEffect, useState } from "react";
import Link from "next/link";
import Logo from "./Logo";
import { useAuth } from "@/lib/useAuth";
import {
  api,
  ApiError,
  isUnauthorized,
  type AdminUser,
  type PromoCode,
} from "@/lib/api";

/**
 * Tiers a code may grant. Free is absent on purpose — a code granting Free
 * would do nothing, and offering it invites somebody to issue one and then
 * wonder why the recipient is unhappy.
 */
const GRANTABLE = [
  { id: "unlimited", label: "Unlimited" },
  { id: "pro", label: "Pro" },
  { id: "enterprise", label: "Enterprise" },
];

/** Tiers an account may be moved to. Free is included: demotion is real. */
const ASSIGNABLE = [{ id: "free", label: "Free" }, ...GRANTABLE];

type Tab = "codes" | "accounts";

function formatDate(iso: string): string {
  return new Date(iso).toLocaleDateString(undefined, {
    day: "numeric",
    month: "short",
    year: "numeric",
  });
}

export default function Admin() {
  const { status, user, signOut } = useAuth("/login");

  const [tab, setTab] = useState<Tab>("codes");
  const [codes, setCodes] = useState<PromoCode[] | null>(null);
  const [users, setUsers] = useState<AdminUser[] | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [copied, setCopied] = useState<string | null>(null);

  // New-code form.
  const [code, setCode] = useState("");
  const [grantPlan, setGrantPlan] = useState("unlimited");
  const [seats, setSeats] = useState(0);
  const [days, setDays] = useState(0);
  const [note, setNote] = useState("");
  const [creating, setCreating] = useState(false);

  const isAdmin = status === "signedIn" && user.is_admin;

  const load = useCallback(
    async (signal?: AbortSignal) => {
      try {
        const [c, u] = await Promise.all([
          api.admin.listPromo(signal),
          api.admin.listUsers(signal),
        ]);
        setCodes(c);
        setUsers(u);
        setError(null);
      } catch (cause) {
        if (cause instanceof DOMException && cause.name === "AbortError") return;
        if (isUnauthorized(cause)) return;
        setError(cause instanceof ApiError ? cause.message : "Could not load the admin data.");
      }
    },
    [],
  );

  useEffect(() => {
    if (!isAdmin) return;
    const controller = new AbortController();
    // Queued past commit for the same reason the dashboard does it: load()
    // resolves synchronously on a cached response, and setting state during
    // the effect body is the cascade React warns about.
    queueMicrotask(() => {
      if (!controller.signal.aborted) void load(controller.signal);
    });
    return () => controller.abort();
  }, [isAdmin, load]);

  async function onCreate(event: React.FormEvent) {
    event.preventDefault();
    if (creating) return;

    setCreating(true);
    setError(null);
    try {
      const created = await api.admin.createPromo({
        code: code.trim(),
        plan: grantPlan,
        maxRedemptions: seats,
        grantDays: days,
        note: note.trim(),
      });
      setCodes((prev) => [created, ...(prev ?? [])]);
      setCode("");
      setNote("");
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Could not create the code.");
    } finally {
      setCreating(false);
    }
  }

  async function onDelete(target: PromoCode) {
    // Two questions, asked separately, because they are different decisions
    // and the destructive one must be opt-in rather than a side effect.
    if (!window.confirm(`Delete ${target.code}? Nobody will be able to claim it again.`)) {
      return;
    }
    let revoke = false;
    if (target.redemptions > 0) {
      revoke = window.confirm(
        `${target.redemptions} account(s) already redeemed ${target.code}.\n\n` +
          "OK — also take their access away.\n" +
          "Cancel — leave what they already have.",
      );
    }

    setBusy(target.code);
    setError(null);
    try {
      const { grants_revoked } = await api.admin.deletePromo(target.code, revoke);
      setCodes((prev) => (prev ?? []).filter((c) => c.code !== target.code));
      if (grants_revoked > 0) void load();
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Could not delete the code.");
    } finally {
      setBusy(null);
    }
  }

  async function onSetPlan(target: AdminUser, next: string) {
    setBusy(`u${target.id}`);
    setError(null);
    try {
      const updated = await api.admin.setUserPlan(target.id, next);
      setUsers((prev) => (prev ?? []).map((u) => (u.id === target.id ? updated : u)));
    } catch (cause) {
      setError(cause instanceof ApiError ? cause.message : "Could not change the plan.");
    } finally {
      setBusy(null);
    }
  }

  async function copy(text: string) {
    try {
      await navigator.clipboard.writeText(text);
      setCopied(text);
      window.setTimeout(() => setCopied((c) => (c === text ? null : c)), 2000);
    } catch {
      window.prompt("Copy this code:", text);
    }
  }

  // Nothing renders while the session is being checked; useAuth redirects.
  if (status !== "signedIn") return null;

  // The server answers 404 for a non-admin, so this branch is a courtesy for
  // somebody who typed the URL — not the security boundary.
  if (!user.is_admin) {
    return (
      <main className="mx-auto flex min-h-screen max-w-lg flex-col items-center justify-center px-6 text-center">
        <h1 className="text-xl font-semibold text-ink">Not available</h1>
        <p className="mt-2 text-sm text-ink-muted">
          This area is for the people running SyncR.
        </p>
        <Link href="/dashboard" className="btn-primary mt-6 h-11 px-5 text-sm">
          Go to interviews
        </Link>
      </main>
    );
  }

  return (
    <div className="min-h-screen">
      <header className="border-b border-line">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-6 px-6">
          <div className="flex items-center gap-3">
            <Link href="/" className="shrink-0 rounded-sm">
              <Logo className="h-[24px] w-auto" priority />
            </Link>
            <span className="rounded-full bg-bg-sunken px-2 py-0.5 text-xs font-medium text-ink-muted">
              Admin
            </span>
          </div>
          <div className="flex items-center gap-4">
            <Link href="/dashboard" className="nav-link text-sm">
              Interviews
            </Link>
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

      <main className="mx-auto max-w-6xl px-6 py-12">
        <h1 className="text-2xl font-semibold tracking-tight text-ink">Administration</h1>
        <p className="mt-1.5 text-sm text-ink-muted">
          Signed in as {user.email}. Everything here affects the live deployment.
        </p>

        <div className="mt-8 flex gap-1 border-b border-line">
          {(
            [
              ["codes", `Promotion codes${codes ? ` (${codes.length})` : ""}`],
              ["accounts", `Accounts${users ? ` (${users.length})` : ""}`],
            ] as const
          ).map(([id, label]) => (
            <button
              key={id}
              type="button"
              onClick={() => setTab(id)}
              className={`-mb-px border-b-2 px-4 py-2 text-sm font-medium transition-colors ${
                tab === id
                  ? "border-accent text-ink"
                  : "border-transparent text-ink-muted hover:text-ink"
              }`}
            >
              {label}
            </button>
          ))}
        </div>

        {error && (
          <p className="form-error mt-4" role="alert" aria-live="polite">
            {error}
          </p>
        )}

        {tab === "codes" ? (
          <>
            <form
              onSubmit={onCreate}
              className="card mt-6 flex flex-col gap-3 p-4 sm:flex-row sm:items-end sm:flex-wrap"
            >
              <div className="min-w-[12rem] flex-1">
                <label htmlFor="code" className="field-label">
                  Code
                </label>
                <input
                  id="code"
                  className="field font-mono uppercase"
                  value={code}
                  onChange={(e) => setCode(e.target.value)}
                  placeholder="UNI-CS-2026"
                  maxLength={64}
                  disabled={creating}
                  autoCapitalize="characters"
                  autoCorrect="off"
                  spellCheck={false}
                />
              </div>
              <div className="sm:w-36">
                <label htmlFor="plan" className="field-label">
                  Grants
                </label>
                <select
                  id="plan"
                  className="field"
                  value={grantPlan}
                  onChange={(e) => setGrantPlan(e.target.value)}
                  disabled={creating}
                >
                  {GRANTABLE.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.label}
                    </option>
                  ))}
                </select>
              </div>
              <div className="sm:w-28">
                <label htmlFor="seats" className="field-label">
                  Seats
                </label>
                <input
                  id="seats"
                  type="number"
                  min={0}
                  className="field"
                  value={seats}
                  onChange={(e) => setSeats(Number(e.target.value))}
                  disabled={creating}
                  title="0 means no ceiling"
                />
              </div>
              <div className="sm:w-28">
                <label htmlFor="days" className="field-label">
                  Days
                </label>
                <input
                  id="days"
                  type="number"
                  min={0}
                  className="field"
                  value={days}
                  onChange={(e) => setDays(Number(e.target.value))}
                  disabled={creating}
                  title="0 means the grant never lapses"
                />
              </div>
              <div className="min-w-[10rem] flex-1">
                <label htmlFor="note" className="field-label">
                  Note
                </label>
                <input
                  id="note"
                  className="field"
                  value={note}
                  onChange={(e) => setNote(e.target.value)}
                  placeholder="placement office"
                  maxLength={200}
                  disabled={creating}
                />
              </div>
              <button
                type="submit"
                className="btn-primary h-11 px-5 text-sm"
                disabled={creating}
              >
                {creating ? "Creating…" : "Issue code"}
              </button>
              <p className="w-full text-xs text-ink-muted">
                Seats 0 means anyone may claim it. Days 0 means the grant never
                lapses. A person can claim each code once.
              </p>
            </form>

            {codes === null ? (
              <p className="py-12 text-center text-sm text-ink-muted">Loading…</p>
            ) : codes.length === 0 ? (
              <div className="card mt-6 p-12 text-center">
                <p className="text-sm text-ink-body">No promotion codes yet.</p>
                <p className="mt-1 text-sm text-ink-muted">
                  Issue one above and send it to whoever needs access.
                </p>
              </div>
            ) : (
              <ul className="mt-6 divide-y divide-line overflow-hidden rounded-xl border border-line">
                {codes.map((c) => (
                  <li
                    key={c.code}
                    className="flex flex-col gap-3 bg-white p-4 sm:flex-row sm:items-center sm:justify-between"
                  >
                    <div className="min-w-0">
                      <div className="flex flex-wrap items-center gap-2">
                        <button
                          type="button"
                          onClick={() => void copy(c.code)}
                          className="font-mono font-medium text-ink hover:text-accent"
                          title="Copy"
                        >
                          {copied === c.code ? "Copied" : c.code}
                        </button>
                        <span className="card-tag">{c.plan}</span>
                      </div>
                      <p className="mt-0.5 text-xs text-ink-muted">
                        {c.redemptions}
                        {c.max_redemptions > 0 ? ` of ${c.max_redemptions}` : ""} claimed
                        {" · "}
                        {c.grant_days > 0 ? `${c.grant_days}-day grant` : "never lapses"}
                        {" · "}
                        {formatDate(c.created_at)}
                        {c.note ? ` · ${c.note}` : ""}
                      </p>
                      {c.redeemers.length > 0 && (
                        <p className="mt-1 truncate text-xs text-ink-muted">
                          {c.redeemers.join(", ")}
                        </p>
                      )}
                    </div>
                    <button
                      type="button"
                      onClick={() => void onDelete(c)}
                      disabled={busy === c.code}
                      className="btn-secondary h-9 shrink-0 px-3 text-sm"
                    >
                      {busy === c.code ? "Working…" : "Delete"}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </>
        ) : users === null ? (
          <p className="py-12 text-center text-sm text-ink-muted">Loading…</p>
        ) : (
          <div className="mt-6 overflow-x-auto rounded-xl border border-line">
            <table className="w-full min-w-[46rem] text-left text-sm">
              <thead className="border-b border-line bg-bg-sunken text-xs text-ink-muted">
                <tr>
                  <th className="px-4 py-2.5 font-medium">Account</th>
                  <th className="px-4 py-2.5 font-medium">Effective</th>
                  <th className="px-4 py-2.5 font-medium">Interviews</th>
                  <th className="px-4 py-2.5 font-medium">Minutes</th>
                  <th className="px-4 py-2.5 font-medium">Joined</th>
                  <th className="px-4 py-2.5 font-medium">Subscription</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-line bg-white">
                {users.map((u) => (
                  <tr key={u.id}>
                    <td className="px-4 py-3">
                      <span className="text-ink">{u.email}</span>
                      {u.is_admin && (
                        <span className="ml-2 rounded-full bg-bg-sunken px-2 py-0.5 text-xs text-ink-muted">
                          admin
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3">
                      <span className="text-ink">{u.effective_plan}</span>
                      {u.promo_code && (
                        <span className="ml-1 font-mono text-xs text-ink-muted">
                          via {u.promo_code}
                        </span>
                      )}
                    </td>
                    <td className="px-4 py-3 text-ink-muted">{u.rooms}</td>
                    <td className="px-4 py-3 text-ink-muted">{u.minutes}</td>
                    <td className="px-4 py-3 text-ink-muted">{formatDate(u.created_at)}</td>
                    <td className="px-4 py-3">
                      <select
                        className="field h-9 py-0 text-sm"
                        value={u.plan}
                        disabled={busy === `u${u.id}`}
                        onChange={(e) => void onSetPlan(u, e.target.value)}
                        // The owner list and a live promotion both outrank the
                        // subscription, so say so rather than letting somebody
                        // change this and wonder why nothing moved.
                        title={
                          u.is_admin
                            ? "This account is an owner; its tier comes from OWNER_EMAILS"
                            : u.promo_code
                              ? `A promotion (${u.promo_code}) is currently overriding this`
                              : undefined
                        }
                      >
                        {ASSIGNABLE.map((p) => (
                          <option key={p.id} value={p.id}>
                            {p.label}
                          </option>
                        ))}
                      </select>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </main>
    </div>
  );
}
