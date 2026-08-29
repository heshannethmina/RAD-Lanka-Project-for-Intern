import SyncLoader from "@/components/SyncLoader";

/**
 * Route-transition cover. Next renders this while a route's data is in
 * flight, so navigations show the same mark as the first paint.
 */
export default function Loading() {
  return (
    <div className="splash">
      <SyncLoader size={56} />
    </div>
  );
}
