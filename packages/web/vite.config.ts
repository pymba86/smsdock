import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

const apiBase = process.env.VITE_API_BASE ?? "http://localhost:8080";

export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
    port: 5173,
    proxy: {
      "/api": apiBase,
      "/health": apiBase,
      "/ready": apiBase,
    },
  },
});

