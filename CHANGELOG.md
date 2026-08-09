# Changelog

All notable implementation changes are recorded here. Formal releases will use Semantic Versioning.

## 1.0.0 — 2026-08-09

- Prepared stable `v1.0.0` publication for 2026-08-09 under an explicit
  project-owner override of the 14-day candidate gate. This is an owner-
  direction exception; publication and pre- and post-publication verification
  remain pending.
- Added bounded Go and TypeScript authoring APIs with an explicit-version
  built-in catalog and deterministic KDL writing. Catalog arguments expose
  finite allowed values and numeric bounds, and TypeScript `float(value)`
  preserves integral float syntax such as `1.0`. The shared tracer output is
  compiled by both implementations, while compiler syntax and Validated IR
  internals remain private.
- Added a compile-checked transform cookbook for localized numbers, optional IDs, relative links, blank cells, nullable pipelines, and date/time strings without changing coercion or type rules.
- Documented caller-owned pagination and list-to-detail extraction patterns with compile-checked KDL plus bounded CLI, Go, and TypeScript loops.
- Added Go `Program.ExtractSnapshot` and TypeScript `program.extractSnapshot` for acquisition-free portable extraction from supplied HTML, including browser-mode programs, with shared fixtures and explicit `E_SNAPSHOT_UNSUPPORTED` rejection for workflows and JavaScript output.
- Added strict, atomic `Result.Decode` conversion for typed Go structs and maps, including nested collections, exact integer conversion, explicit missing/null behavior, and unknown-field rejection without changing warning or partial state.
- Added immutable Go and TypeScript program descriptors for fetch mode, raw URL template, and session policy, avoiding full Validated IR decoding for host-owned acquisition.
- Added `CompileFS` and `ValidateFS` for nested, relative compilation from `fs.FS`, including `embed.FS`, lexical escape rejection, cancellation checks, and documented symlink-containment limits.
- Separated operational compilation failures from document diagnostics. Go compile and validate entry points now return an `error` that preserves cancellation and loader/filesystem causes, while TypeScript rejects loader failures with `SourceLoadError` and retains the original `cause`.
- Added responsible-use guidance and neutralized the remaining real-brand
  references in specifications and fixtures before public release.
- Fixed the public release boundary so private-channel versions cannot reach
  public workflows and public release candidates are marked as GitHub
  prereleases rather than the latest stable release.
- Prevented completed Playwright E2E tests from hanging on a residual macOS
  `fsevents` handle under Node.js 26.
- Added a private operational channel: restricted npm artifacts and OIDC
  publishing, authenticated private Go consumer checks, guarded private GitHub
  prereleases, and Linux/macOS go-rod CLI archives with checksums.
- Added a failure-safe private release bundle containing four CLI archives, two clean-consumer-tested npm archives, Apache `NOTICE` files, and SHA-256 checksums without publishing or changing workspace development versions.
- Added guarded private rehearsal, npm publication, and specification-site workflows; public workflows require visibility checks, typed confirmation, owner-configured GitHub Environments, and protected release tags.
- Pinned every GitHub Action to a full commit SHA and moved npm publication to globally serialized, tokenless OIDC trusted publishing with build/publish permission isolation.
- Unified public core, npm, Playwright, and go-rod publication behind one
  protected, resumable workflow while retaining the required distinct Go module
  tags.
- Prevented release preflight from querying future Go versions through the
  public proxy before their tags exist, avoiding cached negative responses
  without extending the bounded post-tag wait.
- Added v1 migration notes, draft release notes, an explicit maintenance window, post-publication verification, and immutable-version recovery procedures.
- Standardized the Go, TypeScript, and CLI default HTTP User-Agent as `scrape-kdl/1.0`.

