# Versioning policy

The Go modules follow Semantic Versioning independently.

- Core module: `github.com/hsblabs/scrape-kdl`
- go-rod adapter: `github.com/hsblabs/scrape-kdl/adapters/rod`

Core release tags use `vX.Y.Z`. Adapter release tags use `adapters/rod/vX.Y.Z`.

Before `v1.0.0`, a minor release may contain breaking public API changes. Patch releases must remain backward compatible within the same minor version.

The Scraping KDL language version is separate from module versions. An extractor declares `version=1`, while the written specification is named `v0.1`. A module release may support multiple language versions in the future.

Stable diagnostic codes are part of the language tooling contract. Their message text may improve without a major language-version change, but code meaning must not silently change.

The adapter may release more frequently than the core. Its `go.mod` must depend on a published core version and must not contain a local `replace` directive.
