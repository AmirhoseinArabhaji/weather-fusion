import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  output: 'standalone',
  // Without this, Next.js dev blocks its own JS chunks/HMR for any origin
  // besides exactly "localhost" — opening the app via 127.0.0.1 (same
  // machine, different origin as far as this check is concerned) silently
  // kills hydration: the SSR HTML loads, but no script ever runs, so nothing
  // is interactive and no client-side data fetch ever fires.
  allowedDevOrigins: ['127.0.0.1', 'localhost'],
};

export default nextConfig;
