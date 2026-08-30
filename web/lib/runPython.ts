/**
 * Runs Python in the candidate's own browser, in a Web Worker.
 *
 * Why the browser and not a server: an interview is an invitation for a
 * stranger to run code. Executing that on our backend means the process can
 * read our environment, and `os.environ["DATABASE_URL"]` would hand over every
 * account in the database. Judge0 exists to stop exactly that, but it needs
 * cgroup v1 and a privileged container, which no managed host grants. Running
 * in the viewer's tab sidesteps the problem rather than defending against it:
 * there is nothing of ours nearby to compromise.
 *
 * Why a worker and not the main thread: Python does not yield. On the main
 * thread `for i in range(10000000)` freezes the whole tab, and a Stop button
 * cannot help because its click handler never runs. In a worker the page stays
 * responsive and a timeout can call terminate(), which is an unconditional
 * kill. That is the only thing that actually stops a runaway loop.
 *
 * The trade is that output is not verified — someone could patch the result
 * before it is shared. In a live interview you are watching them, so this
 * matters far less than it would for take-home grading.
 */

/** Wall-clock budget for one run. Beyond this the worker is killed outright. */
export const RUN_TIMEOUT_MS = 10_000;

export type PythonResult = {
  output: string;
  failed: boolean;
  /** True when the run was killed for exceeding RUN_TIMEOUT_MS. */
  timedOut: boolean;
};

type WorkerDone = { type: "done"; output: string; failed: boolean; nonce?: string };
type WorkerReady = { type: "ready" };
type WorkerError = { type: "error"; message: string };
type WorkerMessage = WorkerDone | WorkerReady | WorkerError;

/**
 * The live worker.
 *
 * Held across runs so the ~6MB interpreter is loaded once. Set to null after a
 * timeout, because a terminated worker cannot be resumed — its interpreter
 * died with the thread, so the next run has to build a fresh one.
 */
let worker: Worker | null = null;
/** Guards against two runs sharing one worker, which would interleave output. */
let busy = false;

function ensureWorker(): Worker {
  if (!worker) {
    worker = new Worker("/python-worker.js");
  }
  return worker;
}

/** Discards the worker so the next run starts from a clean interpreter. */
function killWorker() {
  worker?.terminate();
  worker = null;
}

/**
 * Starts downloading the interpreter without running anything.
 *
 * Called when Python is selected rather than on the first click: it is a ~6MB
 * download, and paying for it while someone reads the question is invisible,
 * where paying for it after they press Run is a long unexplained wait.
 */
export function loadPython(): void {
  if (typeof window === "undefined") return;
  try {
    ensureWorker().postMessage({ type: "warm" });
  } catch {
    // A browser without workers falls back to failing at Run time, with a
    // message the user can act on. Nothing useful to say about a prefetch.
  }
}

/**
 * Runs source and returns whatever it printed.
 *
 * Never throws. A traceback is the *output* of a failed run and belongs in the
 * same panel as a successful one, not in an error banner beside it.
 */
export function runPython(source: string): Promise<PythonResult> {
  if (typeof window === "undefined" || typeof Worker === "undefined") {
    return Promise.resolve({
      output: "This browser cannot run Python here.",
      failed: true,
      timedOut: false,
    });
  }

  if (busy) {
    return Promise.resolve({
      output: "A run is already in progress.",
      failed: true,
      timedOut: false,
    });
  }

  let w: Worker;
  try {
    w = ensureWorker();
  } catch {
    return Promise.resolve({
      output: "Could not start the Python runtime.",
      failed: true,
      timedOut: false,
    });
  }

  busy = true;

  return new Promise<PythonResult>((resolve) => {
	    const nonce = crypto.randomUUID();
    // settle guarantees exactly one resolution and one cleanup, however the
    // run ends — message, error, or timeout.
    let settled = false;
    const settle = (result: PythonResult) => {
      if (settled) return;
      settled = true;
      busy = false;
      window.clearTimeout(timer);
      w.removeEventListener("message", onMessage);
      w.removeEventListener("error", onError);
      resolve(result);
    };

    const onMessage = (event: MessageEvent<WorkerMessage>) => {
      const msg = event.data;
      if (msg.type === "ready") return; // A prefetch finishing; not our run.
	      if (msg.type === "done" && msg.nonce !== nonce) return;
      if (msg.type === "error") {
        settle({ output: msg.message, failed: true, timedOut: false });
        return;
      }
      settle({ output: msg.output, failed: msg.failed, timedOut: false });
    };

    const onError = () => {
      // The worker itself broke rather than the Python in it. It is not
      // trustworthy afterwards, so discard it.
      killWorker();
      settle({
        output: "The Python runtime stopped unexpectedly.",
        failed: true,
        timedOut: false,
      });
    };

    const timer = window.setTimeout(() => {
      // The only way to stop a tight loop. There is no cooperative signal
      // Python would notice, so the thread goes.
      killWorker();
      settle({
        output:
          `Timed out after ${RUN_TIMEOUT_MS / 1000} seconds and was stopped.\n` +
          `Check for a loop that never ends, or input larger than it looks.`,
        failed: true,
        timedOut: true,
      });
    }, RUN_TIMEOUT_MS);

    w.addEventListener("message", onMessage);
    w.addEventListener("error", onError);
    w.postMessage({ type: "run", source, nonce });
  });
}
