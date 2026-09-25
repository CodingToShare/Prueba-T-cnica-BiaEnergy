import type { NextConfig } from "next";

// Same-origin API access: the browser calls relative /api/v1/... URLs on the
// Next.js origin, and this rewrite forwards them unchanged to the Go API.
// The HttpOnly session cookie therefore stays first-party and no CORS is
// needed. It only forwards requests; no API logic lives in Next.js.
//
// BACKEND_URL is server-only configuration (never exposed to the browser).
// Rewrites are resolved when the server starts in development and when the
// app is built for production, so set it before `next dev` / `next build`.
const backendUrl = (process.env.BACKEND_URL ?? "http://localhost:8080").replace(/\/+$/, "");

const nextConfig: NextConfig = {
  poweredByHeader: false,
  async rewrites() {
    return [{ source: "/api/v1/:path*", destination: `${backendUrl}/api/v1/:path*` }];
  },
};

export default nextConfig;
