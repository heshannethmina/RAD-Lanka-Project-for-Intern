"use client";

import { useCallback, useRef, useState } from "react";
import Link from "next/link";
import Editor, { type Monaco } from "@monaco-editor/react";
import { applyRemoteText, type CodeEditor } from "@/lib/applyRemoteText";
import { useRoomSocket, type ConnectionStatus } from "@/lib/useRoomSocket";
import { formatRunResult, isFailure, runCode, RunError } from "@/lib/runCode";
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

export default function RoomEditor({ roomId }: { roomId: string }) {
  const [language, setLanguage] = useState("python");
  const [output, setOutput] = useState<string | null>(null);
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

  const { status, peers, sendEdit } = useRoomSocket(roomId, {
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
    onEdit: write,
  });

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
      const result = await runCode(language, source);
      setOutput(formatRunResult(result));
      setFailed(isFailure(result));
    } catch (err) {
      setFailed(true);
      setOutput(err instanceof RunError ? err.message : "Run failed.");
    } finally {
      setRunning(false);
    }
  }

  return (
    <div className="flex h-screen flex-col bg-white">
      {/* ---------- top bar ---------- */}
      <header className="flex h-14 shrink-0 items-center justify-between gap-4 border-b border-line px-4">
        <div className="flex min-w-0 items-center gap-3">
          <Link href="/" className="shrink-0 rounded-sm text-accent">
            <LogoMark className="h-[22px] w-auto" />
          </Link>
          <span className="h-5 w-px shrink-0 bg-line" />
          <span className="truncate font-mono text-[13px] text-ink-muted">
            room / <span className="font-medium text-ink">{roomId}</span>
          </span>
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

          <PeerAvatars peers={peers} />

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
            disabled={running}
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

      {/* ---------- editor + sidebar ---------- */}
      <div className="flex min-h-0 flex-1 flex-col lg:flex-row">
        <div className="flex min-h-0 min-w-0 flex-1 flex-col">
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
                scrollBeyondLastLine: false,
                renderLineHighlight: "line",
                smoothScrolling: true,
              }}
            />
          </div>
        </div>

        {/* Sidebar */}
        <aside className="flex w-full shrink-0 flex-col border-t border-line lg:w-[340px] lg:border-l lg:border-t-0">
          <section className="flex min-h-[200px] flex-col border-b border-line p-4">
            <h2 className="text-[13px] font-semibold text-ink">Output</h2>
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

          <section className="flex min-h-0 flex-1 flex-col p-4">
            <h2 className="text-[13px] font-semibold text-ink">Prompt</h2>
            <div className="mt-3 min-h-0 flex-1 overflow-auto rounded-lg border border-line bg-bg-subtle p-4">
              <h3 className="text-[13px] font-semibold text-ink">
                Largest value in a list
              </h3>
              <p className="mt-3 text-[13px] leading-relaxed text-ink-body">
                Given a list of integers, write a function that returns the
                largest value.
              </p>
              <p className="mt-3 text-[13px] leading-relaxed text-ink-body">
                For the input <code className="font-mono">[10, 5, 22, 11]</code>{" "}
                it should return <code className="font-mono">22</code>.
              </p>
              <p className="mt-3 text-[13px] leading-relaxed text-ink-body">
                Handle the edge cases too — an empty list, and a list where the
                largest value appears more than once.
              </p>
            </div>
          </section>
        </aside>
      </div>
    </div>
  );
}
