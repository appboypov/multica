import { svelte } from "@sveltejs/vite-plugin-svelte";
import { defineConfig } from "vite";

/** Static build for Lavish: every asset beside index.html. */
export default defineConfig({
  plugins: [svelte()],
  base: "./",
  build: {
    outDir: "build",
    emptyOutDir: true,
  },
});
