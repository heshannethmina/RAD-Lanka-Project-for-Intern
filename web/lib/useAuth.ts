"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { api, getToken, isUnauthorized, setToken, type User } from "./api";

type AuthState =
  /** Still asking the server whether the stored token is any good. */
  | { status: "loading"; user: null }
  | { status: "signedIn"; user: User }
  | { status: "signedOut"; user: null };

/**
 * Resolves the stored session token into a user.
 *
 * The token is checked against the server rather than trusted, because it may
 * have expired or been revoked from another device since it was written. Until
 * that answer comes back the state is "loading", and pages must render a
 * placeholder rather than either the signed-in or signed-out view — flashing
 * the signed-out one first is the classic version of this bug.
 *
 * Pass `redirectTo` on a page that requires an account.
 */
export function useAuth(redirectTo?: string): AuthState & {
  signOut: () => Promise<void>;
} {
  const router = useRouter();
  const [state, setState] = useState<AuthState>({ status: "loading", user: null });

  useEffect(() => {
    // No token at all: skip the request, there is nothing to verify.
    if (!getToken()) {
      setState({ status: "signedOut", user: null });
      if (redirectTo) router.replace(redirectTo);
      return;
    }

    const controller = new AbortController();
    api
      .me(controller.signal)
      .then((user) => setState({ status: "signedIn", user }))
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") return;
        // Any rejection of the token means it is not usable; drop it so the
        // next load does not repeat the round trip.
        if (isUnauthorized(error)) setToken(null);
        setState({ status: "signedOut", user: null });
        if (redirectTo) router.replace(redirectTo);
      });

    return () => controller.abort();
  }, [router, redirectTo]);

  const signOut = useCallback(async () => {
    await api.logout();
    setState({ status: "signedOut", user: null });
    router.replace("/login");
  }, [router]);

  return { ...state, signOut };
}
