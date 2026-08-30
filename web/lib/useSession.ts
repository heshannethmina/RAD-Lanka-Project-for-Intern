"use client";

import { useSyncExternalStore } from "react";
import { getToken } from "./api";

/**
 * Whether this browser is holding a session token.
 *
 * Deliberately does *not* verify it with the server — this exists so the
 * marketing nav can offer "Dashboard" instead of "Sign in", and being wrong
 * costs a redirect to /login rather than anything unsafe. Use `useAuth` where
 * the answer has to be trustworthy.
 *
 * Read through useSyncExternalStore because localStorage is external state: it
 * returns false on the server and the real value on the client, and React
 * reconciles the two without a hydration mismatch and without the extra render
 * that restoring in an effect would cost.
 */
export function useHasSession(): boolean {
  return useSyncExternalStore(subscribe, () => getToken() !== null, () => false);
}

function subscribe(onChange: () => void) {
  // Fires when another tab signs in or out, so every open tab agrees.
  window.addEventListener("storage", onChange);
  return () => window.removeEventListener("storage", onChange);
}
