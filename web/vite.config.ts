import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "path";
import { readFileSync } from "fs";

let appVersion = "0.0.0";
try {
  appVersion = readFileSync(path.resolve(__dirname, "..", "version.txt"), "utf-8").trim();
} catch {
  console.warn("Failed to read version.txt, using fallback");
}

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const isEnterprise = mode === "enterprise";

  return {
    plugins: [react(), tailwindcss()],
    define: {
      __ENTERPRISE__: JSON.stringify(isEnterprise),
      __APP_VERSION__: JSON.stringify(appVersion),
    },
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
      },
    },
    server: {
      port: 3000,
      proxy: {
        "/api": {
          target: "http://localhost:8080",
          changeOrigin: true,
        },
        "/ws": {
          target: "ws://localhost:8080",
          ws: true,
        },
        "/healthz": {
          target: "http://localhost:8080",
        },
        "/readyz": {
          target: "http://localhost:8080",
        },
      },
    },
    build: {
      outDir: "dist",
      sourcemap: false,
      rollupOptions: {
        output: {
          manualChunks: {
            vendor: ["react", "react-dom", "react-router-dom"],
            query: ["@tanstack/react-query"],
            ui: ["lucide-react"],
            // ReactFlow is lazy-loaded only on RGD detail Resources tab
            graph: ["@xyflow/react"],
          },
        },
      },
    },
  };
});
