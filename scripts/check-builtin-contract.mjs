import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { builtinCatalog } from "../packages/scrape-kdl/dist/authoring.js";
import { BUILTIN_CONTRACT } from "../packages/scrape-kdl/dist/builtin-contract.js";

const approved = JSON.parse(
  await readFile(new URL("../docs/spec/builtins-v0.1.contract.json", import.meta.url), "utf8"),
);
assert.deepEqual(BUILTIN_CONTRACT, approved, "TypeScript built-in registry differs from the normative contract");
const authoring = JSON.parse(
  await readFile(new URL("../docs/spec/builtins-v0.1.authoring.json", import.meta.url), "utf8"),
);
assert.deepEqual(
  builtinCatalog("2026-07-15"),
  authoring,
  "TypeScript authoring catalog differs from the normative contract",
);
console.log(`built-in contract: ${Object.keys(approved).length} definitions and authoring metadata match`);
