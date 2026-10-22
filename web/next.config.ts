import type { NextConfig } from 'next';

const config: NextConfig = {
  output: 'export',
  basePath: process.env.GITHUB_ACTIONS ? '/open-review' : '',
  images: {
    unoptimized: true,
  },
};

export default config;
