import assert from "node:assert/strict";
import { access, readFile } from "node:fs/promises";
import { pathToFileURL } from "node:url";

const requiredChecklist = [
  "contracts-frozen", "support-matrix-frozen", "migration-maintenance-release-notes",
  "supported-release-gate", "clean-package-dry-run", "publication-guarded",
];
export function validateRCState(state, now = Date.now()) {
  assert.equal(state.schemaVersion, "2026-07-15");
  assert.equal(state.prePublication.status, "passed");
  assert.deepEqual(state.prePublication.checklist.map(({ id }) => id).sort(), [...requiredChecklist].sort());
  for (const item of state.prePublication.checklist) {
    assert.equal(item.status, "passed", `${item.id} is not complete`);
    assert.ok(item.evidence.length > 0, `${item.id} has no evidence`);
  }
  assert.equal(state.candidatePeriod.requiredConsecutiveDays, 14);
  assert.ok(["ready", "active", "eligible"].includes(state.candidatePeriod.status));
  for (const blocker of state.candidatePeriod.blockers) {
    assert.match(blocker.id, /^RC-[0-9]{4}$/u);
    assert.ok(["open", "resolved"].includes(blocker.status));
    assert.ok(blocker.summary.length > 0 && blocker.openedAt !== undefined);
    if (blocker.status === "resolved") assert.ok(blocker.resolvedAt !== undefined);
  }
  if (state.candidatePeriod.status === "ready") {
    assert.equal(state.candidatePeriod.candidateTag, null);
    assert.equal(state.candidatePeriod.windowStartedAt, null);
    assert.equal(state.candidatePeriod.minimumEndsAt, null);
  } else {
    assert.match(state.candidatePeriod.candidateTag, /^v1\.0\.0-rc\.[1-9][0-9]*$/u);
    const start = Date.parse(state.candidatePeriod.windowStartedAt);
    const end = Date.parse(state.candidatePeriod.minimumEndsAt);
    assert.ok(Number.isFinite(start) && Number.isFinite(end));
    assert.equal(end - start, 14 * 24 * 60 * 60 * 1000, "candidate window must be exactly 14 days from its latest clean start");
    if (state.candidatePeriod.status === "eligible") {
      assert.ok(now >= end, "candidate period has not reached 14 consecutive days");
      assert.ok(!state.candidatePeriod.blockers.some(({ status }) => status === "open"), "eligible candidate has an open blocker");
    }
  }
  assert.deepEqual(state.publication, {
    ownerApproved: false, repositoryPublic: false, preV1Published: false, stablePublished: false,
  });
}

async function main() {
  const state = JSON.parse(await readFile("release/rc-state.json", "utf8"));
  validateRCState(state);
  for (const path of [
    "docs/pre-publication-checklist.md", "docs/rc-operations.md", "docs/migration-to-v1.md",
    "docs/maintenance-policy.md", "docs/releases/v1.0.0.md", "docs/support-matrix.json",
  ]) await access(path);
  console.log(`RC state: pre-publication passed; candidate period ${state.candidatePeriod.status}; publication remains owner-gated`);
}

if (process.argv[1] !== undefined && import.meta.url === pathToFileURL(process.argv[1]).href) await main();
