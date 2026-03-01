import { build } from "esbuild";
import { mkdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";

const rootDir = resolve(new URL("..", import.meta.url).pathname);
const outFile = resolve(rootDir, "dist/main.js");

await mkdir(dirname(outFile), { recursive: true });

await build({
  entryPoints: [resolve(rootDir, "src/main.tsx")],
  outfile: outFile,
  bundle: true,
  format: "esm",
  platform: "browser",
  target: ["chrome116"],
  jsx: "automatic",
  jsxImportSource: "solid-js",
  sourcemap: true,
  logLevel: "info"
});
