import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import {
  canonicalJSONStringify,
  compileContractSlice,
  supportedIRVersions,
  supportedLanguageVersions,
} from "../dist/index.js";

const root = fileURLToPath(new URL("../../..", import.meta.url));

async function fixture(relativePath) {
  return readFile(`${root}/${relativePath}`, "utf8");
}

const validPath = "fixtures/valid/basic-http.kdl";
const valid = await fixture(validPath);
const expectedIR = JSON.parse(await fixture("fixtures/expected-ir/basic-http.ir.json"));
const compiled = await compileContractSlice({ path: validPath, data: valid });
assert.deepEqual(compiled.diagnostics, [], "the representative valid fixture must compile without diagnostics");
assert.deepEqual(compiled.ir, expectedIR, "TypeScript and Go must lower the representative fixture to identical IR");
assert.equal(
  canonicalJSONStringify(compiled.ir),
  canonicalJSONStringify(expectedIR),
  "TypeScript and Go must serialize the representative IR identically",
);

assert.equal(
  canonicalJSONStringify({ z: -0, "\u{e000}": 123, "\u{10000}": [null, true], a: { present: null } }),
  '{"a":{"present":null},"z":0,"":123,"𐀀":[null,true]}',
  "canonical JSON ordering and number normalization must match the Go implementation",
);
assert.equal(
  canonicalJSONStringify({ html: "<script>&", controls: "\u0000\b\f\n\r\t", unicode: "\u2028" }),
  "{\"controls\":\"\\u0000\\b\\f\\n\\r\\t\",\"html\":\"<script>&\",\"unicode\":\"\u2028\"}",
  "canonical JSON string escaping must match the Go implementation",
);
assert.throws(() => canonicalJSONStringify(Number.POSITIVE_INFINITY), /non-finite/u);

const invalidFixtures = [
  "integer-version.kdl",
  "missing-document-version.kdl",
  "malformed-document-version.kdl",
  "missing-language-version.kdl",
  "malformed-language-version.kdl",
  "unknown-language-version.kdl",
];
const expectedDiagnostics = JSON.parse(await fixture("fixtures/expected-diagnostics/version-contract.json"));

for (const name of invalidFixtures) {
  const relativePath = `fixtures/invalid/${name}`;
  const result = await compileContractSlice({ path: relativePath, data: await fixture(relativePath) });
  assert.equal(result.ir, undefined, `${name} must not produce IR`);
  assert.deepEqual(result.diagnostics, expectedDiagnostics[name], `${name} must match the shared diagnostic fixture exactly`);
}

assert.deepEqual(supportedLanguageVersions(), ["2026-07-15"]);
assert.deepEqual(supportedIRVersions(), ["2026-07-15"]);

console.log("TypeScript contract slice matches Go fixtures and dated registries");
