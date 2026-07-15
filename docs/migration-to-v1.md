# Migration to v1

This guide covers compatibility breaks made while the project was an untagged pre-v1 working draft.

## Compatibility identifiers

Replace integer document versions and the former `0.1` language/IR identifiers with the opaque dated contract `2026-07-15`. Every extractor and module declares both `version="2026-07-15"` and `language-version="2026-07-15"`; consumers must regenerate Validated IR with `irVersion: "2026-07-15"`.

## Go library

Use `github.com/hsblabs/scrape-kdl`; legacy organization paths are not aliases. Compilation is context-first: `CompileFile(ctx, path)` and `ValidateFile(ctx, path)` now require `context.Context`, while in-memory callers use `Compile(ctx, Source, CompileOptions)` or `Validate`. Inject import loading through `CompileOptions.Loader`. `Program.Version()` returns a string rather than an integer.

The Go HTTP runtime now uses WHATWG document tree construction through `golang.org/x/net/html`. Code that depended on the previous approximation must update its expected tree/output instead of requesting legacy parsing.

## TypeScript library

Install the ESM-only `@hsblabs/scrape-kdl` package on Node.js 26 or later. Import filesystem conveniences from `@hsblabs/scrape-kdl/node`; the root entry point has no ambient filesystem or browser dependency. Install `@hsblabs/scrape-kdl-playwright` separately for the official browser adapter.

## CLI and secrets

The v1 CLI is Go-only. Replace direct `--header` and `--cookie` arguments with `--session-file PATH` or `--session-file -`; command-line secret values are deliberately rejected. Automation should use `--json`, rely on documented stdout/stderr separation, and handle exit statuses 0, 1, 2, 130, and 143.

## Browser execution

JavaScript remains disabled by default. Set the library opt-in only for trusted specification code. Browser mode requires an adapter and preserves URL policy, cancellation, timeouts, session boundaries, and extraction-wide page leases. Chromium is supported; Firefox and WebKit Playwright results are best effort.

## Diagnostics and runtime errors

Consume structured diagnostic and execution codes rather than matching arbitrary prose. Codes and meanings are frozen for v1; message wording is stable where cross-runtime fixtures require it and otherwise may improve compatibly.
