"use client";

import { useCallback, useState } from "react";
import dynamic from "next/dynamic";
import Image from "next/image";
import mark from "@/public/syncr-mark.png";

/**
 * The hero's brand visual: the flat mark, replaced by the rotating 3D one
 * once it has loaded.
 *
 * three.js and the model are a few hundred kilobytes that nothing above the
 * fold depends on, so they are split out of the main bundle and fetched on
 * the client only. The still mark holds the space in the meantime — same
 * position, same footprint — so nothing shifts when the canvas takes over.
 */

const Logo3D = dynamic(() => import("./Logo3D"), { ssr: false });

function StillMark({ faded }: { faded: boolean }) {
  return (
    <Image
      src={mark}
      alt=""
      priority
      className={`absolute inset-0 m-auto h-1/2 w-auto transition-opacity duration-700 ${
        faded ? "opacity-0" : "opacity-100"
      }`}
    />
  );
}

export default function HeroVisual() {
  const [ready, setReady] = useState(false);
  const handleReady = useCallback(() => setReady(true), []);

  return (
    <div className="relative mx-auto aspect-square w-full max-w-[440px]">
      <StillMark faded={ready} />
      <Logo3D onReady={handleReady} />
    </div>
  );
}
