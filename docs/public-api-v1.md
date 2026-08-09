---
status: frozen v1 stable contract
---

# Go and TypeScript public API contract

The v1 public surface is frozen. Compatible additions and breaking changes
follow Semantic Versioning; semantic behavior remains governed by the dated
language and IR specifications.

## Shared capability surface

Go and TypeScript expose the same observable capabilities:

| Capability | Go candidate | TypeScript candidate |
| --- | --- | --- |
| compile an in-memory source | `Compile(ctx, Source, CompileOptions) (*Program, Diagnostics, error)` | `compile(source, options)` |
| validate an in-memory source | `Validate(ctx, Source, CompileOptions) (Diagnostics, error)` | `validate(source, options)` |
| compile or validate a file | context-first `CompileFile` and `ValidateFile` | asynchronous Node.js entry-point functions |
| compile or validate an application filesystem | `CompileFS` and `ValidateFS` over `fs.FS` | Node.js entry-point or injected loader |
| resolve imports without filesystem access | `CompileOptions.Loader` | `CompileOptions.loader` |
| inspect validated metadata | `Program.Metadata()` | `Program.metadata` |
| inspect host-facing acquisition settings | `Program.Descriptor()` | `Program.descriptor` |
| inspect Validated IR | `Program.IRJSON()` | `Program.ir` |
| execute configured HTTP or browser acquisition | `Program.Extract` | `Program.extract` |
| execute supplied HTML without acquisition | `Program.ExtractSnapshot` | `Program.extractSnapshot` |
| register external transforms | `Options.ExternalTransforms` | `ExecutionOptions.externalTransforms` |
| enforce URL policy | `Options.URLPolicy` | `ExecutionOptions.urlPolicy` |
| block non-public targets | `PublicInternetURLPolicy`, `NewPublicInternetHTTPClient` | host-supplied policy; built-in equivalent is a follow-up |
| inject HTTP behavior and sessions | `http.Client`, `Session`, charset decoder | platform `fetch`, `Session`, decoded UTF-8 runtime contract |
| execute browser mode | `BrowserAdapter` | `BrowserAdapter` |
| serialize mutable browser pages | optional `BrowserAdapterLease` | optional `BrowserAdapterLease` |
| bound first/one browser queries | optional `BrowserAdapterQueryLimit` | optional `BrowserAdapterQueryLimit` |
| inspect extraction results | `Result`, `Result.Decode`, `Warning`, `ExecutionError` | `ExtractionResult`, `Warning`, `ExecutionError` |
| discover compatibility | `SupportedLanguageVersions`, `SupportedIRVersions` | `supportedLanguageVersions`, `supportedIRVersions` |
| discover built-in authoring metadata | `authoring.BuiltinCatalog(version)` | `builtinCatalog(version)` from `@hsblabs/scrape-kdl/authoring` |
| write a bounded authoring document | `authoring.Write` | `write` from `@hsblabs/scrape-kdl/authoring` |

The contract snapshot remains in `docs/api/typescript/`. The publishable implementation boundary is checked directly through `@hsblabs/scrape-kdl`, `@hsblabs/scrape-kdl/authoring`, and `@hsblabs/scrape-kdl/node`; declaration-to-schema and clean-consumer gates keep the package aligned with the approved surface. The root package implements the complete compiler plus HTTP, offline-HTML, and browser-library-neutral runtimes while preserving that approved surface.

The core declarations map to `@hsblabs/scrape-kdl`; the bounded semantic authoring model maps to `@hsblabs/scrape-kdl/authoring`; Node.js file conveniences map to `@hsblabs/scrape-kdl/node`. The separate `@hsblabs/scrape-kdl-playwright` package supplies an official `BrowserAdapter` without adding Playwright to the core package's runtime dependencies.

## Authoring boundary

The Go `authoring` package and TypeScript authoring entry point expose one
bounded semantic Authoring Document, a deterministic KDL writer, and a built-in
catalog selected by exact language version. The catalog includes transform
input and output constraints, nullability effects, positional arity, named
arguments, defaults, finite allowed values, and numeric bounds. It has no moving
`latest` alias.

Authoring output is KDL Source, not Validated IR. Callers must pass it through
the ordinary compiler and handle structured diagnostics before execution. The
authoring model does not expose compiler syntax nodes or internal IR and does
not model imports, modules, declared transforms, browser workflow, JavaScript,
defaults, comments, or arbitrary KDL nodes in its first tracer.

