import assert from "node:assert/strict";
import { access, readdir, readFile } from "node:fs/promises";
import { join } from "node:path";

const root = process.cwd();
const matrix = JSON.parse(await readFile("conformance/release-matrix.json", "utf8"));
const required = [
  "static-http",
  "malformed-truncated-html",
  "charset-decoding",
  "sessions-redirects-url-policy",
  "transforms",
  "partial-results",
  "imports",
  "browser-workflow-javascript",
];
assert.equal(matrix.schemaVersion, "2026-07-15");
assert.equal(matrix.portableDifferencesAllowed, 0);
assert.deepEqual(matrix.categories.map(({ id }) => id).sort(), [...required].sort());
for (const category of matrix.categories) {
  assert.ok(category.implementations.length >= 2, `${category.id}: cross-runtime coverage is required`);
  assert.ok(category.gates.length > 0, `${category.id}: gate is required`);
  for (const gate of category.gates) assert.ok(matrix.gates[gate], `${category.id}: unknown gate ${gate}`);
  for (const artifact of category.artifacts) await access(join(root, artifact));
}

const html = JSON.parse(await readFile("fixtures/html-compat/manifest.json", "utf8"));
const conformance = JSON.parse(await readFile("conformance/manifest.json", "utf8"));
assert.deepEqual(html.approvedDivergences, []);
assert.deepEqual(conformance.approvedDivergences, []);

for (const entry of await readdir("examples", { withFileTypes: true })) {
  if (!entry.isDirectory()) continue;
  const manifest = JSON.parse(await readFile(join("examples", entry.name, "example.json"), "utf8"));
  const implementations = new Set(manifest.executions.map(({ implementation }) => implementation));
  assert.ok(implementations.has("go"), `${entry.name}: Go example execution is required`);
  assert.ok(implementations.has("typescript"), `${entry.name}: TypeScript example execution is required`);
}

console.log(
  `release matrix: ${matrix.categories.length} categories, zero portable differences, all artifacts and gates declared`,
);
