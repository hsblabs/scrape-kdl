import assert from "node:assert/strict";
import { compareResults } from "./compare-conformance.mjs";

function result(implementation, value) {
  return {
    schemaVersion: "2026-07-15",
    manifestVersion: "2026-07-15",
    implementation,
    suite: implementation === "go" ? "pr" : "typescript-core",
    job: "core",
    status: "passed",
    cases: [
      {
        id: "valid.basic-http",
        status: "passed",
        observations: [{ kind: "ir", value }],
        differences: [],
      },
    ],
  };
}

const manifest = { schemaVersion: "2026-07-15", approvedDivergences: [] };
assert.deepEqual(compareResults(result("go", { value: 1 }), result("typescript", { value: 1 }), manifest), {
  comparisons: 1,
  cases: 1,
  differences: [],
});
assert.match(
  compareResults(result("go", { value: 1 }), result("typescript", { value: 2 }), manifest).differences[0],
  /unapproved/u,
);
assert.match(
  compareResults({ ...result("go", 1), cases: [] }, result("typescript", 1), manifest).differences[0],
  /missing Go result/u,
);

const approvedManifest = {
  ...manifest,
  approvedDivergences: [
    {
      id: "DIV-0001",
      case: "valid.basic-http",
      observation: "ir",
      implementations: ["go", "typescript"],
      contractExclusion: "test-only",
      rationale: "exercise comparator",
      owner: "test",
    },
  ],
};
assert.deepEqual(
  compareResults(result("go", { value: 1 }), result("typescript", { value: 2 }), approvedManifest).differences,
  [],
);

console.log("conformance comparator rejects unapproved differences and accepts explicit records");
