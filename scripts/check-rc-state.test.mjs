import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import { validateRCState } from "./check-rc-state.mjs";

const prepared = JSON.parse(await readFile("release/rc-state.json", "utf8"));
const day = 24 * 60 * 60 * 1000;

test("prepared state is valid without claiming an RC window", () => validateRCState(structuredClone(prepared)));

test("eligible state requires an exact completed 14-day blocker-free window", () => {
  const state = structuredClone(prepared);
  state.candidatePeriod = {
    status: "eligible",
    requiredConsecutiveDays: 14,
    candidateTag: "v1.0.0-rc.1",
    windowStartedAt: "2026-07-01T00:00:00Z",
    minimumEndsAt: "2026-07-15T00:00:00Z",
    blockers: [],
  };
  validateRCState(state, Date.parse("2026-07-15T00:00:00Z"));
  assert.throws(() => validateRCState(state, Date.parse("2026-07-14T23:59:59Z")), /has not reached 14 consecutive days/u);
  state.candidatePeriod.minimumEndsAt = "2026-07-14T00:00:00Z";
  assert.throws(() => validateRCState(state, Date.parse("2026-07-15T00:00:00Z")), /exactly 14 days/u);
});

test("an open blocker prevents eligibility", () => {
  const state = structuredClone(prepared);
  const start = Date.parse("2026-07-01T00:00:00Z");
  state.candidatePeriod = {
    status: "eligible",
    requiredConsecutiveDays: 14,
    candidateTag: "v1.0.0-rc.2",
    windowStartedAt: new Date(start).toISOString(),
    minimumEndsAt: new Date(start + 14 * day).toISOString(),
    blockers: [{ id: "RC-0001", status: "open", summary: "supported Chromium failure", openedAt: "2026-07-10T00:00:00Z" }],
  };
  assert.throws(() => validateRCState(state, start + 15 * day), /open blocker/u);
});
