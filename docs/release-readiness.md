# v1 release readiness

Updated: 2026-07-17

This file separates work that can finish while the repository is private from checks that require public registry or module visibility. It does not authorize publication.

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
| Accidental-publication protection | Ready in code | typed confirmations and GitHub Environment boundaries |
| Private hosted dress rehearsal | Pending push | run `Private release rehearsal` after these changes reach a private branch |

## Owner-controlled setup while still private

- Protect the `npm-publish` GitHub Environment with required reviewers and add `NPM_TOKEN`.
- Protect the `github-pages` Environment with required reviewers and select GitHub Actions as the Pages source.
- Confirm the authenticated npm account can create both packages in the `@hsblabs` scope.
- Review and approve the draft v1 release notes and 90-day previous-minor security window.

These settings do not require changing repository visibility, but they change external repository or registry configuration and therefore remain owner actions.

## Requires public visibility or a public artifact

- Make the repository public.
- Deploy and verify `https://hsblabs.github.io/scrape-kdl/ir/2026-07-15/schema.json`.
- Publish a core release-candidate tag and verify it through the Go module proxy.
- Publish both npm candidates and both official adapters.
- Run clean installs against public Go and npm endpoints on every supported target.
- Complete at least 14 consecutive days with no unresolved release blocker.
- Obtain separate owner approval for stable `v1.0.0` publication.

The exact sequence, post-publication checks, and recovery actions are defined in `docs/releasing.md`.
