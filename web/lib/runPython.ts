/**
 * Runs Python in the candidate's own browser, using Pyodide — CPython
 * compiled to WebAssembly.
 *
 * Why here and not on a server: an interview is an invitation for a stranger
 * to run code. Executing that on our backend means the process can read our
 * environment, and `os.environ["DATABASE_URL"]` would hand over every account
 * in the database. Judge0 exists to stop exactly that, but it needs cgroup v1
 * and a privileged container, which no managed host grants.
 *
 * Running it in the browser sidesteps the problem rather than defending
 * against it: the code executes in the viewer's own tab, inside the sandbox
 * every website already runs in, and there is nothing of ours nearby to
 * compromise. The worst a candidate can do is hang their own tab.
 *
 * The trade is that output is not verified — someone could patch the result
 * before it is shared. In a live interview you are watching them, so this
 * matters far less than it would for take-home grading. If verified execution
 * is ever needed, that is Judge0 on a real Linux host, and this file is not in
 * the way of it.
 */

/** Pinned: Pyodide's wheel URLs are version-scoped, so a floating version breaks. */
const PYODIDE_VERSION = "0.28.3";
const PYODIDE_CDN = `https://cdn.jsdelivr.net/pyodide/v${PYODIDE_VERSION}/full/`;

export type PythonResult = {
  output: string;
  failed: boolean;
};

/** Minimal shape of what we use, so this file needs no Pyodide type package. */
type PyodideInterface = {
  runPythonAsync: (code: string) => Promise<unknown>;
  setStdout: (opts: { batched: (s: string) => void }) => void;
  setStderr: (opts: { batched: (s: string) => void }) => void;
};

declare global {
  interface Window {
    loadPyodide?: (opts: { indexURL: string }) => Promise<PyodideInterface>;
  }
}

/**
 * The one interpreter for the page.
 *
 * Held as the in-flight promise rather than the resolved value, so two quick
 * clicks on Run share a single ~6MB download instead of starting two.
 */
let pyodidePromise: Promise<PyodideInterface> | null = null;

function loadScript(src: string): Promise<void> {
  return new Promise((resolve, reject) => {
    const existing = document.querySelector<HTMLScriptElement>(`script[src="${src}"]`);
    if (existing) {
      // Already requested by an earlier call that has not resolved yet.
      existing.addEventListener("load", () => resolve());
      existing.addEventListener("error", () => reject(new Error("Could not load Pyodide.")));
      return;
    }
    const script = document.createElement("script");
    script.src = src;
    script.async = true;
    script.onload = () => resolve();
    script.onerror = () => reject(new Error("Could not load Pyodide."));
    document.head.appendChild(script);
  });
}

/**
 * Downloads and starts the interpreter. Roughly 6MB on first call and cached
 * by the browser afterwards, which is why the caller should show progress.
 */
export function loadPython(): Promise<PyodideInterface> {
  if (pyodidePromise) return pyodidePromise;

  pyodidePromise = (async () => {
    await loadScript(`${PYODIDE_CDN}pyodide.js`);
    if (!window.loadPyodide) {
      throw new Error("Pyodide loaded but did not register itself.");
    }
    return window.loadPyodide({ indexURL: PYODIDE_CDN });
  })();

  // A failed load must not be cached, or Run stays broken until reload.
  pyodidePromise.catch(() => {
    pyodidePromise = null;
  });

  return pyodidePromise;
}

/** True once the interpreter is warm, so the UI can stop saying "starting". */
export function isPythonLoaded(): boolean {
  return pyodidePromise !== null;
}

/**
 * Runs source and returns whatever it printed.
 *
 * Errors are returned rather than thrown: a traceback is the *output* of a
 * failed run and belongs in the same panel as a successful one, not in an
 * error banner separate from it.
 */
export async function runPython(source: string): Promise<PythonResult> {
  let pyodide: PyodideInterface;
  try {
    pyodide = await loadPython();
  } catch {
    return {
      output: "Could not load the Python runtime. Check your connection and try again.",
      failed: true,
    };
  }

  const chunks: string[] = [];
  pyodide.setStdout({ batched: (s) => chunks.push(s) });
  pyodide.setStderr({ batched: (s) => chunks.push(s) });

  try {
    await pyodide.runPythonAsync(source);
    const output = chunks.join("\n").trimEnd();
    return { output: output === "" ? "(no output)" : output, failed: false };
  } catch (cause) {
    // Pyodide puts the Python traceback in the error message. Keep whatever
    // was printed before the failure — it is often the half of the story that
    // explains the traceback.
    const printed = chunks.join("\n").trimEnd();
    const trace = cause instanceof Error ? cause.message : String(cause);
    return {
      output: printed ? `${printed}\n${trace}`.trimEnd() : trace.trimEnd(),
      failed: true,
    };
  } finally {
    // Detach our collectors so a later run cannot append to this array.
    pyodide.setStdout({ batched: () => {} });
    pyodide.setStderr({ batched: () => {} });
  }
}
