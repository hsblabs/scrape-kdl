# Migrating from the untagged working draft to v1

There was no supported public release before v1. This guide is for applications that tested or vendored an untagged development revision.

## KDL documents and Validated IR

- Replace integer document versions such as `version=1` with a quoted document revision such as `version="2026-07-15"`.
- Add `language-version="2026-07-15"` to every extractor and transform module.
- Regenerate Validated IR. The initial supported IR identifier is `2026-07-15`; working-draft `0.1` IR is rejected before acquisition.
- Do not rely on unspecified KDL 2 syntax. The supported subset is the one documented in `docs/spec/`.
- Replace consumer-owned transform name lists and KDL escaping with the explicit
  versioned authoring catalog and deterministic writer when the bounded
  Authoring Document covers the use case. Compile writer output normally; it is
  KDL Source, not Validated IR.

## Go API

- Use the module path `github.com/hsblabs/scrape-kdl`.
- Pass `context.Context` as the first argument to `Compile`, `Validate`, `CompileFile`, and `ValidateFile`.
- Handle the operational `error` returned by all compile and validate entry points separately from document `Diagnostics`. Use `errors.Is` or `errors.As` for cancellation, deadlines, filesystem failures, and injected-loader causes.
- Use `CompileFS` or `ValidateFS` for specifications stored in `embed.FS` or another application-owned `fs.FS`; import names remain slash-separated `io/fs` paths.
- Use `Program.Descriptor()` when a Go host only needs validated fetch mode, raw URL template, and session policy; avoid decoding `IRJSON` for those acquisition settings.
- Use `Program.ExtractSnapshot` to evaluate saved HTML against the exact compiled HTTP- or browser-mode program. Do not rewrite browser sources to HTTP mode; programs with workflow or JavaScript requirements now fail explicitly with `E_SNAPSHOT_UNSUPPORTED`.
- Use `Result.Decode(&destination)` instead of consumer-owned `map[string]any` assertions. Decoding is strict: missing required fields, nullability mismatches, unknown fields, sign changes, and numeric overflow return errors.
- Treat `Program.Version()` as a string rather than an integer.
- Use the exported supported-language and supported-IR registries instead of comparing version dates.
- Keep browser libraries in adapter modules; the core Go module does not depend on go-rod.
- Import `github.com/hsblabs/scrape-kdl/authoring` for bounded extractor
  construction. Select `BuiltinCatalog("2026-07-15")`; there is no implicit
  latest catalog.

## TypeScript API

- Use Node.js 26 or later and ESM imports.
- Import filesystem-free APIs from `@hsblabs/scrape-kdl` and filesystem helpers from `@hsblabs/scrape-kdl/node`.
- Install `@hsblabs/scrape-kdl-playwright` separately when Playwright browser execution is required.
- Treat the exported IR structures as readonly wire-contract declarations.
- Catch `SourceLoadError` for injected-loader failures and inspect its `cause`; aborts reject with the abort reason rather than becoming diagnostics.
- Use `program.descriptor` for the same immutable acquisition settings in TypeScript; use `program.ir` only when the complete Validated IR is required.
- Use `program.extractSnapshot` for the acquisition-free counterpart of Go `Program.ExtractSnapshot`.
- Import `@hsblabs/scrape-kdl/authoring` for the matching bounded authoring model,
  frozen versioned catalog, built-in call helper, and KDL writer.

## CLI automation

- Replace direct `--header` and `--cookie` values with `--session-file FILE` or `--session-file -`.
- Use `--json` when automation needs a one-document result envelope.
- Handle exit statuses `0`, `1`, `2`, `130`, and `143` as documented in `docs/cli.md`.
- Do not assign standard input to more than one of the KDL source, HTML input, or session input.

## Runtime behavior

- The default HTTP User-Agent is `scrape-kdl/1.0` in Go and TypeScript. Set an explicit user agent if a server policy depends on another value.
- JavaScript remains disabled unless explicitly enabled for a trusted specification.
- Supply a URL policy and isolated HTTP or browser state when specifications or tenants do not share trust.
- Expect WHATWG HTML tree construction from `golang.org/x/net/html` and `parse5`; output may differ from the earlier approximation on malformed HTML.

Run the independent consumer examples and the full conformance suite after migrating. Old document, language, and IR identifiers are intentionally not accepted as aliases.
