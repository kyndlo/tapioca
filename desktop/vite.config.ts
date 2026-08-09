import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [
    react(),
    {
      name: "tapioca-electron-development-csp",
      apply: "serve",
      transformIndexHtml(html) {
        // Vite's React refresh preamble is inline in development. The packaged
        // application retains the strict production policy from index.html.
        return html.replace(
          "script-src 'self'",
          "script-src 'self' 'unsafe-inline'",
        );
      },
    },
  ],
  base: "./",
  build: {
    outDir: "dist/renderer",
    emptyOutDir: true,
  },
});
