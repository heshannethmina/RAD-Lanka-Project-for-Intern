"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import Link from "next/link";
import Editor, { type Monaco } from "@monaco-editor/react";
import { applyRemoteText, type CodeEditor } from "@/lib/applyRemoteText";
import {
  useRoomSocket,
  type ActivityEvent,
  type ActivitySummary,
  type ConnectionStatus,
  type Pointer,
  type Role,
} from "@/lib/useRoomSocket";
import ActivityPanel from "./ActivityPanel";
import Countdown from "./Countdown";
import PointerLayer, { usePointerBroadcast } from "./PointerLayer";
import { useActivityReporter } from "@/lib/useActivityReporter";
import { api } from "@/lib/api";
import SplitPane from "./SplitPane";
import { formatRunResult, isFailure, runCode, RunError } from "@/lib/runCode";
import { loadPython, runPython } from "@/lib/runPython";
import { LogoMark } from "./Logo";

const STARTER = `def max_element(values):
    if not values:
        return None

    largest = values[0]
    for value in values[1:]:
        if value > largest:
            largest = value
    return largest


print(max_element([10, 5, 22, 11]))
`;

const LANGUAGES = [
  { id: "go", label: "Go", file: "main.go" },
  { id: "python", label: "Python", file: "main.py" },
  { id: "javascript", label: "JavaScript", file: "main.js" },
];

const STATUS_LABEL: Record<ConnectionStatus, string> = {
  connecting: "Connecting",
  open: "Live",
  closed: "Reconnecting",
};

const STATUS_DOT: Record<ConnectionStatus, string> = {
  connecting: "bg-[#D97706]",
  open: "bg-[#16A34A]",
  closed: "bg-[#DC2626]",
};

/** Muted, distinguishable, and quiet enough to sit in a working tool. */
const PEER_COLORS = ["#005DED", "#0F766E", "#B45309", "#7C3AED", "#475569"];

const MAX_AVATARS = 5;

/**
 * Stacked participant avatars. The count is real — it comes from the hub's
 * presence frames — so this grows and shrinks as people join and leave.
 */
function PeerAvatars({ peers }: { peers: number }) {
  const shown = Math.min(peers, MAX_AVATARS);
  const overflow = peers - shown;

  return (
    <div className="flex items-center">
      <div className="flex -space-x-2">
        {Array.from({ length: shown }, (_, i) => (
          <span
            key={i}
            title={i === 0 ? "You" : `Participant ${i + 1}`}
            className="relative flex h-7 w-7 items-center justify-center rounded-full text-[11px] font-semibold text-white ring-2 ring-white"
            style={{ background: PEER_COLORS[i % PEER_COLORS.length] }}
          >
            {i === 0 ? "You"[0] : String.fromCharCode(65 + i)}
            {i === 0 && (
              <span className="absolute -bottom-px -right-px h-2.5 w-2.5 rounded-full border-2 border-white bg-[#16A34A]" />
            )}
          </span>
        ))}
      </div>
      {overflow > 0 && (
        <span className="ml-2 text-[12px] text-ink-muted">+{overflow}</span>
      )}
    </div>
  );
}

function Spinner() {
  return (
    <svg viewBox="0 0 16 16" className="h-3.5 w-3.5 animate-spin" aria-hidden="true">
      <circle cx="8" cy="8" r="6" fill="none" stroke="rgba(255,255,255,0.35)" strokeWidth="2.5" />
      <path d="M8 2 a6 6 0 0 1 6 6" fill="none" stroke="#fff" strokeWidth="2.5" strokeLinecap="round" />
    </svg>
  );
}

/**
 * What the candidate sees, always, for the whole interview.
 *
 * The disclosure is the feature. A candidate who knows the interviewer can see
 * tab switches does not switch; silent monitoring catches a few people where
 * visible monitoring prevents it, which is the actual goal. It also gives them
 * the standing to explain a false positive — a notification stealing focus is
 * not the same as leaving.
 */
