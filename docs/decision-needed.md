# Decisions needed

## Secure CLI session input

### Decision

Choose how the CLI should accept cookie and sensitive header values without encouraging secrets in command-line arguments.

### Background

`scrape-kdl extract` currently accepts `--cookie name=value` and `--header 'Name: value'`. Command-line values can be retained in shell history and may be visible through process inspection. Cookies and authorization headers are explicitly treated as secrets by the repository security model, but replacing or restricting these flags would change the CLI contract.

### Options

1. Keep the current flags and document that they are intended only for non-sensitive values.
2. Add file-based session inputs, deprecate direct secret-bearing flags, and remove them only in a future compatibility window.
3. Add a structured session document read from a file or standard input and make it the only supported secret input path.

### Recommendation

Add file-based inputs first, deprecate direct cookie and sensitive-header values with a documented migration path, and retain the existing flags until the project's versioning policy permits removal.

### Compatibility and safety impact

Keeping the current flags preserves compatibility but continues to expose users to accidental secret disclosure. Removing them immediately is safer by default but breaks existing automation. A deprecation period improves safety without an abrupt compatibility break, but temporarily leaves two input paths to maintain and document.

### Related files

- `cmd/scrape-kdl/main.go`
- `README.md`
- `SECURITY.md`
- `docs/security-model.md`
- `docs/compatibility.md`

## Subcommand help exit status

### Decision

Decide whether `validate --help`, `compile --help`, and `extract --help` should change from exit status 2 to exit status 0.

### Background

The top-level `scrape-kdl --help` command exits successfully, while each subcommand currently routes an explicit help request through its invalid-arguments path and exits with status 2. Common CLI conventions treat an explicit `-h` or `--help` request as a successful operation. Changing the status would improve consistency but is observable to scripts.

### Options

1. Preserve exit status 2 for compatibility and document it.
2. Change explicit subcommand help to exit status 0 immediately while keeping malformed arguments at status 2.
3. Announce the new behavior and change it at the next documented compatibility boundary.

### Recommendation

Change explicit `-h` and `--help` requests to exit status 0 at the next compatibility boundary, with tests distinguishing help from malformed arguments.

### Compatibility and safety impact

Returning 0 matches established CLI behavior and prevents automation from treating a help request as a failure. Scripts that currently depend on status 2 would observe a change, although relying on failure for an explicit help request is unlikely. No extraction, network, browser, or security behavior is affected.

### Related files

- `cmd/scrape-kdl/main.go`
- `cmd/scrape-kdl/main_test.go`
- `README.md`
- `docs/compatibility.md`

## Go representation of browser JavaScript results

### Decision

Define which concrete Go representations `BrowserAdapter.Evaluate` may return for JSON-compatible arrays, objects, and numbers.

### Background

The language specification defines JavaScript results as logical JSON values, while the Go adapter documentation only says that `Evaluate` must return a JSON-compatible value. The runtime currently accepts scalar Go numeric types, `json.Number`, `[]string`, `[]any`, and `map[string]any`. It rejects other values that `encoding/json` can encode to the same logical JSON shape, such as `[]map[string]any` and `map[string]string`. The go-rod adapter already returns the accepted `float64`, `[]any`, and `map[string]any` forms, so this ambiguity primarily affects third-party adapter authors.

### Options

1. Document the currently accepted concrete representations as the complete adapter contract.
2. Canonicalize every result through `encoding/json` before declared-type validation, thereby accepting structs, typed slices and maps, and custom marshalers.
3. Expand acceptance only for reflect-based slices and string-keyed maps whose recursively contained values are accepted, without invoking custom JSON marshaling.

### Recommendation

Document a small canonical representation set and provide an adapter-facing normalization helper before considering broader automatic conversion. Keep the runtime's validation side-effect free and avoid implicitly invoking custom marshalers in the trusted-JavaScript boundary.

### Compatibility and safety impact

