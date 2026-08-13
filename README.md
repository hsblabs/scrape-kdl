# scrape-kdl

[![Go Reference](https://pkg.go.dev/badge/github.com/hsblabs/scrape-kdl.svg)](https://pkg.go.dev/github.com/hsblabs/scrape-kdl)
[![Go Reference: go-rod adapter](https://pkg.go.dev/badge/github.com/hsblabs/scrape-kdl/adapters/rod.svg)](https://pkg.go.dev/github.com/hsblabs/scrape-kdl/adapters/rod)
[![npm: @hsblabs/scrape-kdl](https://img.shields.io/npm/v/%40hsblabs%2Fscrape-kdl?logo=npm)](https://www.npmjs.com/package/@hsblabs/scrape-kdl)
[![npm: @hsblabs/scrape-kdl-playwright](https://img.shields.io/npm/v/%40hsblabs%2Fscrape-kdl-playwright?logo=npm)](https://www.npmjs.com/package/@hsblabs/scrape-kdl-playwright)

`scrape-kdl` is a Go reference implementation for declaring HTML extraction in KDL, validating it into a language-neutral IR, and executing it through HTTP or a live browser adapter.

Current stable release target: `v1.0.2`. Install it only after publication and
post-publication verification complete. See `docs/release-readiness.md` and
`docs/adr/0009-stable-v1-owner-gate-override.md` for the release record.
The normative specification documents use the v0.1 document series and the initial compatibility identifiers `language-version="2026-07-15"` and `irVersion: "2026-07-15"`.

```text
KDL source
  -> parser
  -> semantic validation and type checking
  -> Validated IR
  -> HTTP runtime or browser adapter
  -> structured extraction result
```

## Implemented

- extractor and transform-module documents;
- relative imports with aliases and cycle detection;
- stable diagnostics and Validated IR JSON;
- declared transforms, built-ins, match transforms, and external transforms;
- HTTP fetch, sessions, charset decoding, response limits, and explicit offline snapshot execution;
- portable CSS selector profile;
- browser workflow and live-DOM extraction;
- trusted-spec JavaScript evaluation with explicit opt-in;
- dependency-neutral `BrowserAdapter` contract;
- optional go-rod adapter in `adapters/rod`;
- KDL slashdash suppression and common KDL 2 integer forms;
- release, CI, scheduled fuzzing, security, and contribution scaffolding;
- host URL policy for initial targets and HTTP redirects;
- hardened common-HTML parsing for raw-text and truncated documents.

## Responsible use

Use `scrape-kdl` only where you are authorized to automate access. Check the
target's terms and `robots.txt`, limit request rate and concurrency, identify
the client honestly, and handle personal or copyrighted material appropriately.
The project does not support anti-bot or access-control circumvention.

See [`docs/responsible-use.md`](docs/responsible-use.md) before targeting a live
service.

## Authoring patterns

See [`docs/patterns.md`](docs/patterns.md) for caller-owned pagination and
list-to-detail extraction with CLI, Go, and TypeScript loops.
See [`docs/cookbook.md`](docs/cookbook.md) for strict transform pipelines that
handle localized numbers, optional IDs, links, blank cells, and date strings.
See [`docs/authoring.md`](docs/authoring.md) for the bounded Go and TypeScript
Authoring Document, explicit-version built-in catalog, and deterministic KDL
writer.

## Stable release install

Install `v1.0.2`:

```bash
go install github.com/hsblabs/scrape-kdl/cmd/scrape-kdl@v1.0.2
go install github.com/hsblabs/scrape-kdl/adapters/rod/cmd/scrape-kdl-rod@v1.0.2
npm install \
  @hsblabs/scrape-kdl@1.0.2 \
  @hsblabs/scrape-kdl-playwright@1.0.2
```

The same version is available as Linux and macOS CLI archives for amd64 and
arm64 from the core and go-rod GitHub Releases. Verify every downloaded
archive against its accompanying `checksums.txt`.

For source development:

```bash
git clone https://github.com/hsblabs/scrape-kdl.git
cd scrape-kdl
npm ci
make verify
```

## CLI

Validate a specification:

```bash
scrape-kdl validate ./fixtures/valid/basic-http.kdl
```

Emit Validated IR:

```bash
scrape-kdl compile ./fixtures/valid/basic-http.kdl --emit-ir
```

Extract from a saved HTML fixture:

```bash
scrape-kdl extract ./fixtures/valid/basic-http.kdl \
  --html ./fixtures/html/basic-http.html
```

Extract over HTTP:

```bash
scrape-kdl extract ./extractor.kdl \
  --input item_id=123 \
  --session-file ./session.json
```

Session files use JSON and should be protected as secrets:

```json
{
  "headers": {
    "Accept-Language": ["ja"],
    "Authorization": ["Bearer example"]
  },
  "cookies": [
    {"name": "session", "value": "example"}
  ]
}
```

Use `--session-file -` to read the document from standard input without creating a file. The former `--header` and `--cookie` flags were removed at the v0.5 CLI boundary because command arguments can leak through shell history or process inspection. Move those values into the session JSON document.

Every command supports `-h` and `--help`. Use `--json` for a single machine-readable result envelope and `-` for an unambiguous input or output stream. Exit statuses are 0 for success, 1 for processing failure, 2 for usage errors, 130 for `SIGINT`, and 143 for `SIGTERM`. See `docs/cli.md` for the complete automation contract.

Print build metadata:

```bash
scrape-kdl version
```

## Go API

```go
program, diagnostics, err := scrapekdl.CompileFile(ctx, "extractor.kdl")
if err != nil {
    // Cancellation, filesystem, and other operational failures retain their cause.
}
if diagnostics.HasErrors() {
    // Render diagnostics and stop before network or browser activity.
}

result, err := program.Extract(ctx, map[string]any{
    "item_id": "123",
}, scrapekdl.Options{
    RequestTimeout: 15 * time.Second,
    URLPolicy: func(ctx context.Context, target *url.URL) error {
        // Reject private networks or hosts outside an application allowlist.
        return allowTarget(target)
    },
})
if err != nil {
    // Inspect *scrapekdl.ExecutionError.Code.
}

var output struct {
    Title string `json:"title"`
}
if err := result.Decode(&output); err != nil {
    // Reject missing, null, mismatched, or overflowing values.
}
```

The public API includes:

- context-first `Compile`, `Validate`, `CompileFile`, `ValidateFile`, `CompileFS`, and `ValidateFS` entry points;
- injected source loading for deterministic import resolution without filesystem access;
- `fs.FS` compilation for embedded and application-owned specification trees;
- immutable `Program.Metadata` snapshots;
- immutable `Program.Descriptor` acquisition settings;
- `Program.IRJSON`;
- `Program.Extract`, HTTP-only `Program.ExtractHTML`, and acquisition-free `Program.ExtractSnapshot`;
- strict `Result.Decode` conversion into typed Go structs and maps;
- HTTP client and session injection;
- external transform registry;
- custom charset decoding;
- initial-target and HTTP-redirect URL policy hooks;
- browser adapter injection;
- adapter-facing `NormalizeBrowserResult` validation and normalization;
- explicit `SupportedLanguageVersions` and `SupportedIRVersions` registries.
- a separate `authoring` package for bounded semantic construction, explicit
  built-in catalog selection, and deterministic KDL writing.

See `docs/public-api-v1.md` for the shared Go/TypeScript capability contract and intentional idiomatic differences.

The `@hsblabs/scrape-kdl` core package is ESM-only and supports Node.js 22 or later and Bun 1.3 or later.
Its root entry point exposes the approved compiler, diagnostic, IR, runtime, browser-adapter, and extension types; `@hsblabs/scrape-kdl/authoring` exposes bounded semantic authoring and KDL writing; `@hsblabs/scrape-kdl/node` contains filesystem conveniences.
The package independently compiles `fixtures/valid/basic-http.kdl`, matches the Go golden IR and canonical JSON, and matches the shared dated-version diagnostic fixture without invoking Go.
The complete documented KDL parser, injectable import graph, semantic validator, type checker, capability resolver, dated IR lowerer, HTTP/offline-snapshot runtime, and browser-library-neutral runtime run behind this boundary. Shared Go/TypeScript gates compare diagnostics, canonical IR, extraction results, warnings, and partial state; the HTTP runtime uses the pinned `parse5` WHATWG tree builder.
`@hsblabs/scrape-kdl-playwright` is the official Playwright adapter. It owns isolated per-extraction browser contexts and an extraction-wide lease; no concrete browser library is a dependency of the core package.
See `docs/spec/conformance-coverage.md` and `docs/html-compatibility.md` for the audited rule inventory and parser-compatibility gates.

## Browser mode

Browser mode uses an application-supplied adapter. JavaScript is disabled by default.

```go
result, err := program.Extract(ctx, inputs, scrapekdl.Options{
    Browser:         adapter,
    AllowJavaScript: true, // trusted specs only
})
```

The go-rod implementation is a separate module:

For stable `v1.0.2`:

```bash
go get github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.2
```

An adapter wrapping one mutable page can implement `BrowserAdapterLease`. The runtime acquires it for the complete extraction, preventing navigation, workflow, and reads from interleaving across concurrent calls. The go-rod adapter implements this automatically.

See `docs/browser-runtime.md`, `docs/rod-adapter.md`, and `adapters/rod/README.md`.

## Security

KDL is executable configuration. A browser spec containing `evaluate-js` is equivalent to code executed in the target page context. Only execute trusted specs and apply outbound-network, process-isolation, timeout, and secret-handling controls appropriate to the host application.

See `SECURITY.md` and `docs/security-model.md`.

## Supported platforms

- Linux: supported; amd64 and arm64 release artifacts are available from the GitHub Releases.
- macOS: supported; amd64 and arm64 release artifacts are available from the GitHub Releases.
- Windows: explicitly unsupported. No CI coverage, release artifact, compatibility guarantee, or Windows-specific bug support is provided. Incidental compilation does not make Windows supported.

## Compatibility and known limits

- Supported operating systems: Linux and macOS only. Windows is out of scope.
- Minimum Go version: 1.26.
- Minimum Node.js version: 22 for the TypeScript packages.
- The core TypeScript package also supports Bun 1.3 or later; the Playwright adapter is validated on Node.js.
- CI targets Go 1.26 and Node.js 22 on Linux and macOS.
- The language is built on the KDL 2 data model but the reference parser intentionally supports the subset defined by the Scraping KDL v0.1 specification document series.
- The Go HTTP runtime uses pinned `golang.org/x/net/html` WHATWG tree construction and is checked against the versioned portable HTML compatibility manifest.
- Browser mode uses the browser's live DOM and does not serialize/re-associate static nodes.
- The TypeScript compiler and runtime are primary v1 deliverables; type generation, a language server, an inspector UI, and a browser extension remain future milestones.
- The checked-in TypeScript packages include the complete compiler, HTTP/browser runtimes, and official Playwright adapter with publishable package gates.

See `docs/compatibility.md`, `docs/kdl-parser-conformance.md`, and `docs/release-gates.md`. The release-gate document links the executable support/example matrix and measured performance policy.

## Development

Offline verification:

```bash
npm ci
make verify
```

Real go-rod dependency verification:

```bash
make test-rod
```

Chromium E2E:

```bash
make test-rod-e2e
```

Cross-language conformance:

```bash
make conformance
go run ./cmd/conformance-runner --suite invalid --output invalid-go.json
npm run conformance:typescript
```

TypeScript package verification, including a packed clean-consumer install:

```bash
npm run verify:typescript
```

See `conformance/README.md` for suite selection, the language-neutral result format, normalization, and divergence policy.

Release preparation:

```bash
make release-check
```

## Coding agents

- `AGENTS.md`: shared repository instructions for coding agents
- `CLAUDE.md`: Claude Code entrypoint importing `AGENTS.md`

## Documentation

- `docs/spec/language-v0.1.md`
- `docs/spec/builtins-v0.1.md`
- `docs/spec/selectors-v0.1.md`
- `docs/compiler-pipeline.md`
- `docs/http-runtime.md`
- `docs/browser-runtime.md`
- `docs/rod-adapter.md`
- `docs/public-api-v1.md`
- `docs/roadmap-v1.md`
- `docs/versioning.md`
- `docs/releasing.md`
- `docs/release-readiness.md`
- `docs/migrating-to-v1.md`
- `docs/release-notes-v1.md`
- `docs/release-notes-v1.0.1.md`

## License

Apache-2.0. See `LICENSE` and `NOTICE`.
