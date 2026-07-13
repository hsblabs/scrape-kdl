# AGENTS.md

Repository instructions for coding agents working on `github.com/hsblabs/scrape-kdl`.

## Project scope

`scrape-kdl` is the Go reference implementation of Scraping KDL v0.1. It parses KDL, performs semantic validation and type checking, lowers validated programs to a language-neutral IR, then executes extraction through HTTP or a live browser adapter.

The implementation order is:

```text
KDL source
  -> parser
  -> semantic validation and type checking
  -> Validated IR
  -> HTTP runtime or BrowserAdapter
  -> structured extraction result
```

Do not bypass an earlier stage by adding runtime-only interpretation of unresolved KDL nodes.

## Source of truth

Use these documents in descending priority:

1. `docs/spec/language-v0.1.md`
2. `docs/spec/builtins-v0.1.md`
3. `docs/spec/selectors-v0.1.md`
4. `docs/ir/schema.json`
5. Existing conformance fixtures and golden files
6. Implementation behavior

When implementation and specification disagree, do not silently preserve the implementation. Add or update a fixture, fix the implementation, and update the specification only when the language contract is intentionally changing.

## Repository layout

- `cmd/scrape-kdl/`: dependency-light CLI
- `internal/kdl/`: lexical and syntactic parsing
- `internal/compiler/`: imports, symbols, validation, type checking, and IR lowering
- `internal/typesys/`: language type model
- `internal/ir/`: Validated IR representation
- `internal/executor/`: HTTP and browser execution
- `internal/dom/`: internal HTML DOM and portable selector implementation
- `internal/source/`: source loading and decoding
- `fixtures/valid/`: valid language fixtures
- `fixtures/invalid/`: invalid fixtures with expected diagnostics
- `fixtures/expected-ir/`: golden Validated IR
- `fixtures/expected-output/`: golden extraction results
- `adapters/rod/`: separate go-rod module
- `docs/spec/`: normative language documentation

## Supported platforms

Linux and macOS are the only supported operating systems. Windows is explicitly out of scope.

Do not add Windows CI jobs, release targets, artifacts, compatibility shims, or Windows-specific support claims. Incidental Windows compilation is not a supported contract.

## Module boundaries

The root module is:

```text
github.com/hsblabs/scrape-kdl
```

The go-rod adapter is a separate module:

```text
github.com/hsblabs/scrape-kdl/adapters/rod
```

The root module must not import go-rod. Browser libraries belong in adapters. Keep `adapters/rod/go.mod` release-clean; local verification may apply a temporary `replace`, but committed release metadata must not depend on a local path.

## Architectural invariants

- Parse into syntax structures before semantic validation.
- Emit Validated IR only after imports, symbols, types, capabilities, and references are resolved.
- Keep diagnostics deterministic and stably ordered.
- Do not perform network or browser activity before validation succeeds.
- Keep HTTP and browser execution semantics aligned where the specification requires portability.
- Treat browser mode as a capability, not merely a fetch implementation.
- Keep `evaluate-js` as JavaScript executed by a browser adapter; do not translate it into Go.
- Require explicit JavaScript opt-in through `AllowJavaScript`.
- Preserve `URLPolicy` checks for initial HTTP targets, redirects, and browser navigation.
- Preserve extraction-wide browser leases so mutable pages are not shared concurrently.
- Do not weaken timeout, cancellation, response-size, charset, or session handling.

## Language changes

Any grammar or semantic change must include:

- normative specification changes;
- parser and semantic tests;
- at least one valid or invalid fixture;
- expected diagnostic or IR updates;
- compatibility notes when externally observable behavior changes.

Do not add aliases, coercions, fallback behavior, or permissive parsing merely to make one fixture pass. Prefer explicit syntax and stable diagnostics.

KDL support is the subset documented for Scraping KDL v0.1. A change should not claim full KDL 2 conformance unless backed by conformance fixtures.

## Diagnostics

Diagnostic codes are public compatibility surface.

- Reuse an existing code only for the same semantic condition.
- Add new codes to the normative diagnostics documentation.
- Keep source locations and ordering deterministic.
- Run `./scripts/check-diagnostics.sh` after changing diagnostics.
- Do not replace structured diagnostics with ad hoc error strings.

## Security requirements

KDL is executable configuration. Follow these rules:

- Assume `evaluate-js` is arbitrary trusted-spec code.
- Never enable JavaScript by default.
- Never bypass `URLPolicy` or redirect checks.
- Do not log cookies, authorization headers, session values, or extracted secrets.
- Bound HTTP response sizes and operations with context-aware timeouts.
- Validate JSON compatibility of JavaScript results.
- Preserve context cancellation through runtimes and adapters.
- Avoid new subprocess or filesystem capabilities in the language without an explicit security design.

Read `SECURITY.md` and `docs/security-model.md` before security-sensitive changes.

## Go implementation rules

- Target the minimum Go version declared in `go.mod` and `docs/compatibility.md`.
- Use standard Go formatting and idiomatic error wrapping.
- Prefer explicit types and small interfaces at subsystem boundaries.
- Avoid panics for user-controlled KDL, HTML, selectors, inputs, or JavaScript results.
- Keep public APIs minimal; prefer internal packages until an abstraction is stable.
- Add dependencies only when they materially improve correctness or maintainability.
- Do not add a dependency to the root module solely for an optional adapter.
- Keep generated or golden output deterministic.

## Tests and fixtures

Choose the narrowest relevant test first, then run the repository verification suite.

Common commands:

```bash
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go test -race ./...
GOTOOLCHAIN=local go vet ./...
make verify
```

For go-rod work:

```bash
make test-rod-contract
make test-rod
make test-rod-e2e
```

For release-affecting work:

```bash
make release-check
```

Update golden files only after reviewing the semantic difference. Never regenerate them solely to make tests green.

## Definition of done

A change is complete when:

- behavior matches the normative specification;
- focused tests cover success and failure paths;
- diagnostics and golden output are deterministic;
- `make verify` passes;
- adapter tests pass when adapter code changed;
- security and compatibility documents are updated when relevant;
- no temporary files, local `replace` directives, secrets, or platform claims remain.

## Working style

- Inspect nearby implementation, tests, and specification before editing.
- Prefer the smallest coherent patch.
- State assumptions when the specification is ambiguous.
- Do not redesign unrelated subsystems.
- Do not change public APIs, language syntax, diagnostic codes, module paths, release targets, or security defaults without documenting the compatibility impact.
- When blocked by unavailable network or browser dependencies, keep contract tests passing and report exactly what remains unverified.
