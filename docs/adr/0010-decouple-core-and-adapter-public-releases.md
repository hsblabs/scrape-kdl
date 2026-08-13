---
status: accepted
date: 2026-08-13T12:45:00+09:00
updated: 2026-08-13T15:22:27+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0010: Decouple core and adapter public release phases

## Context

The unified public workflow published the core Git tag and GitHub Release, then
waited for the public Go proxy before publishing the go-rod adapter. A proxy
negative cache therefore kept the release workflow open for up to 30 minutes
even though the immutable Git tags and GitHub Releases were already valid.

The core Go/npm release and the Playwright/go-rod adapter release have a real
source dependency, but Go Proxy propagation is eventually consistent and is
not a publication transaction. The adapter can be verified and built against
the same-commit local core before its tag is published. They should therefore
not share a proxy availability failure boundary.

## Decision

Keep one manually dispatched, protected `release.yml` workflow and one
`release-publish` approval job. Split the workflow into job-level failure
boundaries:

```text
prepare
  -> authorize-release
       -> publish-core
            -> publish-core-npm -> publish-playwright-npm
            -> verify-and-build-rod -> publish-rod
```

`publish-core` creates or verifies the core annotated tag and GitHub Release.
`publish-core-npm` and `verify-and-build-rod` then run on independent
publication paths. The Playwright npm package depends only on the core npm
publication. The go-rod path depends only on the core GitHub Release;
`verify-and-build-rod` uses a fresh runner and `scripts/verify-rod.sh` to apply
a temporary local replace for the same-commit core, test/vet the adapter, and
build its archives. `scripts/build-rod-release.sh` applies the same temporary
local replace while cross-compiling, so archive generation does not contact Go
Proxy either.

Remove the fixed adapter release delay and all Go Proxy waits from the release
workflow. The preparation job remains the one full browser-E2E gate;
publication jobs do not rerun that flaky external-process test. Public Go
Proxy propagation is an external, eventually consistent consumer concern and
does not determine whether the GitHub/npm publication succeeded.

The npm publisher accepts an explicit `core` or `playwright` selection and
remains resumable and integrity-checking. A failed branch can be rerun without
repeating successful jobs; matching tags, Releases, assets, and npm versions
are verified and skipped.

## Consequences

- A delayed Go proxy no longer prevents any GitHub or npm publication from
  progressing.
- Playwright is published after the core npm package; go-rod is published only
  after same-commit local verification against the core source. Neither path
  waits for Go Proxy visibility.
- The workflow keeps one caller-visible release command. The
  `release-publish` Environment protects `authorize-release` and is also
  attached to both npm publication jobs so their trusted-publisher OIDC claims
  match the existing npm configuration. This can require additional approval
  prompts, but avoids weakening the npm trust boundary.
- Each job is a retry checkpoint. A go-rod verification/publication failure
  reruns only the adapter chain, while an npm failure reruns only the affected
  npm branch. Recovery uses the same immutable version and fails closed on
  mismatches.
- Core artifacts, including the inspected Playwright archive, are retained as
  workflow artifacts for 14 days so a downstream job can be retried without
  rebuilding the release gate.
