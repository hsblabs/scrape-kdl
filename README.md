# scrape-kdl

`scrape-kdl` is a Go reference implementation for declaring HTML extraction in KDL, validating it into a language-neutral IR, and executing it through HTTP or a live browser adapter.

Status: pre-release working draft, implementation milestone M5.
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
- HTTP fetch, sessions, charset decoding, response limits, and offline HTML fixtures;
- portable CSS selector profile;
- browser workflow and live-DOM extraction;
- trusted-spec JavaScript evaluation with explicit opt-in;
- dependency-neutral `BrowserAdapter` contract;
- optional go-rod adapter in `adapters/rod`;
- KDL slashdash suppression and common KDL 2 integer forms;
- release, CI, scheduled fuzzing, security, and contribution scaffolding;
- host URL policy for initial targets and HTTP redirects;
- hardened common-HTML parsing for raw-text and truncated documents.

## Install

After the first tagged release:

```bash
go install github.com/hsblabs/scrape-kdl/cmd/scrape-kdl@latest
```

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

Use `--session-file -` to read the document from standard input without creating a file. Direct `--header` and `--cookie` values remain accepted for compatibility but are deprecated because they can leak through shell history or process inspection. When combined temporarily during migration, direct values are appended after file values.

Print build metadata:

```bash
scrape-kdl version
```

## Go API

```go
program, diagnostics := scrapekdl.CompileFile(ctx, "extractor.kdl")
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
```

The public API includes:

- context-first `Compile`, `Validate`, `CompileFile`, and `ValidateFile` entry points;
- injected source loading for deterministic import resolution without filesystem access;
- immutable `Program.Metadata` snapshots;
- `Program.IRJSON`;
- `Program.Extract` and `Program.ExtractHTML`;
- HTTP client and session injection;
- external transform registry;
- custom charset decoding;
- initial-target and HTTP-redirect URL policy hooks;
- browser adapter injection.
- adapter-facing `NormalizeBrowserResult` validation and normalization.
- explicit `SupportedLanguageVersions` and `SupportedIRVersions` registries.

See `docs/public-api-v1.md` for the shared Go/TypeScript capability contract and intentional idiomatic differences.

The `@hsblabs/scrape-kdl` workspace is an ESM-only, publishable package scaffold for Node.js 26 and later.
Its root entry point exposes the approved compiler, diagnostic, IR, runtime, browser-adapter, and extension types; `@hsblabs/scrape-kdl/node` contains filesystem conveniences.
The package independently compiles `fixtures/valid/basic-http.kdl`, matches the Go golden IR and canonical JSON, and matches the shared dated-version diagnostic fixture without invoking Go.
The complete documented KDL parser and injectable import graph run behind this boundary with shared Go/TypeScript diagnostic fixtures. Issues #13 and #14 complete the semantic compiler and HTTP runtime; until then, `Program.extract` is intentionally unavailable at runtime.
The reserved `@hsblabs/scrape-kdl-playwright` workspace remains private until Issue #16 and no concrete browser library is a dependency of the core package.
See `docs/spec/conformance-coverage.md` for the audited rule inventory and the exact slice boundary.

## Browser mode

Browser mode uses an application-supplied adapter. JavaScript is disabled by default.

```go
result, err := program.Extract(ctx, inputs, scrapekdl.Options{
    Browser:         adapter,
    AllowJavaScript: true, // trusted specs only
})
```

The go-rod implementation is a separate module:

```bash
go get github.com/hsblabs/scrape-kdl/adapters/rod@latest
```

An adapter wrapping one mutable page can implement `BrowserAdapterLease`. The runtime acquires it for the complete extraction, preventing navigation, workflow, and reads from interleaving across concurrent calls. The go-rod adapter implements this automatically.

See `docs/browser-runtime.md` and `adapters/rod/README.md`.

## Security

KDL is executable configuration. A browser spec containing `evaluate-js` is equivalent to code executed in the target page context. Only execute trusted specs and apply outbound-network, process-isolation, timeout, and secret-handling controls appropriate to the host application.

See `SECURITY.md` and `docs/security-model.md`.

## Supported platforms

- Linux: supported; amd64 and arm64 release artifacts will be provided after an approved release.
- macOS: supported; amd64 and arm64 release artifacts will be provided after an approved release.
- Windows: explicitly unsupported. No CI coverage, release artifact, compatibility guarantee, or Windows-specific bug support is provided. Incidental compilation does not make Windows supported.

## Compatibility and known limits

- Supported operating systems: Linux and macOS only. Windows is out of scope.
- Minimum Go version: 1.26.
- Minimum Node.js version: 26 for the TypeScript packages.
- CI targets Go 1.26 and Node.js 26 on Linux and macOS.
- The language is built on the KDL 2 data model but the reference parser intentionally supports the subset defined by the Scraping KDL v0.1 specification document series.
- The HTTP runtime's internal parser handles ordinary scraping fixtures but is not yet a complete WHATWG HTML tree builder.
- Browser mode uses the browser's live DOM and does not serialize/re-associate static nodes.
- The TypeScript compiler and runtime are primary v1 deliverables; type generation, a language server, an inspector UI, and a browser extension remain future milestones.
- The checked-in TypeScript package has the complete parser and import loader plus a publishable boundary and package gates; semantic compilation and execution are not yet complete.

See `docs/compatibility.md` and `docs/kdl-parser-conformance.md`.

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
npm run conformance:typescript-slice
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
- `docs/public-api-v1.md`
- `docs/roadmap-v1.md`
- `docs/versioning.md`
- `docs/releasing.md`

## License

Apache-2.0. See `LICENSE` and `NOTICE`.
