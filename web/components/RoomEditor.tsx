"use client";

import { useCallback, useRef, useState } from "react";
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
  { id: "python", label: "Python", emoji: "\u{1F40D}", file: "main.py" },
  { id: "go", label: "Go", emoji: "\u{1F439}", file: "main.go" },
  { id: "javascript", label: "JavaScript", emoji: "\u{1F7E8}", file: "main.js" },
];

const STATUS_LABEL: Record<ConnectionStatus, string> = {
  connecting: "connecting",
  open: "live",
  closed: "reconnecting",
};

const STATUS_DOT: Record<ConnectionStatus, string> = {
  connecting: "bg-[#F0A868]",
  open: "bg-[#34D399]",
  closed: "bg-[#F87171]",
};

/** Distinct colours so overlapping avatars stay tellable apart. */
const PEER_COLORS = [
  "linear-gradient(135deg,#5B8CFF,#8B5CF6)",
  "linear-gradient(135deg,#F472B6,#8B5CF6)",
  "linear-gradient(135deg,#34D399,#3B82F6)",
  "linear-gradient(135deg,#F0A868,#F472B6)",
  "linear-gradient(135deg,#22D3EE,#5B8CFF)",
];

const MAX_AVATARS = 5;

/**
 * Overlapping presence avatars. The count is real — it comes from the hub's
 * presence frames — so this grows and shrinks as people join and leave.
 */
function PeerAvatars({ peers }: { peers: number }) {
  const shown = Math.min(peers, MAX_AVATARS);
  const overflow = peers - shown;

  return (
    <div className="flex items-center">
      <div className="flex -space-x-2.5">
        {Array.from({ length: shown }, (_, i) => (
          <span
            key={i}
            title={i === 0 ? "You" : `Participant ${i + 1}`}
            className={`relative flex h-7 w-7 items-center justify-center rounded-full text-[10px] font-bold text-white ring-2 ${
              i === 0 ? "ring-[#34D399]" : "ring-[#0B1029]"
            }`}
            style={{ background: PEER_COLORS[i % PEER_COLORS.length] }}
          >
            {i === 0 ? "Y" : String.fromCharCode(65 + i)}
            {i === 0 && (
              <span className="absolute -bottom-0.5 -right-0.5 h-2.5 w-2.5 rounded-full border-2 border-[#0B1029] bg-[#34D399]" />
            )}
          </span>
        ))}
      </div>
      {overflow > 0 && (
        <span className="ml-2 font-mono text-[10px] text-ink-faint">
          +{overflow}
        </span>
      )}
    </div>
  );
}

