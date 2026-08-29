import Image from "next/image";
import lockup from "@/public/syncr-logo.png";
import mark from "@/public/syncr-mark.png";

/**
 * The SyncR logo.
 *
 * Both assets are the supplied artwork, cropped to the ink and re-encoded as
 * a flat #005DED over the alpha channel — same pixels, a third of the bytes.
 * They are 256px tall, so they stay sharp at 3x anywhere they are used.
 */

/** The mark on its own: the sync cycle between two code chevrons. */
export function LogoMark({
  className = "h-7 w-auto",
  priority = false,
}: {
  className?: string;
  priority?: boolean;
}) {
  return <Image src={mark} alt="" className={className} priority={priority} />;
}

type LogoProps = {
  /** Full lockup with the wordmark, or just the mark. */
  withWordmark?: boolean;
  className?: string;
  /** Only the nav copy is above the fold; everywhere else can lazy-load. */
  priority?: boolean;
};

export default function Logo({
  withWordmark = true,
  className = "h-7 w-auto",
  priority = false,
}: LogoProps) {
  return (
    <Image
      src={withWordmark ? lockup : mark}
      alt="SyncR"
      className={className}
      priority={priority}
    />
  );
}
