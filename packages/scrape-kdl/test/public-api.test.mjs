import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";
import {
  ExecutionError,
  compile,
  supportedIRVersions,
  supportedLanguageVersions,
  validate,
} from "../dist/index.js";
import { compileFile, validateFile } from "../dist/node.js";

const root = fileURLToPath(new URL("../../..", import.meta.url));
const validPath = `${root}/fixtures/valid/basic-http.kdl`;
const invalidPath = `${root}/fixtures/invalid/unknown-language-version.kdl`;

test("public in-memory compilation returns immutable metadata and IR", async () => {
  const [data, html, expected] = await Promise.all([
    readFile(validPath),
    readFile(`${root}/fixtures/html/basic-http.html`, "utf8"),
    readFile(`${root}/fixtures/expected-output/basic-http.json`, "utf8").then(JSON.parse),
  ]);
  const result = await compile({ path: "logical/basic-http.kdl", data });
  assert.deepEqual(result.diagnostics, []);
  assert.equal(result.program.metadata.name, "basic-http");
  assert.deepEqual(result.program.metadata.capabilities, ["http.fetch"]);
  assert.deepEqual(result.program.descriptor, {
    source: {
      fetchMode: "http",
      urlTemplate: "https://example.invalid/{id}",
      sessionPolicy: "none",
    },
  });
  assert.equal(result.program.ir.irVersion, "2026-07-15");
  assert.ok(Object.isFrozen(result.program.metadata));
  assert.ok(Object.isFrozen(result.program.descriptor));
  assert.ok(Object.isFrozen(result.program.descriptor.source));
  assert.ok(Object.isFrozen(result.program.metadata.files));
  assert.ok(Object.isFrozen(result.program.ir));
  assert.throws(() => { result.program.metadata.capabilities.push("mutated"); }, TypeError);
  assert.throws(() => { result.program.descriptor.source.urlTemplate = "mutated"; }, TypeError);
  assert.deepEqual(await result.program.extract({ id: "fixture" }, {
    fetch: async () => new Response(html, { status: 200, headers: { "content-type": "text/html; charset=utf-8" } }),
  }), expected);
});

test("public validation and compatibility registries retain exact diagnostics", async () => {
  const data = await readFile(invalidPath);
  const diagnostics = await validate({ path: invalidPath, data });
  assert.deepEqual(diagnostics.map((item) => item.code), ["E_LANGUAGE_VERSION_UNSUPPORTED"]);
  assert.deepEqual(supportedLanguageVersions(), ["2026-07-15"]);
  assert.deepEqual(supportedIRVersions(), ["2026-07-15"]);
  assert.throws(() => { supportedIRVersions().push("mutated"); }, TypeError);
});

test("Node entry point compiles and validates files", async () => {
  const compiled = await compileFile(validPath);
  assert.equal(compiled.program.metadata.name, "basic-http");
  assert.deepEqual(await validateFile(invalidPath).then((items) => items.map((item) => item.code)), ["E_LANGUAGE_VERSION_UNSUPPORTED"]);
});

test("abort and execution error surfaces are stable", async () => {
  const controller = new AbortController();
  controller.abort(new Error("stop"));
  await assert.rejects(compile({ path: "aborted.kdl", data: "" }, { signal: controller.signal }), /stop/u);
  const cause = new Error("cause");
  const error = new ExecutionError("E_TIMEOUT", "timed out", { path: "output.title", cause });
  assert.equal(error.name, "ExecutionError");
  assert.equal(error.code, "E_TIMEOUT");
  assert.equal(error.path, "output.title");
  assert.equal(error.cause, cause);
});
