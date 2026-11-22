import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

// The page uses no Wails bindings or runtime: it talks to the core over
// loopback HTTP and WebSockets.
export default defineConfig({
  server: {
    host: "127.0.0.1",
    port: Number(process.env.WAILS_VITE_PORT) || 9245,
    strictPort: true,
  },
  plugins: [react()],
});
