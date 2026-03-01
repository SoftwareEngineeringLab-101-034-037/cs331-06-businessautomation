/**
 * warm-routes.mjs
 * Starts the dev server + pre-compiles all routes in one go.
 * Usage: node scripts/warm-routes.mjs
 */

import { spawn } from "node:child_process";
import { resolve, dirname } from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(__dirname, "..");

const PORT = process.env.PORT || 3000;
const BASE = `http://localhost:${PORT}`;

const ROUTES = [
  "/",
  "/dashboard",
  "/dashboard/tasks",
  "/dashboard/requests",
  "/dashboard/workstation",
  "/dashboard/analytics",
  "/dashboard/team",
  "/dashboard/profile",
  "/dashboard/settings",
  "/workflow-builder",
];

// Start Next.js dev server as a child process
function startServer() {
  const nextBin = resolve(ROOT, "node_modules/next/dist/bin/next");
  const child = spawn("node", [nextBin, "dev", "--turbopack", "--port", String(PORT)], {
    cwd: ROOT,
    stdio: "inherit",
    shell: false,
  });
  child.on("error", (err) => {
    console.error("Failed to start dev server:", err.message);
    process.exit(1);
  });
  // Forward kill signals so Ctrl+C stops both
  process.on("SIGINT", () => child.kill("SIGINT"));
  process.on("SIGTERM", () => child.kill("SIGTERM"));
  return child;
}

async function waitForServer(maxRetries = 40) {
  for (let i = 0; i < maxRetries; i++) {
    try {
      await fetch(BASE, { signal: AbortSignal.timeout(2000) });
      return true;
    } catch {
      await new Promise((r) => setTimeout(r, 1000));
    }
  }
  return false;
}

async function warmRoutes() {
  console.log("\n🔥 Pre-compiling all routes...\n");
  const t0 = Date.now();

  for (const route of ROUTES) {
    const start = Date.now();
    try {
      const res = await fetch(`${BASE}${route}`, {
        signal: AbortSignal.timeout(30000),
        headers: { "x-warm-up": "1" },
      });
      const ms = Date.now() - start;
      console.log(`  ${res.ok ? "✓" : "⚠"} ${route} — ${res.status} (${ms}ms)`);
    } catch {
      const ms = Date.now() - start;
      console.log(`  ✗ ${route} — failed (${ms}ms)`);
    }
  }

  console.log(`\n✅ All routes ready! (${((Date.now() - t0) / 1000).toFixed(1)}s total)\n`);
}

async function isServerAlready() {
  try {
    await fetch(BASE, { signal: AbortSignal.timeout(2000) });
    return true;
  } catch {
    return false;
  }
}

async function main() {
  const alreadyUp = await isServerAlready();

  if (alreadyUp) {
    console.log("\n⚡ Dev server already running — skipping start.\n");
  } else {
    startServer();
    console.log("\n⏳ Waiting for dev server to start...");
    const ready = await waitForServer();
    if (!ready) {
      console.log("❌ Dev server not reachable — timed out.");
      process.exit(1);
    }
  }

  await warmRoutes();
  // Server keeps running — Ctrl+C to stop
}

main();
