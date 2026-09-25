import type { Metadata } from "next";

import { InvestigationView } from "@/features/anomalies/investigation-view";

export const metadata: Metadata = { title: "Investigation" };

export default async function InvestigationPage({ params }: PageProps<"/anomalies/[id]">) {
  const { id } = await params;
  const parsed = /^\d{1,15}$/.test(id) ? Number(id) : Number.NaN;
  return <InvestigationView id={parsed} />;
}
