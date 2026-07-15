# Versioning policy

The Go modules follow Semantic Versioning independently.

- Core module: `github.com/hsblabs/scrape-kdl`
- go-rod adapter: `github.com/hsblabs/scrape-kdl/adapters/rod`

Core release tags use `vX.Y.Z`. Adapter release tags use `adapters/rod/vX.Y.Z`.

Before `v1.0.0`, a minor release may contain breaking public API changes. Patch releases must remain backward compatible within the same minor version.

Document, language, and Validated IR compatibility identifiers are separate from module versions and use opaque calendar-date strings. The initial contract is `2026-07-15`: an extractor declares a document `version`, explicitly selects `language-version="2026-07-15"`, and compiles to `irVersion: "2026-07-15"`. Implementations compare identifiers against explicit supported-version registries and do not infer compatibility from date ordering. A later document revision may advance independently; an observable language or IR contract change requires a newly approved date and compatibility notes.

Stable diagnostic codes are part of the language tooling contract. Their message text may improve without a major language-version change, but code meaning must not silently change.

The adapter may release more frequently than the core. Its `go.mod` must depend on a published core version and must not contain a local `replace` directive.
