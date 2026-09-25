import type { Metadata } from "next";
import { Plus_Jakarta_Sans } from "next/font/google";

import { Providers } from "@/lib/query/providers";

import "./globals.css";

// Self-hosted at build time by next/font; no request to Google at runtime.
const jakarta = Plus_Jakarta_Sans({
  subsets: ["latin"],
  weight: ["400", "500", "600", "700", "800"],
  variable: "--font-jakarta",
  display: "swap",
});

export const metadata: Metadata = {
  title: { default: "Bia Energy", template: "%s | Bia Energy" },
  description: "Energy management with explainable AI anomaly detection.",
  robots: { index: false, follow: false },
};

export default function RootLayout({ children }: LayoutProps<"/">) {
  return (
    <html lang="en" className={jakarta.variable}>
      <body>
        <Providers>{children}</Providers>
      </body>
    </html>
  );
}
