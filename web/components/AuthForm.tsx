"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import Logo from "./Logo";
import { api, ApiError, setToken } from "@/lib/api";

type Mode = "login" | "register";

const COPY = {
  login: {
    title: "Sign in",
    subtitle: "Run your next technical interview.",
    submit: "Sign in",
    pending: "Signing in…",
    switchPrompt: "New here?",
    switchLabel: "Create an account",
    switchHref: "/register",
    autoComplete: "current-password",
  },
  register: {
    title: "Create an account",
    subtitle: "Free while SyncR is in pilot.",
    submit: "Create account",
    pending: "Creating…",
    switchPrompt: "Already have an account?",
    switchLabel: "Sign in",
    switchHref: "/login",
    autoComplete: "new-password",
  },
} as const;

/**
 * Sign-in and sign-up, which differ only in wording and which endpoint they
 * call. One component rather than two near-identical pages, so the two cannot
 * drift apart.
 *
 * Only interviewers ever see this. Candidates arrive through an invite link
 * and never hold an account at all.
 */
export default function AuthForm({ mode }: { mode: Mode }) {
  const copy = COPY[mode];
  const router = useRouter();

  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [pending, setPending] = useState(false);

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (pending) return;

    setPending(true);
    setError(null);
    try {
      const result =
        mode === "login"
          ? await api.login(email, password)
          : await api.register(email, password);

      setToken(result.token);
      // replace, not push: the back button should not return to a sign-in
      // form the user has already passed through.
      router.replace("/dashboard");
    } catch (cause) {
      setError(
        cause instanceof ApiError
          ? cause.message
          : "Something went wrong. Please try again.",
      );
      // Only the password is cleared. Retyping a correct email to fix a typo
      // in the password is a small, avoidable annoyance.
      setPassword("");
      setPending(false);
    }
  }

  return (
    <main className="flex min-h-screen flex-col items-center justify-center px-6 py-16">
      <div className="w-full max-w-sm">
        <Link href="/" className="mb-10 inline-block rounded-sm">
          <Logo className="h-[26px] w-auto" priority />
        </Link>

        <h1 className="text-2xl font-semibold tracking-tight text-ink">
          {copy.title}
        </h1>
        <p className="mt-1.5 text-sm text-ink-muted">{copy.subtitle}</p>

        <form onSubmit={onSubmit} className="mt-8 space-y-4" noValidate>
          {error && (
            /* aria-live so a screen reader announces a failure that happens
               well after the button was pressed. */
            <p className="form-error" role="alert" aria-live="polite">
              {error}
            </p>
          )}

          <div>
            <label htmlFor="email" className="field-label">
              Email
            </label>
            <input
              id="email"
              type="email"
              className="field"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              autoComplete="email"
              autoFocus
              required
              disabled={pending}
              placeholder="you@company.com"
            />
          </div>

          <div>
            <label htmlFor="password" className="field-label">
              Password
            </label>
            <input
              id="password"
              type="password"
              className="field"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              autoComplete={copy.autoComplete}
              required
              disabled={pending}
              // Mirrors auth.MinPasswordLen. The server is still the authority;
              // this only saves a round trip.
              minLength={mode === "register" ? 8 : undefined}
              placeholder={mode === "register" ? "At least 8 characters" : ""}
            />
          </div>

          <button
            type="submit"
            className="btn-primary h-11 w-full text-sm"
            disabled={pending}
          >
            {pending ? copy.pending : copy.submit}
          </button>
        </form>

        <p className="mt-6 text-sm text-ink-muted">
          {copy.switchPrompt}{" "}
          <Link
            href={copy.switchHref}
            className="font-medium text-accent hover:underline"
          >
            {copy.switchLabel}
          </Link>
        </p>
      </div>
    </main>
  );
}
