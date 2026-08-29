import Image from "next/image";
import cycle from "@/public/syncr-cycle.png";
import chevLeft from "@/public/syncr-chev-left.png";
import chevRight from "@/public/syncr-chev-right.png";

/**
 * The logo, animated: the cycle turns between the two brackets while the
 * brackets ease apart and back.
 *
 * The three pieces are the supplied artwork, cropped to the ink and flattened
 * to the brand blue over their alpha. Sizes are computed from each piece's
 * own aspect ratio rather than set in CSS, so `next/image` gets real
 * dimensions and the row never reflows as the images arrive.
 *
 * Everything moves in CSS, so this renders and animates server-side — the
 * splash is already spinning in the first paint, before any JS runs.
 */

// Intrinsic aspect ratios (width ÷ height) of the three exported pieces.
const CYCLE_RATIO = 463 / 512;
const CHEV_RATIO = 309 / 512;
// The brackets sit a little shorter than the cycle, as they do in the mark.
const CHEV_SCALE = 0.8;

type SyncLoaderProps = {
  /** Height of the cycle in pixels; everything else scales from it. */
  size?: number;
  /** Announced to screen readers. */
  label?: string;
  className?: string;
};

export default function SyncLoader({
  size = 48,
  label = "Loading",
  className,
}: SyncLoaderProps) {
  const chevHeight = Math.round(size * CHEV_SCALE);
  const chevWidth = Math.round(chevHeight * CHEV_RATIO);
  const cycleWidth = Math.round(size * CYCLE_RATIO);

  return (
    <span
      role="status"
      className={`syncr-loader ${className ?? ""}`}
      style={{ gap: Math.round(size * 0.1) }}
    >
      <Image
        src={chevLeft}
        alt=""
        width={chevWidth}
        height={chevHeight}
        style={{ width: chevWidth, height: chevHeight }}
        className="syncr-loader__chev--left"
        priority
      />
      <Image
        src={cycle}
        alt=""
        width={cycleWidth}
        height={size}
        style={{ width: cycleWidth, height: size }}
        className="syncr-loader__cycle"
        priority
      />
      <Image
        src={chevRight}
        alt=""
        width={chevWidth}
        height={chevHeight}
        style={{ width: chevWidth, height: chevHeight }}
        className="syncr-loader__chev--right"
        priority
      />
      <span className="sr-only">{label}</span>
    </span>
  );
}
