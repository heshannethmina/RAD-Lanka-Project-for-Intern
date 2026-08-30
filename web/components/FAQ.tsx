"use client";

import { useId, useState } from "react";
import Reveal from "./Reveal";

const FAQS = [
  {
    q: "Do candidates need to install anything?",
    a: "No. They open the link you send and land straight in the room — no account, no extension, no download.",
  },
  {
    q: "Is the code actually executed, or just displayed?",
    a: "It is executed, in an isolated sandbox, on every run. What you both see is the real result, not a simulation.",
  },
  {
    q: "Can I bring my own interview questions?",
    a: "Yes. You can write the question straight into the room and the candidate sees it as you type. Pro adds a question bank, so you can save and reuse your prompts and starter code.",
  },
  {
    q: "How is interview time counted?",
    a: "The timer starts when the interview begins and stops when it ends. A room left open in a tab is not counted, and you are billed for the minutes you actually used.",
  },
  {
    q: "What happens when I run out of time?",
    a: "The interview in progress finishes — nothing is cut off mid-sentence. You are told before you start a session you do not have the time for.",
  },
  {
    q: "What languages are supported?",
    a: "Python, Go and JavaScript at launch, with more added based on what early teams actually ask for.",
  },
];

function Chevron({ open }: { open: boolean }) {
  return (
    <svg
      viewBox="0 0 16 16"
      className={`h-4 w-4 shrink-0 text-ink-muted transition-transform duration-300 ${
        open ? "rotate-180" : ""
      }`}
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
  // One open at a time: opening the next closes the last, so the section
  // never grows into a wall of text and the two motions read as one.
  const [openIndex, setOpenIndex] = useState<number | null>(0);
  const baseId = useId();

  return (
    <section id="faq" className="px-6 py-24 sm:py-28">
      <div className="mx-auto max-w-3xl">
        <Reveal>
          <span className="eyebrow">FAQ</span>
          <h2 className="mt-4 font-display text-[32px] font-semibold leading-[1.12] tracking-[-0.025em] text-ink sm:text-[40px]">
            Common questions.
          </h2>

          <div className="mt-10 border-t border-line">
            {FAQS.map((faq, i) => {
              const open = openIndex === i;
              const buttonId = `${baseId}-q-${i}`;
              const panelId = `${baseId}-a-${i}`;

              return (
                <div key={faq.q} className="border-b border-line">
                  <h3>
                    <button
                      type="button"
                      id={buttonId}
                      aria-expanded={open}
                      aria-controls={panelId}
                      onClick={() => setOpenIndex(open ? null : i)}
                      className="flex w-full items-center justify-between gap-6 py-5 text-left text-[16px] font-medium text-ink transition-colors hover:text-accent"
                    >
                      {faq.q}
                      <Chevron open={open} />
                    </button>
                  </h3>

                  <div
                    id={panelId}
                    role="region"
                    aria-labelledby={buttonId}
                    className="faq-panel"
                    data-open={open}
                    // Collapsed content stays in the DOM for the animation,
                    // so it has to be taken out of the a11y tree and the tab
                    // order explicitly.
                    inert={!open}
                  >
                    <div>
                      <p className="faq-panel__inner pb-5 pr-10 text-[15px] leading-relaxed text-ink-body">
                        {faq.a}
                      </p>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </Reveal>
      </div>
    </section>
  );
}
