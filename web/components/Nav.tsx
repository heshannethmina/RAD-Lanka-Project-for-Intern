import Link from "next/link";
import Logo from "./Logo";

const LINKS = [
  { href: "#home", label: "Home" },
  { href: "#product", label: "Product" },
  { href: "#pricing", label: "Pricing" },
  { href: "#faq", label: "FAQ" },
];

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
                className="text-[13px] text-ink-dim transition hover:text-ink"
              >
                {link.label}
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
