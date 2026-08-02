import { defineConfig } from "vite";

export default defineConfig({
  ssr: {
    noExternal: ["zod"],
  },
  build: {
    outDir: "dist-electron",
    emptyOutDir: false,
    ssr: "electron/preload.ts",
    target: "node22",
    rollupOptions: {
      external: ["electron", /^node:/],
      output: {
        entryFileNames: "preload.cjs",
        format: "cjs",
      },
    },
  },
});
