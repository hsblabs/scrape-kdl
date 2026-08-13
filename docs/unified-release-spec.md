---
status: accepted
updated: 2026-08-01
owner: project owner
---

# Unified public release specification

## Goal

Publish the core Go module and CLI, `@hsblabs/scrape-kdl`,
`@hsblabs/scrape-kdl-playwright`, and the go-rod module and CLI from one
owner-initiated workflow run while preserving their dependency order and
immutable-version guarantees.

Private operational releases and specification-site publication are outside
this interface.

## Interface

The caller selects the default branch and supplies:

- `version`: SemVer without a leading `v`;
- `confirmation`: exactly `PUBLISH_RELEASE`.

No caller-visible switches select individual distribution surfaces or reorder
publication. The source commit is the workflow dispatch commit and must be the
default branch.

Before dispatch, the release preparation commit must contain the changelog and
release documentation plus go-rod metadata prepared for `v<version>`. Workspace
`package.json` versions remain `0.0.0-development`; npm versions and public
access are injected only into inspected staging archives.

## State transitions

| State | Invariant | Next transition |
|---|---|---|
| Requested | Version, confirmation, visibility, branch, and source are valid | Build and inspect artifacts |
| Prepared | Complete release gate passed; core/npm bundle matches the source | Enter protected publication |
| Core published | Annotated core tag points to the source; GitHub assets match | Publish core npm and await public Go resolution in parallel |
| Core visible | Public proxy resolves the exact core module | Verify and build go-rod |
| Core npm published | Core archive, version, integrity, and selected dist-tag match | Publish Playwright npm |
| Playwright npm published | Playwright archive, version, integrity, and selected dist-tag match | Finish npm branch |
| go-rod built | Clean-consumer verification passed and archives match the source | Publish go-rod |
| go-rod published | Annotated adapter tag points to the source; assets match | Wait for public Go resolution |
| Verified | Both Go modules and npm packages resolve and artifacts match | Finish successfully |

A failed transition leaves all completed immutable states intact. Rerunning the
same version is legal only from the same source and with the same artifacts.

## Failure and security rules

- No repository or registry write occurs before the protected authorization
  job and its dependent publication jobs.
- The protected approval job receives no write permission. Each publication job
  receives only the permission it needs: GitHub Release jobs use
  `contents: write`, npm jobs use `id-token: write`, and wait/verification jobs
  are read-only.
- The workflow refuses private-channel versions and non-public repositories.
- Existing tags must be annotated and peel to the dispatched source commit.
- Existing GitHub Release assets must byte-match locally inspected artifacts;
  missing assets may be attached, but differing or extra assets fail the run.
- Existing npm versions must have the same registry integrity as the inspected
  archive before they are skipped.
- Stable versions select npm `latest`; prereleases select `next`.
- go-rod uses the exact core version and no committed `replace` directive.
- A future Go version is checked only against exact Git tags and GitHub
  Releases before publication. Its first public-proxy request occurs after the
  corresponding tag exists, so a pre-tag negative response cannot be cached.
- Go proxy and npm registry reads use bounded retries and report the last value.
- Publication is globally serialized and never canceled in progress.

## External configuration migration

The repository change does not mutate external settings. Before the unified
workflow is first used, an administrator must:

1. create `release-publish` with required reviewers, no administrator bypass,
   and default-branch-only deployment; allow self-review when a single owner
   must dispatch and approve the release, otherwise prevent it;
2. change both npm trusted publishers to workflow `release.yml`, Environment
   `release-publish`, and allow the `npm publish` action. The separate
   `authorize-release` job protects the GitHub write path, while both npm jobs
   reference the Environment directly so their OIDC claims match;
3. leave creation unrestricted for `v*.*.*` and
   `adapters/rod/v*.*.*`, while retaining update, deletion, and
   non-fast-forward protection plus an audited release-owner emergency bypass;
4. verify the Environment and both tag rulesets before dispatch.

## Implementation tickets

| ID | Tracer-bullet outcome | Blocked by | Status |
|---|---|---|---|
| UR-01 | Prepare and verify a future core dependency in go-rod from an exact Git revision, with checksum regression tests | none | Completed |
| UR-02 | Make GitHub Release, Go proxy, and npm publication steps idempotent and artifact-verifying | UR-01 | Completed |
| UR-03 | Replace the three public workflows with one protected sequential orchestrator | UR-01, UR-02 | Completed |
| UR-04 | Align release contracts, workflow regression tests, changelog, and administrator handoff | UR-03 | Completed |

Each ticket is committed after its focused tests pass. The complete change must
pass `make verify`, `make release-check`, workflow linting, and release-plan
tests before publication to a Draft PR.
