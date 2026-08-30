import type { Metadata } from "next";
import Dashboard from "@/components/Dashboard";

export const metadata: Metadata = {
  title: "Interviews — SyncR",
  robots: { index: false, follow: false },
};

export default function DashboardPage() {
  return <Dashboard />;
}
