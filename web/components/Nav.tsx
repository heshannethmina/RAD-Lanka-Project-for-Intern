import Link from "next/link";
import Logo from "./Logo";

const LINKS = [
  { href: "#home", label: "Home", active: true, caret: false },
  { href: "#product", label: "Product", active: false, caret: true },
  { href: "#pricing", label: "Pricing", active: false, caret: false },
  { href: "#faq", label: "FAQ", active: false, caret: false },
];

function Caret() {
  return (
    <svg viewBox="0 0 16 16" className="h-3 w-3" aria-hidden="true">
      <path
        d="M4 6.5l4 4 4-4"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export default function Nav() {
  return (
    <header className="sticky top-0 z-50 w-full">
      <div className="mx-auto max-w-6xl px-6">
        <div className="glass mt-4 flex items-center justify-between rounded-2xl px-4 py-2.5">
          <Link href="/" className="flex items-center">
            <Logo />
          </Link>

          <nav className="hidden items-center gap-8 md:flex">
            {LINKS.map((link) => (
              <a
                key={link.href}
                href={link.href}
                className={`flex items-center gap-1 text-[13px] transition hover:text-ink ${
                  link.active ? "text-ink" : "text-ink-dim"
                }`}
              >
                {link.label}
                {link.caret && <Caret />}
              </a>
            ))}
          </nav>

          <div className="flex items-center gap-4">
            <Link
              href="/room/demo"
              className="hidden text-[13px] text-ink-dim transition hover:text-ink sm:block"
            >
              Sign in
            </Link>
            <Link
              href="/room/demo"
              className="btn-accent rounded-xl px-4 py-2 text-[13px] font-semibold"
            >
              Try live demo
            </Link>
          </div>
        </div>
      </div>
    </header>
  );
}
