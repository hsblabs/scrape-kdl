import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const root = process.cwd();
const snapshots = [
  ["core", "packages/scrape-kdl/src/public-api.ts", "docs/api/typescript/index.d.ts"],
  ["authoring", "packages/scrape-kdl/src/authoring.ts", "docs/api/typescript/authoring.d.ts"],
];
let total = 0;
for (const [label, source, snapshot] of snapshots) {
  const actual = exportedNames(resolve(root, source));
  const approved = exportedNames(resolve(root, snapshot));
  assert.deepEqual(actual, approved, `TypeScript ${label} exports differ from ${snapshot}`);
  total += actual.length;
}
console.log(`TypeScript API snapshots: ${total} exports match`);

function exportedNames(path) {
  const source = readFileSync(path, "utf8");
  const names = new Set();
  for (const match of source.matchAll(
    /^export\s+(?:declare\s+)?(?:async\s+)?(?:interface|type|class|function)\s+([A-Za-z0-9_]+)/gmu,
  ))
    names.add(match[1]);
  for (const match of source.matchAll(/^export\s+(?:type\s+)?\{([^}]+)\}/gmu)) {
    for (const item of match[1].split(","))
      names.add(
        item
          .trim()
          .split(/\s+as\s+/u)
          .at(-1),
      );
  }
  return [...names].sort();
}
