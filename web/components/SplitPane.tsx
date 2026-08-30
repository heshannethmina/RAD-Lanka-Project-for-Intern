"use client";

import {
  useCallback,
  useEffect,
  useRef,
  useState,
  useSyncExternalStore,
} from "react";

/**
 * Two panes with a draggable divider between them.
 *
 * Hand-rolled rather than pulled from a package, for the same reason the
 * WebSocket hub is: it is a small, well-understood thing, and owning it means
 * the behaviour is exactly what this layout needs.
 *
 * Three details that are easy to get wrong and are the whole reason this is a
 * component rather than an inline handler:
 *
 *  - **Pointer capture.** Without setPointerCapture, dragging fast moves the
 *    cursor off the divider and the drag stops. Capture routes every move to
 *    the divider until release, however far the pointer strays.
 *  - **The iframe problem.** Monaco renders into its own stacking context and
 *    swallows pointer events. While dragging, an overlay covers the whole
 *    split so the editor never sees the moves and the drag stays smooth.
 *  - **Keyboard.** A divider that only responds to a mouse is unusable for
 *    anyone who does not use one, so arrow keys nudge it too.
 */

type Props = {
  /** "vertical" stacks the panes; "horizontal" puts them side by side. */
  direction: "horizontal" | "vertical";
  first: React.ReactNode;
  second: React.ReactNode;
  /** Initial size of the first pane, as a percentage. */
  defaultSize?: number;
  /** Percentage bounds, so neither pane can be dragged out of existence. */
  min?: number;
  max?: number;
  /** localStorage key. Omit to not remember the position. */
  storageKey?: string;
  className?: string;
};

/** How far one arrow-key press moves the divider, in percent. */
const KEY_STEP = 2;

function clamp(value: number, min: number, max: number) {
  return Math.min(max, Math.max(min, value));
}

/**
 * Cache of parsed positions.
 *
 * useSyncExternalStore calls getSnapshot on every render and requires a stable
 * value, so reading localStorage each time would be both slow and wrong — a
 * fresh read is fine here only because the result is a number, but the cache
 * keeps it off the render path entirely.
 */
const cache = new Map<string, number>();

function readStored(key: string, fallback: number): number {
  const hit = cache.get(key);
  if (hit !== undefined) return hit;

  let value = fallback;
  try {
    const raw = window.localStorage.getItem(key);
    if (raw !== null) {
      const parsed = Number.parseFloat(raw);
      if (Number.isFinite(parsed)) value = parsed;
    }
  } catch {
    // Private mode throws rather than returning null.
  }
  cache.set(key, value);
  return value;
}

/**
 * Fires when another tab moves the same divider. Same-tab writes do not raise
 * a storage event, which is correct here: the tab that made the change already
 * holds the new value in its own state.
 */
function subscribeToStorage(onChange: () => void) {
  const handler = (e: StorageEvent) => {
    if (e.key !== null) cache.delete(e.key);
    onChange();
  };
  window.addEventListener("storage", handler);
  return () => window.removeEventListener("storage", handler);
}

