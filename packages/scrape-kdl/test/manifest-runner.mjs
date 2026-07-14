import { parseArgs } from "node:util";
import { readFile, writeFile } from "node:fs/promises";
import { fileURLToPath, pathToFileURL } from "node:url";
import { canonicalJSONStringify } from "../dist/canonical-json.js";
import { compileContractSlice } from "../dist/compiler.js";

export async function runTypeScriptSlice({ root, manifestPath = "conformance/manifest.json", suite = "typescript-slice", job = "core" }) {
  const manifest = JSON.parse(await readFile(`${root}/${manifestPath}`, "utf8"));
  if (manifest.suites[suite] === undefined) throw new Error(`unknown suite ${JSON.stringify(suite)}`);
  const selected = manifest.cases.filter((testCase) =>
    testCase.suites.includes(suite) && executionFor(testCase, job) !== undefined,
  );
  if (selected.length === 0) throw new Error(`suite ${JSON.stringify(suite)} selects no typescript/${job} executions`);
  const result = {
    schemaVersion: "2026-07-15",
    manifestVersion: manifest.schemaVersion,
    implementation: "typescript",
    suite,
    job,
    status: "passed",
    cases: [],
  };
  for (const testCase of selected) {
    const caseResult = await runCase(root, manifest, testCase, executionFor(testCase, job));
    approveDifferences(manifest, caseResult);
    caseResult.status = caseResult.differences.some((difference) => difference.approvedBy === undefined) ? "failed" : "passed";
    if (caseResult.status === "failed") result.status = "failed";
    result.cases.push(caseResult);
  }
  return result;
}

async function runCase(root, manifest, testCase, execution) {
  const sourcePath = testCase.artifacts.find((artifact) => artifact.role === "source")?.path;
  if (sourcePath === undefined) throw new Error(`${testCase.id} has no source artifact`);
  const compiled = await compileContractSlice(
    { path: sourcePath, data: await readFile(`${root}/${sourcePath}`) },
    { loader: { async load(path) { return readFile(`${root}/${path}`); } } },
  );
  const result = { id: testCase.id, status: "passed", observations: [], differences: [] };
  if (execution.stages.includes("validate")) {
    result.observations.push({ kind: "diagnostics", value: compiled.diagnostics });
  }
  if (testCase.expectations.outcome === "valid") {
    if (compiled.ir === undefined || compiled.diagnostics.some((item) => item.severity === "error")) {
      result.differences.push({ kind: "diagnostics", message: "expected valid program without error diagnostics" });
      return result;
    }
  } else {
    if (compiled.ir !== undefined || !compiled.diagnostics.some((item) => item.severity === "error")) {
      result.differences.push({ kind: "diagnostics", message: "expected invalid program with error diagnostics and no IR" });
    }
    const reference = testCase.expectations.diagnostics;
    const document = JSON.parse(await readFile(`${root}/${reference.artifact}`, "utf8"));
    if (!sameJSON(compiled.diagnostics, document[reference.key])) {
      result.differences.push({ kind: "diagnostics", message: `diagnostics differ from ${reference.artifact}#${reference.key}` });
    }
    return result;
  }
  if (execution.stages.includes("ir")) {
    result.observations.push({ kind: "ir", value: compiled.ir });
    const expected = JSON.parse(await readFile(`${root}/${testCase.expectations.ir}`, "utf8"));
    if (!sameJSON(compiled.ir, expected)) {
      result.differences.push({ kind: "ir", message: `value differs from ${testCase.expectations.ir}` });
    }
  }
  return result;
}

function executionFor(testCase, job) {
  return testCase.executions.find((execution) => execution.implementation === "typescript" && execution.job === job);
}

function sameJSON(left, right) {
  return canonicalJSONStringify(left) === canonicalJSONStringify(right);
}

function approveDifferences(manifest, result) {
  for (const difference of result.differences) {
    const divergence = manifest.approvedDivergences.find((candidate) =>
      candidate.case === result.id
      && candidate.observation === difference.kind
      && candidate.implementations.includes("typescript"),
    );
    if (divergence !== undefined) difference.approvedBy = divergence.id;
  }
}

async function main() {
  const root = fileURLToPath(new URL("../../..", import.meta.url)).replace(/\/$/u, "");
  let options;
  try {
    options = parseArgs({
      options: {
        help: { type: "boolean", short: "h" },
        manifest: { type: "string", default: "conformance/manifest.json" },
        suite: { type: "string", default: "typescript-slice" },
        job: { type: "string", default: "core" },
        output: { type: "string", short: "o", default: "-" },
      },
      allowPositionals: false,
      strict: true,
    }).values;
  } catch (error) {
    console.error(`typescript-conformance: ${error.message}`);
    console.error("Run with --help for usage.");
    process.exitCode = 2;
    return;
  }
  if (options.help) {
    console.log(`Run the bounded TypeScript implementation through the shared conformance manifest.

Usage:
  node packages/scrape-kdl/test/manifest-runner.mjs [flags]

Flags:
  -h, --help             show this help
      --manifest PATH    manifest path (default: conformance/manifest.json)
      --suite NAME       focused suite (default: typescript-slice)
      --job NAME         manifest job (default: core)
  -o, --output PATH      result path, or - for stdout

The command writes only the stable JSON result to stdout. Errors go to stderr.`);
    return;
  }
  try {
    const result = await runTypeScriptSlice({ root, manifestPath: options.manifest, suite: options.suite, job: options.job });
    const encoded = `${JSON.stringify(result, null, 2)}\n`;
    if (options.output === "-") process.stdout.write(encoded);
    else await writeFile(options.output, encoded);
    if (result.status === "failed") {
      const failed = result.cases.filter((testCase) => testCase.status === "failed").length;
      console.error(`typescript-conformance: ${failed} case(s) have unapproved differences; inspect the JSON result`);
      process.exitCode = 1;
    }
  } catch (error) {
    console.error(`typescript-conformance: ${error.message}`);
    process.exitCode = 1;
  }
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) await main();
