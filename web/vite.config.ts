import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    port: 5173,
    proxy: {
      // scripts/dev-instance.sh points this at an isolated API on a free port.
      "/api": process.env.DASHBOARD_API_URL ?? "http://localhost:8484",
    },
  },
});
