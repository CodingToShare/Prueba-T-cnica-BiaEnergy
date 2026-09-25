import type { Metadata } from "next";

import { MeterDetailView } from "@/features/meters/meter-detail-view";

export async function generateMetadata({ params }: PageProps<"/meters/[meterId]">): Promise<Metadata> {
  const { meterId } = await params;
  return { title: `Meter ${decodeURIComponent(meterId)}` };
}

export default async function MeterPage({ params }: PageProps<"/meters/[meterId]">) {
  const { meterId } = await params;
  return <MeterDetailView meterId={decodeURIComponent(meterId)} />;
}
