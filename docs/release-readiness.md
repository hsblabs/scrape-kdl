# v1 release readiness

Updated: 2026-07-24

This file separates verified repository preparation, private operation, and
checks that require public registry or module visibility. It does not authorize
creating a tag, Release, package version, Pages deployment, or visibility change.

## Private preparation

| Area | Status | Evidence |
|---|---|---|
| Go and TypeScript API contracts | Ready | independent consumer checks in `make api-contract` |
| Language, diagnostics, and IR parity | Ready | conformance and canonical-IR gates |
| HTTP and Chromium browser behavior | Ready | release matrix, go-rod E2E, and Playwright E2E |
| Supported target matrix | Ready | Linux/macOS amd64/arm64 cross-build gates |
| CLI and npm artifacts | Ready | failure-safe `make release-dist` bundle and clean consumers |
| License and notice payloads | Ready | `LICENSE` and `NOTICE` required in CLI and npm archives |
| Migration and release notes | Ready | `docs/migrating-to-v1.md` and `docs/release-notes-v1.md` |
| Maintenance and recovery policy | Ready | `SUPPORT.md`, `SECURITY.md`, and `docs/releasing.md` |
| Accidental-publication protection | Ready in code | public-visibility checks, typed confirmations, globally serialized npm publishing, and GitHub Environment boundaries |
| Release credential isolation | Ready in code | tokenless npm OIDC is available only to the protected publish job |
| Private hosted dress rehearsal | Pending execution | run `Private release rehearsal` from the private branch |
| Restricted npm artifact metadata | Ready in code | source and private bundles use `publishConfig.access=restricted`; public staging must opt in explicitly |
| Private core GitHub Release | Ready in code | manual `Release private core` workflow, private visibility guard, annotated `-private.N` tag requirement |
| Private go-rod GitHub Release | Ready in code | manual adapter workflow builds four CLI archives with checksums |
| Private npm OIDC publication | Ready after bootstrap | manual restricted bootstrap creates package records; later versions use `release-npm-private.yml` |
| Private Go/npm consumer checks | Ready in code | authenticated scripts verify both Go modules and both npm packages |

The complete local `make release-gate` passed on 2026-07-24 before these release
changes. The current repository has no release tags or GitHub Releases.

## Owner-controlled private setup

- Confirm the npm `@hsblabs` organization has paid private-package support and
  that the publishing owner has 2FA enabled.
- Run the private rehearsal for the exact first version, proposed as
  `v0.9.0-private.1`.
- Bootstrap the two restricted npm package records from the inspected archives
  with an interactive npm session. Do not create or store an npm publication
  token in GitHub.
- Configure both package records to trust `hsblabs/scrape-kdl` workflow
  `release-npm-private.yml` for `npm publish`. Do not specify a GitHub
  Environment while the current plan cannot provide one.
- Grant npm teams read access and give CI consumers separate read-only tokens.
- Grant intended Go and CLI consumers read access to the private GitHub
  repository.

GitHub currently returns `403` for protected Environments and repository
rulesets on this private repository's plan. The private workflows therefore do
not claim those controls. They use typed confirmations, private-visibility
checks, annotated existing tags, serialized jobs, and least-privilege
permissions instead.

## Owner-controlled public setup while still private

- Protect the `npm-publish` GitHub Environment with required reviewers, prevent self-review and administrator bypass, and allow only `main`.
- Configure both npm package records to trust `hsblabs/scrape-kdl` workflow `release-npm.yml` with Environment `npm-publish` for `npm publish`; do not configure `NPM_TOKEN`.
- Protect the `github-release` GitHub Environment and add restrictive rulesets for core and go-rod release tags.
- Protect the `github-pages` Environment with required reviewers and select GitHub Actions as the Pages source.
- Confirm the authenticated npm account can create both packages in the `@hsblabs` scope.
- Review and approve the draft v1 release notes and 90-day previous-minor security window.

These settings change external repository or registry configuration and therefore remain owner actions. Some GitHub plans expose required-reviewer protection only after the repository is public; where that limitation applies, complete the settings after publicization but before any release tag or publication workflow.

## Requires public visibility or a public artifact

- Make the repository public.
- Deploy and verify `https://hsblabs.github.io/scrape-kdl/ir/2026-07-15/schema.json`.
- Publish a core release-candidate tag and verify it through the Go module proxy.
- Publish both npm candidates and both official adapters.
- Run clean installs against public Go and npm endpoints on every supported target.
- Complete at least 14 consecutive days with no unresolved release blocker.
- Obtain separate owner approval for stable `v1.0.0` publication.

The exact sequence, post-publication checks, and recovery actions are defined in `docs/releasing.md`.
