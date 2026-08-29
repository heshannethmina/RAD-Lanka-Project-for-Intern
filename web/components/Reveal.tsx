"use client";

import { useEffect, useRef, useState, type ReactNode } from "react";

type RevealProps = {
  children: ReactNode;
  /** Milliseconds to stagger this group behind the one before it. */
  delay?: number;
  className?: string;
};

/**
 * Fades and slides its children up as they enter the viewport.
 *
 * The observer disconnects after the first intersection: a reveal that
 * re-plays every time you scroll past turns into a distraction. Elements
 * already on screen at load fire immediately, so the hero reads as a load-in
 * rather than needing a scroll to start.
 *
 * The hidden state lives in CSS keyed off `data-reveal`, with a `<noscript>`
 * override in the layout, so content is never invisible when JS does not run.
 */
export default function Reveal({
  children,
  delay = 0,
  className,
}: RevealProps) {
  const ref = useRef<HTMLDivElement>(null);
  const [shown, setShown] = useState(false);

  useEffect(() => {
    const el = ref.current;
    if (!el) return;

    // Safety net for anything without IntersectionObserver: show everything.
    // Deferred to the next frame rather than set here, because a synchronous
    // setState in an effect body triggers a cascading render.
    if (typeof IntersectionObserver === "undefined") {
      const frame = requestAnimationFrame(() => setShown(true));
      return () => cancelAnimationFrame(frame);
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        if (!entry.isIntersecting) return;
        setShown(true);
        observer.disconnect();
      },
      // A little inset at the bottom, so a group starts moving once it is
      // properly in view rather than the instant its first pixel appears.
      { threshold: 0.05, rootMargin: "0px 0px -8% 0px" },
    );

    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <div
      ref={ref}
      data-reveal
      data-shown={shown}
      style={delay ? { transitionDelay: `${delay}ms` } : undefined}
      className={className}
    >
      {children}
    </div>
  );
}
