---
updated: 2026-08-09
status: stable v1.0.0 published and verified
---

# v1 release readiness

This file records verified release state and the remaining dependency order. It
does not authorize creating or pushing a tag, publishing a package or Release,
deploying Pages, or changing external configuration. Every publication step
still requires explicit project-owner approval immediately before it runs.
## Qualified `v1.0.0-rc.3` candidate

The third public candidate was published and independently qualified on
2026-08-01 UTC. It retains the frozen runtime and API contracts from RC2 and
uses the unified, resumable public release workflow completed by
[#65](https://github.com/hsblabs/scrape-kdl/pull/65) and
[#67](https://github.com/hsblabs/scrape-kdl/pull/67).

| Surface | Verified state |
|---|---|
| Repository | Public, with the specification site as its homepage |
| Specification site | Dated IR schema returned HTTP 200 with `application/json` on 2026-08-01 |
| Core Git tag | Annotated `v1.0.0-rc.3`, peeled commit `01d046087474ecab11210630b9382c005a7f1e75` |
| Core GitHub Release | Published prerelease with four Linux/macOS CLI archives, two npm archives, and checksums; unified workflow run `30689019694` passed |
| Core Go module | `github.com/hsblabs/scrape-kdl@v1.0.0-rc.3` resolves through the module proxy |
| Core npm package | `@hsblabs/scrape-kdl@1.0.0-rc.3` resolves with registry integrity, signatures, and provenance |
| Playwright npm package | `@hsblabs/scrape-kdl-playwright@1.0.0-rc.3` resolves with registry integrity, signatures, and provenance |
| npm publication | Unified workflow run `30689019694` passed; both packages have `next=1.0.0-rc.3` and retain `latest=1.0.0-rc.1` |
| go-rod Git tag | Annotated `adapters/rod/v1.0.0-rc.3`, peeled commit `01d046087474ecab11210630b9382c005a7f1e75` |
| go-rod GitHub Release | Published prerelease with four Linux/macOS CLI archives and checksums; unified workflow run `30689019694` passed |
| go-rod Go module | `github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.0-rc.3` resolves through the module proxy and a clean `go install` |
| Release controls | `release-publish` is restricted to `main`, requires owner review, and disables administrator bypass; both release-tag rulesets are active |

The immutable `rc.1`, `rc.2`, and `rc.3` versions remain published. Stable
publication therefore created new `v1.0.0` artifacts and moved npm `latest` to
`1.0.0` without moving, replacing, or reusing a candidate tag or package
version.

## Candidate verification

- Unified workflow run `30689019694` passed the release plan, complete release
  gate, inspected bundle build, sequential core/npm/go-rod publication, and
  post-tag Go proxy checks.
- SHA-256 verification passed for all six core Release archives and all four
  go-rod Release archives.
- A clean npm consumer installed both `1.0.0-rc.3` packages. Registry signatures
  and provenance attestations were verified, including both hsblabs packages.
- Clean Go consumers, both CLI `go install` commands, and both Go module paths
  resolved `v1.0.0-rc.3` through the public module proxy.
- The core and go-rod release binaries executed successfully for
  `darwin/arm64`, `darwin/amd64`, `linux/arm64`, and `linux/amd64`. The
  `darwin/amd64` smoke used Rosetta, and the Linux smokes used matching-platform
  Docker containers. Every binary reported the expected immutable tag and
  commit.
- The published core CLI executed the documented static-HTML basic extraction
  without warnings or partial output.
- The dated specification schema returned HTTP 200 as JSON after publication.
- Local `make release-check` and `make verify` passed after publication.

The independent RC3 verification record completed at `2026-08-01T08:01:28Z`.
The candidate qualification period is measured conservatively from that time.
Assuming no unresolved release blocker interrupts it, the 14-day gate would
complete no earlier than `2026-08-15T08:01:28Z`. ADR 0009 records the project
owner's later one-release exception and explicit approval to publish before
that time.

Issue #69 corrected release documentation and validation without changing a Go
or TypeScript API, language or IR contract, runtime behavior, diagnostics, or a
security default. It is therefore a non-blocking release-guidance correction
and does not reset the RC3 qualification period. PR #70 integrated the source
changes on 2026-08-09, and the RC1 through RC3 Release bodies now link to the Go
migration guidance.

## Stable `v1.0.0` publication

On 2026-08-09, the project owner explicitly approved the ADR 0009 override of
the 14-day candidate gate. This is an owner-direction exception only; it does
not convert the RC qualification record into a technical safety claim.

The reviewed preparation landed in PR #72 at
`4204e3660eafe9a6129312d98b4c041e70124f9f`. Protected unified workflow
[run 31308137799](https://github.com/hsblabs/scrape-kdl/actions/runs/31308137799)
then published the stable release and completed its post-tag Go proxy checks.
Independent verification produced this record:

| Surface | Verified stable state |
|---|---|
| Core Git tag | Annotated `v1.0.0`, peeled commit `4204e3660eafe9a6129312d98b4c041e70124f9f` |
| Core GitHub Release | Published non-draft, non-prerelease with four Linux/macOS CLI archives, two npm archives, and checksums |
| Core Go module | `github.com/hsblabs/scrape-kdl@v1.0.0` resolves through the public proxy; clean compile, snapshot extraction, and CLI install passed |
| Core npm package | `@hsblabs/scrape-kdl@1.0.0` resolves with `latest=1.0.0`; clean compile, validation, extraction, signatures, and SLSA provenance passed |
| Playwright npm package | `@hsblabs/scrape-kdl-playwright@1.0.0` resolves with `latest=1.0.0`; clean adapter lease smoke, signatures, and SLSA provenance passed |
| go-rod Git tag | Annotated `adapters/rod/v1.0.0`, peeled commit `4204e3660eafe9a6129312d98b4c041e70124f9f` |
| go-rod GitHub Release | Published non-draft, non-prerelease with four Linux/macOS CLI archives and checksums |
| go-rod Go module | `github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.0` resolves through the public proxy; clean contract test and CLI install passed |
| Release archives | SHA-256 checks passed for all six core payloads and four go-rod payloads; core extraction and go-rod version smokes passed on Linux and macOS, amd64 and arm64 |
| Specification site | Landing page, `llms.txt`, and the dated IR schema returned HTTP 200; the schema used `application/json` |
| Release controls | Protected Environment approval and both immutable release-tag rulesets remained in force |

Both npm packages retain `next=1.0.0-rc.3`; only `latest` moved to stable.
The first-24-hour observation continues on issue #18 and does not change the
already published immutable artifacts.

A broader `go test ./...` from the public core module zip passes every consumer
package and fails only repository-internal `scripts` tests whose checkout-only
shell permissions, nested go-rod module, or documentation inputs are absent
from module zips. This does not affect the public APIs, CLIs, adapters, or clean
consumer checks and is tracked as non-blocking release hygiene in
[#73](https://github.com/hsblabs/scrape-kdl/issues/73).

Existing `rc.1`, `rc.2`, and `rc.3` tags, Releases, and package versions remain
immutable. If stable artifacts have a problem, leave those candidates
unchanged and publish `v1.0.1` from a reviewed fix or revert.

## Completion order for issue #18

| ID | Work | Blocked by | Status |
|---|---|---|---|
| V1-01 | Review and integrate the post-candidate issue commits | none | Completed by #60 |
| V1-02 | Pass required remote CI and supported-target release gates on the integrated commit | V1-01 | Completed; all 14 required checks passed |
| V1-03 | Obtain explicit owner approval and publish a new core, npm, and go-rod candidate in documented dependency order | V1-02 | Completed with `v1.0.0-rc.3` |
| V1-04 | Verify the new candidate through GitHub Releases, Go proxies, npm, Pages, clean consumers, checksums, provenance, and native archives | V1-03 | Completed at `2026-08-01T08:01:28Z` |
| V1-05 | Complete at least 14 consecutive days with no unresolved release blocker | V1-04 | Overridden by explicit project-owner direction on 2026-08-09 for this release only |
| V1-06 | Verify the unified Environment, npm trusted publishers, and tag rulesets, then obtain separate explicit owner approval for stable `v1.0.0` | V1-05 | Completed 2026-08-09; owner override and protected publication approval recorded |
| V1-07 | Run the unified publication once, independently verify every stable distribution surface, then close issue #18 | V1-06 | Publication and immediate independent verification completed 2026-08-09; stable Pages redeploy and first-24-hour observation remain before closing |

If a stable defect is found, publish a reviewed `v1.0.1`; do not move, delete,
replace, or reuse the immutable `v1.0.0` tags, Releases, modules, packages, or
assets.

The exact publication commands, dependency order, post-publication checks, and
immutable-version recovery rules are in [`docs/releasing.md`](releasing.md).