export default function SplitPane({
  direction,
  first,
  second,
  defaultSize = 50,
  min = 15,
  max = 85,
  storageKey,
  className = "",
}: Props) {
  const horizontal = direction === "horizontal";
  const containerRef = useRef<HTMLDivElement>(null);

  // The remembered position, read through useSyncExternalStore rather than
  // restored in an effect. localStorage is external state, and this is what
  // that hook is for: the server snapshot is the default, the client snapshot
  // is what was stored, and React reconciles the two without a hydration
  // mismatch and without the extra render an effect would cost.
  const stored = useSyncExternalStore(
    subscribeToStorage,
    () => (storageKey ? clamp(readStored(storageKey, defaultSize), min, max) : defaultSize),
    () => defaultSize,
  );

  // Set once this pane has been dragged. Until then the stored value wins, so
  // a change made in another tab is still picked up.
  const [override, setOverride] = useState<number | null>(null);
  const size = override ?? stored;

  const [dragging, setDragging] = useState(false);

  // The effective size, mirrored into a ref so the drag and key handlers can
  // read the latest value without listing it as a dependency — which would
  // rebuild them on every pixel of a drag. Written in an effect rather than
  // during render, because a render may be discarded and must not have side
  // effects.
  const sizeRef = useRef(size);
  useEffect(() => {
    sizeRef.current = size;
  }, [size]);

  const persist = useCallback(
    (next: number) => {
      if (!storageKey) return;
      try {
        cache.set(storageKey, next);
        window.localStorage.setItem(storageKey, String(next));
      } catch {
        // Not remembering a divider position is not worth reporting.
      }
    },
    [storageKey],
  );

  const sizeFromPointer = useCallback(
    (clientX: number, clientY: number) => {
      const box = containerRef.current?.getBoundingClientRect();
      if (!box) return null;
      const pct = horizontal
        ? ((clientX - box.left) / box.width) * 100
        : ((clientY - box.top) / box.height) * 100;
      return clamp(pct, min, max);
    },
    [horizontal, min, max],
  );

  const onPointerDown = useCallback((e: React.PointerEvent<HTMLDivElement>) => {
    // Capture first: without it a fast drag outstrips the pointer and the
    // divider is left behind.
    e.currentTarget.setPointerCapture(e.pointerId);
    setDragging(true);
  }, []);

  const onPointerMove = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      if (!dragging) return;
      const next = sizeFromPointer(e.clientX, e.clientY);
      if (next !== null) setOverride(next);
    },
    [dragging, sizeFromPointer],
  );

  const endDrag = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      if (!dragging) return;
      e.currentTarget.releasePointerCapture(e.pointerId);
      setDragging(false);
      // Persist on release rather than on every move: a drag is one decision,
      // and writing localStorage per pixel is pure waste.
      persist(sizeRef.current);
    },
    [dragging, persist],
  );

  const onKeyDown = useCallback(
    (e: React.KeyboardEvent<HTMLDivElement>) => {
      const back = horizontal ? "ArrowLeft" : "ArrowUp";
      const forward = horizontal ? "ArrowRight" : "ArrowDown";
      if (e.key !== back && e.key !== forward) return;
      e.preventDefault();
      const next = clamp(
        sizeRef.current + (e.key === forward ? KEY_STEP : -KEY_STEP),
        min,
        max,
      );
      setOverride(next);
      persist(next);
    },
    [horizontal, min, max, persist],
  );

  return (
    <div
      ref={containerRef}
      className={`flex min-h-0 min-w-0 ${horizontal ? "flex-row" : "flex-col"} ${className}`}
    >
      {/* min-h-0 / min-w-0 on the panes: a flex child defaults to min-size
          auto, which lets content push it past its share and makes the
          divider appear stuck. This is what allows the panes to scroll. */}
      <div
        className="min-h-0 min-w-0 overflow-hidden"
        style={{ [horizontal ? "width" : "height"]: `${size}%` }}
      >
        {first}
      </div>

      <div
        role="separator"
        aria-orientation={horizontal ? "vertical" : "horizontal"}
        aria-valuenow={Math.round(size)}
        aria-valuemin={min}
        aria-valuemax={max}
        aria-label="Resize panels"
        tabIndex={0}
        onPointerDown={onPointerDown}
        onPointerMove={onPointerMove}
        onPointerUp={endDrag}
        onPointerCancel={endDrag}
        onKeyDown={onKeyDown}
        className={[
          "group relative shrink-0 bg-line transition-colors",
          horizontal ? "w-px cursor-col-resize" : "h-px cursor-row-resize",
          dragging ? "bg-accent" : "hover:bg-accent",
        ].join(" ")}
      >
        {/* The visible line is 1px to match the hairline design, but a 1px
            hit target is unusable. This pad widens the grab area to 9px
            without changing what is drawn. */}
        <span
          aria-hidden="true"
          className={
            horizontal
              ? "absolute inset-y-0 -left-1 -right-1"
              : "absolute inset-x-0 -top-1 -bottom-1"
          }
        />
      </div>

      <div className="min-h-0 min-w-0 flex-1 overflow-hidden">{second}</div>

      {/* Monaco swallows pointer events, so without this the drag dies the
          moment the cursor crosses the editor. Only mounted while dragging,
          so it never blocks normal interaction. */}
      {dragging && (
        <div
          className="fixed inset-0 z-50"
          style={{ cursor: horizontal ? "col-resize" : "row-resize" }}
        />
      )}
    </div>
  );
}
