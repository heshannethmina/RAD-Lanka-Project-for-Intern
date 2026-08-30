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

export type User = { id: number; email: string };

export type Room = {
  id: string;
  title: string;
  language: string;
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

  me: (signal?: AbortSignal) => request<User>("/api/me", { auth: true, signal }),

  createRoom: (title: string, language: string) =>
    request<CreatedRoom>("/api/rooms", {
      method: "POST",
      auth: true,
      body: { title, language },
    }),

  listRooms: (signal?: AbortSignal) =>
    request<{ rooms: Room[] }>("/api/rooms", { auth: true, signal }).then((r) => r.rooms),

  /**
   * One of my own rooms. The server answers 404 rather than 403 when the room
   * belongs to someone else, so this doubles as the ownership check.
   */
  getRoom: (roomId: string, signal?: AbortSignal) =>
    request<Room>(`/api/rooms/${encodeURIComponent(roomId)}`, { auth: true, signal }),

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
