# Versioning policy

The Go modules and npm packages each follow Semantic Versioning. Public release
trains assign them one shared version after validating every surface together.

- Core module: `github.com/hsblabs/scrape-kdl`
- go-rod adapter: `github.com/hsblabs/scrape-kdl/adapters/rod`
- TypeScript core: `@hsblabs/scrape-kdl`
- Playwright adapter: `@hsblabs/scrape-kdl-playwright`

Core release tags use `vX.Y.Z`. Adapter release tags use `adapters/rod/vX.Y.Z`.

Before `v1.0.0`, a minor release may contain breaking public API changes. Patch releases must remain backward compatible within the same minor version.

Document, language, and Validated IR compatibility identifiers are separate from module versions and use opaque calendar-date strings. The initial contract is `2026-07-15`: an extractor declares a document `version`, explicitly selects `language-version="2026-07-15"`, and compiles to `irVersion: "2026-07-15"`. Implementations compare identifiers against explicit supported-version registries and do not infer compatibility from date ordering. A later document revision may advance independently; an observable language or IR contract change requires a newly approved date and compatibility notes.

Stable diagnostic codes are part of the language tooling contract. Their message text may improve without a major language-version change, but code meaning must not silently change.

The CLI contract candidate began at v0.5 and was frozen at v1.0.0. Changes to
commands, help, streams, JSON envelopes, exit status classes, signals, or secret
input follow v1 Semantic Versioning and require black-box tests, compatibility
notes, and a migration update when consumers must act.

Public core, npm, Playwright, and go-rod releases use the same version and are
published by one orchestrator. Go still requires separate `vX.Y.Z` and
`adapters/rod/vX.Y.Z` tag names for the two modules; both tags point to the same
release commit. The adapter's `go.mod` must depend on that exact core version
and must not contain a local `replace` directive. Private operational releases
retain their separately approved workflows.

The npm packages are ESM-only and require Node.js 26 or later. Package versions do not select a language or IR contract; the supported opaque identifiers are exposed separately by the package API. Before publication, workspace manifests use `0.0.0-development`, and packed artifacts must contain no `workspace:`, `file:`, or local-path dependency.

Private operational versions use `X.Y.Z-private.N`, where `N` starts at 1 and
increases for every replacement. npm publishes these versions with restricted
access and the `private` dist-tag. GitHub marks their CLI Releases as
prereleases. Never move or reuse a private tag or npm version. The latest private
version supersedes earlier private versions; no compatibility promise applies
between private versions.

Workspace package manifests default to restricted access so an accidental
authenticated publish cannot make them public. Public release staging must
explicitly set `publishConfig.access=public` and still passes the repository
visibility and owner-controlled publication gates.

Release candidates and stable releases follow this dependency order:

1. privately build and inspect the complete CLI and npm artifact bundle;
2. validate and tag the Go core and CLI source, then create CLI archives from that exact tag;
3. confirm the tagged core through the Go module proxy;
4. publish the clean-consumer-tested TypeScript core, then the Playwright adapter against that published core;
5. validate and publish the go-rod adapter against the published Go core.

A partial public release may be retried only for the same version, source
commit, and inspected artifacts. Matching immutable states are skipped; a
mismatch requires a new prerelease or patch version. The Playwright adapter must
declare a published compatible `@hsblabs/scrape-kdl` range and must never rely
on the workspace-only development version in a published artifact. Publication
remains a separate project-owner gate; repository verification only prepares
and inspects artifacts.
