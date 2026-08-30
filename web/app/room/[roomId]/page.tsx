import { Suspense } from "react";
import type { Metadata } from "next";
import RoomGate from "@/components/RoomGate";
import SyncLoader from "@/components/SyncLoader";

export const metadata: Metadata = {
  // An interview room is nobody's business but the two people in it.
  robots: { index: false, follow: false },
};

export default async function RoomPage({
  params,
}: {
  params: Promise<{ roomId: string }>;
}) {
  const { roomId } = await params;

  // useSearchParams needs a Suspense boundary, or the whole route opts out of
  // static rendering and the build warns about it. The fallback is the same
  // loader RoomGate shows while it resolves access, so there is no visible
  // handover between the two.
  return (
    <Suspense
      fallback={
        <main className="flex min-h-screen items-center justify-center">
          <SyncLoader />
        </main>
      }
    >
      <RoomGate roomId={roomId} />
    </Suspense>
  );
}
