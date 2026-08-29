const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

/** Mirrors judge0.Result in backend/internal/judge0/client.go. */
export type RunResult = {
  stdout: string;
  stderr: string;
  compile_output: string;
  /** Judge0's verdict: "Accepted", "Compilation Error", and so on. */
  status: string;
  time: string;
  memory: number;
};

export class RunError extends Error {}

/**
 * Sends source to the Go backend, which proxies it to Judge0.
 *
 * The browser never talks to Judge0 directly: it is a separate service with
 * no auth of its own, and putting it behind the Go API keeps the sandbox off
 * the public surface.
 */
export async function runCode(
  language: string,
  source: string,
  signal?: AbortSignal,
): Promise<RunResult> {
  let response: Response;
  try {
    response = await fetch(`${API_BASE}/api/run`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ language, source }),
      signal,
    });
  } catch (cause) {
    if (cause instanceof DOMException && cause.name === "AbortError") throw cause;
    throw new RunError("Could not reach the server. Is the backend running?");
  }

  if (!response.ok) {
    // The API reports failures as { error }, but a proxy or a crash can
    // return something else entirely.
    const message = await response
      .json()
      .then((body: { error?: string }) => body.error)
      .catch(() => null);
    throw new RunError(message ?? `Run failed (${response.status})`);
  }

  return response.json() as Promise<RunResult>;
}

/**
 * Flattens a result into what the output panel shows.
 *
 * Compile output comes first: when compilation fails there is no stdout, and
 * the compiler's message is the only thing that explains why.
 */
export function formatRunResult(result: RunResult): string {
  const parts: string[] = [];

  if (result.compile_output.trim()) parts.push(result.compile_output.trimEnd());
  if (result.stdout.trim()) parts.push(result.stdout.trimEnd());
  if (result.stderr.trim()) parts.push(result.stderr.trimEnd());

  if (parts.length === 0) {
    parts.push(`(no output)`);
  }

  const timing = result.time ? ` in ${result.time}s` : "";
  parts.push(`\n// ${result.status}${timing}`);

  return parts.join("\n");
}

/** Anything other than a clean run is shown in red rather than green. */
export function isFailure(result: RunResult): boolean {
  return result.status !== "Accepted";
}