Documenting the current set may require third-party adapters to normalize typed containers but preserves deterministic runtime behavior. Broadening acceptance is mostly backward compatible, but JSON round-tripping can invoke user code through `json.Marshaler`, erase concrete numeric precision, reject cycles late, and allocate a second complete result tree. A reflection-based expansion avoids custom marshaler side effects but creates a larger public compatibility surface that must remain stable.

### Related files

- `internal/executor/browser.go`
- `internal/executor/executor.go`
- `docs/browser-runtime.md`
- `docs/spec/language-v0.1.md`
- `adapters/rod/adapter.go`

## External transform result-type diagnostics

### Decision

Choose when and under which diagnostic code the runtime should reject a host external transform result that does not match the transform's declared output type.

### Background

Validated IR records the input and output type of every external transform. The host callback currently returns `any`, and its value is not checked at the external-transform boundary. A mismatch is therefore reported later as `E_OUTPUT_TYPE` when it reaches a field unchanged, or as `E_TRANSFORM` when a downstream built-in cannot consume it. The diagnostics specification defines `E_EXTERNAL_TRANSFORM` for callbacks that return an error and `E_OUTPUT_TYPE` for a field output mismatch, but does not assign a code to a successful callback returning a value of the wrong declared type.

### Options

1. Preserve deferred validation at downstream built-ins and final fields.
2. Validate immediately and report the mismatch as `E_EXTERNAL_TRANSFORM` because the host callback violated its declared contract.
3. Validate immediately and report `E_OUTPUT_TYPE` or generic `E_TRANSFORM` at the field path.
4. Add a dedicated external-transform result-type diagnostic at a documented compatibility boundary.

### Recommendation

Validate immediately after every external transform and use a dedicated diagnostic code. This localizes host integration defects before downstream transforms run and avoids broadening the documented meaning of existing codes.

### Compatibility and safety impact

Immediate validation prevents mismatched host values from reaching downstream transforms and makes failures deterministic. Reusing an existing code changes its documented semantic scope, while adding a code expands the public diagnostics surface. Preserving current behavior avoids an immediate compatibility change but produces different diagnostics depending on pipeline shape and can obscure the responsible host callback.

### Related files

- `internal/executor/transforms.go`
- `internal/executor/executor.go`
- `internal/executor/browser.go`
- `internal/executor/types.go`
- `docs/spec/diagnostics.md`
- `docs/spec/language-v0.1.md`

## Ambient client state under `session policy="none"`

### Decision

Define whether `session policy="none"` ignores only the explicitly supplied runtime `Session`, or must also suppress ambient authentication state already held by an `http.Client` cookie jar or browser context.

### Background

The language specification says that `none` ignores runtime session input, but does not define whether host-owned client or browser state is part of that input. The HTTP runtime currently clears `Options.Session` while preserving the supplied `http.Client`; a non-nil client jar can therefore add cookies to requests and persist response `Set-Cookie` updates. Browser execution similarly passes a nil `Session`, but the adapter may use a browser context that already contains cookies or storage. Suppressing only the HTTP jar would make the two runtimes inconsistent, while clearing browser state cannot be expressed through the current `BrowserAdapter` contract.

### Options

1. Define runtime session input as the explicit `Session` value only, preserve ambient client and browser state, and document that hosts must supply isolated stateless clients or contexts when stronger separation is required.
2. Clone the HTTP client with `Jar=nil` for `none`, while leaving browser ambient state unchanged.
3. Expand the runtime and browser adapter contracts so `none` requires an isolated or cleared execution context across both modes.
4. Reject stateful HTTP clients or browser adapters for `none`, which would also require a reliable capability check and a documented diagnostic.

### Recommendation

For v0.1, define `none` as ignoring only the explicit `Session` and document ambient-state isolation as a host responsibility. Consider an explicit stateless execution capability at a future compatibility boundary if the language contract is intended to guarantee credential-free execution rather than only the absence of supplied session input.

### Compatibility and safety impact

Documenting the current behavior preserves the public Go and adapter APIs but means `none` is not by itself a guarantee that no ambient cookie, storage, or authentication state is used. Removing the HTTP jar would be safer for that runtime but create cross-runtime inconsistency and could break hosts that intentionally reuse authenticated clients. A cross-runtime stateless guarantee offers the clearest security contract but requires a breaking adapter/API design and lifecycle rules for browser contexts.

