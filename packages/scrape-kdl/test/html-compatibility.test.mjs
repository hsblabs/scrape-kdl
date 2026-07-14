import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { attribute, innerHTML, parseHTML, queryAll, textContent } from "../dist/dom.js";

const root = new URL("../../../", import.meta.url);

test("parse5 has zero unapproved differences in the pinned HTML compatibility manifest", async (t) => {
  const manifest = JSON.parse(await readFile(new URL("fixtures/html-compat/manifest.json", root), "utf8"));
  assert.equal(manifest.schemaVersion, "2026-07-15");
  assert.deepEqual(manifest.approvedDivergences, []);
  assert.match(manifest.upstream.revision, /^[a-f0-9]{40}$/u);
  for (const fixture of manifest.cases) {
    await t.test(fixture.id, async () => {
      assert.equal(fixture.decodedEncoding, "utf-8");
      assert.equal(fixture.parserMode, "document");
      const document = parseHTML(await readFile(new URL(fixture.input, root), "utf8"));
      for (const observation of fixture.observations) {
        const nodes = queryAll(document, observation.selector);
        if (observation.text !== undefined) assert.deepEqual(nodes.map(textContent), observation.text, `${fixture.id}/${observation.selector}/text`);
        if (observation.innerHTML !== undefined) assert.deepEqual(nodes.map(innerHTML), observation.innerHTML, `${fixture.id}/${observation.selector}/innerHTML`);
        if (observation.attributes !== undefined) {
          assert.deepEqual(nodes.map((node, index) => Object.fromEntries(Object.keys(observation.attributes[index]).map((name) => [name, attribute(node, name) ?? null]))), observation.attributes, `${fixture.id}/${observation.selector}/attributes`);
        }
      }
    });
  }
});
