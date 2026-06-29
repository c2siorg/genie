import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The app is served by the Go backend under /ui/app/ (the legacy vanilla UI
// stays at /ui/ during migration). `base` makes Vite emit asset URLs as
// /ui/app/assets/... so they resolve through the embedded file server.
// `outDir` writes the build straight into the Go embed tree
// (pkg/web/handlers/ui/app), so `go build` ships the bundle in the binary.
export default defineConfig({
  base: "/ui/app/",
  plugins: [react()],
  build: {
    outDir: "../pkg/web/handlers/ui/app",
    emptyOutDir: true,
  },
  test: {
    globals: true,
    environment: "jsdom",
    setupFiles: ["./src/test/setup.ts"],
    css: false,
  },
});
