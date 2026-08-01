import assert from "node:assert/strict";
import { fileURLToPath } from "node:url";
import { canonicalJSONStringify } from "../dist/canonical-json.js";
import { supportedIRVersions, supportedLanguageVersions } from "../dist/index.js";
import { runTypeScriptConformance } from "./manifest-runner.mjs";

const root = fileURLToPath(new URL("../../..", import.meta.url));

const manifestResult = await runTypeScriptConformance({ root });
assert.equal(manifestResult.status, "passed", JSON.stringify(manifestResult.cases.filter((testCase) => testCase.status === "failed"), null, 2));
assert.equal(manifestResult.cases.length, 18, "the manifest must retain the complete shared TypeScript compiler suite");
assert.deepEqual(manifestResult.cases.map((testCase) => testCase.id), [
  "valid.basic-http",
  "valid.race-detail",
  "valid.document-version-advance",
  "valid.browser-js",
  "valid.rod-browser-e2e",
  "valid.authoring-tracer",
  "invalid.duplicate-property",
  "invalid.http-js",
  "invalid.import-cycle",
  "invalid.timeout-overflow",
  "invalid.regex-lookahead",
  "invalid.transform-type-mismatch",
  "invalid.integer-version",
  "invalid.missing-document-version",
  "invalid.malformed-document-version",
  "invalid.missing-language-version",
  "invalid.malformed-language-version",
  "invalid.unknown-language-version",
]);

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

assert.deepEqual(supportedLanguageVersions(), ["2026-07-15"]);
assert.deepEqual(supportedIRVersions(), ["2026-07-15"]);

console.log("TypeScript semantic compiler matches Go fixtures and dated registries");
