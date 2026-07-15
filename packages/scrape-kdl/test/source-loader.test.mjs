import assert from "node:assert/strict";
import { mkdtemp, readFile, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { compile } from "../dist/index.js";
import { compileFile } from "../dist/node.js";
import { loadSourceGraph } from "../dist/source-loader.js";

const root = fileURLToPath(new URL("../../..", import.meta.url));
const minimalExtractor = `extractor "entry" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.invalid/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`;
const minimalModule = `module "common" version="2026-07-15" language-version="2026-07-15" {}`;

test("the injected loader resolves imports lexically with stable host context", async () => {
  const requested = [];
  const loader = {
    async load(path, context) {
      requested.push({ path, fromPath: context.fromPath, signal: context.signal });
      assert.equal(path, "spec/modules/common.kdl");
      return minimalModule;
    },
  };
  const source = {
    path: "spec/main.kdl",
    data: `import "./modules/../modules/common.kdl" as="common"\n${minimalExtractor}`,
  };
  const graph = await loadSourceGraph(source, { loader });
  assert.deepEqual(graph.diagnostics, []);
  assert.equal(graph.documents.size, 2);
  assert.equal(graph.entry.displayPath, "main.kdl");
  assert.equal(graph.entry.imports.get("common").displayPath, "modules/common.kdl");
  assert.deepEqual(requested, [{ path: "spec/modules/common.kdl", fromPath: "spec/main.kdl", signal: undefined }]);

  const compiled = await compile(source, { loader });
  assert.deepEqual(compiled.diagnostics, []);
  assert.equal(compiled.program.metadata.name, "entry");
});

test("the shared import-cycle fixture matches the exact Go diagnostic", async () => {
  const sourcePath = "fixtures/invalid/import-cycle.kdl";
  const calls = [];
  const result = await compile(
    { path: sourcePath, data: await readFile(`${root}/${sourcePath}`) },
    { loader: { async load(path) { calls.push(path); return readFile(`${root}/${path}`); } } },
  );
  const expected = JSON.parse(await readFile(`${root}/fixtures/expected-diagnostics/go-contract.json`, "utf8"))["import-cycle.kdl"];
  assert.equal(result.program, undefined);
  assert.deepEqual(result.diagnostics, expected);
  assert.deepEqual(calls, ["fixtures/invalid/cycle/a.kdl", "fixtures/invalid/cycle/b.kdl"]);
});

test("missing, duplicate, wrong-kind, and non-relative imports are rejected before execution", async () => {
  const missing = await loadSourceGraph({
    path: "spec/main.kdl",
    data: `import "./missing.kdl" as="missing"\n${minimalExtractor}`,
  });
  assert.deepEqual(missing.diagnostics.map(({ code, message, span }) => [code, message, span.file]), [
    ["E_KDL_SYNTAX", "read \"missing.kdl\": no source loader configured", "missing.kdl"],
  ]);

  let remoteLoads = 0;
  const remote = await loadSourceGraph({
    path: "main.kdl",
    data: `import "https://example.invalid/module.kdl" as="remote"\nimport "/absolute.kdl" as="absolute"\n${minimalExtractor}`,
  }, { loader: { async load() { remoteLoads++; return minimalModule; } } });
  assert.equal(remoteLoads, 0);
  assert.deepEqual(remote.diagnostics.map((item) => item.code), ["E_REMOTE_IMPORT_UNSUPPORTED", "E_REMOTE_IMPORT_UNSUPPORTED"]);

  const duplicate = await loadSourceGraph({
    path: "main.kdl",
    data: `import "./one.kdl" as="same"\nimport "./two.kdl" as="same"\n${minimalExtractor}`,
  }, { loader: { async load() { return minimalModule; } } });
  assert.deepEqual(duplicate.diagnostics.map((item) => item.code), ["E_DUPLICATE_SYMBOL"]);

  const wrongKind = await loadSourceGraph({
    path: "main.kdl",
    data: `import "./extractor.kdl" as="wrong"\n${minimalExtractor}`,
  }, { loader: { async load() { return minimalExtractor; } } });
  assert.deepEqual(wrongKind.diagnostics.map((item) => item.code), ["E_IMPORT_KIND"]);
});

test("loader cancellation propagates as an AbortError instead of a source diagnostic", async () => {
  const controller = new AbortController();
  const pending = compile({
    path: "main.kdl",
    data: `import "./slow.kdl" as="slow"\n${minimalExtractor}`,
  }, {
    signal: controller.signal,
    loader: {
      load(_path, context) {
        return new Promise((_resolve, reject) => {
          context.signal.addEventListener("abort", () => reject(context.signal.reason), { once: true });
        });
      },
    },
  });
  controller.abort(new DOMException("stopped", "AbortError"));
  await assert.rejects(pending, { name: "AbortError", message: "stopped" });
});

test("invalid UTF-8 bytes match the shared stable syntax diagnostic", async () => {
  const testCase = JSON.parse(await readFile(`${root}/fixtures/parser/invalid-utf8.json`, "utf8"));
  const graph = await loadSourceGraph({ path: testCase.path, data: new Uint8Array(testCase.bytes) });
  assert.deepEqual(graph.diagnostics, testCase.diagnostics);
});

test("the Node entry point supplies explicit filesystem import loading", async () => {
  const directory = await mkdtemp(join(tmpdir(), "scrape-kdl-loader-"));
  try {
    await writeFile(join(directory, "common.kdl"), minimalModule);
    await writeFile(join(directory, "main.kdl"), `import "./common.kdl" as="common"\n${minimalExtractor}`);
    const result = await compileFile(join(directory, "main.kdl"));
    assert.deepEqual(result.diagnostics, []);
    assert.equal(result.program.metadata.name, "entry");
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test("shared import cases match Go diagnostics exactly", async () => {
  const cases = JSON.parse(await readFile(`${root}/fixtures/imports/cases.json`, "utf8"));
  for (const testCase of cases) {
    const graph = await loadSourceGraph({ path: testCase.path, data: testCase.source }, {
      loader: {
        async load(path) {
          const source = testCase.files[path];
          if (source === undefined) throw new Error("missing source");
          return source;
        },
      },
    });
    assert.deepEqual(graph.diagnostics, testCase.diagnostics, testCase.name);
  }
});
