import Link from "next/link";

const plans = [
  {
    name: "Starter",
    price: "$0",
    period: "/forever",
    desc: "5 interviews/month, 1 seat, core languages, 7-day history",
    cta: "Start free",
    highlighted: false,
  },
  {
    name: "Team",
    price: "$39",
    period: "/interviewer/month",
    desc: "Unlimited interviews, up to 10 seats, all languages, question bank, 90-day history",
    cta: "Start trial",
    highlighted: true,
  },
  {
    name: "Campus",
    price: "Custom pricing",
    period: "",
    desc: "Unlimited interviews and seats, bulk candidate links, priority support",
    cta: "Talk to us",
    highlighted: false,
  },
];

export default function Pricing() {
  return (
    <section id="pricing" className="px-6 py-20">
      <div className="mx-auto max-w-5xl text-center">
        <span className="chip inline-flex items-center rounded-full px-3 py-1 text-[11px] tracking-wide">
          Pricing
        </span>
        <h2 className="mt-4 font-display text-3xl font-semibold tracking-tight sm:text-[2.5rem]">
          Select your Plan
        </h2>
        <p className="mt-3 text-sm text-ink-dim">
          All plans include essential collaborative features
        </p>

        <div className="mt-10 grid gap-5 md:grid-cols-3">
          {plans.map((plan) => (
            <div
              key={plan.name}
              className={`flex flex-col rounded-2xl p-7 text-center ${
                plan.highlighted
                  ? "glass-bright ring-1 ring-[var(--accent-ring)] shadow-[0_20px_60px_-25px_rgba(91,140,255,0.7)]"
                  : "glass"
              }`}
            >
              <h3 className="font-display text-lg font-semibold tracking-tight text-ink">
                {plan.name}
              </h3>

              <p className="mt-5">
                <span
                  className={
                    plan.period
                      ? "font-display text-[2.6rem] font-semibold leading-none tracking-tight text-ink"
                      : "font-display text-[1.6rem] font-semibold leading-none tracking-tight text-ink"
                  }
                >
                  {plan.price}
                </span>
                {plan.period && (
                  <span className="ml-1 text-[12px] text-ink-faint">
                    {plan.period}
                  </span>
                )}
              </p>

              <p className="mt-5 flex-1 text-[13px] leading-relaxed text-ink-dim">
                {plan.desc}
              </p>

              <Link
                href="/room/demo"
                className={`mt-7 rounded-xl px-5 py-2.5 text-sm font-semibold transition ${
                  plan.highlighted
                    ? "btn-accent"
                    : "glass text-ink hover:bg-white/[0.08]"
                }`}
              >
                {plan.cta}
              </Link>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
