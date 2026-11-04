import { defineConfig } from "vite";
import { fileURLToPath, URL } from "node:url";

const appRoot = fileURLToPath(new URL(".", import.meta.url));
const dist = fileURLToPath(new URL("../dist", import.meta.url));

export default defineConfig({
  root: appRoot,
  publicDir: "public",
  server: {
    host: "127.0.0.1",
    port: 5173,
    strictPort: true,
  },
  preview: {
    host: "127.0.0.1",
    port: 4173,
    strictPort: true,
  },
  build: {
    outDir: dist,
    emptyOutDir: true,
  },
});
