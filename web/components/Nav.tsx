"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import Logo from "./Logo";

const LINKS = [
  { href: "#product", label: "Product" },
  { href: "#pricing", label: "Pricing" },
  { href: "#faq", label: "FAQ" },
];

/** Past this many pixels the bar stops being transparent. */
const SETTLE_AT = 8;

export default function Nav() {
  const [scrolled, setScrolled] = useState(false);
  const [menuOpen, setMenuOpen] = useState(false);

  useEffect(() => {
    // Passive, and only ever flips a boolean — the actual easing is CSS, so
    // scrolling never waits on React.
    const onScroll = () => setScrolled(window.scrollY > SETTLE_AT);
    onScroll();
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  // The open menu is a panel over the page; let Escape close it.
  useEffect(() => {
    if (!menuOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setMenuOpen(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [menuOpen]);

  return (
    <header
      className="nav-bar sticky top-0 z-50 w-full"
      data-scrolled={scrolled || menuOpen}
    >
      <div className="mx-auto flex h-16 max-w-6xl items-center justify-between gap-6 px-6">
        <Link
          href="/"
          className="shrink-0 rounded-sm"
          onClick={() => setMenuOpen(false)}
        >
          <Logo className="h-[26px] w-auto" priority />
        </Link>

        <nav
          aria-label="Main"
          className="hidden items-center gap-9 md:flex"
        >
          {LINKS.map((link) => (
            <a
              key={link.href}
              href={link.href}
              className="nav-link text-[14px] font-medium"
            >
              {link.label}
            </a>
          ))}
        </nav>

        <div className="flex items-center gap-2 sm:gap-5">
          <Link
            href="/room/demo"
            className="hidden text-[14px] font-medium text-ink-body transition-colors hover:text-ink sm:block"
          >
            Sign in
          </Link>
          <Link
            href="/room/demo"
            className="btn-primary h-9 px-4 text-[14px]"
          >
            Try live demo
          </Link>

          <button
            type="button"
            onClick={() => setMenuOpen((open) => !open)}
            aria-expanded={menuOpen}
            aria-controls="nav-menu"
            aria-label={menuOpen ? "Close menu" : "Open menu"}
            className="-mr-2 flex h-9 w-9 items-center justify-center rounded-full text-ink transition-colors hover:bg-bg-subtle md:hidden"
          >
            <svg viewBox="0 0 20 20" className="h-5 w-5" aria-hidden="true">
              {menuOpen ? (
                <path
                  d="M5 5l10 10M15 5L5 15"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.6"
                  strokeLinecap="round"
                />
              ) : (
                <path
                  d="M3 6h14M3 13h14"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="1.6"
                  strokeLinecap="round"
                />
              )}
            </svg>
          </button>
        </div>
      </div>

      {menuOpen && (
        <div
          id="nav-menu"
          className="border-t border-line bg-white px-6 py-4 md:hidden"
        >
          <nav aria-label="Main" className="flex flex-col">
            {LINKS.map((link) => (
              <a
                key={link.href}
                href={link.href}
                onClick={() => setMenuOpen(false)}
                className="py-2.5 text-[15px] font-medium text-ink-body transition-colors hover:text-ink"
              >
                {link.label}
              </a>
            ))}
            <Link
              href="/room/demo"
              onClick={() => setMenuOpen(false)}
              className="py-2.5 text-[15px] font-medium text-ink-body transition-colors hover:text-ink sm:hidden"
            >
              Sign in
            </Link>
          </nav>
        </div>
      )}
    </header>
  );
}
