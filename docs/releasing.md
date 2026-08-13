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

The `Private release rehearsal` workflow performs the same complete gate and
stores the bundle as a private GitHub Actions artifact for 14 days. It has no
registry or repository write permission.

## Retired private publication channel

The repository is public and the supported distribution channel is the public
release workflow. The private npm publication workflow and private core/rod
GitHub Release workflows were removed after the stable public release. Existing
historical `-private.N` records remain immutable; they are not a supported path
for new releases.

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
unified public workflow and can be removed after confirming that no historical
audit process depends on them.

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

The npm publication jobs require Node.js 26, npm 11.5.1 or later, a
GitHub-hosted runner, `id-token: write`, and the `release-publish` Environment
so the trusted-publisher OIDC claim matches the npm configuration. They never
read `NPM_TOKEN`: the preparation and verification jobs have no
OIDC or write permission, and each public write job receives only the
permission it needs. Trusted publishing automatically emits provenance.

The protected workflow is split into independent checkpoints:

```text
prepare
  -> authorize-release
       -> publish-core
            -> publish-core-npm -> publish-playwright-npm
            -> verify-and-build-rod -> publish-rod
```

`publish-core-npm` depends only on the core GitHub Release. The go-rod path
depends only on the core GitHub Release. The preparation gate runs the full
browser E2E once; the post-publication adapter gate uses a fresh runner and
the existing temporary-modfile verification to test the same-commit local
core. The archive builder uses the same local replace, then publishes the
adapter archives. The release workflow does not wait for Go Proxy visibility;
Go Proxy propagation is eventually consistent and is not a publication failure
boundary. The initial approval job protects GitHub writes; the two npm jobs
also reference the Environment directly to preserve the npm trusted-publisher
claim.

## Release candidate and stable sequence

Use one preparation PR and one publication run for release candidates and stable
releases:

1. Update `CHANGELOG.md`, release notes, migration notes, and implementation
   status, then commit every release-facing root-module change.
2. Run `make prepare-public-release VERSION=vX.Y.Z`. Review and commit the exact
   `adapters/rod/go.mod` and `adapters/rod/go.sum` changes. The command computes
   the future root module checksum from the committed Git tree; do not change a
   root-module file afterward without rerunning it.
3. Run the release rehearsal, retain its checksums, merge the reviewed release
   preparation PR, and confirm the unified external settings above.
4. Confirm that both Go versions are unused from the exact Git tags and GitHub
   Releases only. Before the corresponding tag exists, do not query
   `proxy.golang.org` and do not run `go list`, `go get`, or `go mod download`
   for the future version. The public Go proxy is eventually consistent and
   is not part of the publication transaction.
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
   verifies the core annotated tag and Release, then independently publishes
   the core npm package and, after that package is verified, the Playwright npm
   package. In parallel with the npm path, it verifies go-rod against the
   same-commit local core, builds the adapter archives, and publishes the
   adapter Release without waiting for Go Proxy visibility. Prereleases use npm
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

For the first 24 hours, monitor GitHub Actions, GitHub security advisories, npm
package metadata, installation issues, and reports of diagnostic, IR, HTTP,
browser, or CLI contract regressions.

## Rollback and recovery

Go module tags and versions observed by a module proxy are immutable. npm consumers may also cache published artifacts. Never move or reuse a published tag or version.

- Failed prerelease: mark it as superseded or deprecated and publish the next `-rc.N` version.
- Failed stable npm release: deprecate the affected npm version with a useful message and publish a patch.
- Failed stable Go, adapter, or CLI release: publish a patch from a reviewed revert or fix; do not replace tag assets with different bytes.
- Security incident: stop the remaining publication sequence, open a private security advisory, rotate any affected credentials, and publish coordinated patched versions.
- Partial publication: use the run UI to rerun the failed job and its skipped
  dependents. A failed core npm job resumes only the npm chain; a failed
  go-rod verification or publication resumes only the adapter chain.
  Successful core tags, Releases, npm versions, and adapter artifacts are not
  republished. If a new run is required, use the same version and source
  commit; the idempotent publishers verify matching state and fail closed on
  mismatches.

## Supported release targets

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

Windows artifacts must not be added. The release script rejects `windows/*` explicitly.
