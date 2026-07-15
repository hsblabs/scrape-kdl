# Release-candidate operations

The v1 candidate must remain free of unresolved release blockers for 14 consecutive 24-hour periods. Repository preparation does not start that clock and does not imply publication approval.

## Start the period

After the project owner approves a pre-v1 publication, create and verify the candidate artifacts through the protected manual workflows. In the same reviewed change, update `release/rc-state.json`:

- set `candidatePeriod.status` to `active`;
- set `candidateTag` to the published `v1.0.0-rc.N` tag;
- set `windowStartedAt` to the UTC publication time;
- set `minimumEndsAt` to exactly 14 days later.

Run `make rc-state`; the validator checks the tag shape and exact interval. Record daily release-gate, supported-browser, and clean-install workflow URLs in the Issue #18 log. Firefox and WebKit observations stay separately visible and are blockers only when they expose a supported portable-contract defect.

## Handle blockers

A release blocker is any supported-target failure, portable conformance difference, security-default regression, public-contract incompatibility, corrupt artifact, or clean-install failure. Add it immediately to `candidatePeriod.blockers` with a stable `RC-NNNN` ID, summary, UTC open time, and `open` status.

After the fix is reviewed and every gate passes, mark the blocker `resolved`, record its UTC resolution time, and restart `windowStartedAt` from that resolution. Set `minimumEndsAt` to exactly 14 days after the restarted time. The elapsed time before a blocker does not count toward the new window.

## Become publication-eligible

Only after the current time is at or beyond `minimumEndsAt`, no blocker is open, and supported-target workflows are green may a reviewed change set `candidatePeriod.status` to `eligible`. `make rc-state` enforces these conditions. Eligibility still does not authorize stable publication.

Stable publication requires a separate explicit project-owner decision, the protected `v1-publication` environment, an exact workflow confirmation string, and a temporary environment secret `PUBLICATION_APPROVAL` whose value is `APPROVE <exact-tag>`. An absent or stale secret fails closed. Remove it after the run and keep Issue #18 open until every public artifact is verified.
