import { defineConfig } from 'tsdown';

export default defineConfig({
  entry: ['src/index.ts'],
  format: ['esm'],
  outputOptions: { codeSplitting: false },
  deps: { alwaysBundle: [/.*/] },
});
