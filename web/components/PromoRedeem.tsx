"use client";

import { useState } from "react";
import { api, ApiError, type Me, type Usage } from "@/lib/api";

/**
 * Redeems a promotion code.
 *
 * Collapsed behind a link by default. Most people have no code, and a form
 * asking for one is a permanent invitation to feel they are missing a
 * discount — the same reason checkout pages hide the coupon field.
 *
 * When a grant is live this shows that instead, because the useful question
 * flips: not "do I have a code" but "how long does this last".
 */
export default function PromoRedeem({
  usage,
  onRedeemed,
}: {
  usage: Usage;
  onRedeemed: (me: Me) => void;
}) {
  const [open, setOpen] = useState(false);
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function onSubmit(event: React.FormEvent) {
    event.preventDefault();
    if (busy || !code.trim()) return;

    setBusy(true);
    setError(null);
    try {
      const me = await api.redeemPromo(code.trim());
      setCode("");
      setOpen(false);
      onRedeemed(me);
    } catch (cause) {
      // The server distinguishes not-valid, expired, claimed and already-used,
      // and each needs a different reaction from a person — so show its
      // wording rather than flattening all four into "invalid code".
      setError(cause instanceof ApiError ? cause.message : "Could not redeem that code.");
    } finally {
      setBusy(false);
    }
  }

  if (usage.promo_code) {
    return (
      <p className="mt-3 text-[13px] text-ink-muted">
        <span className="font-medium text-ink">Promotion applied</span>
        {" · "}
        <span className="font-mono">{usage.promo_code}</span>
        {usage.promo_expires_at
          ? ` · until ${new Date(usage.promo_expires_at).toLocaleDateString(undefined, {
              day: "numeric",
              month: "short",
              year: "numeric",
            })}`
          : " · no end date"}
      </p>
    );
  }

  if (!open) {
    return (
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="mt-3 text-[13px] font-medium text-accent hover:underline"
      >
        Have a promotion code?
      </button>
    );
  }

  return (
    <form onSubmit={onSubmit} className="mt-3 flex flex-wrap items-end gap-2">
      <div className="min-w-[12rem] flex-1">
        <label htmlFor="promo" className="field-label">
          Promotion code
        </label>
        <input
          id="promo"
          className="field font-mono uppercase"
          value={code}
          onChange={(e) => setCode(e.target.value)}
          placeholder="SYNCR-PILOT"
          maxLength={64}
          disabled={busy}
          autoFocus
          // Codes are typed off a slide or out of an email, and the server
          // upper-cases and strips whitespace anyway. Autocorrect would fight
          // that on a phone.
          autoCapitalize="characters"
          autoCorrect="off"
          spellCheck={false}
        />
      </div>
      <button type="submit" className="btn-primary h-11 px-5 text-sm" disabled={busy}>
        {busy ? "Checking…" : "Redeem"}
      </button>
      <button
        type="button"
        onClick={() => {
          setOpen(false);
          setError(null);
        }}
        className="btn-secondary h-11 px-4 text-sm"
        disabled={busy}
      >
        Cancel
      </button>
      {error && (
        <p className="form-error w-full" role="alert" aria-live="polite">
          {error}
        </p>
      )}
    </form>
  );
}
