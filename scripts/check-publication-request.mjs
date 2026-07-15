import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

const [kind, tag, channel, confirmation, ownerApproval] = process.argv.slice(2);
assert.ok(["core", "rod"].includes(kind), "kind must be core or rod");
assert.ok(["rc", "stable"].includes(channel), "channel must be rc or stable");
const coreTag = kind === "rod" ? tag?.replace(/^adapters\/rod\//u, "") : tag;
assert.match(coreTag ?? "", channel === "rc" ? /^v1\.0\.0-rc\.[1-9][0-9]*$/u : /^v1\.0\.0$/u);
assert.equal(confirmation, `PUBLISH ${tag}`, "publication confirmation does not exactly match the requested tag");
assert.equal(ownerApproval, `APPROVE ${tag}`, "protected environment approval secret does not authorize the requested tag");
const state = JSON.parse(await readFile("release/rc-state.json", "utf8"));
assert.equal(state.prePublication.status, "passed");
assert.equal(state.publication.ownerApproved, false, "approval is represented by the protected workflow environment, not a committed boolean");
assert.equal(state.publication.stablePublished, false);
if (channel === "stable") assert.equal(state.candidatePeriod.status, "eligible", "stable publication requires a completed blocker-free candidate period");
else assert.ok(["ready", "active", "eligible"].includes(state.candidatePeriod.status));
console.log(`publication request validated for ${tag}; protected environment approval is still required`);
