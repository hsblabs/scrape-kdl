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

## Private operational release

Private operation uses versions ending in `-private.N`. The proposed first
version is `v0.9.0-private.1`; increment `N` instead of moving or reusing a
failed version.

### Core Go module and CLI

From an exact reviewed `main` commit:

```bash
version=v0.9.0-private.1
mise exec -- npm ci
mise exec -- make release-gate
mise exec -- make release-dist \
  VERSION="$version" \
  OUT=dist \
  NPM_ACCESS=restricted
git tag -a "$version" -m "Private release $version"
git push origin "$version"
gh workflow run release-private.yml \
  --repo hsblabs/scrape-kdl \
  -f version="$version" \
  -f confirmation=PUBLISH_PRIVATE_GITHUB_RELEASE
```

Creating and pushing the tag and dispatching the workflow are owner-authorized
publication actions. The workflow refuses a non-private repository, requires an
existing annotated `-private.N` tag, reruns the release checks, and creates a
GitHub prerelease with four CLI archives and checksums.

Private Go consumers need repository read access, `GOPRIVATE`, and
non-interactive Git authentication. For example, an SSH-authenticated developer
can configure GitHub URL rewriting:

```bash
go env -w GOPRIVATE=github.com/hsblabs/scrape-kdl
git config --global url."ssh://git@github.com/".insteadOf https://github.com/
go install github.com/hsblabs/scrape-kdl/cmd/scrape-kdl@"$version"
```

CI should use a read-only GitHub App installation token or fine-grained token
through its secret manager. Do not place credentials in `GOPROXY`, command
arguments, repository files, or logs.

### Restricted npm packages

Private npm packages require a paid `@hsblabs` organization and authenticated
read access for every consumer. The first package versions must be created
before npm can attach a trusted publisher. Bootstrap both records once from the
already inspected restricted archives with an interactive npm session and 2FA:

```bash
npm login --scope=@hsblabs
npm publish "dist/hsblabs-scrape-kdl-${version#v}.tgz" \
  --access restricted \
  --tag private
npm publish "dist/hsblabs-scrape-kdl-playwright-${version#v}.tgz" \
  --access restricted \
  --tag private
```

After both records exist, configure each package's trusted publisher with:

- organization: `hsblabs`;
- repository: `scrape-kdl`;
- workflow: `release-npm-private.yml`;
- Environment: none while the current private-repository plan cannot provide
  protected Environments;
- allowed action: `npm publish`.

Do not create `NPM_TOKEN` or any other npm write secret in GitHub. Subsequent
private versions use OIDC:

```bash
gh workflow run release-npm-private.yml \
  --repo hsblabs/scrape-kdl \
  -f version="${version#v}" \
  -f confirmation=PUBLISH_PRIVATE_NPM
```

OIDC authenticates `npm publish` only. Private `npm view`, install, and
post-publication checks still require an interactive session or a separate
read-only token:

```bash
./scripts/verify-private-npm-release.sh "${version#v}"
```

npm provenance is unavailable while the repository is private.

### go-rod module and CLI

The adapter cannot be released before the private core tag exists. Update its
release-clean dependency to the exact core version, review the resulting
`go.mod` and `go.sum`, and verify without a local replacement:

```bash
(
  cd adapters/rod
  GOPRIVATE=github.com/hsblabs/scrape-kdl \
    GONOSUMDB=github.com/hsblabs/scrape-kdl \
    go get github.com/hsblabs/scrape-kdl@"$version"
  go mod tidy
  GOWORK=off GOPRIVATE=github.com/hsblabs/scrape-kdl \
    GONOSUMDB=github.com/hsblabs/scrape-kdl \
    go test ./...
)
git add adapters/rod/go.mod adapters/rod/go.sum
git commit -m "Prepare go-rod for $version"
```

After that commit reaches `main`, create the adapter tag and dispatch its
private release:

```bash
adapter_tag="adapters/rod/$version"
git tag -a "$adapter_tag" -m "Private go-rod release $adapter_tag"
git push origin "$adapter_tag"
gh workflow run release-private-rod.yml \
  --repo hsblabs/scrape-kdl \
  -f version="$adapter_tag" \
  -f confirmation=PUBLISH_PRIVATE_ROD_RELEASE
```

The workflow verifies the real private core dependency and builds
`scrape-kdl-rod` archives for Linux and macOS on amd64 and arm64. After both Go
tags exist, run the authenticated clean-consumer check:

```bash
./scripts/verify-private-go-release.sh "$version" "$adapter_tag"
```

### CLI download

Users with repository read access can download and verify private Release
assets:

```bash
gh release download "$version" \
  --repo hsblabs/scrape-kdl \
  --dir scrape-kdl-release
(
  cd scrape-kdl-release
  shasum -a 256 -c checksums.txt
)
```

Before making the repository public, review all private tags and GitHub Releases:
the visibility change would expose their source references, notes, and attached
assets. Restricted npm packages remain private until their package access is
changed explicitly.

## One-time public release setup

Complete these owner-controlled settings before the first public candidate:

1. Configure the `npm-publish` GitHub Environment with required reviewers, prevent self-review and administrator bypass, and permit deployments only from `main`. Do not add an npm publication token.
2. Configure both npm packages with the GitHub Actions trusted publisher `hsblabs/scrape-kdl`, workflow filename `release-npm.yml`, Environment `npm-publish`, and allowed action `npm publish`. After verifying OIDC publication, set publishing access to require 2FA and disallow tokens.
3. Configure the `github-release` GitHub Environment with required reviewers, prevent self-review and administrator bypass, and permit only `v*.*.*` and `adapters/rod/v*.*.*` tags.
4. Add active tag rulesets for `v*.*.*` and `adapters/rod/v*.*.*` that restrict tag creation, update, deletion, and bypass rights to the release owners.
5. Configure the `github-pages` Environment with required reviewers and GitHub Pages using GitHub Actions as its source.
6. Confirm ownership of both npm package names.
7. Decide when to make the repository public. The public npm, Pages, core
   release, and adapter release jobs refuse to run while repository visibility
   is private; the private workflows remain separate.

If the private npm channel has been used, replace each package's single trusted
publisher configuration from `release-npm-private.yml` to `release-npm.yml`.
The public workflow's explicit `--access public` changes the package access
level, making the package and its existing versions downloadable by everyone.
Review all private versions before approving that change.

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

- Failed private release: retain the failed version for auditability and publish
  the next `-private.N` version. Stop dependent npm or adapter publication if the
  core fails.
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
