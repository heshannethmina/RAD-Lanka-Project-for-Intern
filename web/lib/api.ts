/**
 * Client for the Go REST API.
 *
 * The session token lives in localStorage and is sent as an Authorization
 * header. That is a deliberate trade made on the server side — see the note at
 * the top of backend/internal/api/auth.go. The short version: the web app and
 * the API are different origins in every environment, so a cookie would need
 * `SameSite=None; Secure` and would not be sent over plain http, which breaks
 * local development. The cost is that this token is reachable from JavaScript,
 * so keep the XSS surface small.
 */

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

/** Namespaced so it cannot collide with anything else on localhost. */
const TOKEN_KEY = "syncr.session";

/** `plan` is the subscription, not necessarily the tier in force — a
 * promotion can be granting more. Read `usage` for what actually applies. */
export type User = { id: number; email: string; plan: string };

/** What the current plan allows, and how much of it is gone. */
export type Usage = {
  plan_label: string;
  interviews_used: number;
  interviews_included: number;
  unlimited_interviews: boolean;
  /** Longest a single interview may run, 0 when unlimited. */
  max_minutes: number;
  used_minutes: number;
  /** True when the allowance never resets — a trial, not a monthly budget. */
  lifetime: boolean;
  /**
   * Set when a redeemed promotion is what is granting the limits above,
   * rather than a subscription. The UI has to say which, or somebody sees
   * "Unlimited" and assumes they are being charged for it.
   */
  promo_code?: string;
  /** When that grant lapses. Null for one that does not. */
  promo_expires_at: string | null;
};

export type Me = User & { usage: Usage };

export type Room = {
  id: string;
  title: string;
  language: string;
  /** The interview question. Only the owner may change it. */
  prompt: string;
  duration_minutes: number;
  scheduled_at: string | null;
  started_at: string | null;
  /** Absolute deadline, null until somebody opens the room. */
  ends_at: string | null;
  created_at: string;
  closed_at: string | null;
  open: boolean;
};

/**
 * A room as returned by create and rotate — the only two responses that ever
 * carry the invite token, because the server stores just its hash.
 */
export type CreatedRoom = Room & { invite_token: string };

export type AuthResult = {
  token: string;
  expires_at: string;
  user: User;
};

