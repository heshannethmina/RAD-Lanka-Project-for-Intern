"use client";

import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Canvas, useFrame } from "@react-three/fiber";
import { Center, Environment, Lightformer, useGLTF } from "@react-three/drei";
import { Box3, Vector3, type Group } from "three";

/**
 * The SyncR mark in three dimensions, turning slowly.
 *
 * The model is meshopt-compressed (3.2MB of unwelded float32 down to 400KB).
 * drei's `useGLTF` wires the meshopt decoder from three-stdlib, so nothing is
 * fetched from a CDN — unlike the Draco path, which pulls its decoder from
 * gstatic.
 *
 * Two things keep it from being a battery drain on a marketing page: the
 * frame loop stops entirely when the hero scrolls out of view, and the
 * environment renders a single frame rather than every tick.
 */

const MODEL = "/syncr-mark.glb";
/** World units for the model's largest dimension. */
const TARGET_SIZE = 3.4;
/** Radians per second — about fourteen seconds a turn. */
const SPIN = 0.45;

function Mark({ spin, onReady }: { spin: boolean; onReady: () => void }) {
  const { scene } = useGLTF(MODEL);
  const group = useRef<Group>(null);

  // Scale from the model's own bounds, computed once. Fitting the camera to
  // the bounds every frame instead would make the mark pulse as it turns,
  // because a rotating box's bounding box changes size.
  const scale = useMemo(() => {
    const size = new Box3().setFromObject(scene).getSize(new Vector3());
    return TARGET_SIZE / Math.max(size.x, size.y, size.z);
  }, [scene]);

  // useGLTF suspends, so by the time this runs the model is decoded and the
  // still placeholder underneath can be faded out.
  useEffect(() => {
    onReady();
  }, [onReady]);

  useFrame((_, delta) => {
    if (spin && group.current) group.current.rotation.y += delta * SPIN;
  });

  return (
    <group ref={group} rotation={[0.16, -0.55, 0]} scale={scale}>
      <Center>
        <primitive object={scene} />
      </Center>
    </group>
  );
}

export default function Logo3D({ onReady }: { onReady?: () => void }) {
  const holder = useRef<HTMLDivElement>(null);
  const [visible, setVisible] = useState(true);
  const [spin, setSpin] = useState(true);

  const handleReady = useCallback(() => onReady?.(), [onReady]);

  useEffect(() => {
    const reduce = window.matchMedia("(prefers-reduced-motion: reduce)");
    const apply = () => setSpin(!reduce.matches);
    apply();
    reduce.addEventListener("change", apply);
    return () => reduce.removeEventListener("change", apply);
  }, []);

  // Stop rendering altogether once the hero is off screen.
  useEffect(() => {
    const el = holder.current;
    if (!el || typeof IntersectionObserver === "undefined") return;
    const observer = new IntersectionObserver(
      ([entry]) => setVisible(entry.isIntersecting),
      { rootMargin: "120px" },
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);

  return (
    <div ref={holder} className="h-full w-full" aria-hidden="true">
      <Canvas
        dpr={[1, 2]}
        gl={{ antialias: true, alpha: true }}
        camera={{ fov: 35, position: [0, 0, 6.5] }}
        frameloop={visible && spin ? "always" : "demand"}
      >
        <Suspense fallback={null}>
          <ambientLight intensity={0.55} />
          <directionalLight position={[4, 6, 5]} intensity={1.5} />
          <directionalLight
            position={[-5, -2, -4]}
            intensity={0.45}
            color="#cfe0ff"
          />

          <Mark spin={spin} onReady={handleReady} />

          {/* Built in-memory from light shapes rather than loaded as an HDR,
              so the metallic surfaces get real reflections without a CDN
              round trip. One frame is enough — none of it moves. */}
          <Environment frames={1} resolution={256}>
            <Lightformer
              form="rect"
              intensity={2.4}
              position={[0, 3, 4]}
              scale={8}
            />
            <Lightformer
              form="rect"
              intensity={1.2}
              position={[-4, 1, 2]}
              scale={6}
              color="#dce8ff"
            />
            <Lightformer
              form="circle"
              intensity={1.6}
              position={[3, -2, 2]}
              scale={5}
            />
          </Environment>
        </Suspense>
      </Canvas>
    </div>
  );
}

useGLTF.preload(MODEL);
