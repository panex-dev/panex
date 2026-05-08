import { build } from "esbuild";
import { mkdir, readFile, writeFile } from "node:fs/promises";
import { resolve } from "node:path";
import { DEFAULT_DAEMON_WEBSOCKET_URL } from "@panex/protocol";

import { injectChromeSimIntoHTML } from "./preview_injection";

const rootDir = resolve(new URL("..", import.meta.url).pathname);
const outDir = resolve(rootDir, "dist");
const sourceHTML = resolve(rootDir, "index.html");
const outHTML = resolve(outDir, "index.html");

await mkdir(outDir, { recursive: true });

await build({
  absWorkingDir: rootDir,
  entryPoints: {
    main: resolve(rootDir, "src/main.tsx"),
    "chrome-sim": resolve(rootDir, "src/chrome-sim.ts")
  },
  nodePaths: [resolve(rootDir, "node_modules")],
  outdir: outDir,
  bundle: true,
  format: "esm",
  platform: "browser",
  target: ["chrome116"],
  jsx: "transform",
  jsxFactory: "h",
  jsxFragment: "h.Fragment",
  sourcemap: true,
  logLevel: "info"
});

const sourceMarkup = await readFile(sourceHTML, "utf8");
const previewMarkup = injectChromeSimIntoHTML(normalizeMainScriptPath(sourceMarkup), {
  daemonURL: readEnv("PANEX_DAEMON_URL", DEFAULT_DAEMON_WEBSOCKET_URL),
  authToken: readEnv("PANEX_DAEMON_TOKEN", "dev-token"),
  extensionID: readEnv("PANEX_EXTENSION_ID", ""),
  moduleURL: "./chrome-sim.js"
});
await writeFile(outHTML, previewMarkup);

function readEnv(name: string, fallback: string): string {
  const value = process.env[name];
  if (typeof value !== "string") {
    return fallback;
  }

  const trimmed = value.trim();
  return trimmed.length > 0 ? trimmed : fallback;
}

function normalizeMainScriptPath(html: string): string {
  return html
    .replace(/src=(["'])\.\/dist\/main\.js\1/i, "src=\"./main.js\"")
    .replace(/href=(["'])\.\/dist\/main\.css\1/i, "href=\"./main.css\"");
}
