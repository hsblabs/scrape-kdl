import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { validateJSONSchema } from "../../../scripts/json-schema-validator.mjs";

test("frozen IR schema validator accepts the golden and rejects structural drift", async () => {
  const [schema, golden] = await Promise.all([
    readFile(new URL("../../../docs/ir/schema.json", import.meta.url), "utf8").then(JSON.parse),
    readFile(new URL("../../../fixtures/expected-ir/basic-http.ir.json", import.meta.url), "utf8").then(JSON.parse),
  ]);
  assert.deepEqual(validateJSONSchema(schema, golden), []);
  assert.match(validateJSONSchema(schema, { ...golden, irVersion: "future" })[0], /schema variant|constant/u);
  assert.match(validateJSONSchema(schema, { ...golden, unexpected: true })[0], /additional property/u);
});
