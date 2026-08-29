import type { Metadata } from "next";
import { Inter, JetBrains_Mono } from "next/font/google";
import "./globals.css";

// One typeface for the whole interface. Hierarchy comes from size, weight and
// tracking rather than from mixing faces.
const inter = Inter({
  variable: "--font-sans",
  subsets: ["latin"],
  weight: ["400", "500", "600", "700"],
});

const jetbrainsMono = JetBrains_Mono({
  variable: "--font-mono",
  subsets: ["latin"],
  weight: ["400", "500"],
});

export const metadata: Metadata = {
  title: "SyncR — Collaborative technical interviews",
  description:
    "Run real-time technical interviews in a shared editor with sandboxed code execution. Candidates join from a link — no install, no account.",
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html
      lang="en"
      className={`${inter.variable} ${jetbrainsMono.variable} h-full`}
    >
      <body className="min-h-full flex flex-col bg-white text-ink-body">
        {children}
      </body>
    </html>
  );
}
