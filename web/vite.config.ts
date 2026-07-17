import { defineConfig } from 'vite';
import react from '@vitejs/plugin-react';
import { fileURLToPath } from 'node:url';

// The pdfkit repo is served as a GitHub *project* page at
// https://malcolmston.github.io/pdfkit/, so assets must be based under /pdfkit/.
export default defineConfig({
  base: '/pdfkit/',
  plugins: [react()],
  resolve: {
    alias: {
      // Import the vendored shared library from source.
      'go-ui': fileURLToPath(new URL('./vendor/go/ui/src/index.ts', import.meta.url)),
    },
  },
  build: { outDir: 'dist', emptyOutDir: true },
});
