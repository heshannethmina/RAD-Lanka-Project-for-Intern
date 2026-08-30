import Link from "next/link";
import Reveal from "./Reveal";

/*
 * Priced on interview time, not seats.
 *
 * A seat licence punishes a small team that interviews occasionally, which is
 * exactly who this is for. Billing the thing that costs us something — a live
 * room holding a hub goroutine and a sandbox — means a company that runs two
 * interviews a month pays for two interviews a month.
 *
 * The unit prices are deliberately ordered: Pro works out at roughly $0.33 an
 * interview-hour, Enterprise at $0.50. Pay-as-you-go costing *more* per unit
 * than the committed plan is the normal shape — you pay a premium for not
 * committing — and it means Pro is the better deal right up to its ceiling,
 * after which Enterprise is the only thing that fits.
 *
 * Time is counted only while a room is live. The timer starts when the
 * interview opens and stops when it ends, so a room left open in a tab
 * overnight is not billed for the night.
 */

type Plan = {
  name: string;
  amount: string;
  period: string;
  /** The line under the price: what it works out at, or what it covers. */
  note: string;
  features: string[];
  cta: string;
  href: string;
  recommended?: boolean;
};

const PLANS: Plan[] = [
  {
    name: "Free",
    amount: "$0",
    period: "for life",
    note: "2 interviews, 20 minutes of interview time in total",
    features: [
      "2 interviews — ever, not per month",
      "10 minutes each",
      "Shared editor and live code execution",
      "Candidate joins from a link, no account",
    ],
    cta: "Start free",
    href: "/register",
  },
  {
    name: "Pro",
    amount: "$10",
    period: "per month",
    note: "About $0.33 per interview-hour",
    features: [
      "30 interviews a month",
      "Up to 1 hour each — 30 interview-hours",
      "Schedule ahead, timed automatically",
      "Question bank and templates",
      "Interview history",
    ],
    cta: "Choose Pro",
    href: "/register",
    recommended: true,
  },
  {
    name: "Enterprise",
    amount: "$0.50",
    period: "per interview-hour",
    note: "Billed by the minute. No monthly commitment",
    features: [
      "Unlimited interviews",
      "Unlimited hours",
      "Pay only for time actually used",
      "Everything in Pro",
      "Priority support",
    ],
    cta: "Talk to us",
    href: "/register",
  },
];

/** Milliseconds between one card starting and the next, left to right. */
const STAGGER = 80;

function Check() {
  return (
    <svg
      viewBox="0 0 16 16"
      className="mt-[3px] h-4 w-4 shrink-0 text-ink-muted"
      aria-hidden="true"
    >
      <path
        d="M3.5 8.5l3 3 6-6.5"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export default function Pricing() {
  return (
    <section id="pricing" className="px-6 py-24 sm:py-28">
      <div className="mx-auto max-w-6xl">
        <Reveal>
          <span className="eyebrow">Pricing</span>
          <h2 className="mt-4 font-display text-[32px] font-semibold leading-[1.12] tracking-[-0.025em] text-ink sm:text-[40px]">
            Simple pricing.
          </h2>
          <p className="mt-4 max-w-xl text-[16px] leading-relaxed text-ink-body">
            You pay for interview time, not seats. The timer starts when an
            interview begins and stops when it ends, so you are never charged
            for a room sitting open in a tab.
          </p>
        </Reveal>

        <div className="mt-12 grid gap-5 md:grid-cols-3">
          {PLANS.map((plan, i) => (
            <Reveal key={plan.name} delay={i * STAGGER}>
              <article
                className={`card flex h-full flex-col rounded-xl p-7 ${
                  plan.recommended ? "card--pick" : ""
                }`}
              >
                {/* The slot is rendered on every card, empty or not, so the
                    three plan names sit on the same line. */}
                <div className="flex h-6 items-center">
                  {plan.recommended && (
                    <span className="rounded-full bg-[var(--accent-wash)] px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.08em] text-accent">
                      Recommended
                    </span>
                  )}
                </div>

                <h3 className="mt-4 font-display text-[17px] font-semibold tracking-[-0.01em] text-ink">
                  {plan.name}
                </h3>

                <p className="mt-3 flex items-baseline gap-1.5">
                  <span className="font-display text-[38px] font-semibold leading-none tracking-[-0.03em] text-ink">
                    {plan.amount}
                  </span>
                  <span className="text-[13px] text-ink-muted">
                    {plan.period}
                  </span>
                </p>

                {/* The unit price, stated plainly. The whole model turns on
                    comparing $0.33 with $0.50, and a reader who has to work
                    that out themselves will not bother. */}
                <p className="mt-2 min-h-[32px] text-[13px] leading-snug text-ink-muted">
                  {plan.note}
                </p>

                <ul className="mt-7 flex-1 space-y-2.5">
                  {plan.features.map((feature) => (
                    <li
                      key={feature}
                      className="flex gap-2.5 text-[14px] leading-relaxed text-ink-body"
                    >
                      <Check />
                      {feature}
                    </li>
                  ))}
                </ul>

                <Link
                  href={plan.href}
                  className={`mt-8 h-11 w-full text-[14px] ${
                    plan.recommended ? "btn-primary" : "btn-secondary"
                  }`}
                >
                  {plan.cta}
                </Link>
              </article>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}
