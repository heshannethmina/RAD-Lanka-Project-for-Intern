import Link from "next/link";
import Reveal from "./Reveal";

type Plan = {
  name: string;
  amount: string;
  period: string;
  features: string[];
  cta: string;
  href: string;
  recommended?: boolean;
};

const PLANS: Plan[] = [
  {
    name: "Starter",
    amount: "$0",
    period: "forever",
    features: [
      "5 interviews per month",
      "1 seat",
      "Core languages",
      "7-day history",
    ],
    cta: "Start free",
    href: "/room/demo",
  },
  {
    name: "Team",
    amount: "$39",
    period: "per interviewer / month",
    features: [
      "Unlimited interviews",
      "Up to 10 seats",
      "All languages",
      "Question bank",
      "90-day history",
    ],
    cta: "Start trial",
    href: "/room/demo",
    recommended: true,
  },
  {
    name: "Campus",
    amount: "Custom",
    period: "pricing",
    features: [
      "Unlimited interviews and seats",
      "Bulk candidate links",
      "Priority support",
    ],
    cta: "Talk to us",
    href: "/room/demo",
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
            Every plan includes the shared editor and real, sandboxed
            execution.
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