Deterministic KDL writing is also distinct from lossless source formatting. The
writer creates new canonical source and owns string escaping; it does not retain
comments or the lexical choices of an existing document. See
[`docs/authoring.md`](authoring.md) for the supported model and examples.

## Source loading

`Source.Path` is a stable logical identity used for diagnostics, source spans, import resolution, and IR file identities. It is not required to name an operating-system file. Imports are resolved lexically relative to the importing source before the loader is called.

The Go loader receives the compile context and the resolved path. The TypeScript loader receives the resolved path, importing path, and optional abort signal. A source compile with imports and no loader fails before IR is returned. Loaders are responsible only for returning bytes or text; parsing, validation, cycle detection, hashing, and deterministic ordering remain compiler responsibilities.

Cancellation, filesystem failures, and injected-loader failures are operational errors rather than document diagnostics. Go returns them as `error` while preserving the cause for `errors.Is` and `errors.As`. TypeScript rejects with the abort reason or `SourceLoadError`; `SourceLoadError.cause` retains the loader failure. Loader implementations must not place credentials, source contents, or other secrets in returned error messages.

The file conveniences are separate from injected loading. Go uses the host filesystem in `CompileFile` and `ValidateFile`. `CompileFS` and `ValidateFS` accept an application-provided `fs.FS`, require a non-directory `fs.ValidPath` root, resolve every nested import within that same filesystem, and reject lexical parent escapes before calling the filesystem. They preserve import-cycle, duplicate-import, and source-metadata behavior from ordinary compilation. Reads are checked for cancellation before and after each `fs.ReadFile`; the `fs.FS` interface cannot interrupt a read already in progress.

`CompileFS` provides lexical containment, not stronger capabilities than its supplied filesystem. In particular, `os.DirFS` can follow a symlink outside its directory. Use `os.Root.FS` when symlink escape prevention is a security requirement. TypeScript keeps filesystem functions in the Node.js entry point so the core package does not acquire ambient filesystem authority.

## Program descriptor

`Program.Descriptor()` in Go and `Program.descriptor` in TypeScript expose the validated fetch mode, raw URL template, and explicit-session policy without requiring a host to decode the complete Validated IR. Go returns a value copy. TypeScript returns a recursively frozen object. Mutating either returned value cannot change the compiled program.

The descriptor is intentionally limited to acquisition settings. Runtime inputs, workflow steps, output shapes, transforms, spans, and complete source metadata remain available through the Validated IR or `Program.Metadata`. Use the descriptor for a host-owned acquisition decision; use `IRJSON` or `program.ir` for language-neutral interchange, inspection, or tooling that needs the complete program.

## Validated program and diagnostics

A successful compile returns an opaque program plus ordered structured diagnostics and no operational error. A program exposes document name and version, language and IR versions, source identities, and the exact derived capability set. Metadata collections are snapshots and cannot mutate compiler state.

Compilation or validation never performs HTTP or browser activity. A program is not returned when error diagnostics exist or an operational error occurs. Syntax, semantic, and type findings remain deterministic diagnostics; cancellation, deadlines, filesystem failures, and loader failures never masquerade as KDL findings. Diagnostic codes, severities, paths, spans, and ordering are shared compatibility surfaces; language-specific operational error surfaces do not replace structured diagnostics.

## Execution and extension boundaries

Execution accepts inputs, session state, URL policy, time and size bounds, external transforms, and an optional browser adapter. JavaScript remains disabled unless explicitly enabled. URL policy applies to initial targets and redirects. Cancellation must reach source loading, HTTP work, browser operations, and external transforms.

Browser elements are opaque adapter-owned handles. The public browser interfaces are declared in the root packages; no internal Go package type or Playwright/rod type appears in a public signature. Official adapters translate their library-specific handles behind this boundary. An adapter wrapping a mutable page may implement the optional extraction-wide lease. An adapter may also implement the optional bounded-query interface; the core falls back to `QueryAll`, so this addition does not break existing adapters.

External transforms receive cancellation and JSON-compatible values. Implementations validate returned values immediately against declared output types. Registration does not grant browser, filesystem, or subprocess access.

### Offline snapshot execution

