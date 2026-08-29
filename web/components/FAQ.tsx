const faqs = [
  {
    q: "Do candidates need to install anything?",
    a: "No, it's all in the browser. They open the link you send and land straight in the room — no account, no extension, no download.",
  },
  {
    q: "Is the code actually executed, or just displayed?",
    a: "It is executed, in an isolated sandbox per run. What the candidate sees is what really happened, not a simulation.",
  },
  {
    q: "Can I bring my own interview questions?",
    a: "Yes, with the Team plan. It includes a question bank where you can save and reuse your own prompts and starter code.",
  },
  {
    q: "What languages are supported?",
    a: "Go, Python, and JavaScript at launch, with more added based on demand from early teams.",
  },
];

function Chevron() {
  return (
    <svg
      viewBox="0 0 16 16"
      className="h-4 w-4 shrink-0 text-ink-faint transition-transform duration-200 group-open:rotate-180"
      aria-hidden="true"
    >
      <path
        d="M4 6l4 4 4-4"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export default function FAQ() {
  return (
    <section id="faq" className="px-6 pb-20">
      <div className="mx-auto max-w-5xl space-y-3">
        {faqs.map((faq, i) => (
          <details
            key={faq.q}
            open={i === 0}
            className="group glass glass-hover rounded-xl px-5 py-4 [&_summary::-webkit-details-marker]:hidden"
          >
            <summary className="flex cursor-pointer list-none items-center justify-between gap-4">
              <span className="text-sm font-medium text-ink">
                <span className="text-ink-faint">Q: </span>
                {faq.q}
              </span>
              <Chevron />
            </summary>
            <p className="mt-2 text-[13px] leading-relaxed text-ink-dim">
              <span className="text-ink-faint">A: </span>
              {faq.a}
            </p>
          </details>
        ))}
      </div>
    </section>
  );
}
