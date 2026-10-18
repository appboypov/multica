import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import react from "@vitejs/plugin-react";
import { defineConfig, type Plugin } from "vite";

const systemJson = resolve(__dirname, "../system.json");

// The page fetches ./system.json beside it; in dev it comes from the folder above.
const serveSystemJson: Plugin = {
  name: "serve-system-json",
  configureServer(server) {
    server.middlewares.use("/system.json", (_req, res) => {
      res.setHeader("Content-Type", "application/json");
      res.end(readFileSync(systemJson));
    });
  },
};

// The build lands beside system.json, with stable asset names, so the folder
// is served as it is.
export default defineConfig({
  base: "./",
  plugins: [react(), serveSystemJson],
  build: {
    outDir: "..",
    emptyOutDir: false,
    assetsInlineLimit: 0,
    rollupOptions: {
      output: {
        entryFileNames: "assets/app.js",
        chunkFileNames: "assets/[name].js",
        assetFileNames: "assets/[name][extname]",
      },
    },
  },
});
