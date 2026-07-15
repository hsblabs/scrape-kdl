import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { readFile } from "node:fs/promises";
import { performance } from "node:perf_hooks";
import { parseHTML, queryAll } from "../packages/scrape-kdl/dist/dom.js";
import { compile } from "../packages/scrape-kdl/dist/index.js";

const root = process.cwd();
const measureOnly = process.argv.includes("--measure");
const [source, html, inputsText, irText, outputText] = await Promise.all([
  readFile("examples/basic-http/extractor.kdl"),
  readFile("examples/basic-http/page.html"),
  readFile("examples/basic-http/inputs.json", "utf8"),
  readFile("examples/basic-http/expected-ir.json", "utf8"),
  readFile("examples/basic-http/expected-output.json", "utf8"),
]);
const inputs = JSON.parse(inputsText);
const compileWorkload = async () => {
  const result = await compile({ path: "extractor.kdl", data: source });
  assert.ok(result.program);
};
const prepared = await compile({ path: "extractor.kdl", data: source });
assert.ok(prepared.program);
const extractionWorkload = async () => {
  await prepared.program.extract(inputs, {
    fetch: async () => new Response(html, { headers: { "content-type": "text/html; charset=utf-8" } }),
  });
};
const selectorDocument = parseHTML(`<main>${'<i class="entry"></i>'.repeat(10_000)}</main>`);
const selectorAll = () => {
  assert.equal(queryAll(selectorDocument, ".entry").length, 10_000);
};
const selectorFirst = () => {
  assert.equal(queryAll(selectorDocument, ".entry", 1).length, 1);
};
const typescript = {
  compileRatio: await ratio(compileWorkload, async () => {
    JSON.parse(irText);
  }),
  extractRatio: await ratio(extractionWorkload, async () => {
    JSON.parse(outputText);
  }),
  selectorAllMillis: await medianDuration(selectorAll),
  selectorFirstMillis: await medianDuration(selectorFirst),
};
const go = JSON.parse(
  execFileSync("go", ["run", "./scripts/performanceprobe"], {
    cwd: root,
    encoding: "utf8",
    env: { ...process.env, GOTOOLCHAIN: "local" },
    stdio: ["ignore", "pipe", "inherit"],
  }),
);
const observed = { go, typescript };

if (measureOnly) {
  console.log(JSON.stringify(observed, null, 2));
} else {
  const contract = JSON.parse(await readFile("performance/baselines.json", "utf8"));
  for (const runtime of ["go", "typescript"]) {
    for (const workload of ["compileRatio", "extractRatio"]) {
      const actual = observed[runtime][workload];
      const gate = contract.runtimes[runtime][workload];
      assert.ok(
        actual <= gate.maximum,
        `${runtime}.${workload} ${actual.toFixed(3)} exceeds measured baseline ${gate.baseline} and maximum ${gate.maximum}`,
      );
      console.log(
        `performance: ${runtime}.${workload}=${actual.toFixed(3)} (baseline ${gate.baseline}, maximum ${gate.maximum})`,
      );
    }
    console.log(
      `performance: ${runtime}.selectorAllMillis=${observed[runtime].selectorAllMillis.toFixed(3)}, ` +
        `selectorFirstMillis=${observed[runtime].selectorFirstMillis.toFixed(3)} (10,000 elements; informational)`,
    );
  }
}

async function ratio(workload, calibration) {
  const workloadDuration = await medianDuration(workload);
  const calibrationDuration = await medianDuration(calibration);
  return workloadDuration / calibrationDuration;
}

async function medianDuration(operation) {
  for (let index = 0; index < 3; index++) await operation();
  const values = [];
  for (let sample = 0; sample < 7; sample++) {
    const started = performance.now();
    for (let iteration = 0; iteration < 12; iteration++) await operation();
    values.push((performance.now() - started) / 12);
  }
  return values.sort((left, right) => left - right)[3];
}
