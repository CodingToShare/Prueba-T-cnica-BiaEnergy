import type { Metadata } from "next";

import { AnomaliesView } from "@/features/anomalies/anomalies-view";

export const metadata: Metadata = { title: "AI Anomalies" };

export default function AnomaliesPage() {
  return <AnomaliesView />;
}
