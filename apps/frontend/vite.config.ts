import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react()],
  server: {
    proxy: {
      "/api": {
        target: "http://localhost:18080",
        changeOrigin: true,
      },
      "/collector": {
        target: "http://localhost:19091",
        changeOrigin: true,
        rewrite: (path) => path.replace(/^\/collector/, ""),
      },
    },
  },
});
