# Versioning policy

The Go modules and npm packages follow Semantic Versioning independently.

- Core module: `github.com/hsblabs/scrape-kdl`
- go-rod adapter: `github.com/hsblabs/scrape-kdl/adapters/rod`
- TypeScript core: `@hsblabs/scrape-kdl`
- Playwright adapter: `@hsblabs/scrape-kdl-playwright`

Core release tags use `vX.Y.Z`. Adapter release tags use `adapters/rod/vX.Y.Z`.

The public API, CLI, diagnostic, runtime, adapter, support, language, and IR candidates are frozen for v1 as of 2026-07-15. A release-candidate change requires a documented blocker and restarts the 14-day compatibility window. After stable `v1.0.0`, releases follow the maintenance policy and Semantic Versioning.

Document, language, and Validated IR compatibility identifiers are separate from module versions and use opaque calendar-date strings. The initial contract is `2026-07-15`: an extractor declares a document `version`, explicitly selects `language-version="2026-07-15"`, and compiles to `irVersion: "2026-07-15"`. Implementations compare identifiers against explicit supported-version registries and do not infer compatibility from date ordering. A later document revision may advance independently; an observable language or IR contract change requires a newly approved date and compatibility notes.

Stable diagnostic codes are part of the language tooling contract. Their message text may improve without a major language-version change, but code meaning must not silently change.

The CLI contract candidate began at v0.5 and is now frozen for v1. Commands, help, streams, JSON envelopes, exit status classes, signals, and secret input receive Semantic Versioning guarantees after stable publication.

The adapter may release more frequently than the core. Its `go.mod` must depend on a published core version and must not contain a local `replace` directive.

The npm packages are ESM-only and require Node.js 26 or later. Package versions do not select a language or IR contract; the supported opaque identifiers are exposed separately by the package API. Before publication, workspace manifests use `0.0.0-development`, and packed artifacts must contain no `workspace:`, `file:`, or local-path dependency.

Release candidates and stable releases follow this dependency order:

1. validate and tag the Go core and CLI source;
2. validate the packed TypeScript core against a clean consumer, then publish the core npm package;
3. validate and publish the go-rod and Playwright adapters against already published compatible core versions;
4. build CLI archives from the validated Go core tag.

An adapter release may be retried without changing a core version. The Playwright adapter must declare a published compatible `@hsblabs/scrape-kdl` range and must never rely on the workspace-only development version in a published artifact. Publication remains a separate project-owner gate; repository verification only prepares and inspects artifacts. `make rc-check` is non-publishing, while the manual release workflows require an exact confirmation and approval through the protected `v1-publication` environment.
