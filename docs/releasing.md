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

This repository does not use Changesets and does not commit release versions to
workspace manifests. Release-facing changes are recorded directly in
`CHANGELOG.md`, release notes, migration notes, and implementation status. The
checked-in workspace manifests remain `0.0.0-development`; npm versions,
compatible adapter ranges, and public access are changed only in temporary
staging directories. Every npm archive is installed into a clean consumer and
exercised before it enters the bundle.

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

## Unified public release setup

Complete these owner-controlled settings before the unified workflow is first
used:

1. Configure the `release-publish` GitHub Environment with required reviewers,
   prevent administrator bypass, and permit deployments only from `main`. If a
   single owner must dispatch and approve a release, allow self-review;
   otherwise prevent it. Do not add an npm publication token.
2. Configure both npm packages with the GitHub Actions trusted publisher
   `hsblabs/scrape-kdl`, workflow filename `release.yml`, Environment
   `release-publish`, and allowed action `npm publish`. After verifying OIDC
   publication, retain the package setting that requires 2FA and disallows
   tokens.
3. Keep active tag rulesets for `v*.*.*` and `adapters/rod/v*.*.*`. Leave tag
   creation unrestricted so the protected workflow can create annotated tags,
   but restrict update, deletion, and non-fast-forward changes. Release owners
   retain their audited emergency bypass.
4. Configure the `github-pages` Environment with required reviewers and GitHub
   Pages using GitHub Actions as its source.
5. Confirm that both npm package trusted-publisher records and both tag rulesets
   show the new workflow before dispatching a release.

The legacy `npm-publish` and `github-release` Environments are not used by the
unified public workflow. Remove them only after confirming that no retained
private or historical workflow depends on them.

Trusted publishing authorizes `npm publish`, not an independent dist-tag repair.
If a version already exists without its expected `latest` or `next` tag, the
workflow stops for owner investigation instead of mutating registry metadata.

Both npm packages are public. Their public release archives contain
`publishConfig.access=public`, while `release.yml` omits the `--access`
argument so publishing a new version does not mutate the existing package
visibility. If a package is ever restricted again, change its access explicitly
before using the public workflow; do not make a visibility change an incidental
side effect of version publication.

The release and Pages workflows require separate typed confirmations.
`release-publish` approval remains the primary protection against accidental
core, npm, and go-rod publication.

The publish job requires Node.js 26, npm 11.5.1 or later, a GitHub-hosted
runner, and `id-token: write`. It never reads `NPM_TOKEN`: the preparation job
has no OIDC or write permission, and only the protected publish job can request
short-lived credentials. Trusted publishing automatically emits provenance.

The protected publish job has two phases. It publishes the core GitHub Release
and `@hsblabs/scrape-kdl` first. It then waits for the core Go module to resolve
through `proxy.golang.org` and for a 30-minute adapter release window measured
from the core Release publication before publishing
`@hsblabs/scrape-kdl-playwright` and go-rod. This keeps proxy propagation from
blocking the useful core npm release and gives adapter consumers a stable core
to consume.

## Release candidate and stable sequence

Use one preparation PR and one publication run for release candidates and stable
releases:

1. Update `CHANGELOG.md`, release notes, migration notes, and implementation
   status, then commit every release-facing root-module change.
2. Run `make prepare-public-release VERSION=vX.Y.Z`. Review and commit the exact
   `adapters/rod/go.mod` and `adapters/rod/go.sum` changes. The command computes
   the future root module checksum from the committed Git tree; do not change a
   root-module file afterward without rerunning it.
3. Run the private rehearsal, retain its checksums, merge the reviewed release
   preparation PR, and confirm the unified external settings above.
4. Confirm that both Go versions are unused from the exact Git tags and GitHub
   Releases only. Before the corresponding tag exists, do not query
   `proxy.golang.org` and do not run `go list`, `go get`, or `go mod download`
   for the future version. The public proxy can cache that negative answer for
   up to 30 minutes; its first request must follow tag publication. See the
   [official Go module proxy FAQ](https://proxy.golang.org/).
5. After the separate project-owner approval, dispatch the workflow from
   `main`:

   ```bash
   gh workflow run release.yml \
     --repo hsblabs/scrape-kdl \
     --ref main \
     -f version=X.Y.Z \
     -f confirmation=PUBLISH_RELEASE
   ```

6. Approve the single `release-publish` deployment. The workflow creates or
   verifies the core annotated tag and Release, publishes and verifies the core
   npm archive, waits for core proxy visibility, and enforces the 30-minute
   adapter window. It then publishes and verifies the Playwright npm archive,
   tests go-rod against the published core, creates or verifies the adapter tag
   and Release, and waits for adapter proxy visibility. Prereleases use npm
   `next`; stable releases use `latest`.
7. Run every independent post-publication check below and record the results on
   issue #18.

The Go module rules still require distinct `vX.Y.Z` and
`adapters/rod/vX.Y.Z` tag names. Their version, commit, workflow run, and owner
approval are shared.

Stable `v1.0.0` additionally requires at least 14 consecutive days with no
unresolved release blocker after the public release candidate and a separate
project-owner approval. ADR 0009 records a one-release owner-directed exception
to that waiting period for `v1.0.0`; every other preparation, approval,
publication, verification, and immutable-version rule remains required.

## Post-publication checks

Verify from clean directories rather than the repository workspace:

```bash
go list -m github.com/hsblabs/scrape-kdl@vX.Y.Z
go list -m github.com/hsblabs/scrape-kdl/adapters/rod@vX.Y.Z
npm view @hsblabs/scrape-kdl@X.Y.Z version
npm view @hsblabs/scrape-kdl-playwright@X.Y.Z version
ax https://hsblabs.github.io/scrape-kdl/ir/2026-07-15/schema.json
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
- Partial publication: rerun `release.yml` for the same version and source. The
  orchestrator verifies and skips matching core tags, Release assets, and npm
  versions, then resumes the adapter phase after the proxy and timing gates.
  Any mismatch fails closed.

## Supported release targets

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

Windows artifacts must not be added. The release script rejects `windows/*` explicitly.
