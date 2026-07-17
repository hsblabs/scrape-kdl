# Release process

Public release is an explicit project-owner gate. The commands in the private rehearsal section do not create tags, GitHub Releases, registry entries, Pages deployments, or visibility changes.

Current progress and the remaining visibility-dependent checks are tracked in `docs/release-readiness.md`.

## Private rehearsal

From a clean checkout with the pinned Go and Node.js toolchains and Chromium available:

```bash
npm ci
make release-gate
make release-dist VERSION=v1.0.0-rc.1 OUT=dist
```

`release-dist` builds the complete bundle before replacing the requested output directory, so a build or validation failure preserves the previous output. The inspected bundle contains:

- Linux and macOS CLI archives for amd64 and arm64;
- `@hsblabs/scrape-kdl` and `@hsblabs/scrape-kdl-playwright` npm archives with release-only metadata;
- SHA-256 checksums covering all six archives.

The checked-in workspace manifests remain `0.0.0-development`. npm versions and compatible adapter ranges are changed only in temporary staging directories. Every npm archive is installed into a clean consumer and exercised before it enters the bundle.

The `Private release rehearsal` workflow performs the same complete gate and stores the bundle as a private GitHub Actions artifact for 14 days. It has no registry or repository write permission.

## One-time public release setup

Complete these owner-controlled settings before the first public candidate:

1. Configure the `npm-publish` GitHub Environment with required reviewers, prevent self-review and administrator bypass, and permit deployments only from `main`. Do not add an npm publication token.
2. Configure both npm packages with the GitHub Actions trusted publisher `hsblabs/scrape-kdl`, workflow filename `release-npm.yml`, Environment `npm-publish`, and allowed action `npm publish`. After verifying OIDC publication, set publishing access to require 2FA and disallow tokens.
3. Configure the `github-release` GitHub Environment with required reviewers, prevent self-review and administrator bypass, and permit only `v*.*.*` and `adapters/rod/v*.*.*` tags.
4. Add active tag rulesets for `v*.*.*` and `adapters/rod/v*.*.*` that restrict tag creation, update, deletion, and bypass rights to the release owners.
5. Configure the `github-pages` Environment with required reviewers and GitHub Pages using GitHub Actions as its source.
6. Confirm ownership of both npm package names.
7. Decide when to make the repository public. The npm, Pages, core release, and adapter release jobs refuse to run publicly while repository visibility is private.

The npm and Pages workflows also require typed confirmations. Environment approval remains the primary protection against accidental publication. Required-reviewer availability for private repositories depends on the GitHub plan; if it is unavailable while private, configure and verify it immediately after publicization and before creating or publishing any release tag.

The npm workflow requires Node.js 26, npm 11.5.1 or later, a GitHub-hosted runner, and `id-token: write`. It never reads `NPM_TOKEN`: the build job has no OIDC permission, and only the protected publish job can request short-lived credentials. Trusted publishing automatically emits provenance after the repository and packages are public.

## Release candidate and stable sequence

Use the same dependency order for release candidates and stable releases:

1. Update `CHANGELOG.md`, `docs/release-notes-v1.md`, `docs/migrating-to-v1.md`, and `IMPLEMENTATION_STATUS.md`.
2. Run the private rehearsal and retain its checksums.
3. After owner approval, make the repository public and manually run `Publish specification site`; verify the dated schema URL returns the checked-in JSON.
4. Create and push an annotated core tag such as `v1.0.0-rc.1`. The core workflow reruns `release-check`, rebuilds the complete bundle from the tag, and creates the GitHub Release.
5. Confirm the core module is visible through the Go module proxy before releasing anything that depends on it.
6. Manually run `Publish npm packages` for the same version. Prereleases use the `next` dist-tag; stable releases use `latest`. All npm publications are globally serialized. The workflow builds and inspects both archives without OIDC permission, then the protected publish job downloads only those archives, publishes core through trusted publishing, confirms its version and dist-tag, and repeats the process for the Playwright adapter.
7. Update `adapters/rod/go.mod` to require the published core version. Run `make test-rod` and `make test-rod-e2e`, then create and push the annotated `adapters/rod/vX.Y.Z` tag.
8. Run every post-publication check below and record the results on issue #18.

Stable `v1.0.0` additionally requires at least 14 consecutive days with no unresolved release blocker after the public release candidate and a separate project-owner approval.

## Post-publication checks

Verify from clean directories rather than the repository workspace:

```bash
go list -m github.com/hsblabs/scrape-kdl@vX.Y.Z
go list -m github.com/hsblabs/scrape-kdl/adapters/rod@vX.Y.Z
npm view @hsblabs/scrape-kdl@X.Y.Z version
npm view @hsblabs/scrape-kdl-playwright@X.Y.Z version
curl --fail https://hsblabs.github.io/scrape-kdl/ir/2026-07-15/schema.json
```

Also verify npm provenance, install the Go and npm modules into new consumer projects, run `npm audit signatures`, execute the documented basic example, download every CLI archive, verify `checksums.txt`, and run the native archive on Linux amd64, Linux arm64, macOS amd64, and macOS arm64.

For the first 24 hours, monitor GitHub Actions, GitHub security advisories, npm package metadata, Go proxy resolution, installation issues, and reports of diagnostic, IR, HTTP, browser, or CLI contract regressions.

## Rollback and recovery

Go module tags and versions observed by a module proxy are immutable. npm consumers may also cache published artifacts. Never move or reuse a published tag or version.

- Failed prerelease: mark it as superseded or deprecated and publish the next `-rc.N` version.
- Failed stable npm release: deprecate the affected npm version with a useful message and publish a patch.
- Failed stable Go, adapter, or CLI release: publish a patch from a reviewed revert or fix; do not replace tag assets with different bytes.
- Security incident: stop the remaining publication sequence, open a private security advisory, rotate any affected credentials, and publish coordinated patched versions.
- Partial publication: resume from the first missing dependent artifact. The npm workflow is idempotent and skips versions already visible in the registry.

## Supported release targets

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

Windows artifacts must not be added. The release script rejects `windows/*` explicitly.
