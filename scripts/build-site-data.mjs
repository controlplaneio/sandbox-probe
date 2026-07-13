// Concatenate every per-run report stored on the data branch into the single
// all-reports.json the page fetches (see ADR 0001). Pure byte-mover: no
// collapse/normalize/categorize — all semantics live in the page.
//
// Layout read:  <dataDir>/<run-ts>/<os>-<harness>.json
// Run-ts dirs are filesystem-safe (colons -> dashes); reconstructed to ISO here.
// Usage: node build-site-data.mjs <dataDir>   (writes JSON to stdout)
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join } from "node:path";

const dataDir = process.argv[2];
if (!dataDir) {
  console.error("usage: build-site-data.mjs <dataDir>");
  process.exit(1);
}

// "2026-07-13T08-30-00Z" -> "2026-07-13T08:30:00Z" (only the time half has dashes swapped)
function toIso(dir) {
  const [d, t] = dir.split("T");
  return t ? `${d}T${t.replace(/-/g, ":")}` : d;
}

const out = [];
let dirs = [];
try { dirs = readdirSync(dataDir); } catch { /* empty data branch -> [] */ }

for (const runDir of dirs.sort()) {
  const runPath = join(dataDir, runDir);
  if (!statSync(runPath).isDirectory()) continue;
  const runTimestamp = toIso(runDir);
  for (const file of readdirSync(runPath)) {
    if (!file.endsWith(".json")) continue;
    const m = file.replace(/\.json$/, "").match(/^([^-]+)-(.+)$/); // <os>-<harness>
    if (!m) continue;
    let report;
    try { report = JSON.parse(readFileSync(join(runPath, file), "utf8")); }
    catch { continue; } // skip a corrupt report rather than fail the whole build
    out.push({ os: m[1], harness: m[2], runTimestamp, report });
  }
}

process.stdout.write(JSON.stringify(out));
console.error(`build-site-data: ${out.length} reports from ${dataDir}`);
