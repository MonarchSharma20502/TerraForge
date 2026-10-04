import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The Go core compiles to web/src/core/autonation.wasm. Vite serves it as a
// static asset so the preview runs entirely in the browser - no backend.
export default defineConfig({
  plugins: [react()],
  server: {
    port: 5173,
  },
  optimizeDeps: {
    // The wasm glue is loaded at runtime, not imported through the bundler.
    exclude: ["./core/wasm_exec"],
  },
});