function MonitoringNotice() {
  return (
    <div className="flex shrink-0 items-center gap-2 border-b border-line bg-bg-subtle px-4 py-2 text-[12px] text-ink-muted">
      <svg viewBox="0 0 16 16" className="h-3.5 w-3.5 shrink-0" aria-hidden="true">
        <circle cx="8" cy="8" r="6.25" fill="none" stroke="currentColor" strokeWidth="1.4" />
        <path d="M8 7.25v4" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
        <circle cx="8" cy="4.9" r="0.85" fill="currentColor" />
      </svg>
      <span>
        Your interviewer can see when you leave this tab, and can see any code
        you paste in. Nothing else about your device or your typing is
        recorded.
      </span>
    </div>
  );
}

/**
 * Shown once the interview's time is up.
 *
 * A banner rather than a modal on purpose: both people still want to read the
 * code and copy it out, and a dialog sitting over it would be in the way at
 * exactly the wrong moment. The editor goes read-only underneath, which is
 * what actually stops the interview.
 */
function EndedNotice() {
  return (
    <div className="flex shrink-0 items-center gap-2 border-b border-[#F1AEAE] bg-[#FDF3F3] px-4 py-2 text-[12px] text-[#B42318]">
      <svg viewBox="0 0 16 16" className="h-3.5 w-3.5 shrink-0" aria-hidden="true">
        <circle cx="8" cy="8" r="6.25" fill="none" stroke="currentColor" strokeWidth="1.4" />
        <path d="M8 4.5V8l2.5 1.5" fill="none" stroke="currentColor" strokeWidth="1.4" strokeLinecap="round" />
      </svg>
      <span>
        This interview has ended. The code is still here to read and copy, but
        it can no longer be edited or run.
      </span>
    </div>
  );
}

/** Browser-side Python. Never throws: a traceback is output, not an error. */
async function runPythonLocally(source: string) {
  const result = await runPython(source);
  return { text: result.output, failed: result.failed };
}

/** Everything else, through the backend to Judge0. */
async function runViaJudge0(language: string, source: string, roomId: string, token: string) {
  try {
    const result = await runCode(language, source, roomId, token);
    return { text: formatRunResult(result), failed: isFailure(result) };
  } catch (err) {
    return {
      text: err instanceof RunError ? err.message : "Run failed.",
      failed: true,
    };
  }
}

