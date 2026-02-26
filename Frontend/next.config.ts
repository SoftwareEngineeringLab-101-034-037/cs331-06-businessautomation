import type { NextConfig } from 'next';

const nextConfig: NextConfig = {
  /* Pre-compile dashboard routes on dev startup so first visit is instant */
  experimental: {
    optimizePackageImports: ['@clerk/nextjs'],
  },
};

export default nextConfig;