/** Carries the status so callers can tell "wrong password" from "server down". */
export class ApiError extends Error {
  readonly status: number;
  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

/** True when the server rejected our token — the caller should send us to /login. */
export function isUnauthorized(error: unknown): boolean {
  return error instanceof ApiError && error.status === 401;
}

export function getToken(): string | null {
  // Guarded for server rendering, where there is no localStorage at all.
  if (typeof window === "undefined") return null;
  try {
    return window.localStorage.getItem(TOKEN_KEY);
  } catch {
    // Private mode and "block site data" both throw rather than return null.
    return null;
  }
}

export function setToken(token: string | null): void {
  if (typeof window === "undefined") return;
  try {
    if (token === null) window.localStorage.removeItem(TOKEN_KEY);
    else window.localStorage.setItem(TOKEN_KEY, token);
  } catch {
    // Not being able to persist is survivable: the session lasts the tab.
  }
}

type RequestOptions = {
  method?: string;
  body?: unknown;
  /** Send the stored session token. Off for register, login and invite links. */
  auth?: boolean;
  signal?: AbortSignal;
};

async function request<T>(path: string, options: RequestOptions = {}): Promise<T> {
  const { method = "GET", body, auth = false, signal } = options;

  const headers: Record<string, string> = {};
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (auth) {
    const token = getToken();
    if (!token) {
      // Fail here rather than send an anonymous request and read a 401 back:
      // the caller gets the same answer without the round trip.
      throw new ApiError("You are not signed in.", 401);
    }
    headers["Authorization"] = `Bearer ${token}`;
  }

  let response: Response;
  try {
    response = await fetch(`${API_BASE}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
      signal,
    });
  } catch (cause) {
    if (cause instanceof DOMException && cause.name === "AbortError") throw cause;
    throw new ApiError("Could not reach the server. Is the backend running?", 0);
  }

  if (response.status === 204) return undefined as T;

  if (!response.ok) {
    // The API reports failures as { error }, but a proxy or a crash can return
    // something else entirely, so never assume the body parses.
    const message = await response
      .json()
      .then((b: { error?: string }) => b.error)
      .catch(() => null);
    throw new ApiError(message ?? `Request failed (${response.status})`, response.status);
  }

  return response.json() as Promise<T>;
}

export const api = {
  register: (email: string, password: string) =>
    request<AuthResult>("/api/auth/register", {
      method: "POST",
      body: { email, password },
    }),

  login: (email: string, password: string) =>
    request<AuthResult>("/api/auth/login", {
      method: "POST",
      body: { email, password },
    }),

  /**
   * Revokes the token server-side. Errors are swallowed on purpose: whatever
   * the server says, the local token is being discarded, and a failed logout
   * must not leave someone stuck on a page they wanted to leave.
   */
  logout: async () => {
    try {
      await request<void>("/api/auth/logout", { method: "POST", auth: true });
    } catch {
      // Intentionally ignored.
    }
    setToken(null);
  },

  me: (signal?: AbortSignal) => request<Me>("/api/me", { auth: true, signal }),

  /**
   * Applies a promotion code to the signed-in account.
   *
   * Answers with the same shape as `me`, so the caller replaces its user
   * wholesale and the new limits are on screen without a second round trip.
   */
  redeemPromo: (code: string) =>
    request<Me>("/api/promo/redeem", {
      method: "POST",
      auth: true,
      body: { code },
    }),

  createRoom: (opts: {
    title: string;
    language: string;
    /** Clamped to the plan by the server; 0 means "whatever the plan allows". */
    durationMinutes?: number;
    /** When the interviewer means to hold it. Advisory. */
    scheduledAt?: string | null;
  }) =>
    request<CreatedRoom>("/api/rooms", {
      method: "POST",
      auth: true,
      body: {
        title: opts.title,
        language: opts.language,
        duration_minutes: opts.durationMinutes ?? 0,
        scheduled_at: opts.scheduledAt ?? null,
      },
    }),

  listRooms: (signal?: AbortSignal) =>
    request<{ rooms: Room[] }>("/api/rooms", { auth: true, signal }).then((r) => r.rooms),

  /**
   * One of my own rooms. The server answers 404 rather than 403 when the room
   * belongs to someone else, so this doubles as the ownership check.
   */
  getRoom: (roomId: string, signal?: AbortSignal) =>
    request<Room>(`/api/rooms/${encodeURIComponent(roomId)}`, { auth: true, signal }),

  /**
   * Saves the interview question.
   *
   * The socket relays prompt edits live; this is the durable half, so a
   * reload does not lose them. Both enforce owner-only.
   */
  updatePrompt: (roomId: string, prompt: string) =>
    request<void>(`/api/rooms/${encodeURIComponent(roomId)}/prompt`, {
      method: "PUT",
      auth: true,
      body: { prompt },
    }),

  closeRoom: (roomId: string) =>
    request<void>(`/api/rooms/${encodeURIComponent(roomId)}`, {
      method: "DELETE",
      auth: true,
    }),

  rotateInvite: (roomId: string) =>
    request<CreatedRoom>(`/api/rooms/${encodeURIComponent(roomId)}/invite`, {
      method: "POST",
      auth: true,
    }),

  /** The candidate's side of a shareable link. No account, no session token. */
  joinRoom: (roomId: string, inviteToken: string, signal?: AbortSignal) =>
    request<Room>(
      `/api/rooms/${encodeURIComponent(roomId)}/join?token=${encodeURIComponent(inviteToken)}`,
      { signal },
    ),
};

/**
 * Builds the link an interviewer sends to a candidate.
 *
 * Absolute, because it is going into an email or a chat message where a
 * relative path means nothing.
 */
export function inviteURL(roomId: string, inviteToken: string): string {
  const origin = typeof window === "undefined" ? "" : window.location.origin;
  return `${origin}/room/${encodeURIComponent(roomId)}?t=${encodeURIComponent(inviteToken)}`;
}