- Made `scrape-kdl-rod` a usable browser CLI: typed `--input` (read from the Validated IR contract), `--session-file` with the core CLI's JSON schema and plaintext-flag rejection, `--timeout`, `--user-agent`, exact one-document `--json` envelopes, `-o`/`--out`, core-aligned help and exit statuses including `SIGINT`/`SIGTERM`, and default rejection of non-public initial navigation targets with `--allow-private-hosts` opt-out.
- Added `PublicInternetURLPolicy` and `NewPublicInternetHTTPClient` to the Go API. The CLI now rejects IANA special-purpose addresses not marked globally reachable, rejects empty DNS results, re-checks redirect and dial-time resolution against DNS rebinding, and uses direct connections so environment proxies cannot hide the selected target address. Pass `--allow-private-hosts` to restore ordinary local-network and proxy behavior. Library `Options` defaults are unchanged.
- Decoded WHATWG-labelled charsets (Shift_JIS, EUC-JP, ...) in the Go HTTP runtime by default via `golang.org/x/text/encoding/htmlindex`, aligning with the TypeScript runtime's `TextDecoder` fallback. Both runtimes now reject malformed byte sequences with `E_HTML_DECODE` through a shared compatibility manifest; `Options.CharsetDecoder` still overrides, and `E_HTML_CHARSET_UNSUPPORTED` indicates labels outside the WHATWG index.
- Prepared immutable execution plans once per compiled program, centralized output recovery/cardinality semantics across HTTP and browser modes, bounded first/one selector queries and collection overflow work, and de-duplicated diamond import traversal.
- Replaced TypeScript native regular-expression execution with the pinned RE2-compatible `re2js` runtime and added a nested-repetition regression test.
- Added machine-checked built-in, diagnostic, TypeScript export, and exact IR declaration contract gates; narrowed the exported IR types to the approved public surface.
- Added optional `BrowserAdapterQueryLimit` support with backward-compatible fallback, plus Playwright and go-rod implementations.
- Aligned runtime cancellation on `E_EXECUTION_CANCELED`, added a shared cross-runtime fixture, and propagated CLI diagnostic/JSON writer failures.
- Renamed TypeScript "slice" implementation and automation identifiers to the completed compiler/runtime terminology; use `conformance:typescript` and `test:typescript-contract` in workspace automation.
- Replaced the untagged integer/`0.1` working-draft identifiers with independent `2026-07-15` document, language, and Validated IR date strings; `Program.Version` now returns a string and the Go API exposes exact supported-version registries.
- Added the dated Validated IR schema layout, canonical JSON normalization, schema/golden validation, declaration-shape drift checks, and acquisition-time file/capability metadata preflight.
- Defined equivalent Go and TypeScript v1 API candidates, made Go compilation context-first with injected source loading, removed internal executor aliases from public browser signatures, and added independent consumer contract checks. This is a pre-v0.9 Go API break: `CompileFile` and `ValidateFile` now require `context.Context`.
- Completed the `2026-07-15` contract audit with a drift-checked normative coverage inventory, corrected the dated grammar summary, and added a bounded independent Node.js 26 TypeScript parser/compiler slice that matches the Go IR and shared diagnostic fixtures.
- Added the dated cross-language conformance manifest and result schemas, focused suite selection, a Go runner, the manifest-driven TypeScript slice runner, and CI failure gates for missing fixtures/artifacts and unapproved differences.
- Added the ESM-only `@hsblabs/scrape-kdl` npm package scaffold and reserved Playwright adapter workspace, approved root and Node.js entry points, full IR declarations, Node.js 26 Linux/macOS gates, coverage and source lint checks, and packed clean-consumer verification without a core Playwright dependency.
- Completed the TypeScript KDL subset parser and injectable source graph, added explicit Node.js filesystem loading, and introduced shared Go/TypeScript parser, invalid-UTF-8, import, cycle, and bounded-mutation fixtures with exact diagnostic parity.
- Moved all module paths to `github.com/hsblabs/scrape-kdl`.
- Added KDL slashdash suppression for nodes, arguments, properties, and child blocks.
- Added hexadecimal, octal, and binary integer literals.
- Added the optional `BrowserAdapterLease` contract.
- Added extraction-wide serialization to the go-rod adapter.
- Added browser failure-path, lease, and JavaScript result tests.
- Added a release-clean go-rod module and repository workspace.
- Added CLI build-version metadata.
- Added CI, scheduled browser E2E, core release, and adapter release workflows.
- Added Apache-2.0 licensing and project security/contribution policies.
- Added release, compatibility, parser-conformance, and security-model documentation.
- Added raw-text/RCDATA handling, truncated-document recovery, and common optional-end-tag normalization to the internal HTML parser.
- Added `Options.URLPolicy` for initial URL and HTTP redirect enforcement.
- Aligned runtime diagnostics with the normative diagnostic registry, including timeout and partial-extraction codes.
- Added KDL, CSS selector, and HTML parser fuzz tests with scheduled CI.
- Added release-tag validation and automatic diagnostic-documentation drift checks.
- Added `--session-file` JSON and standard-input support for secrets and deprecated direct `--header` and `--cookie` values.
- Changed explicit subcommand `-h` and `--help` requests to exit successfully.
- Defined browser JavaScript result representations and added `NormalizeBrowserResult` for adapter authors.
- Added immediate external-transform result validation with `E_EXTERNAL_TRANSFORM_RESULT_TYPE`.
- Defined `session policy="none"` as suppressing only explicit session input while preserving documented host-owned ambient state.
- Bounded millisecond workflow durations to the portable `time.Duration` maximum.
- Added coarse-grained offline HTML cancellation with `E_EXECUTION_CANCELED`.

## M4 — go-rod adapter

- Added an independent go-rod `BrowserAdapter` module.
- Added browser session headers, cookies, User-Agent handling, workflow actions, document/current JavaScript evaluation, scoped queries, and element reads.
- Added a go-rod example CLI and a build-tagged Chromium E2E test.

## M3 — browser runtime

- Added the dependency-neutral browser adapter contract.
- Added browser navigation, workflow execution, live-DOM extraction, and JavaScript opt-in.

## M2 — HTTP interpreter

- Added HTTP and offline HTML extraction.
- Added the public Go API and `scrape-kdl extract`.
- Added runtime inputs, URL-template expansion, sessions, timeout, and response-size limits.
- Added charset decoding and a custom charset callback.
- Added the internal DOM and portable CSS selector engine.
- Implemented built-in, declared pipeline, match, and external transforms.
- Added missing-value semantics, field recovery, row skipping, warnings, partial state, and runtime type checks.

## M1 — parser, validator, and Validated IR

- Implemented the Go reference parser, semantic validator, and Validated IR emitter.
- Added module imports, type checking, diagnostics, fixtures, and golden IR tests.
