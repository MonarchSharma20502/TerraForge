import { readFileSync } from "fs";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The Go core compiles to web/src/core/autonation.wasm. Vite serves it as a
// static asset so the preview runs entirely in the browser - no backend.
//
// The Azure icon set is inlined as markup rather than served as files: the
// canvas scales with zoom and the palette never shows a broken image.
export default defineConfig({
  plugins: [
    react(),
    {
      name: "inline-svgs",
      enforce: "pre",
      transform(_code, id) {
        if (!id.endsWith(".svg")) return null;
        const svg = readFileSync(id, "utf8");
        return `export default ${JSON.stringify(svg)};`;
      },
    },
  ],
  server: {
    port: 5173,
  },
  optimizeDeps: {
    // The wasm glue is loaded at runtime, not imported through the bundler.
    exclude: ["./core/wasm_exec"],
  },
});
