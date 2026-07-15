import assert from "node:assert/strict";
import { readdir, readFile } from "node:fs/promises";
import { join, relative } from "node:path";
import { compile } from "../packages/scrape-kdl/dist/index.js";
import { canonicalJSONStringify } from "../packages/scrape-kdl/dist/canonical-json.js";
import { executeHTML } from "../packages/scrape-kdl/dist/runtime.js";

const root = process.cwd();
const examplesRoot = join(root, "examples");
let examples = 0;
let executions = 0;

for (const entry of await readdir(examplesRoot, { withFileTypes: true })) {
  if (!entry.isDirectory()) continue;
  const directory = join(examplesRoot, entry.name);
  const manifest = JSON.parse(await readFile(join(directory, "example.json"), "utf8"));
  const sourcePath = join(directory, manifest.source);
  const source = await readFile(sourcePath);
  const compiled = await compile(
    { path: relative(root, sourcePath), data: source },
    { loader: { async load(path) { return readFile(join(root, path)); } } },
  );
  assert.ok(compiled.program, `${entry.name}: ${JSON.stringify(compiled.diagnostics)}`);
  assert.equal(compiled.program.metadata.name, manifest.name);
  assert.equal(compiled.program.metadata.languageVersion, manifest.languageVersion);
  assert.equal(compiled.program.metadata.irVersion, manifest.irVersion);
  const expectedIR = JSON.parse(await readFile(join(directory, manifest.expected.ir), "utf8"));
  assert.equal(canonicalJSONStringify(compiled.program.ir), canonicalJSONStringify(expectedIR), `${entry.name}: IR differs`);
  const inputs = JSON.parse(await readFile(join(directory, manifest.inputs), "utf8"));
  const expectedOutput = JSON.parse(await readFile(join(directory, manifest.expected.output), "utf8"));
  const selected = manifest.executions.filter(({ implementation }) => implementation === "typescript");
  assert.ok(selected.length > 0, `${entry.name}: at least one TypeScript execution is required`);
  for (const execution of selected) {
    assert.notEqual(execution.mode, "browser", `${entry.name}: browser entries run in the official adapter gate`);
    const html = await readFile(join(directory, execution.fixture));
    const result = execution.mode === "offline-html"
      ? await executeHTML(compiled.program.ir, html.toString())
      : await compiled.program.extract(inputs, {
        fetch: fixtureFetch(html),
        urlPolicy(_context, url) { assert.equal(url.hostname, "example.invalid"); },
      });
    assert.equal(canonicalJSONStringify(result), canonicalJSONStringify(expectedOutput), `${entry.name}/${execution.mode}: output differs`);
    executions++;
  }
  examples++;
}

assert.ok(examples > 0);
console.log(`TypeScript examples: ${examples} example(s), ${executions} execution(s) passed`);

function fixtureFetch(html) {
  return async () => new Response(html, { status: 200, headers: { "content-type": "text/html; charset=utf-8" } });
}
