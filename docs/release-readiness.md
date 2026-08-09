---
updated: 2026-08-09
status: public release candidate active
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
publication must publish new `v1.0.0` artifacts and move npm `latest` to
`1.0.0`; it must not move, replace, or reuse a candidate tag or package version.

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
Assuming no unresolved release blocker interrupts it, the 14-day gate can
complete no earlier than `2026-08-15T08:01:28Z`. Stable publication still
requires a new, explicit project-owner approval after that time.

Issue #69 corrected release documentation and validation without changing a Go
or TypeScript API, language or IR contract, runtime behavior, diagnostics, or a
security default. It is therefore a non-blocking release-guidance correction
and does not reset the RC3 qualification period. PR #70 integrated the source
changes on 2026-08-09, and the RC1 through RC3 Release bodies now link to the Go
migration guidance.

## Remaining dependency order for issue #18

| ID | Work | Blocked by | Status |
|---|---|---|---|
| V1-01 | Review and integrate the post-candidate issue commits | none | Completed by #60 |
| V1-02 | Pass required remote CI and supported-target release gates on the integrated commit | V1-01 | Completed; all 14 required checks passed |
| V1-03 | Obtain explicit owner approval and publish a new core, npm, and go-rod candidate in documented dependency order | V1-02 | Completed with `v1.0.0-rc.3` |
| V1-04 | Verify the new candidate through GitHub Releases, Go proxies, npm, Pages, clean consumers, checksums, provenance, and native archives | V1-03 | Completed at `2026-08-01T08:01:28Z` |
| V1-05 | Complete at least 14 consecutive days with no unresolved release blocker | V1-04 | In progress; earliest completion `2026-08-15T08:01:28Z` |
| V1-06 | Verify the unified Environment, npm trusted publishers, and tag rulesets, then obtain separate explicit owner approval for stable `v1.0.0` | V1-05 | External controls verified 2026-08-09; owner gate pending |
| V1-07 | Run the unified publication once, independently verify every stable distribution surface, then close issue #18 | V1-06 | Pending |

Issue #18 remains open during V1-05. A release blocker resets the consecutive
14-day period after the blocker is resolved and the candidate is requalified.

The exact publication commands, dependency order, post-publication checks, and
immutable-version recovery rules are in [`docs/releasing.md`](releasing.md).
