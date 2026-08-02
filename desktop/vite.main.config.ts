import { defineConfig } from "vite";

export default defineConfig({
  ssr: {
    noExternal: ["zod"],
  },
  build: {
    outDir: "dist-electron",
    emptyOutDir: true,
    ssr: "electron/main.ts",
    target: "node22",
    rollupOptions: {
      external: ["electron", /^node:/],
      output: {
        entryFileNames: "main.js",
        format: "es",
      },
    },
  },
});
