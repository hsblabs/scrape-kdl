# Cross-language conformance

`manifest.json` is the source of truth for release-blocking and provisional conformance cases.
It registers every file below `fixtures/`, the implementations and jobs that own each case, focused suites, expected artifacts, and the currently empty approved-divergence registry. `fixtureInventories` registers shared non-case corpora, such as parser, fuzz, and HTML-compatibility inputs consumed directly by both implementations, without pretending a mixed corpus is one executable extractor case. Inventory files ending in `.json` are syntax-checked as JSON; byte-oriented source artifacts retain their checked-in contents unchanged.

The dated schemas are:

- `manifest.schema.json` for the fixture inventory;
- `result.schema.json` for language-neutral implementation results.

## Running suites

Run the pull-request Go suite and complete TypeScript compiler suite, then compare their shared observations:

```bash
make conformance
```

Emit the Go result directly:

```bash
go run ./cmd/conformance-runner --suite pr
go run ./cmd/conformance-runner --suite invalid --output invalid-go.json
go run ./cmd/conformance-runner --suite release --job browser-e2e --list
```

Emit the complete TypeScript compiler result:

```bash
npm run conformance:typescript
```

Both runners write only the JSON result to standard output.
Errors and actionable guidance use standard error.
Exit status `0` means every observed difference is absent or explicitly approved, `1` means execution or conformance failure, and `2` means invalid runner usage or suite selection.

The Go runner executes compiler, diagnostic, canonical IR, and offline runtime stages.
Browser adapter jobs use the same manifest selection but their owning harness must emit the `browser` observation; the core runner intentionally does not impersonate an adapter.

## Normalization

- Manifest and result paths use repository-relative forward-slash form on Linux and macOS.
- KDL and HTML source bytes are parsed exactly as checked in. Line endings are not rewritten before parsing or SHA-256 calculation.
- JSON observations compare after the dated canonical JSON normalization: object names in Unicode code-point order, arrays in source order, finite binary64 numbers only, and negative zero normalized to zero.
- Diagnostics compare code, severity, message, span, path, and array order exactly.
- Span offsets count UTF-8 bytes. Lines and columns are one-based and the end position is exclusive.

Expected JSON files may use any insignificant whitespace, but their values must canonicalize identically.
A final newline in a JSON file is therefore formatting, while a newline inside a JSON string remains data.

## Suites and release status

Every case declares one or more focused suites.
`pr` is the normal pull-request selection and `release` contains every `release-blocking` case across all declared jobs.
Other suites select language, invalid, IR, runtime, HTML, browser, or complete TypeScript compiler work without changing case semantics.

`release-blocking` means an unexplained failure blocks the release candidate.
`provisional` is reserved for examples or observations that are still informative but not yet a compatibility gate.
Moving a case between these states is a reviewed contract change; a runner never downgrades it automatically.

## Divergences

The default is exact agreement and `approvedDivergences` is empty.
An approval record must identify the case, observation, implementations, explicit contract exclusion, rationale, and owner.
Portable-profile language, IR, diagnostic, static DOM, or extraction differences are release blockers and cannot be approved as divergences.
The registry exists only for an observation that is demonstrably outside the portable contract.

Adding a divergence merely records an already approved exclusion; it is not the approval mechanism itself.
The pull request must link the owner decision and explain why no portable behavior is affected.

## Inventory invariants

CI fails when:

- a file under `fixtures/` is absent from all manifest cases and fixture inventories;
- a registered or expected artifact is missing;
- a suite, case, execution, or expected diagnostic key is invalid;
- a result violates its dated schema;
- an implementation differs from its expected artifact;
- Go and TypeScript shared observations differ without a matching approved-divergence record.

The manifest tests also ensure that every release-blocking case belongs to the complete `release` suite and that focused suite selection remains stable.
