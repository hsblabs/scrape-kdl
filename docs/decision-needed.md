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
