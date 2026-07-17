# scrape-kdl v1.0.0 release notes

Status: private draft. No public v1 artifact has been published.

## Highlights

- Dated `2026-07-15` language and Validated IR contracts with deterministic diagnostics and canonical JSON.
- Go and TypeScript compiler, HTTP runtime, browser runtime, and clean public API boundaries.
- Official go-rod and Playwright adapters with extraction-wide browser isolation.
- Go-only automation-friendly CLI with stable streams, JSON envelopes, exit statuses, signal handling, and session-file secret input.
- Cross-language fixtures, examples, conformance, HTML differential, race, fuzz, performance, packaging, and browser release gates.

## Supported environment

- Linux and macOS on amd64 and arm64.
- Go 1.26, with release binaries built using the repository-pinned security patch level.
- Node.js 26 or later; ESM-only npm packages.
- Chromium supported. Playwright Firefox and WebKit are best effort.
- Windows is not supported.

## Security defaults

JavaScript is disabled by default. Validation completes before network or browser acquisition. HTTP and browser operations preserve cancellation and timeouts, HTTP bodies are bounded, URL policy applies to initial HTTP targets and redirects, and session secrets are not accepted directly on the CLI command line.

## Compatibility

This is the first supported public contract. Applications using an untagged development revision must follow `docs/migrating-to-v1.md`. Future compatible v1 additions and fixes follow Semantic Versioning and the maintenance window in `SUPPORT.md`.

## Known limits

- Scraping KDL documents use the documented project subset rather than every generic KDL 2 representation.
- Browser JavaScript and external transforms execute trusted application code and are not a sandbox for untrusted specifications.
- Type generation, a language server, an inspector UI, and a browser authoring extension are outside the v1 scope.

## Publication checklist

The authoritative checklist and dependency order are in `docs/releasing.md` and issue #18. Replace this status with the release date and verified registry/module links only after stable publication succeeds.
