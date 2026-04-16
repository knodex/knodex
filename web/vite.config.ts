import { defineConfig, type PluginOption } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import compression from "vite-plugin-compression2";
import path from "path";
import { readFileSync } from "fs";

let appVersion = "0.0.0";
try {
  appVersion = readFileSync(path.resolve(__dirname, "..", "version.txt"), "utf-8").trim();
} catch {
  console.warn("Failed to read version.txt, using fallback");
}

// https://vite.dev/config/
export default defineConfig(async ({ mode }) => {
  const isEnterprise = mode === "enterprise";
  const analyze = process.env.ANALYZE === "true";

  const plugins: PluginOption[] = [
    react(),
    tailwindcss(),
    compression({
      algorithms: ["gzip", "brotliCompress"],
      threshold: 1024,
      include: /\.(js|css|html|json|svg|txt|xml|wasm)$/i,
    }),
  ];

  if (analyze) {
    const { visualizer } = await import("rollup-plugin-visualizer");
    plugins.push(visualizer({ open: true, filename: "dist/stats.html" }) as PluginOption);
  }

  return {
    plugins,
    define: {
      __ENTERPRISE__: JSON.stringify(isEnterprise),
      __APP_VERSION__: JSON.stringify(appVersion),
    },
    resolve: {
      alias: {
        "@": path.resolve(__dirname, "./src"),
      },
    },
    optimizeDeps: {
      include: [
        // react-core
        "react",
        "react-dom",
        "react-dom/client",
        // router
        "react-router-dom",
        // state
        "@tanstack/react-query",
        "zustand",
        // radix-ui
        "@radix-ui/react-alert-dialog",
        "@radix-ui/react-dialog",
        "@radix-ui/react-dropdown-menu",
        "@radix-ui/react-popover",
        "@radix-ui/react-scroll-area",
        "@radix-ui/react-select",
        "@radix-ui/react-slot",
        "@radix-ui/react-tabs",
        "@radix-ui/react-tooltip",
        "cmdk",
        // forms
        "react-hook-form",
        "@hookform/resolvers",
        "zod",
        // lucide-react: intentionally EXCLUDED from pre-bundling.
        // Pre-bundling collapses the entire icon library into one module,
        // defeating Rollup tree-shaking. Without pre-bundling, only imported
        // icons are included in the production build (~93 icons vs ~1300+).
        // utils
        "axios",
        "clsx",
        "tailwind-merge",
        "class-variance-authority",
        "sonner",
        "js-yaml",
      ],
    },
    server: {
      port: 3000,
      warmup: {
        clientFiles: [
          "./src/main.tsx",
          "./src/App.tsx",
        ],
      },
      proxy: {
        "/api": {
          target: "http://localhost:8088",
          changeOrigin: true,
        },
        "/ws": {
          target: "ws://localhost:8088",
          ws: true,
        },
        "/healthz": {
          target: "http://localhost:8088",
        },
        "/readyz": {
          target: "http://localhost:8088",
        },
      },
    },
    build: {
      target: "es2022",
      outDir: "dist",
      sourcemap: false,
      reportCompressedSize: false,
      modulePreload: { polyfill: false },
      cssCodeSplit: true,
      minify: "terser",
      terserOptions: {
        compress: {
          drop_debugger: true,
          pure_funcs: ["console.log", "console.debug", "console.info"],
        },
        format: {
          comments: false,
        },
      },
      chunkSizeWarningLimit: 250,
      rollupOptions: {
        output: {
          entryFileNames: "assets/[name]-[hash].js",
          chunkFileNames: "assets/[name]-[hash].js",
          assetFileNames: "assets/[name]-[hash].[ext]",
          manualChunks: (id) => {
            if (!id.includes("node_modules")) return undefined;

            // React core -- rarely changes
            if (
              id.includes("node_modules/react-dom/") ||
              id.includes("node_modules/react/") ||
              id.includes("node_modules/scheduler/")
            )
              return "react-core";
            // Router -- changes with major react-router updates
            if (
              id.includes("node_modules/react-router") ||
              id.includes("node_modules/@remix-run/router")
            )
              return "router";
            // State management -- changes when react-query/zustand update
            if (
              id.includes("node_modules/zustand") ||
              id.includes("node_modules/@tanstack/react-query") ||
              id.includes("node_modules/@tanstack/query")
            )
              return "state";
            // Radix UI primitives -- changes on Radix version bumps
            if (
              id.includes("node_modules/@radix-ui") ||
              id.includes("node_modules/cmdk")
            )
              return "radix-ui";
            // Form libraries -- changes on form lib updates
            if (
              id.includes("node_modules/react-hook-form") ||
              id.includes("node_modules/@hookform") ||
              id.includes("node_modules/zod")
            )
              return "forms";
            // Icons -- changes on lucide updates (frequent)
            // lucide-react is excluded from optimizeDeps to allow Rollup tree-shaking.
            // Only icons actually imported in the codebase are included.
            if (id.includes("node_modules/lucide-react")) return "icons";
            // Graph visualization -- lazy-loaded, rarely changes
            if (id.includes("node_modules/@xyflow")) return "graph";
            // YAML parser -- only used in deploy wizard; lazy-loaded separately
            if (id.includes("node_modules/js-yaml")) return "yaml";
            // HTTP client -- used broadly but benefits from independent caching
            if (id.includes("node_modules/axios")) return "http";
            // Utility libraries -- small, stable
            if (
              /node_modules\/(clsx|tailwind-merge|class-variance-authority|sonner)/.test(
                id
              )
            )
              return "utils";

            // Catch-all for remaining node_modules
            return "vendor";
          },
        },
      },
    },
  };
});
