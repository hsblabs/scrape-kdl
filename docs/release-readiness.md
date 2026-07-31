---
updated: 2026-08-01
status: public release candidate active
---

# v1 release readiness

This file records verified release state and the remaining dependency order. It
does not authorize creating or pushing a tag, publishing a package or Release,
deploying Pages, or changing external configuration. Every publication step
still requires explicit project-owner approval immediately before it runs.

## Published `v1.0.0-rc.1`

The first public candidate was published on 2026-07-30 UTC. The following
surfaces were rechecked read-only on 2026-08-01:

| Surface | Verified state |
|---|---|
| Repository | Public, with the specification site as its homepage |
| Specification site | Dated IR schema returns HTTP 200 as JSON |
| Core Git tag | Annotated `v1.0.0-rc.1`, peeled commit `65ebd8e19fd2b7ff8086adfa22dd6a1df89da7db` |
| Core GitHub Release | Published prerelease with four Linux/macOS CLI archives, two npm archives, and checksums |
| Core Go module | `github.com/hsblabs/scrape-kdl@v1.0.0-rc.1` resolves through the module proxy |
| Core npm package | `@hsblabs/scrape-kdl@1.0.0-rc.1` resolves with registry integrity and signature metadata |
| Playwright npm package | `@hsblabs/scrape-kdl-playwright@1.0.0-rc.1` resolves with registry integrity and signature metadata |
| go-rod Git tag | Annotated `adapters/rod/v1.0.0-rc.1`, peeled commit `c188d77dec2d32a0557989fcda6162cfcf9718dd` |
| go-rod GitHub Release | Published prerelease with four Linux/macOS CLI archives and checksums |
| go-rod Go module | `github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.0-rc.1` resolves through the module proxy |
| Release controls | `github-pages`, `github-release`, and `npm-publish` Environments exist; both release-tag rulesets are active |

Both npm packages currently show `next` and `latest` pointing to the only
published version, `1.0.0-rc.1`. Stable publication must move `latest` to
`1.0.0`. Do not delete or rewrite the immutable candidate version.

## Post-candidate development

The current development branch adds the following issue implementations after
the `v1.0.0-rc.1` commit:

| Issue | Change | Commit |
|---|---|---|
| #57 | Separate operational compilation failures from diagnostics | `68319c5` |
| #58 | Add `fs.FS` compilation entry points | `8fd43a9` |
| #56 | Expose immutable acquisition descriptors | `e2d5552` |
| #54 | Add strict typed Go result decoding | `855ba0f` |
| #55 | Define offline snapshot execution | `cbb5bb4` |
| #43 | Document pagination and list-to-detail patterns | `7ed06f6` |
| #45 | Add the strict transform cookbook | `3ecf8c4` |
| #59 | Add bounded authoring APIs and the versioned built-in catalog | `bbe0587`, `24a7038` |

These changes alter the intended stable public surface and are not present in
`v1.0.0-rc.1`. The immutable candidate therefore cannot qualify the new surface
for stable release. After review and integration, the next candidate must use a
new version such as `v1.0.0-rc.2`; it must not move or reuse the rc.1 tags.

On 2026-08-01, commit `755d40a` and its ancestors passed `make verify` and
`make release-check`, including conformance, race, clean Go and npm consumers,
package contents, performance, release-matrix, and archive gates.
Remote CI and supported-target checks remain required after integration.

## Remaining dependency order for issue #18

| ID | Work | Blocked by | Status |
|---|---|---|---|
| V1-01 | Review and integrate the post-candidate issue commits | none | Pending |
| V1-02 | Pass required remote CI and supported-target release gates on the integrated commit | V1-01 | Pending |
| V1-03 | Obtain explicit owner approval and publish a new core, npm, and go-rod candidate in documented dependency order | V1-02 | Owner gate |
| V1-04 | Verify the new candidate through GitHub Releases, Go proxies, npm, Pages, clean consumers, checksums, provenance, and native archives | V1-03 | Pending |
| V1-05 | Complete at least 14 consecutive days with no unresolved release blocker | V1-04 | Time gate |
| V1-06 | Obtain separate explicit owner approval for stable `v1.0.0` | V1-05 | Owner gate |
| V1-07 | Publish and independently verify every stable distribution surface, then close issue #18 | V1-06 | Pending |

The 14-day period starts only after the new candidate in V1-03 is published and
V1-04 passes. No stable publication date can be calculated before then.

The exact publication commands, dependency order, post-publication checks, and
immutable-version recovery rules are in [`docs/releasing.md`](releasing.md).
