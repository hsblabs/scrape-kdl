---
status: accepted
date: 2026-08-13T12:45:00+09:00
updated: 2026-08-13T14:19:28+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0010: Decouple core and adapter public release phases

## Context

The unified public workflow published the core Git tag and GitHub Release, then
waited up to five minutes for the public Go proxy before publishing either npm
package or the go-rod adapter. A proxy negative cache therefore left the core
release partially published and skipped unrelated npm publication. The same
failure recurred whenever the proxy took longer than the bounded wait.

The core Go/npm release and the Playwright/go-rod adapter release have a real
dependency boundary: adapters must consume the published core, while the core
package does not need adapter publication to be useful. They should therefore
not share a failure boundary.

## Decision

Keep one manually dispatched, protected `release.yml` workflow and one
`release-publish` approval job. Split the workflow into job-level failure
boundaries:

```text
prepare
  -> authorize-release
       -> publish-core
            -> publish-core-npm -> publish-playwright-npm
            -> await-core-go -> verify-and-build-rod -> publish-rod -> await-rod-go
```

`publish-core` creates or verifies the core annotated tag and GitHub Release.
`publish-core-npm` and `await-core-go` then run on independent dependency
paths. The Playwright npm package depends only on the core npm publication.
The go-rod path depends only on the public core Go proxy, and
`verify-and-build-rod` uses a fresh runner to download the published core,
verify/tidy/test/vet the adapter, and build its archives.

Remove the fixed adapter release delay. Public Go proxy waits use 180
ten-second attempts, and the proxy check starts only after the corresponding
annotated tag exists. The preparation job remains the one full browser-E2E
gate; publication jobs do not rerun that flaky external-process test.

The npm publisher accepts an explicit `core` or `playwright` selection and
remains resumable and integrity-checking. A failed branch can be rerun without
repeating successful jobs; matching tags, Releases, assets, and npm versions
are verified and skipped.

## Consequences

- A delayed Go proxy no longer prevents the core npm package or Playwright npm
  branch from progressing.
- Playwright is published after the core npm package; go-rod is published only
  after the core Go module is publicly resolvable. Neither path uses a fixed
  sleep.
- The workflow keeps one caller-visible release command. The
  `release-publish` Environment protects `authorize-release` and is also
  attached to both npm publication jobs so their trusted-publisher OIDC claims
  match the existing npm configuration. This can require additional approval
  prompts, but avoids weakening the npm trust boundary.
- Each job is a retry checkpoint. A proxy failure reruns only the go-rod chain,
  while an npm failure reruns only the affected npm branch. Recovery uses the
  same immutable version and fails closed on mismatches.
- Core artifacts, including the inspected Playwright archive, are retained as
  workflow artifacts for 14 days so a downstream job can be retried without
  rebuilding the release gate.
