import Link from "next/link";
import Logo from "./Logo";
import Reveal from "./Reveal";

/**
 * PLACEHOLDER CONTACT DETAILS — replace before launch.
 *
 * These are invented. Publishing a fabricated address or phone number is
 * worse than publishing none, so they are kept together here and named
 * loudly rather than scattered through the markup.
 */
const CONTACT_PLACEHOLDER = {
  email: "hello@syncr.dev",
  phone: "+94 00 000 0000",
  address: ["Colombo", "Sri Lanka"],
};

const PRODUCT_LINKS = [
  { href: "#product", label: "Features" },
  { href: "#pricing", label: "Pricing" },
  { href: "#faq", label: "FAQ" },
  { href: "/room/demo", label: "Live demo" },
];

export default function Footer() {
  return (
    <footer>
      {/* Closing CTA. A full-width tint rather than a card — the band itself
          is the separation, so nothing needs a shadow to lift off the page. */}
      <section className="bg-bg-subtle px-6 py-24 sm:py-28">
        <Reveal>
          <div className="mx-auto max-w-2xl text-center">
            <h2 className="font-display text-[32px] font-semibold leading-[1.12] tracking-[-0.025em] text-ink sm:text-[40px]">
              Ready to run your first interview?
            </h2>
            <p className="mt-4 text-[16px] leading-relaxed text-ink-body">
              Free to start, and nothing for candidates to install.
            </p>
            <Link
              href="/room/demo"
              className="btn-primary mt-8 h-12 px-6 text-[15px]"
            >
              Start a free interview
            </Link>
          </div>
        </Reveal>
      </section>

      {/* Directory */}
      <div className="border-t border-line px-6">
        <div className="mx-auto grid max-w-6xl gap-10 py-14 sm:grid-cols-2 lg:grid-cols-[1.7fr_1fr_1.2fr] lg:gap-16">
          <div>
            <Link href="/" className="inline-block rounded-sm">
              <Logo className="h-[24px] w-auto" />
            </Link>
            <p className="mt-4 max-w-xs text-[14px] leading-relaxed text-ink-body">
              Collaborative technical interviews with a shared editor and real,
              sandboxed code execution.
            </p>
          </div>

          <nav aria-labelledby="footer-product">
            <h2
              id="footer-product"
              className="text-[13px] font-semibold text-ink"
            >
              Product
            </h2>
            <ul className="mt-4 space-y-2.5">
              {PRODUCT_LINKS.map((link) => (
                <li key={link.href}>
                  <Link
                    href={link.href}
                    className="text-[14px] text-ink-body transition-colors hover:text-accent"
                  >
                    {link.label}
                  </Link>
                </li>
              ))}
            </ul>
          </nav>

          <div>
            <h2 className="text-[13px] font-semibold text-ink">Contact</h2>
            <ul className="mt-4 space-y-2.5 text-[14px] text-ink-body">
              <li>
                <a
                  href={`mailto:${CONTACT_PLACEHOLDER.email}`}
                  className="transition-colors hover:text-accent"
                >
                  {CONTACT_PLACEHOLDER.email}
                </a>
              </li>
              <li>
                <a
                  href={`tel:${CONTACT_PLACEHOLDER.phone.replace(/\s/g, "")}`}
                  className="transition-colors hover:text-accent"
                >
                  {CONTACT_PLACEHOLDER.phone}
                </a>
              </li>
              <li>
                <address className="not-italic leading-relaxed">
                  {CONTACT_PLACEHOLDER.address.map((line) => (
                    <span key={line} className="block">
                      {line}
                    </span>
                  ))}
                </address>
              </li>
            </ul>
          </div>
        </div>
      </div>

      {/* Utility bar */}
      <div className="border-t border-line px-6">
        <div className="mx-auto flex max-w-6xl items-center justify-between py-6">
          <p className="text-[13px] text-ink-muted">
            © {new Date().getFullYear()} SyncR. All rights reserved.
          </p>
        </div>
      </div>
    </footer>
  );
}
