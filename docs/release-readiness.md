---
updated: 2026-08-01
status: public release candidate active
---

# v1 release readiness

This file records verified release state and the remaining dependency order. It
does not authorize creating or pushing a tag, publishing a package or Release,
deploying Pages, or changing external configuration. Every publication step
still requires explicit project-owner approval immediately before it runs.

## Qualified `v1.0.0-rc.2` candidate

The second public candidate was published and qualified on 2026-08-01 UTC. It
contains the issue implementations integrated by
[#60](https://github.com/hsblabs/scrape-kdl/pull/60), the npm release repairs in
[#61](https://github.com/hsblabs/scrape-kdl/pull/61) and
[#62](https://github.com/hsblabs/scrape-kdl/pull/62), and the go-rod dependency
update in [#63](https://github.com/hsblabs/scrape-kdl/pull/63).

| Surface | Verified state |
|---|---|
| Repository | Public, with the specification site as its homepage |
| Specification site | Dated IR schema returned HTTP 200 with `application/json` on 2026-08-01 |
| Core Git tag | Annotated `v1.0.0-rc.2`, peeled commit `396e1be5d34d57764e503b63dd9d8f7b45aa2131` |
| Core GitHub Release | Published prerelease with four Linux/macOS CLI archives, two npm archives, and checksums; workflow run `30684012908` passed |
| Core Go module | `github.com/hsblabs/scrape-kdl@v1.0.0-rc.2` resolves through the module proxy |
| Core npm package | `@hsblabs/scrape-kdl@1.0.0-rc.2` resolves with registry integrity, signatures, and provenance |
| Playwright npm package | `@hsblabs/scrape-kdl-playwright@1.0.0-rc.2` resolves with registry integrity, signatures, and provenance |
| npm publication | Workflow run `30685139043` passed; both packages have `next=1.0.0-rc.2` and retain `latest=1.0.0-rc.1` |
| go-rod Git tag | Annotated `adapters/rod/v1.0.0-rc.2`, peeled commit `c850bc67162eed3d63f34922e2aac58c37f11d52` |
| go-rod GitHub Release | Published prerelease with four Linux/macOS CLI archives and checksums; workflow run `30685417102` passed |
| go-rod Go module | `github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.0-rc.2` resolves through the module proxy and a clean `go install` |
| Release controls | `github-pages`, `github-release`, and `npm-publish` Environments exist; both release-tag rulesets are active |

The immutable `rc.1` and `rc.2` versions remain published. Stable publication
must publish new `v1.0.0` artifacts and move npm `latest` to `1.0.0`; it must not
move, replace, or reuse either candidate tag or package version.

## Candidate verification

- All 14 required PR checks passed after each release repair and on the go-rod
  dependency update. The matrix covers Linux and macOS core tests, the race
  suite, four package targets, go-rod contract and real-dependency tests,
  Playwright Chromium, and CodeQL for Go, TypeScript, and Actions.
- The core tag workflow reran `make release-check`; the adapter tag workflow
  passed real-dependency tests, vet, browser E2E, and all four release builds.
- SHA-256 verification passed for all six core Release archives and all four
  go-rod Release archives.
- A clean npm consumer installed both `1.0.0-rc.2` packages. The command
  `npm audit signatures` verified all eight installed registry signatures and
  five attestations, including both hsblabs packages.
- A clean `go install` resolved the public go-rod module. Both Go module paths
  resolve at `v1.0.0-rc.2` through the public module proxy.
- The core and go-rod release binaries executed successfully for
  `darwin/arm64`, `darwin/amd64`, `linux/arm64`, and `linux/amd64`. The
  `darwin/amd64` smoke used Rosetta, and the Linux smokes used matching-platform
  Docker containers. Every binary reported the expected immutable tag and
  commit.
- The published core CLI executed the documented static-HTML basic extraction
  without warnings or partial output.
- The dated specification schema returned HTTP 200 as JSON after publication.
- Recovery from the two npm publication failures preserved immutable versions:
  run `30684358577` failed before publishing, while run `30684908771` published
  only the core package before an immediate registry read returned 404. The
  idempotent retry skipped the published core, published Playwright, and passed.

The candidate qualification period began at `2026-08-01T05:22:31Z`. Assuming
no unresolved release blocker interrupts it, the 14-day gate can complete no
earlier than `2026-08-15T05:22:31Z`. Stable publication still requires a new,
explicit project-owner approval after that time.

## Remaining dependency order for issue #18

| ID | Work | Blocked by | Status |
|---|---|---|---|
| V1-01 | Review and integrate the post-candidate issue commits | none | Completed by #60 |
| V1-02 | Pass required remote CI and supported-target release gates on the integrated commit | V1-01 | Completed; all 14 required checks passed |
| V1-03 | Obtain explicit owner approval and publish a new core, npm, and go-rod candidate in documented dependency order | V1-02 | Completed with `v1.0.0-rc.2` |
| V1-04 | Verify the new candidate through GitHub Releases, Go proxies, npm, Pages, clean consumers, checksums, provenance, and native archives | V1-03 | Completed at `2026-08-01T05:22:31Z` |
| V1-05 | Complete at least 14 consecutive days with no unresolved release blocker | V1-04 | In progress; earliest completion `2026-08-15T05:22:31Z` |
| V1-06 | Obtain separate explicit owner approval for stable `v1.0.0` | V1-05 | Owner gate |
| V1-07 | Publish and independently verify every stable distribution surface, then close issue #18 | V1-06 | Pending |

Issue #18 remains open during V1-05. A release blocker resets the consecutive
14-day period after the blocker is resolved and the candidate is requalified.

The exact publication commands, dependency order, post-publication checks, and
immutable-version recovery rules are in [`docs/releasing.md`](releasing.md).
