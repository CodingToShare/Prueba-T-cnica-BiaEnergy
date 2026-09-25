import type { Metadata } from "next";

import { MetersView } from "@/features/meters/meters-view";

export const metadata: Metadata = { title: "Meters" };

export default function MetersPage() {
  return <MetersView />;
}
