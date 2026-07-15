import { readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";
import { isDeepStrictEqual, parseArgs } from "node:util";

export function compareResults(goResult, typeScriptResult, manifest) {
  const differences = [
    ...validateResult(goResult, "go", manifest.schemaVersion),
    ...validateResult(typeScriptResult, "typescript", manifest.schemaVersion),
  ];
  if (goResult.status !== "passed" || typeScriptResult.status !== "passed") {
    differences.push("an implementation result contains unapproved expected-artifact differences");
  }
  const goCases = new Map(goResult.cases.map((testCase) => [testCase.id, testCase]));
  let comparisons = 0;
  for (const typeScriptCase of typeScriptResult.cases) {
    const goCase = goCases.get(typeScriptCase.id);
    if (goCase === undefined) {
      differences.push(`${typeScriptCase.id}: missing Go result`);
      continue;
    }
    const goObservations = new Map(goCase.observations.map((observation) => [observation.kind, observation.value]));
    for (const observation of typeScriptCase.observations) {
      comparisons++;
      if (!goObservations.has(observation.kind)) {
        differences.push(`${typeScriptCase.id}/${observation.kind}: missing Go observation`);
        continue;
      }
      if (isDeepStrictEqual(goObservations.get(observation.kind), observation.value)) continue;
      const approval = manifest.approvedDivergences.find((candidate) =>
        candidate.case === typeScriptCase.id
        && candidate.observation === observation.kind
        && candidate.implementations.includes("go")
        && candidate.implementations.includes("typescript"),
      );
      if (approval === undefined) differences.push(`${typeScriptCase.id}/${observation.kind}: unapproved Go/TypeScript difference`);
    }
  }
  return { comparisons, cases: typeScriptResult.cases.length, differences };
}

function validateResult(result, implementation, manifestVersion) {
  const problems = [];
  if (result.schemaVersion !== "2026-07-15") problems.push(`${implementation}: invalid result schemaVersion`);
  if (result.manifestVersion !== manifestVersion) problems.push(`${implementation}: result manifestVersion drift`);
  if (result.implementation !== implementation) problems.push(`${implementation}: result implementation drift`);
  if (!(["passed", "failed"].includes(result.status))) problems.push(`${implementation}: invalid result status`);
  if (!Array.isArray(result.cases)) return [...problems, `${implementation}: result cases must be an array`];
  for (const testCase of result.cases) {
    if (typeof testCase.id !== "string" || !["passed", "failed"].includes(testCase.status)) problems.push(`${implementation}: invalid case identity or status`);
    if (!Array.isArray(testCase.observations) || !Array.isArray(testCase.differences)) problems.push(`${implementation}/${testCase.id}: observations and differences must be arrays`);
    for (const observation of testCase.observations ?? []) {
      if (!["diagnostics", "ir", "runtime", "browser"].includes(observation.kind) || !("value" in observation)) {
        problems.push(`${implementation}/${testCase.id}: invalid observation`);
      }
    }
  }
  return problems;
}

async function main() {
  let options;
  try {
    options = parseArgs({
      options: {
        go: { type: "string" },
        typescript: { type: "string" },
        manifest: { type: "string", default: "conformance/manifest.json" },
      },
      allowPositionals: false,
      strict: true,
    }).values;
  } catch (error) {
    console.error(`compare-conformance: ${error.message}`);
    process.exitCode = 2;
    return;
  }
  if (options.go === undefined || options.typescript === undefined) {
    console.error("compare-conformance: --go and --typescript result paths are required");
    process.exitCode = 2;
    return;
  }
  try {
    const [goResult, typeScriptResult, manifest] = await Promise.all([
      readJSON(options.go),
      readJSON(options.typescript),
      readJSON(options.manifest),
    ]);
    const comparison = compareResults(goResult, typeScriptResult, manifest);
    if (comparison.differences.length > 0) {
      console.error(`compare-conformance: ${comparison.differences.length} difference(s)\n- ${comparison.differences.join("\n- ")}`);
      process.exitCode = 1;
      return;
    }
    console.log(`cross-language conformance: ${comparison.cases} cases and ${comparison.comparisons} observations matched`);
  } catch (error) {
    console.error(`compare-conformance: ${error.message}`);
    process.exitCode = 1;
  }
}

async function readJSON(path) {
  return JSON.parse(await readFile(path, "utf8"));
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) await main();