export default function RoomEditor({
  roomId,
  token,
  title,
  initialLanguage = "python",
}: {
  roomId: string;
  /** Session token or invite token; RoomGate has already checked it works. */
  token: string;
  /** What the interviewer called this session, for the header. */
  title?: string;
  /** The room's own language, so both sides start on the same one. */
  initialLanguage?: string;
}) {
  const [language, setLanguage] = useState(initialLanguage);
  const [output, setOutput] = useState<string | null>(null);
  // Until the snapshot arrives we do not know which side of the interview this
  // is. Default to candidate — the read-only view — so a slow connection never
  // briefly offers an editable question to someone who may not have one.
  const [role, setRole] = useState<Role>("candidate");
  const [prompt, setPrompt] = useState("");
  const [activity, setActivity] = useState<ActivitySummary | null>(null);
  const [events, setEvents] = useState<ActivityEvent[]>([]);
  const [pointer, setPointer] = useState<Pointer | null>(null);
  // Off by default: a cursor drifting across the screen while somebody is
  // trying to think is a real distraction. It earns its place when two people
  // are looking at the same code, not the rest of the time.
  const [pointersOn, setPointersOn] = useState(false);
  // Which of the two bottom-right tabs is showing.
  const [sideTab, setSideTab] = useState<"prompt" | "activity">("prompt");
  // Absolute deadline in ms, or null when the plan does not meter this room.
  const [endsAt, setEndsAt] = useState<number | null>(null);
  const [ended, setEnded] = useState(false);

  // onDidPaste is registered once when Monaco mounts, so it would otherwise
  // capture the role and sender from that first render — before the snapshot
  // has said which side of the interview this is.
  const roleRef = useRef<Role>("candidate");
  const sendActivityRef = useRef<
    (
      kind: "away" | "back" | "paste",
      extra?: { ms?: number; lines?: number; text?: string },
    ) => void
  >(() => {});
  const [failed, setFailed] = useState(false);
  const [running, setRunning] = useState(false);

  const editorRef = useRef<CodeEditor | null>(null);
  const monacoRef = useRef<Monaco | null>(null);
  // Set while a remote edit is being written into the model, so the onChange
  // it triggers is not bounced straight back to the server.
  const applyingRemote = useRef(false);
  // Monaco may mount after the socket has already delivered the snapshot.
  const pendingText = useRef<string | null>(null);

  const active = LANGUAGES.find((l) => l.id === language) ?? LANGUAGES[1];

  const write = useCallback((text: string) => {
    const editor = editorRef.current;
    const monaco = monacoRef.current;
    if (!editor || !monaco) {
      pendingText.current = text;
      return;
    }
    applyingRemote.current = true;
    try {
      applyRemoteText(editor, monaco, text);
    } finally {
      applyingRemote.current = false;
    }
  }, []);

  const {
    status,
    peers,
    sendEdit,
    sendResult,
    sendPrompt,
    sendActivity,
    sendPointer,
  } = useRoomSocket(roomId, token, {
    onSnapshot: (text, send) => {
      if (text !== "") {
        write(text);
        return;
      }
      // An empty document means we are the first one here (or the room was
      // torn down while we were away), so seed it rather than blanking our
      // own editor.
      send(editorRef.current?.getValue() ?? STARTER);
    },
    onJoin: (roomPrompt, joinedAs, tally, log, deadline, alreadyEnded) => {
      setRole(joinedAs);
      setPrompt(roomPrompt);
      setActivity(tally);
      setEvents(log);
      setEndsAt(deadline);
      setEnded(alreadyEnded);
    },
    onEnded: () => setEnded(true),
    onActivity: (summary, event) => {
      setActivity(summary);
      // Bounded here too. The server caps what it keeps; this keeps a long
      // interview from growing the tab's memory without limit.
      setEvents((prev) => [...prev, event].slice(-100));
    },
    onPointer: setPointer,
    onPrompt: setPrompt,
    onEdit: write,
    // Someone else pressed Run. Show what they got, so an interviewer can see
    // their candidate's output instead of only their own.
    onResult: (text, didFail) => {
      setOutput(text);
      setFailed(didFail);
      // Their run, not ours — make sure our own button is not left spinning.
      setRunning(false);
    },
  });

  // Start fetching the Python runtime as soon as Python is selected, rather
  // than on the first click. It is a ~6MB download, and paying for it while
  // someone is still reading the question is invisible; paying for it after
  // they press Run is a long, unexplained wait.
  useEffect(() => {
    if (language !== "python") return;
    loadPython();
  }, [language]);

  // Saving the prompt is debounced, but relaying it is not: the candidate
  // should see the question appear as it is typed, while Postgres only needs
  // the version that is left after the interviewer stops.
  const saveTimer = useRef<ReturnType<typeof setTimeout> | null>(null);

  const handlePromptChange = useCallback(
    (next: string) => {
      setPrompt(next);
      sendPrompt(next);

      if (saveTimer.current) clearTimeout(saveTimer.current);
      saveTimer.current = setTimeout(() => {
        // Failure is deliberately quiet. The candidate already has the text
        // over the socket, so a failed save costs a reload, not the interview;
        // an error banner over a question someone is mid-sentence in would be
        // worse than the problem.
        void api.updatePrompt(roomId, next).catch(() => {});
      }, 800);
    },
    [roomId, sendPrompt],
  );

  // Flush nothing on unmount, but do stop the timer: firing after the room is
  // gone would be a write nobody is waiting for.
  useEffect(() => {
    return () => {
      if (saveTimer.current) clearTimeout(saveTimer.current);
    };
  }, []);

  // Only the candidate is observed, and only while connected. The server
  // drops activity frames from an interviewer regardless, so this is about not
  // sending pointless traffic rather than about enforcement.
  useActivityReporter(role === "candidate" && status === "open", sendActivity);

  // Both sides may point; it is a gesture, not a permission.
  usePointerBroadcast(pointersOn && status === "open", sendPointer);

  useEffect(() => {
    roleRef.current = role;
    sendActivityRef.current = sendActivity;
  }, [role, sendActivity]);

  // Warn before the tab is closed or reloaded during a live interview.
  //
  // Worth the friction because the document is not persisted: it lives in the
  // room's hub goroutine and dies when the last person disconnects, so a
  // mistaken reload can lose a candidate's work with no way back. Only armed
  // while the socket is actually open, so it never fires on a page that has
  // nothing at stake.
  //
  // Browsers ignore any custom message and show their own wording; the
  // preventDefault is what arms the dialog at all.
  useEffect(() => {
    if (status !== "open") return;
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      e.preventDefault();
    };
    window.addEventListener("beforeunload", onBeforeUnload);
    return () => window.removeEventListener("beforeunload", onBeforeUnload);
  }, [status]);

  const handleBeforeMount = useCallback((monaco: Monaco) => {
    // The same GitHub-light token palette the marketing editor mockup uses,
    // so the product and the page it is advertised on agree.
    monaco.editor.defineTheme("syncr-light", {
      base: "vs",
      inherit: true,
      rules: [
        { token: "keyword", foreground: "CF222E" },
        { token: "string", foreground: "0A3069" },
        { token: "number", foreground: "0550AE" },
        { token: "comment", foreground: "6E7781", fontStyle: "italic" },
        { token: "type", foreground: "8250DF" },
        { token: "function", foreground: "8250DF" },
      ],
      colors: {
        "editor.background": "#FFFFFF",
        "editorGutter.background": "#FFFFFF",
        "editor.lineHighlightBackground": "#F6F8FA",
        "editor.selectionBackground": "#DDEAFF",
        "editorLineNumber.foreground": "#AFB8C1",
        "editorLineNumber.activeForeground": "#6E7781",
        "editorIndentGuide.background1": "#EDEFF2",
        "minimap.background": "#FFFFFF",
      },
    });
  }, []);

  const handleMount = useCallback((editor: CodeEditor, monaco: Monaco) => {
    editorRef.current = editor;
    monacoRef.current = monaco;

    // A pasted block is the highest-signal thing here. People switch tabs to
    // read documentation that many interviewers explicitly allow; forty lines
    // arriving at once is a different kind of event, and worth surfacing.
    //
    // Reported by the candidate's client only. Guarded with optional chaining
    // because onDidPaste is not part of the editor interface this project
    // types against, and a Monaco upgrade renaming it must not crash the room.
    type PasteRange = {
      startLineNumber: number;
      startColumn: number;
      endLineNumber: number;
      endColumn: number;
    };
    const withPaste = editor as unknown as {
      onDidPaste?: (cb: (e: { range: PasteRange }) => void) => void;
    };
    withPaste.onDidPaste?.((e) => {
      if (roleRef.current !== "candidate") return;
      const lines = e.range.endLineNumber - e.range.startLineNumber + 1;
      // Read the pasted span back out of the model rather than the clipboard:
      // no permission prompt, and it captures what actually landed in the
      // document, which is what the interviewer is looking at anyway. The
      // server truncates; this bound just keeps the frame small.
      let text = "";
      try {
        text = editor.getModel()?.getValueInRange(e.range).slice(0, 4000) ?? "";
      } catch {
        // A model swapped out mid-paste is not worth failing the report over.
      }
      sendActivityRef.current("paste", { lines, text });
    });
    if (pendingText.current !== null) {
      applyingRemote.current = true;
      try {
        applyRemoteText(editor, monaco, pendingText.current);
      } finally {
        applyingRemote.current = false;
        pendingText.current = null;
      }
    }
  }, []);

  const handleChange = useCallback(
    (value: string | undefined) => {
      if (applyingRemote.current) return;
      sendEdit(value ?? "");
    },
    [sendEdit],
  );

  async function handleRun() {
    const source = editorRef.current?.getValue() ?? "";
    if (!source.trim()) {
      setFailed(true);
      setOutput("Nothing to run.");
      return;
    }

    setRunning(true);
    setOutput(null);
    setFailed(false);

    try {
      // Python runs in this browser, through Pyodide. Every other language
      // goes to Judge0 via the backend, which needs a Linux host with cgroup
      // v1 and so is not available on the current deployment.
      //
      // The split is deliberate rather than a stopgap: browser execution
      // cannot be verified, but it also cannot reach anything of ours, and an
      // interview is a stranger running code. See lib/runPython.ts.
      const { text, failed: didFail } =
        language === "python"
          ? await runPythonLocally(source)
          : await runViaJudge0(language, source, roomId, token);

      setOutput(text);
      setFailed(didFail);
      // Share it, so the other side of the interview sees what happened.
      // The hub relays this to everyone but us; we already have it.
      sendResult(text, didFail);
    } finally {
      setRunning(false);
    }
  }

  return (
    <div className="flex h-screen flex-col bg-white">
      {/* ---------- top bar ---------- */}
      <header className="flex h-14 shrink-0 items-center justify-between gap-4 border-b border-line px-4">
        <div className="flex min-w-0 items-center gap-3">
          {/*
            Only the interviewer gets a way out. A candidate has no dashboard
            to return to, and a stray click on a logo mid-interview would drop
            them out of the room — so for them the mark is decoration, not a
            link. Leaving is what the browser's own back button is for.
          */}
          {role === "interviewer" ? (
            <Link
              href="/dashboard"
              className="group flex shrink-0 items-center gap-2 rounded-md px-1.5 py-1 text-ink-muted transition-colors hover:bg-bg-subtle hover:text-ink"
              title="Back to your interviews"
            >
              <svg viewBox="0 0 16 16" className="h-4 w-4" aria-hidden="true">
                <path
                  d="M10 3.5L5.5 8l4.5 4.5"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.6"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
              <LogoMark className="h-[22px] w-auto text-accent" />
              <span className="hidden text-[13px] font-medium sm:inline">
                Interviews
              </span>
            </Link>
          ) : (
            <span className="shrink-0 text-accent" aria-label="SyncR">
              <LogoMark className="h-[22px] w-auto" />
            </span>
          )}

          <span className="h-5 w-px shrink-0 bg-line" />

          <div className="flex min-w-0 flex-col leading-tight">
            <span className="truncate text-[13px] font-medium text-ink">
              {title?.trim() || "Untitled interview"}
            </span>
            <span className="truncate font-mono text-[11px] text-ink-muted">
              {roomId}
            </span>
          </div>
        </div>

        <div className="flex items-center gap-4">
          <span
            className="hidden items-center gap-1.5 sm:flex"
            title={`${STATUS_LABEL[status]} · ${peers} in room`}
          >
            <span className={`h-1.5 w-1.5 rounded-full ${STATUS_DOT[status]}`} />
            <span className="text-[12px] text-ink-muted">
              {STATUS_LABEL[status]}
            </span>
          </span>

          <Countdown endsAt={ended ? null : endsAt} />

          <PeerAvatars peers={peers} />

          {/* Only the state that needs to be seen without looking: away now.
              The detail lives in the Activity tab, where there is room for it. */}
          {role === "interviewer" && activity?.away && (
            <span className="hidden items-center gap-1.5 rounded-full border border-[#F5C4A0] bg-[#FFF7ED] px-2.5 py-1 text-[11px] font-medium text-[#9A5B1E] sm:inline-flex">
              <span className="h-1.5 w-1.5 rounded-full bg-[#D97706]" />
              Away
            </span>
          )}

          <button
            type="button"
            onClick={() => setPointersOn((on) => !on)}
            aria-pressed={pointersOn}
            title={
              pointersOn
                ? "Stop sharing mouse pointers"
                : "Share mouse pointers, so you can point at code"
            }
            className={[
              "hidden h-9 items-center gap-1.5 rounded-lg border px-2.5 text-[13px] transition-colors sm:inline-flex",
              pointersOn
                ? "border-accent bg-accent-wash text-accent"
                : "border-line bg-white text-ink-muted hover:border-line-strong hover:text-ink",
            ].join(" ")}
          >
            <svg viewBox="0 0 16 16" className="h-3.5 w-3.5" aria-hidden="true">
              <path
                d="M3 2l9 5.5-4 1-1.8 4z"
                fill="currentColor"
                stroke="none"
              />
            </svg>
            Pointer
          </button>

          <span className="h-5 w-px bg-line" />

          <div className="relative">
            <select
              value={language}
              onChange={(e) => setLanguage(e.target.value)}
              aria-label="Language"
              className="h-9 cursor-pointer appearance-none rounded-lg border border-line bg-white py-0 pl-3 pr-9 text-[13px] text-ink outline-none transition-colors hover:border-line-strong"
            >
              {LANGUAGES.map((l) => (
                <option key={l.id} value={l.id}>
                  {l.label}
                </option>
              ))}
            </select>
            <svg
              viewBox="0 0 16 16"
              className="pointer-events-none absolute right-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-ink-muted"
              aria-hidden="true"
            >
              <path
                d="M4 6l4 4 4-4"
                fill="none"
                stroke="currentColor"
                strokeWidth="1.6"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </div>

          <button
            onClick={handleRun}
            disabled={running || ended}
            className="btn-primary h-9 gap-2 px-4 text-[13px] disabled:cursor-not-allowed disabled:opacity-70"
          >
            {running ? (
              <>
                <Spinner />
                Running
              </>
            ) : (
              <>
                <svg viewBox="0 0 16 16" className="h-3.5 w-3.5" aria-hidden="true">
                  <path d="M5 3.5v9l7-4.5z" fill="currentColor" />
                </svg>
                Run
              </>
            )}
          </button>
        </div>
      </header>

      <PointerLayer pointer={pointer} enabled={pointersOn} />

      {ended && <EndedNotice />}

      {role === "candidate" && !ended && <MonitoringNotice />}

      {/* ---------- three resizable panes ---------- */}
      {/*
        Editor on the left; output above prompt on the right. Each divider
        remembers where it was put, per browser, so someone who prefers a tall
        prompt panel does not re-drag it every interview.

        min-h-0 runs the whole way down. A flex child defaults to min-height
        auto, which lets long output push a pane past its share and makes the
        divider look stuck; without it none of these panels would scroll.
      */}
      <SplitPane
        direction="horizontal"
        className="flex-1"
        defaultSize={62}
        min={30}
        max={80}
        storageKey="syncr.split.main"
        first={
          <div className="flex h-full min-h-0 min-w-0 flex-col">
            <div className="flex h-9 shrink-0 items-center border-b border-line px-3">
              <span className="rounded-md border border-line bg-bg-subtle px-2.5 py-1 font-mono text-[11px] text-ink">
                {active.file}
              </span>
            </div>

            <div className="min-h-0 flex-1">
              <Editor
                height="100%"
                language={language}
                defaultValue={STARTER}
                theme="syncr-light"
                beforeMount={handleBeforeMount}
                onMount={handleMount}
                onChange={handleChange}
                options={{
                  fontFamily: "var(--font-mono)",
                  fontSize: 13.5,
                  minimap: { enabled: true, size: "fit", showSlider: "always" },
                  padding: { top: 16 },
                  readOnly: ended,
                  scrollBeyondLastLine: false,
                  renderLineHighlight: "line",
                  smoothScrolling: true,
                }}
              />
            </div>
          </div>
        }
        second={
          <SplitPane
            direction="vertical"
            className="h-full border-l border-line"
            defaultSize={45}
            min={15}
            max={85}
            storageKey="syncr.split.side"
            first={
              <section className="flex h-full min-h-0 flex-col p-4">
                <h2 className="shrink-0 text-[13px] font-semibold text-ink">Output</h2>
                <div className="mt-3 min-h-0 flex-1 overflow-auto rounded-lg border border-line bg-bg-subtle p-3">
                  {output ? (
                    <pre
                      className={`whitespace-pre-wrap font-mono text-[12px] leading-relaxed ${
                        failed ? "text-[#B42318]" : "text-ink"
                      }`}
                    >
                      {output}
                    </pre>
                  ) : (
                    <p className="font-mono text-[12px] text-ink-muted">
                      Run the code to see output here.
                    </p>
                  )}
                </div>
              </section>
            }
            second={
              <section className="flex h-full min-h-0 flex-col p-4">
                {/*
                  Tabs rather than a fourth pane: three horizontal dividers in
                  a laptop-height window leaves each panel too short to read.
                  The candidate has one tab, so no tab strip is drawn for them.
                */}
                <div className="flex shrink-0 items-center justify-between gap-3">
                  {role === "interviewer" ? (
                    <div className="flex items-center gap-1" role="tablist">
                      {(["prompt", "activity"] as const).map((tab) => (
                        <button
                          key={tab}
                          role="tab"
                          aria-selected={sideTab === tab}
                          onClick={() => setSideTab(tab)}
                          className={[
                            "rounded-md px-2 py-1 text-[13px] font-semibold transition-colors",
                            sideTab === tab
                              ? "bg-bg-sunken text-ink"
                              : "text-ink-muted hover:text-ink",
                          ].join(" ")}
                        >
                          {tab === "prompt" ? "Prompt" : "Activity"}
                          {tab === "activity" &&
                            activity &&
                            activity.away_count + activity.paste_count > 0 && (
                              <span className="ml-1.5 text-[11px] font-normal text-ink-muted">
                                {activity.away_count + activity.paste_count}
                              </span>
                            )}
                        </button>
                      ))}
                    </div>
                  ) : (
                    <h2 className="text-[13px] font-semibold text-ink">Prompt</h2>
                  )}

                  <span className="text-[11px] text-ink-muted">
                    {role !== "interviewer"
                      ? "Set by your interviewer"
                      : sideTab === "prompt"
                        ? "The candidate sees changes live"
                        : null}
                  </span>
                </div>

                {role === "interviewer" && sideTab === "activity" ? (
                  <div className="mt-3 min-h-0 flex-1 overflow-hidden rounded-lg border border-line bg-bg-subtle p-3">
                    <ActivityPanel summary={activity} events={events} />
                  </div>
                ) : (
                <>

                {/*
                  The interviewer gets a textarea, the candidate gets text.
                  This is a convenience, not the control: the server ignores a
                  prompt frame from a candidate regardless of what the UI does.
                */}
                {role === "interviewer" ? (
                  <textarea
                    value={prompt}
                    onChange={(e) => handlePromptChange(e.target.value)}
                    placeholder="Type the question here. The candidate sees it as you type."
                    spellCheck={false}
                    className="mt-3 min-h-0 w-full flex-1 resize-none overflow-auto rounded-lg border border-line bg-bg-subtle p-4 text-[13px] leading-relaxed text-ink-body outline-none transition-colors focus:border-accent focus:bg-white"
                  />
                ) : (
                  <div className="mt-3 min-h-0 flex-1 overflow-auto rounded-lg border border-line bg-bg-subtle p-4">
                    {prompt.trim() ? (
                      // whitespace-pre-wrap so an interviewer's line breaks and
                      // indentation survive, without needing a markdown parser.
                      <p className="whitespace-pre-wrap text-[13px] leading-relaxed text-ink-body">
                        {prompt}
                      </p>
                    ) : (
                      <p className="text-[13px] leading-relaxed text-ink-muted">
                        Your interviewer has not set a question yet.
                      </p>
                    )}
                  </div>
                )}
                </>
                )}
              </section>
            }
          />
        }
      />
    </div>
  );
}