function Spinner() {
  return (
    <svg
      viewBox="0 0 16 16"
      className="h-3.5 w-3.5 animate-spin"
      aria-hidden="true"
    >
      <circle
        cx="8"
        cy="8"
        r="6"
        fill="none"
        stroke="rgba(255,255,255,0.35)"
        strokeWidth="2.5"
      />
      <path
        d="M8 2 a6 6 0 0 1 6 6"
        fill="none"
        stroke="#fff"
        strokeWidth="2.5"
        strokeLinecap="round"
      />
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

  const active = LANGUAGES.find((l) => l.id === language) ?? LANGUAGES[0];

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
    // Transparent background so the glass panel shows through the editor,
    // which is what makes the room read as one surface rather than a widget
    // pasted onto a card.
    monaco.editor.defineTheme("interview-dark", {
      base: "vs-dark",
      inherit: true,
      rules: [
        { token: "keyword", foreground: "A78BFA" },
        { token: "string", foreground: "7DD3A8" },
        { token: "number", foreground: "F0A868" },
        { token: "comment", foreground: "5B6390", fontStyle: "italic" },
        { token: "type", foreground: "60A5FA" },
        { token: "function", foreground: "60A5FA" },
      ],
      colors: {
        "editor.background": "#00000000",
        "editorGutter.background": "#00000000",
        "minimap.background": "#00000000",
        "editor.lineHighlightBackground": "#FFFFFF0A",
        "editorLineNumber.foreground": "#4C5480",
        "editorLineNumber.activeForeground": "#9AA2C8",
        "editorIndentGuide.background1": "#FFFFFF12",
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
      setOutput("// Nothing to run.");
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
      setOutput(
        err instanceof RunError ? `// ${err.message}` : "// Run failed.",
      );
    } finally {
      setRunning(false);
    }
  }

  return (
    <div className="flex h-screen flex-col gap-4 p-4 sm:p-5">
      {/* ---------- top bar ---------- */}
      <div className="relative shrink-0">
        <div className="glass flex items-center justify-between rounded-2xl px-4 py-3">
          <div className="flex items-center gap-4">
            <span className="flex items-center gap-2.5">
              <LogoMark className="h-6 w-6" />
              <span className="hidden font-display text-[12px] font-semibold leading-[1.15] tracking-tight text-ink sm:block">
                Interview
                <br />
                Platform
              </span>
            </span>
            <span className="h-7 w-px bg-white/10" />
            <span className="font-mono text-[13px] text-ink-dim">
              room / <span className="font-semibold text-ink">{roomId}</span>
            </span>
          </div>

          <div className="flex items-center gap-4">
            <span
              className="flex items-center gap-1.5"
              title={`${STATUS_LABEL[status]} · ${peers} in room`}
            >
              <span
                className={`h-1.5 w-1.5 rounded-full ${STATUS_DOT[status]}`}
              />
              <span className="hidden font-mono text-[10px] text-ink-faint lg:block">
                {STATUS_LABEL[status]}
              </span>
            </span>

            <PeerAvatars peers={peers} />

            <span className="h-7 w-px bg-white/10" />

            <div className="relative">
              <select
                value={language}
                onChange={(e) => setLanguage(e.target.value)}
                aria-label="Language"
                className="glass appearance-none rounded-xl py-2 pl-3 pr-9 text-[13px] text-ink outline-none"
              >
                {LANGUAGES.map((l) => (
                  <option key={l.id} value={l.id} className="bg-[#0E1330]">
                    {l.emoji} {l.label}
                  </option>
                ))}
              </select>
              <svg
                viewBox="0 0 16 16"
                className="pointer-events-none absolute right-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-ink-faint"
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
              className="btn-accent flex items-center gap-2 rounded-xl px-4 py-2 text-[13px] font-semibold disabled:opacity-80"
            >
              {running ? (
                <>
                  <Spinner />
                  Running...
                </>
              ) : (
                <>
                  <svg
                    viewBox="0 0 16 16"
                    className="h-3.5 w-3.5"
                    aria-hidden="true"
                  >
                    <path d="M5 3.5v9l7-4.5z" fill="currentColor" />
                  </svg>
                  Run
                </>
              )}
            </button>
          </div>
        </div>

        {running && (
          <p className="absolute right-4 top-full mt-1 font-mono text-[10px] text-ink-faint">
            {"// Execution in progress..."}
          </p>
        )}
      </div>

      {/* ---------- editor + side panels ---------- */}
      <div className="flex min-h-0 flex-1 flex-col gap-4 lg:flex-row">
        <div className="glass flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl">
          {/* tab bar */}
          <div className="flex items-center justify-between border-b border-white/[0.07] px-3 py-2">
            <span className="inset flex items-center gap-2 rounded-lg px-3 py-1.5">
              <span className="text-[11px]">{active.emoji}</span>
              <span className="font-mono text-[12px] text-ink">
                {active.file}
              </span>
              <span className="text-ink-faint">&#10005;</span>
            </span>

            <div className="flex items-center gap-3 pr-1 text-ink-faint">
              <svg viewBox="0 0 16 16" className="h-4 w-4" aria-hidden="true">
                <path
                  d="M5 3.5v9l7-4.5z"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.4"
                  strokeLinejoin="round"
                />
              </svg>
              <svg viewBox="0 0 16 16" className="h-4 w-4" aria-hidden="true">
                <rect
                  x="2"
                  y="3"
                  width="12"
                  height="10"
                  rx="1.5"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.4"
                />
                <path d="M8 3v10" stroke="currentColor" strokeWidth="1.4" />
              </svg>
              <svg viewBox="0 0 16 16" className="h-4 w-4" aria-hidden="true">
                <circle cx="4" cy="8" r="1.2" fill="currentColor" />
                <circle cx="8" cy="8" r="1.2" fill="currentColor" />
                <circle cx="12" cy="8" r="1.2" fill="currentColor" />
              </svg>
            </div>
          </div>

          <div className="min-h-0 flex-1">
            <Editor
              height="100%"
              language={language}
              defaultValue={STARTER}
              theme="interview-dark"
              beforeMount={handleBeforeMount}
              onMount={handleMount}
              onChange={handleChange}
              options={{
                fontFamily: "var(--font-mono)",
                fontSize: 13.5,
                minimap: { enabled: true, size: "fit", showSlider: "always" },
                padding: { top: 18 },
                scrollBeyondLastLine: false,
                renderLineHighlight: "line",
                smoothScrolling: true,
              }}
            />
          </div>
        </div>

        {/* right column */}
        <div className="flex w-full shrink-0 flex-col gap-4 lg:w-[330px]">
          <div className="glass flex flex-col rounded-2xl p-5">
            <h2 className="font-display text-lg font-semibold tracking-tight text-ink">
              Output
            </h2>
            <div className="inset mt-3 min-h-[150px] rounded-xl px-4 py-3 font-mono text-[12px] leading-relaxed">
              {output ? (
                <pre
                  className={`whitespace-pre-wrap ${
                    failed ? "text-[#F8A2A2]" : "text-[#7DD3A8]"
                  }`}
                >
                  {output}
                </pre>
              ) : (
                <span className="text-ink-faint">
                  Run the code to see output here.
                </span>
              )}
            </div>
          </div>

          <div className="glass flex min-h-0 flex-1 flex-col rounded-2xl p-5">
            <h2 className="font-display text-lg font-semibold tracking-tight text-ink">
              Prompt
            </h2>
            <div className="inset mt-3 min-h-0 flex-1 overflow-auto rounded-xl px-4 py-4">
              <h3 className="text-[13px] font-semibold text-ink">
                Interview Question
              </h3>
              <p className="mt-3 text-[13px] leading-relaxed text-ink-dim">
                <span className="font-semibold text-ink">Problem:</span> Given a
                list of integers, write a function that returns the largest
                value.
              </p>
              <p className="mt-3 text-[13px] leading-relaxed text-ink-dim">
                <span className="font-semibold text-ink">Example:</span>
                <br />
                <span className="font-mono text-[12px]">
                  &nbsp;&nbsp;Input: [10, 5, 22, 11]
                  <br />
                  &nbsp;&nbsp;Output: 22
                </span>
              </p>
              <p className="mt-3 text-[13px] leading-relaxed text-ink-dim">
                Handle edge cases, such as an empty list.
              </p>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
