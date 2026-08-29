const plans = [
  {
    name: "Starter",
    price: "$0",
    period: "forever",
    desc: "For trying it out on a real interview.",
    features: ["5 interviews / month", "1 interviewer seat", "Go, Python, JS", "7-day session history"],
    cta: "Start free",
    highlighted: false,
  },
  {
    name: "Team",
    price: "$39",
    period: "/ interviewer / mo",
    desc: "For startups hiring regularly.",
    features: [
      "Unlimited interviews",
      "Up to 10 interviewer seats",
      "All languages",
      "Question bank + templates",
      "90-day session history",
    ],
    cta: "Start 14-day trial",
    highlighted: true,
  },
  {
    name: "Campus",
    price: "Custom",
    period: "",
    desc: "For university placement cells and bootcamps.",
    features: [
      "Unlimited interviews",
      "Unlimited seats",
      "Bulk candidate links",
      "Priority support",
    ],
    cta: "Talk to us",
    highlighted: false,
  },
];

export default function Pricing() {
  return (
    <section id="pricing" className="px-6 py-24">
      <div className="mx-auto max-w-6xl">
        <div className="max-w-lg">
          <span className="font-mono text-xs tracking-wide text-ink-faint">
            /pricing
          </span>
          <h2 className="mt-3 font-display text-3xl font-semibold tracking-tight sm:text-4xl">
            Priced for teams, not enterprises.
          </h2>
          <p className="mt-3 text-[15px] text-ink-dim">
            No per-candidate fees. No annual lock-in required.
          </p>
        </div>

        <div className="mt-12 grid gap-5 lg:grid-cols-3">
          {plans.map((p) => (
            <div
              key={p.name}
              className={
                p.highlighted
                  ? "relative rounded-2xl border border-[var(--accent-a)]/30 bg-gradient-to-b from-white/[0.07] to-white/[0.02] p-7 shadow-[0_20px_60px_-20px_rgba(94,234,212,0.25)] backdrop-blur-xl"
                  : "glass rounded-2xl p-7"
              }
            >
              {p.highlighted && (
                <span className="absolute -top-3 left-7 rounded-full bg-gradient-to-br from-[var(--accent-a)] to-[var(--accent-b)] px-3 py-1 font-mono text-[10px] font-bold tracking-wide text-[#0B0E14]">
                  MOST TEAMS PICK THIS
                </span>
              )}

              <h3 className="font-display text-lg font-semibold">{p.name}</h3>
              <p className="mt-1 text-sm text-ink-dim">{p.desc}</p>

              <div className="mt-6 flex items-baseline gap-1.5">
                <span className="font-display text-4xl font-semibold tracking-tight">
                  {p.price}
                </span>
                {p.period && (
                  <span className="text-sm text-ink-faint">{p.period}</span>
                )}
              </div>

              <ul className="mt-6 space-y-3">
                {p.features.map((f) => (
                  <li
                    key={f}
                    className="flex items-start gap-2.5 text-sm text-ink-dim"
                  >
                    <span className="mt-1.5 h-1 w-1 shrink-0 rounded-full bg-[#5EEAD4]" />
                    {f}
                  </li>
                ))}
              </ul>

              <button
                className={
                  p.highlighted
                    ? "mt-8 w-full rounded-xl bg-gradient-to-br from-[var(--accent-a)] to-[var(--accent-b)] px-5 py-3 text-sm font-semibold text-[#0B0E14] transition hover:brightness-110"
                    : "mt-8 w-full rounded-xl border border-white/[0.12] px-5 py-3 text-sm font-semibold text-ink transition hover:bg-white/[0.06]"
                }
              >
                {p.cta}
              </button>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
