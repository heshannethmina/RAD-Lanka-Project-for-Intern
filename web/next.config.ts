import type { NextConfig } from "next";

/**
 * Where third-party code is loaded from at runtime.
 *
 * Two things come from here and neither is optional: Monaco, which
 * @monaco-editor/react fetches rather than bundling, and Pyodide, which
 * public/python-worker.js pulls in to run Python in the candidate's own tab.
 */
const CDN = "https://cdn.jsdelivr.net";

/**
 * Content Security Policy.
 *
 * Written as a list rather than one long string because it was one long
 * string, and a directive going missing in it is invisible until something
 * breaks in a way that looks like a bug in the app.
 *
 * That already happened once. `script-src` allowed the CDN but `style-src`
 * did not, so Monaco's JavaScript loaded while `editor.main.css` was refused.
 * Monaco then rendered without the rules that position its hidden textarea and
 * map clicks onto text — the textarea appeared as a plain, resizable form
 * field over the code and the caret stopped following the mouse. It read as a
 * broken editor, not as a blocked stylesheet, and it only failed in the
 * browser, so no build or test caught it.
 *
 * **If a directive here needs the CDN, it needs it for every resource type
 * Monaco and Pyodide fetch: script, style and font.** Monaco ships its icons
 * as codicon.ttf from the same path, so tightening `font-src` alone would
 * silently lose the editor's icons.
 */
const csp = [
  "default-src 'self'",
  "base-uri 'self'",
  "object-src 'none'",
  "frame-ancestors 'none'",
  "form-action 'self'",
  // 'unsafe-eval' is Monaco's; it compiles its own workers.
  `script-src 'self' 'unsafe-inline' 'unsafe-eval' ${CDN}`,
  // blob: is the Pyodide worker, which is created from a blob URL.
  "worker-src 'self' blob:",
  // The Go API and its WebSocket live on another origin in every environment.
  "connect-src 'self' https: wss:",
  `style-src 'self' 'unsafe-inline' ${CDN}`,
  "img-src 'self' data: blob:",
  `font-src 'self' data: https://fonts.gstatic.com ${CDN}`,
].join("; ");

const nextConfig: NextConfig = {
  async headers() {
    return [
      {
        source: "/(.*)",
        headers: [
          { key: "Content-Security-Policy", value: csp },
          { key: "X-Content-Type-Options", value: "nosniff" },
          { key: "X-Frame-Options", value: "DENY" },
          { key: "Referrer-Policy", value: "no-referrer" },
          { key: "Permissions-Policy", value: "camera=(), microphone=(), geolocation=()" },
          { key: "Strict-Transport-Security", value: "max-age=31536000; includeSubDomains" },
        ],
      },
    ];
  },
};

export default nextConfig;
