import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { compile, ExecutionError } from "../dist/index.js";

const root = new URL("../../../", import.meta.url);
const source = `extractor "charset-compat" version="2026-07-15" language-version="2026-07-15" {
  source "html" { fetch mode="http" url="https://example.test/" }
  field "title" type="string" required=#true { select "h1"; value "text" }
}`;

test("Go and TypeScript share the pinned charset compatibility manifest", async (t) => {
  const manifest = JSON.parse(await readFile(new URL("fixtures/html-compat/charset-manifest.json", root), "utf8"));
  assert.equal(manifest.schemaVersion, "2026-07-15");
  assert.ok(manifest.cases.length > 0);
  const compiled = await compile({ path: "charset-compat.kdl", data: source });
  assert.ok(compiled.program, JSON.stringify(compiled.diagnostics, null, 2));

  for (const fixture of manifest.cases) {
    await t.test(fixture.id, async () => {
      const body = Uint8Array.from(Buffer.from(fixture.bytesBase64, "base64"));
      const extract = () => compiled.program.extract({}, {
        fetch: async () => new Response(body, { headers: { "content-type": fixture.contentType } }),
      });
      if (fixture.expectedError !== undefined) {
        await assert.rejects(extract(), (error) => error instanceof ExecutionError && error.code === fixture.expectedError);
        return;
      }
      const result = await extract();
      assert.equal(result.value.title, fixture.expectedText);
    });
  }
});
