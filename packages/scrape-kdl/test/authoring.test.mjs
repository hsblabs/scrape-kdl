import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { builtinCatalog, callBuiltin, supportedBuiltinCatalogVersions, write } from "../dist/authoring.js";
import { compile } from "../dist/index.js";

const root = fileURLToPath(new URL("../../..", import.meta.url));
const languageVersion = "2026-07-15";

test("authoring tracer writes the shared fixture deterministically and compiles", async () => {
  const catalog = builtinCatalog(languageVersion);
  const normalize = catalog.builtins.find(({ name }) => name === "normalize-whitespace");
  const emptyToNull = catalog.builtins.find(({ name }) => name === "empty-to-null");
  const assertEnum = catalog.builtins.find(({ name }) => name === "assert-enum");
  const prepend = catalog.builtins.find(({ name }) => name === "prepend");
  assert.ok(normalize);
  assert.ok(emptyToNull);
  assert.ok(assertEnum);
  assert.ok(prepend);
  const document = {
    languageVersion,
    extractor: {
      name: "authoring-tracer",
      version: "2026-08-01",
      source: {
        fetchMode: "http",
        urlTemplate: "https://example.invalid/items/{item_id}",
        sessionPolicy: "optional",
      },
      inputs: [{ name: "item_id", type: "string", required: true }],
      members: [
        {
          kind: "field",
          name: "title",
          type: "string",
          required: true,
          selector: "h1",
          match: "one",
          value: { kind: "text" },
          transforms: [callBuiltin(normalize), callBuiltin(prepend, [], { value: 'prefix "quoted"\n' })],
          onError: "fail",
        },
        {
          kind: "collection",
          name: "items",
          selector: "ul > li",
          required: false,
          minItems: 1,
          onRowError: "skip",
          members: [
            {
              kind: "field",
              name: "label",
              type: "string?",
              required: false,
              selector: ".label",
              match: "first",
              value: { kind: "attribute", name: "data-label" },
              transforms: [callBuiltin(normalize), callBuiltin(emptyToNull), callBuiltin(assertEnum, ["known", null])],
              onError: "warn",
            },
          ],
        },
      ],
    },
  };

  const first = write(document);
  assert.equal(write(document), first);
  assert.equal(first, await readFile(`${root}/fixtures/authoring/tracer.kdl`, "utf8"));
  const result = await compile({ path: "authoring-tracer.kdl", data: first });
  assert.deepEqual(result.diagnostics, []);
  assert.equal(result.program?.metadata.name, "authoring-tracer");
});

test("authoring catalog matches the normative contract and is immutable", async () => {
  const catalog = builtinCatalog(languageVersion);
  const expected = JSON.parse(await readFile(`${root}/docs/spec/builtins-v0.1.authoring.json`, "utf8"));
  assert.deepEqual(catalog, expected);
  assert.deepEqual(supportedBuiltinCatalogVersions(), [languageVersion]);
  assert.ok(Object.isFrozen(supportedBuiltinCatalogVersions()));
  assert.ok(Object.isFrozen(catalog));
  assert.ok(Object.isFrozen(catalog.builtins));
  assert.ok(Object.isFrozen(catalog.builtins[0].namedArguments));
  assert.throws(() => builtinCatalog(""), RangeError);
  assert.throws(() => {
    catalog.builtins[0].name = "mutated";
  }, TypeError);
});

test("authoring writer rejects calls and states outside the selected contract", () => {
  const base = {
    languageVersion,
    extractor: {
      name: "invalid",
      version: "2026-08-01",
      source: { fetchMode: "http", urlTemplate: "https://example.invalid/", sessionPolicy: "none" },
      inputs: [],
      members: [
        {
          kind: "field",
          name: "value",
          type: "string",
          required: true,
          selector: "body",
          match: "one",
          value: { kind: "text" },
          transforms: [{ name: "private-transform", positional: [], named: {} }],
          onError: "fail",
        },
      ],
    },
  };
  assert.throws(() => write(base), /unknown built-in/u);
  const invalidMatch = structuredClone(base);
  invalidMatch.extractor.members[0].transforms = [];
  invalidMatch.extractor.members[0].match = "many";
  assert.throws(() => write(invalidMatch), /match must be one of/u);
  const invalidRequired = structuredClone(base);
  invalidRequired.extractor.members[0].transforms = [];
  invalidRequired.extractor.members[0].required = "yes";
  assert.throws(() => write(invalidRequired), /boolean value must be bool/u);
});
