import Reveal from "./Reveal";

const FEATURES = [
  {
    tag: "Collaboration",
    title: "Real-time sync",
    desc: "No lag between collaborators — every keystroke lands in the same document.",
  },
  {
    tag: "Execution",
    title: "Real code execution",
    desc: "Sandboxed, not simulated. What the candidate runs is what really ran.",
  },
  {
    tag: "Access",
    title: "No setup for candidates",
    desc: "They join from a link. Nothing to install, no account to create.",
  },
  {
    tag: "Pricing",
    title: "Built for small teams",
    desc: "Startup and campus pricing, not enterprise procurement.",
  },
];

/** Milliseconds between one card starting and the next. */
const STAGGER = 80;

export default function Features() {
  return (
    <section id="product" className="px-6 py-24 sm:py-28">
      <div className="mx-auto max-w-6xl">
        <Reveal>
          <span className="eyebrow">Features</span>
          <h2 className="mt-4 max-w-2xl font-display text-[32px] font-semibold leading-[1.12] tracking-[-0.025em] text-ink sm:text-[40px]">
            Everything an interview needs.
          </h2>
          <p className="mt-4 max-w-xl text-[16px] leading-relaxed text-ink-body">
            And none of the enterprise weight that usually comes with it.
          </p>
        </Reveal>

        <div className="mt-12 grid gap-5 sm:grid-cols-2">
          {FEATURES.map((feature, i) => (
            <Reveal key={feature.title} delay={i * STAGGER}>
              <article className="card h-full rounded-xl p-7">
                <span className="card-tag">{feature.tag}</span>
                <h3 className="mt-3 font-display text-[19px] font-semibold tracking-[-0.015em] text-ink">
                  {feature.title}
                </h3>
                <p className="mt-2 text-[15px] leading-relaxed text-ink-body">
                  {feature.desc}
                </p>
              </article>
            </Reveal>
          ))}
        </div>
      </div>
    </section>
  );
}
