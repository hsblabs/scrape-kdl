import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";

const root = process.cwd();
const actual = exportedNames(resolve(root, "packages/scrape-kdl/src/public-api.ts"));
const approved = exportedNames(resolve(root, "docs/api/typescript/index.d.ts"));
assert.deepEqual(actual, approved, "TypeScript package exports differ from docs/api/typescript/index.d.ts");
console.log(`TypeScript API snapshot: ${actual.length} exports match`);

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
