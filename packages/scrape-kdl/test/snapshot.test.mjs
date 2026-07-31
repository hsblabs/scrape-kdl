import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { fileURLToPath } from "node:url";
import { compile, ExecutionError } from "../dist/index.js";

const root = fileURLToPath(new URL("../../..", import.meta.url));

test("Go and TypeScript share offline snapshot execution fixtures", async (t) => {
  const cases = JSON.parse(await readFile(`${root}/fixtures/snapshot/cases.json`, "utf8"));
  for (const testCase of cases) {
    await t.test(testCase.name, async () => {
      const compiled = await compile({ path: `${testCase.name}.kdl`, data: testCase.source });
      assert.deepEqual(
        compiled.diagnostics.filter((item) => item.severity === "error"),
        [],
      );
      assert.ok(compiled.program);
      let fetchCalls = 0;
      let policyCalls = 0;
      const operation = compiled.program.extractSnapshot(testCase.html, {
        async fetch() {
          fetchCalls++;
          throw new Error("snapshot execution attempted fetch");
        },
        urlPolicy() {
          policyCalls++;
          throw new Error("snapshot execution attempted URL policy");
        },
        ...(testCase.externalTransform === true
          ? {
              externalTransforms: {
                async decorate(_context, input) {
                  return `${input}!`;
                },
              },
            }
          : {}),
      });
      if (testCase.error !== undefined) {
        await assert.rejects(
          operation,
          (error) =>
            error instanceof ExecutionError && error.code === testCase.error.code && error.path === testCase.error.path,
        );
      } else assert.deepEqual(await operation, testCase.expected);
      assert.equal(fetchCalls, 0);
      assert.equal(policyCalls, 0);
    });
  }
});

test("snapshot execution preserves cancellation", async () => {
  const source = `extractor "snapshot-canceled" version="2026-07-15" language-version="2026-07-15" {
    source "html" { fetch mode="browser" url="https://example.invalid/" }
    field "title" type="string" required=#true { select "h1"; value "text" }
  }`;
  const compiled = await compile({ path: "snapshot-canceled.kdl", data: source });
  const controller = new AbortController();
  controller.abort(new DOMException("stopped", "AbortError"));
  await assert.rejects(
    compiled.program.extractSnapshot("<h1>Saved</h1>", { signal: controller.signal }),
    (error) => error instanceof ExecutionError && error.code === "E_EXECUTION_CANCELED" && error.path === "output",
  );
});
