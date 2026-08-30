/*
 * Runs Python off the main thread.
 *
 * This file exists because of one hard constraint: Pyodide executes on
 * whatever thread it is loaded on, and Python does not yield. On the main
 * thread, `for i in range(10000000)` freezes the entire tab — the Stop button
 * cannot be clicked, because the click handler never gets to run. No amount of
 * async/await changes that; `runPythonAsync` only yields at await points, and
 * a tight loop has none.
 *
 * In a worker, the main thread stays responsive and a timeout can call
 * worker.terminate(), which kills the thread unconditionally. That is the only
 * reliable way to stop a runaway loop. It is also why the caller throws the
 * worker away after a timeout and builds a fresh one: a terminated worker
 * cannot be resumed, and its interpreter is gone with it.
 *
 * Served from /public as a classic worker rather than bundled, so Pyodide's
 * importScripts and its wheel fetches resolve against the CDN without the
 * bundler trying to follow them.
 */

const PYODIDE_VERSION = "0.28.3";
const PYODIDE_CDN = `https://cdn.jsdelivr.net/pyodide/v${PYODIDE_VERSION}/full/`;

// Output caps. A loop that prints forever will otherwise grow a string until
// the tab dies of memory exhaustion — which looks exactly like the freeze this
// file is meant to prevent.
const MAX_LINES = 5000;
const MAX_CHARS = 200_000;

let pyodidePromise = null;

function loadPyodideOnce() {
  if (!pyodidePromise) {
    importScripts(`${PYODIDE_CDN}pyodide.js`);
    pyodidePromise = self.loadPyodide({ indexURL: PYODIDE_CDN });
  }
  return pyodidePromise;
}

self.onmessage = async (event) => {
  const { type, source } = event.data ?? {};

  if (type === "warm") {
    // Prefetch so the first real run is not also a ~6MB download.
    try {
      await loadPyodideOnce();
      self.postMessage({ type: "ready" });
    } catch (err) {
      self.postMessage({ type: "error", message: String(err) });
    }
    return;
  }

  if (type !== "run") return;

  let pyodide;
  try {
    pyodide = await loadPyodideOnce();
  } catch {
    self.postMessage({
      type: "done",
      output: "Could not load the Python runtime. Check your connection and try again.",
      failed: true,
    });
    return;
  }

  const chunks = [];
  let chars = 0;
  let truncated = false;

  const collect = (s) => {
    if (truncated) return;
    if (chunks.length >= MAX_LINES || chars + s.length > MAX_CHARS) {
      truncated = true;
      chunks.push(`\n... output truncated at ${MAX_LINES} lines.`);
      return;
    }
    chunks.push(s);
    chars += s.length;
  };

  pyodide.setStdout({ batched: collect });
  pyodide.setStderr({ batched: collect });

  try {
    await pyodide.runPythonAsync(source);
    const output = chunks.join("\n").trimEnd();
    self.postMessage({
      type: "done",
      output: output === "" ? "(no output)" : output,
      failed: false,
    });
  } catch (err) {
    // Pyodide puts the Python traceback in the message. Keep whatever was
    // printed first — it is usually the half that explains the traceback.
    const printed = chunks.join("\n").trimEnd();
    const trace = (err && err.message ? err.message : String(err)).trimEnd();
    self.postMessage({
      type: "done",
      output: printed ? `${printed}\n${trace}` : trace,
      failed: true,
    });
  } finally {
    pyodide.setStdout({ batched: () => {} });
    pyodide.setStderr({ batched: () => {} });
  }
};