`Program.ExtractSnapshot` in Go and `Program.extractSnapshot` in TypeScript execute a whole compiled HTTP- or browser-mode program against an already-decoded HTML string without performing acquisition. They do not resolve URL inputs, invoke URL policy, use a session, send HTTP requests, acquire a browser lease, navigate, or call a browser adapter.

Snapshot eligibility is a derived property of the complete immutable program and is prepared once with its other runtime state. It is not a new source mode and does not alter Validated IR. A program is eligible only when its output can be reproduced by the portable DOM runtime: it has no browser workflow and no JavaScript field value source, including inside nested collections. An ineligible call fails with `E_SNAPSHOT_UNSUPPORTED`; the runtime never skips those operations or returns output from a semantically different projection.

Eligible programs retain ordinary selector, value-source, built-in and declared transform, external-transform, required/default, recovery, warning, partial-result, and cancellation behavior. Go's existing `Program.ExtractHTML` remains the narrower HTTP-mode entry point. Snapshot execution does not enable JavaScript and does not weaken browser security or URL-policy requirements for normal acquisition.

## Strict Go result decoding

`Result.Decode(destination)` converts the completed `Result.Value` into a fresh Go struct or string-keyed map and assigns the destination only after the entire conversion succeeds. The method does not start extraction, mutate `Result`, or hide `Warnings` and `Partial`; callers retain and inspect those fields independently. A partial result may be decoded, but a recovered null still fails when its destination is non-nullable.

Field matching is exact. An exported struct field uses the non-empty name in its `json` tag, or its exact Go field name when the tag has no name. Unexported fields and fields tagged `json:"-"` are ignored. Embedded fields are not flattened implicitly. Nested structs, pointers, maps, slices, and fixed arrays are decoded recursively. No custom JSON unmarshaler is invoked.

Missing and null values are accepted only by pointer, map, slice, or interface destinations. A missing non-nullable struct field and null for a non-nullable destination are errors. A map destination preserves the distinction between an absent key and a present key with a nil value; a nullable struct field is nil for both missing and explicit null.

Signed and unsigned integer sources are converted directly through exact integer arithmetic. Sign changes, overflow, fractional `json.Number` values, and float-to-integer conversions are rejected; integer values are never rounded through `float64`. Float destinations accept finite floating sources, and `float32` rejects overflow or precision loss.

Unknown source fields are errors for struct destinations. Conversely, every exported, non-skipped, non-nullable destination field must exist in the source. A destination must be a non-nil pointer to a struct or string-keyed map; invalid destinations return an error rather than panicking.

## Intentional language differences

- Go uses `context.Context`, `(value, diagnostics, error)` compilation results, `error`, `net/http`, and `time.Duration`. TypeScript uses promise rejection for operational compilation failures, optional `AbortSignal`, readonly data, platform `fetch`, `URL`, and millisecond numbers.
- Go file and `fs.FS` functions belong to the primary package because filesystem interfaces are standard Go library conventions. TypeScript file functions belong only to the Node.js entry point.
- Go keeps Validated IR opaque and exposes JSON plus metadata so internal compiler representations do not become public. TypeScript exposes the published readonly IR declaration because it is already the package's cross-language wire contract.
- Go browser methods accept context as the first argument. TypeScript carries abort signals in operation options.
- Go external transforms are named functions over `any` with runtime validation. TypeScript narrows the proposal to `JsonValue` at the type boundary and still performs runtime validation.
- Go exposes `Result.Decode` because its extraction value is dynamically shaped; TypeScript callers apply their own static schema or type-narrowing tools to `ExtractionResult.value`.
- Go returns independent built-in catalog values and uses typed scalar
  constructors. TypeScript returns a recursively frozen catalog, uses native
  scalar values, and provides `float(value)` when an integral number must retain
  float syntax. Both writers validate calls against the selected catalog and
  emit the same KDL Source.

These are surface differences only. They do not permit different accepted KDL, diagnostics, IR, capability derivation, extraction values, warnings, timeout behavior, or security defaults.

## Compatibility process

Public API changes must update this document, the relevant authoring or core
declaration snapshot, both independent consumer checks, `CHANGELOG.md`, and
`docs/migrating-to-v1.md`. Compatible additions and breaking changes follow
Semantic Versioning independently for the Go modules and npm packages.
