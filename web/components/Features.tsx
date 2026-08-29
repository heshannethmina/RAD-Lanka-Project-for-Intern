const features = [
  {
    title: "Real-time, no lag",
    desc: "Every keystroke syncs instantly. Built on a persistent connection designed to hold up under real interview conditions, not a chat-app retrofit.",
    tag: "sync",
  },
  {
    title: "Actually runs the code",
    desc: "Candidates execute in sandboxed containers, not a fake console. Go, Python, and JavaScript today, more on the way.",
    tag: "exec",
  },
  {
    title: "One link, zero setup",
    desc: "No installs, no accounts required for candidates. Send a link, they land in the room.",
    tag: "join",
  },
  {
    title: "Built for small teams",
    desc: "Priced for a startup running a handful of interviews a week, not a 500-person recruiting org.",
    tag: "price",
  },
];

export default function Features() {
  return (
    <section id="product" className="px-6 py-24">
      <div className="mx-auto max-w-6xl">
        <div className="max-w-lg">
          <span className="font-mono text-xs tracking-wide text-ink-faint">
            /product
          </span>
          <h2 className="mt-3 font-display text-3xl font-semibold tracking-tight sm:text-4xl">
            Everything the interview needs.
            <br />
            Nothing it doesn&apos;t.
          </h2>
        </div>

        <div className="mt-12 grid gap-4 sm:grid-cols-2">
          {features.map((f) => (
            <div
              key={f.title}
              className="glass group rounded-2xl p-6 transition hover:bg-white/[0.06]"
            >
              <span className="font-mono text-[11px] text-[#5EEAD4]">
                {f.tag}
              </span>
              <h3 className="mt-3 font-display text-lg font-semibold tracking-tight">
                {f.title}
              </h3>
              <p className="mt-2 text-sm leading-relaxed text-ink-dim">
                {f.desc}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