### Related files

- `docs/spec/language-v0.1.md`
- `docs/http-runtime.md`
- `docs/browser-runtime.md`
- `docs/security-model.md`
- `internal/executor/executor.go`
- `internal/executor/fetch.go`
- `internal/executor/browser.go`
- `internal/executor/types.go`

## Workflow timeout upper bound

### Decision

Define the maximum supported `timeout-ms` value and the behavior when a larger positive integer is compiled or loaded from Validated IR.

### Background

The language specification and IR schema require workflow timeouts to be positive integers but do not define an upper bound. The Go runtime converts milliseconds to `time.Duration`; sufficiently large positive `int` values overflow that nanosecond-based representation and can become zero or negative before reaching a browser adapter. Rejecting, clamping, or representing those values differently would each add an externally observable rule that is not currently normative.

### Options

1. Define a maximum of `math.MaxInt64 / time.Millisecond` milliseconds and reject larger values during compilation and IR validation.
2. Clamp larger values to the maximum representable `time.Duration`.
3. Change the adapter timeout representation to preserve a wider millisecond range.
4. Leave the specification unbounded and require each runtime to document its representable limit.

### Recommendation

Define the portable maximum explicitly and reject larger values with the existing type/range validation path before emitting Validated IR. Runtime preflight should defensively reject out-of-range hand-built IR using `E_IR_INVALID`.

### Compatibility and safety impact

An explicit maximum prevents overflow from turning a long timeout into an immediate or otherwise incorrect timeout and gives adapters a deterministic contract. Rejecting values that currently compile is a compatibility change, although those values cannot execute with their stated semantics in the Go reference runtime. Clamping avoids failure but silently changes requested behavior. A wider adapter representation would be a breaking public API change.

### Related files

- `docs/spec/language-v0.1.md`
- `docs/ir/schema.json`
- `internal/compiler/source.go`
- `internal/executor/browser.go`
- `internal/executor/browser_test.go`
- `internal/executor/types.go`

## Offline `ExecuteHTML` cancellation diagnostic

### Decision

Define whether and how `ExecuteHTML` must stop when its context is canceled before or during in-memory HTML parsing and output extraction, including the structured runtime diagnostic to return.

### Background

`ExecuteHTML` accepts a context and passes it to external transforms, but it does not currently check cancellation before parsing the supplied string or between output members. HTTP and browser operations map cancellation through operation-specific diagnostics such as `E_HTTP_FETCH` and `E_BROWSER_QUERY`; none of the existing runtime codes describes cancellation of offline parsing or extraction as a whole. Reusing an operation-specific code would broaden its normative meaning, while returning `context.Canceled` directly would break the structured-error convention.

### Options

1. Define `ExecuteHTML` cancellation only at external-transform boundaries and document that in-memory parsing and extraction are non-interruptible.
2. Add a general execution-canceled diagnostic and check the context before parsing and between fields and collection rows.
3. Reuse `E_HTML_PARSE` before or during parsing and `E_FIELD_EXECUTION` during output traversal.
4. Return the raw context error without an `ExecutionError` wrapper.

### Recommendation

Add a general execution-canceled diagnostic at a compatibility boundary, preserve `context.Canceled` as its cause, and check at deterministic coarse-grained boundaries. Avoid operation-specific codes whose current meanings do not cover offline cancellation.

### Compatibility and safety impact

Explicit cancellation prevents large offline documents or outputs from continuing after callers abandon the operation. A new diagnostic expands the public compatibility surface but keeps failure classification stable. Reusing current codes avoids a registry change but makes their meanings mode-dependent. Fine-grained parser interruption would add overhead and requires a separate implementation design; coarse checks cannot interrupt one long tokenizer call.

### Related files

- `internal/executor/executor.go`
- `internal/executor/types.go`
- `internal/dom/parser.go`
- `docs/spec/diagnostics.md`
- `docs/http-runtime.md`
