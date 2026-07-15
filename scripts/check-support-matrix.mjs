import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { parseArgs } from "node:util";

const root = process.cwd();
const { values } = parseArgs({ options: { target: { type: "string" } }, strict: true });
const matrix = JSON.parse(await readFile(join(root, "docs/support-matrix.json"), "utf8"));
const packageJSON = JSON.parse(await readFile(join(root, "package.json"), "utf8"));
const goMod = await readFile(join(root, "go.mod"), "utf8");
const compatibility = await readFile(join(root, "docs/compatibility.md"), "utf8");

assert.equal(matrix.schemaVersion, "2026-07-15");
assert.equal(matrix.minimums.go, "1.26");
assert.equal(matrix.minimums.node, "26");
assert.equal(packageJSON.engines.node, ">=26");
assert.match(goMod, /^go 1\.26$/mu);
assert.deepEqual(matrix.outOfScope, ["windows"]);
assert.ok(!matrix.targets.some(({ os }) => os === "windows"));
assert.deepEqual(matrix.targets.map(({ os, arch }) => `${os}/${arch}`).sort(), ["darwin/amd64", "darwin/arm64", "linux/amd64", "linux/arm64"]);
assert.deepEqual(matrix.browsers.filter(({ blocking }) => blocking).map(({ name }) => name), ["chromium"]);
for (const target of matrix.targets) assert.match(compatibility, new RegExp(`\\b${target.os}/${target.arch}\\b`, "u"));

const requested = values.target === undefined ? matrix.targets : matrix.targets.filter(({ os, arch }) => `${os}/${arch}` === values.target);
assert.ok(requested.length > 0, `unknown support target ${JSON.stringify(values.target)}`);
const temporary = await mkdtemp(join(tmpdir(), "scrape-kdl-support-"));
try {
  for (const target of requested) {
    const output = join(temporary, `scrape-kdl-${target.os}-${target.arch}`);
    execFileSync("go", ["build", "-trimpath", "-o", output, "./cmd/scrape-kdl"], {
      cwd: root,
      env: { ...process.env, GOOS: target.os, GOARCH: target.arch, CGO_ENABLED: "0", GOTOOLCHAIN: "local" },
      stdio: "pipe",
    });
    console.log(`support matrix: built ${target.os}/${target.arch}`);
  }
} finally {
  await rm(temporary, { recursive: true, force: true });
}
