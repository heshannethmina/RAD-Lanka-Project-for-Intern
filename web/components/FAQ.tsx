const faqs = [
  {
    q: "Do candidates need to install anything?",
    a: "No. They open the link you send and land directly in the room — no account, no extension, no download.",
  },
  {
    q: "Is the code actually executed, or just displayed?",
    a: "Actually executed, in an isolated sandbox per run. What the candidate sees is what really happened, not a simulation.",
  },
  {
    q: "Can I bring my own interview questions?",
    a: "Yes, the Team plan includes a question bank where you can save and reuse your own prompts and starter code.",
  },
  {
    q: "What languages are supported?",
    a: "Go, Python, and JavaScript at launch, with more added based on demand from early teams.",
  },
];

export default function FAQ() {
  return (
    <section id="faq" className="px-6 py-24">
      <div className="mx-auto max-w-3xl">
        <span className="font-mono text-xs tracking-wide text-ink-faint">
          /faq
        </span>
        <h2 className="mt-3 font-display text-3xl font-semibold tracking-tight sm:text-4xl">
          Questions, answered.
        </h2>

        <div className="mt-10 divide-y divide-white/[0.07] overflow-hidden rounded-2xl glass">
          {faqs.map((f) => (
            <div key={f.q} className="p-6">
              <h3 className="font-display text-[15px] font-semibold">
                {f.q}
              </h3>
              <p className="mt-2 text-sm leading-relaxed text-ink-dim">
                {f.a}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
