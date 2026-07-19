# Go and TypeScript public API contract

Status: v1 contract candidate. Breaking changes remain possible before the v0.9 API freeze, but every change requires tests, compatibility notes, and a migration path. Semantic behavior remains governed by the dated language and IR specifications.

## Shared capability surface

Go and TypeScript expose the same observable capabilities:

| Capability | Go candidate | TypeScript candidate |
| --- | --- | --- |
| compile an in-memory source | `Compile(ctx, Source, CompileOptions)` | `compile(source, options)` |
| validate an in-memory source | `Validate(ctx, Source, CompileOptions)` | `validate(source, options)` |
| compile or validate a file | context-first `CompileFile` and `ValidateFile` | asynchronous Node.js entry-point functions |
| resolve imports without filesystem access | `CompileOptions.Loader` | `CompileOptions.loader` |
| inspect validated metadata | `Program.Metadata()` | `Program.metadata` |
| inspect Validated IR | `Program.IRJSON()` | `Program.ir` |
| execute HTTP or saved HTML | `Program.Extract` and `Program.ExtractHTML` | `Program.extract`; the implementation may expose an equivalent saved-HTML test hook |
| register external transforms | `Options.ExternalTransforms` | `ExecutionOptions.externalTransforms` |
| enforce URL policy | `Options.URLPolicy` | `ExecutionOptions.urlPolicy` |
| block non-public targets | `PublicInternetURLPolicy`, `NewPublicInternetHTTPClient` | host-supplied policy; built-in equivalent is a follow-up |
| inject HTTP behavior and sessions | `http.Client`, `Session`, charset decoder | platform `fetch`, `Session`, decoded UTF-8 runtime contract |
| execute browser mode | `BrowserAdapter` | `BrowserAdapter` |
| serialize mutable browser pages | optional `BrowserAdapterLease` | optional `BrowserAdapterLease` |
| bound first/one browser queries | optional `BrowserAdapterQueryLimit` | optional `BrowserAdapterQueryLimit` |
| inspect extraction results | `Result`, `Warning`, `ExecutionError` | `ExtractionResult`, `Warning`, `ExecutionError` |
| discover compatibility | `SupportedLanguageVersions`, `SupportedIRVersions` | `supportedLanguageVersions`, `supportedIRVersions` |

The contract snapshot remains in `docs/api/typescript/`. The publishable implementation boundary is checked directly through `@hsblabs/scrape-kdl` and `@hsblabs/scrape-kdl/node`; declaration-to-schema and clean-consumer gates keep the package aligned with the approved Issue #19 surface. The root package implements the complete compiler plus HTTP, offline-HTML, and browser-library-neutral runtimes while preserving that approved surface.

The core declarations map to `@hsblabs/scrape-kdl`; Node.js file conveniences map to `@hsblabs/scrape-kdl/node`. The separate `@hsblabs/scrape-kdl-playwright` package supplies an official `BrowserAdapter` without adding Playwright to the core package's runtime dependencies.

## Source loading

`Source.Path` is a stable logical identity used for diagnostics, source spans, import resolution, and IR file identities. It is not required to name an operating-system file. Imports are resolved lexically relative to the importing source before the loader is called.

The Go loader receives the compile context and the resolved path. The TypeScript loader receives the resolved path, importing path, and optional abort signal. A source compile with imports and no loader fails before IR is returned. Loaders are responsible only for returning bytes or text; parsing, validation, cycle detection, hashing, and deterministic ordering remain compiler responsibilities.

Loader errors become structured diagnostics. Loader implementations must not place credentials, source contents, or other secrets in returned error messages.

The file conveniences are separate from injected loading. Go uses the host filesystem in `CompileFile` and `ValidateFile`. TypeScript keeps filesystem functions in the Node.js entry point so the core package does not acquire ambient filesystem authority.

## Validated program and diagnostics

A successful compile returns an opaque program plus ordered structured diagnostics. A program exposes document name and version, language and IR versions, source identities, and the exact derived capability set. Metadata collections are snapshots and cannot mutate compiler state.

Compilation or validation never performs HTTP or browser activity. A program is not returned when error diagnostics exist. Diagnostic codes, severities, paths, spans, and ordering are shared compatibility surfaces; language-specific error classes do not replace structured diagnostics.

## Execution and extension boundaries

Execution accepts inputs, session state, URL policy, time and size bounds, external transforms, and an optional browser adapter. JavaScript remains disabled unless explicitly enabled. URL policy applies to initial targets and redirects. Cancellation must reach source loading, HTTP work, browser operations, and external transforms.

Browser elements are opaque adapter-owned handles. The public browser interfaces are declared in the root packages; no internal Go package type or Playwright/rod type appears in a public signature. Official adapters translate their library-specific handles behind this boundary. An adapter wrapping a mutable page may implement the optional extraction-wide lease. An adapter may also implement the optional bounded-query interface; the core falls back to `QueryAll`, so this addition does not break existing adapters.

External transforms receive cancellation and JSON-compatible values. Implementations validate returned values immediately against declared output types. Registration does not grant browser, filesystem, or subprocess access.

## Intentional language differences

- Go uses `context.Context`, `(value, diagnostics)` results, `error`, `net/http`, and `time.Duration`. TypeScript uses promises, optional `AbortSignal`, readonly data, platform `fetch`, `URL`, and millisecond numbers.
- Go file functions belong to the primary package because filesystem APIs are standard Go library conventions. TypeScript file functions belong only to the Node.js entry point.
- Go keeps Validated IR opaque and exposes JSON plus metadata so internal compiler representations do not become public. TypeScript exposes the published readonly IR declaration because it is already the package's cross-language wire contract.
- Go browser methods accept context as the first argument. TypeScript carries abort signals in operation options.
- Go external transforms are named functions over `any` with runtime validation. TypeScript narrows the proposal to `JsonValue` at the type boundary and still performs runtime validation.

These are surface differences only. They do not permit different accepted KDL, diagnostics, IR, capability derivation, extraction values, warnings, timeout behavior, or security defaults.

## Compatibility process

Before v0.9, a breaking public API change must update this document, both independent consumer checks, and `CHANGELOG.md` with a migration path. At v0.9 the candidate freezes. At v1.0.0, compatible additions and breaking changes follow Semantic Versioning independently for the Go modules and npm packages.
