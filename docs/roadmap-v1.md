---
status: approved
approved: 2026-07-14
implementation-decisions-updated: 2026-07-15
---

# Roadmap to v1.0.0

This roadmap defines the compatibility contracts and release gates required before `scrape-kdl` reaches `v1.0.0`.
The primary v1 products are the Go library and the TypeScript library.
The CLI is the shared command-line interface for validation, compilation, extraction, and automation.

## v1 product boundary

The Go and TypeScript libraries MUST each support the complete language pipeline:

```text
KDL source
  -> parser
  -> semantic validation and type checking
  -> Validated IR
  -> HTTP runtime or browser adapter
  -> structured extraction result
```

The TypeScript library is not limited to consuming IR produced by the Go compiler.
It MUST parse `.kdl` files, resolve imports, produce deterministic diagnostics, lower valid programs to the same IR, and execute the resulting program without requiring a Go binary.

The following surfaces are stable contracts at v1:

- the dated Scraping KDL language snapshot;
- the dated Validated IR schema;
- diagnostic codes, severity, paths, spans, and deterministic ordering;
- the Go and TypeScript public APIs;
- HTTP and browser execution semantics;
- the portable selector profile and built-in transform registry;
- the bounded semantic authoring model, exact-version built-in catalog, and
  deterministic KDL writer;
- CLI commands, exit status classes, standard streams, and machine-readable output;
- supported Linux and macOS distributions.

Type generation, a language server, an inspector UI, and a complete browser
authoring extension are not v1 release requirements. The bounded authoring API
does not expose a syntax tree, lossless formatter, or mutable Validated IR.
The stable core contract is the browser-library-neutral adapter interface.
The v1 distribution MUST also include at least one official adapter for each language so the browser contract can be tested and used without a downstream adapter project.
The official adapters are go-rod for Go and Playwright for TypeScript.
Chromium is the supported browser target at v1; Playwright Firefox and WebKit coverage is best effort.

## Distribution and public API policy

The TypeScript implementation is distributed as ESM-only npm packages in an npm-workspaces monorepo:

- `@hsblabs/scrape-kdl` contains the compiler, diagnostics, IR, HTTP runtime, and browser-library-neutral interfaces;
- `@hsblabs/scrape-kdl/authoring` contains the bounded authoring model, versioned
  built-in catalog, and deterministic KDL writer;
- `@hsblabs/scrape-kdl-playwright` contains the official Playwright adapter.

The complete TypeScript pipeline targets Node.js 26 or later.
A compiler or runtime bundled for direct execution in a web browser is outside the v1 scope.

Go and TypeScript MUST expose the same language semantics, Validated IR, diagnostics, extraction behavior, and extension capabilities.
Their public APIs SHOULD remain idiomatic to each language instead of being mechanically identical.
TypeScript loading, compilation, and execution APIs are asynchronous by default, source loading is injectable, and Node.js filesystem conveniences are exposed through a Node.js entry point.
The Go API remains context-first and MUST NOT expose internal package types.

## Dated compatibility versions

Go modules and npm packages continue to use Semantic Versioning.
Language artifacts and serialized contracts use ISO 8601 calendar dates as opaque compatibility identifiers.

The initial identifiers are:

```kdl
extractor "race-detail" version="2026-07-15" language-version="2026-07-15" {
}

module "common-transforms" version="2026-07-15" language-version="2026-07-15" {
}
```

```json
{
  "version": "2026-07-15",
  "languageVersion": "2026-07-15",
  "irVersion": "2026-07-15"
}
```

Every compatibility date MUST be a real calendar date in the exact `YYYY-MM-DD` form.
The root `version` property identifies the revision of an extractor or transform module.
The root `language-version` property selects the language contract used to interpret that document.
The compiler copies both values into Validated IR.

The identifiers have these meanings:

- `version` identifies the revision of an extractor or transform module;
- `languageVersion` records the language contract selected by the source `language-version` property;
- `irVersion` identifies the serialized IR schema and semantics.

The initial values are equal because the first documents, language contract, and IR contract are published together.
Later document, language, and IR revisions may advance independently.

`2026-07-15` is the planned first v1 language contract, not a temporary identifier that must be replaced or re-frozen at a later milestone.
It remains the current language version unless an approved semantic change alters accepted programs, diagnostics, IR lowering, or execution semantics.
Completing the contract documentation at `v0.2.0` does not by itself create a new date.

Implementations MUST treat these dates as exact identifiers rather than assuming that an earlier date is compatible with a later date.
Each implementation MUST publish the set of language and IR versions it accepts.
An implementation MUST reject a malformed document version or an unknown or malformed language or IR version before network or browser activity.
The migration to `2026-07-15` replaces the untagged working-draft identifiers `version=1` and `0.1`; they do not remain supported identifiers unless a later compatibility decision explicitly restores them.
Editorial corrections do not create a new dated version, but a semantic change requires a new date and compatibility notes.

The initial Validated IR schema uses this permanent project-owned identifier:

```text
https://hsblabs.github.io/scrape-kdl/ir/2026-07-15/schema.json
```

The schema is published at that project-owned static path.
Canonical JSON comparison uses UTF-8, emits object member names in lexicographic Unicode code-point order, preserves array order, rejects non-finite numbers, normalizes negative zero to zero, and distinguishes an omitted optional property from a property whose value is `null`.

## Cross-language conformance

The Go implementation remains the reference implementation, but reference status does not permit unexplained TypeScript differences.
Both implementations MUST pass the same language fixtures, IR golden files, diagnostic expectations, and runtime examples.

Conformance requires:

- acceptance and rejection of the same KDL documents;
- the same import, symbol, type, capability, and reference resolution;
- the same diagnostic codes, severity, paths, source spans, and ordering;
- structurally identical Validated IR after canonical JSON normalization;
- the same built-in and declared transform behavior;
- the same missing-value, recovery, warning, partial-result, timeout, and cancellation behavior;
- the same observable HTTP and browser extraction results where the specification requires portability.

A mismatch is either an implementation defect or a specification ambiguity.
The mismatch MUST be reduced to a fixture before either implementation changes behavior.

## HTML compatibility

The HTTP runtimes should delegate WHATWG tree construction to maintained, conforming parsers instead of extending an XML-based approximation.
The planned defaults are [`golang.org/x/net/html`](https://pkg.go.dev/golang.org/x/net/html) for Go and [`parse5`](https://parse5.js.org/) for TypeScript.

The public guarantee covers observable static extraction after response bytes have been decoded to UTF-8.
It includes element order, selector matches, text, inner HTML, attributes, missing values, and the resulting extraction value.
It does not claim that an HTTP parse includes JavaScript mutations or other live-browser state.

HTML compatibility is verified in three layers:

1. A small pinned parsing corpus runs in every pull request.
2. A broader pinned subset of the [Web Platform Tests HTML parsing suite](https://github.com/web-platform-tests/wpt/tree/master/html/syntax/parsing) runs on a schedule and before a release candidate.
3. Differential examples compare Go, TypeScript, and Chromium observations for the portable selector profile.

The repository maintains a versioned HTML compatibility manifest.
Each manifest entry identifies the input bytes or fixture, decoded encoding, parser mode, selectors, expected observations, normalization rules, upstream test identifier, and any approved divergence with a rationale.
Pull requests MUST produce zero unapproved differences against the manifest.
An observable difference inside the portable selector and extraction contract is a release blocker and cannot be approved as a divergence.
An approved divergence MUST identify the explicit contract exclusion that places the case outside the portable guarantee.
The pinned upstream revision, selected test identifiers, exclusions, and Chromium version are reviewable repository files.

Parser versions and the upstream test revision MUST be pinned.
Upgrades MUST include a reviewed compatibility diff rather than silently changing extraction output.

## Executable examples

The `examples/` tree is both user documentation and a cross-language acceptance suite.
Each example MUST be self-contained and MUST declare the language version, IR version, inputs, expected artifacts, execution modes, and required adapters in `example.json`.

A complete example contains:

```text
examples/<name>/
  example.json
  extractor.kdl
  inputs.json
  expected-ir.json
  expected-output.json
  README.md
```

HTTP examples add a saved HTML fixture or local test-server definition.
Browser examples add a deterministic local page and the adapter requirements needed by the browser E2E job.

The initial suite covers:

- basic static HTTP extraction;
- malformed and truncated HTML;
- charset decoding;
- sessions, cookies, redirects, and URL policy;
- declared, built-in, match, and external transforms;
- missing values, row recovery, warnings, and partial results;
- browser workflow actions;
- trusted JavaScript evaluation;
- imports shared by more than one extractor.

Pull requests use saved HTML and local HTTP servers so the acceptance suite remains deterministic.
Every example MUST compile and run with Go in pull-request CI once it is added.
Starting at `v0.4.0`, every example marked for TypeScript MUST also compile and run with TypeScript, and both implementations MUST match its canonical IR and expected output.
Browser examples MUST run in the Browser E2E workflow with the official adapter for each implemented language.
Live-site smoke tests are opt-in or scheduled because remote pages can change without a repository change.

## CLI completion at v0.5.0

The `scrape-kdl` CLI is feature complete at `v0.5.0` when `validate`, `compile`, `extract`, and `version` have documented human and automation contract candidates.
The Go CLI is the only v1 command-line distribution; a TypeScript CLI is not required.

The completion gate includes:

- `-h` and `--help` on the root command and every subcommand;
- useful concise help for missing arguments and complete help when requested;
- primary results on standard output and diagnostics, warnings, and progress on standard error;
- `--json` output as exactly one JSON document on standard output with no human-oriented messages;
- `-` where a command accepts a streamable input or output;
- non-interactive behavior that never waits for a prompt in CI;
- exit status `0` for success, `1` for processing failure, `2` for usage error, `130` for `SIGINT`, and `143` for `SIGTERM`;
- secret input through `--session-file` or standard input rather than command-line values;
- context cancellation and predictable `SIGINT` and `SIGTERM` behavior;
- removal of direct `--header` and `--cookie` values;
- install and smoke tests for released binaries.

When output is not attached to a TTY, the CLI disables color, progress rendering, and prompts.

New CLI dependencies are optional.
The public behavior matters more than the argument-parsing implementation.
Between `v0.5.0` and the first public `v1.0.0-rc.N`, a contract candidate may
change only with tests, compatibility notes, and a migration path.
The contract freezes when `v1.0.0-rc.1` is published and receives Semantic
Versioning guarantees at `v1.0.0`.

## Release milestones

### v0.1.0: contract foundation

- adopt dated document, language, and IR identifiers;
- establish a Go-backed `examples/` harness with provisional goldens and an extension point for TypeScript;
- validate the Go HTML parser replacement against the initial checked-in HTML compatibility manifest;
- establish repeatable verification for the core module and go-rod adapter.

Repository publicization and every public pre-v1 release are intentionally outside this roadmap until the project owner gives a separate instruction.
Operational `-private.N` versions may be used by authorized users under
`docs/releasing.md`. They do not start the public `v1.0.0-rc.N` candidate
period, freeze the API, or satisfy a public release milestone.

### v0.2.0: contract completion

- complete the `2026-07-15` language, built-in, selector, diagnostic, and IR contracts;
- establish the minimal TypeScript package scaffold and runtime policy needed for an independent implementation slice;
- pass that TypeScript parser and compiler slice over representative valid and invalid fixtures before declaring the contracts complete;
- close every accepted specification blocker for the initial contract;
- define canonical IR comparison and the cross-language conformance manifest;
- define the approved Go and TypeScript public API principles;
- require a new dated language or IR version for later semantic changes.

Issue #8 owns the bounded TypeScript package and parser/compiler slice used to cross-check the contract at this milestone.
Issues #11 through #13 expand that slice into the complete TypeScript compiler implementation after contract completion; issues #14 and #16 add the HTTP and browser runtimes.

After this milestone, the `2026-07-15` contracts receive errata and clarifications that do not alter observable behavior.
An ambiguity discovered by later parity work requires a new dated version when resolving it changes accepted programs, diagnostics, IR, or execution semantics.

### v0.3.0: TypeScript compiler foundation

- expand the contract-completion TypeScript scaffold into the publishable packages;
- expand the initial TypeScript slice into the complete KDL subset parser and source loader;
- resolve relative imports and document kinds;
- compile the first shared fixtures to canonical IR without invoking Go.

### v0.4.0: TypeScript semantic and runtime parity

- implement symbols, type checking, capabilities, references, and IR lowering;
- match Go diagnostics for the shared invalid fixture suite;
- implement the HTTP runtime, portable selectors, transforms, warnings, and partial results;
- run the same HTTP examples and every non-browser example marked for TypeScript through both implementations in CI.

### v0.5.0: CLI feature completion

- satisfy the CLI feature-completion gate defined above;
- make compilation and extraction suitable for both human use and build automation;
- document all stable commands with copyable examples;
- remove direct `--header` and `--cookie` input and every other behavior scheduled for removal by v0.5.

### v0.6.0 through v0.8.0: browser parity and field validation

- implement the TypeScript browser adapter contract and trusted JavaScript opt-in;
- ship and test `@hsblabs/scrape-kdl-playwright` as the official TypeScript adapter;
- complete browser workflow, lease, timeout, cancellation, and result-normalization parity;
- expand examples across HTTP and browser modes;
- complete HTML differential testing and the scheduled upstream conformance suite;
- establish performance, resource-limit, fuzzing, and security regression gates;
- validate Go and TypeScript use from independent example applications;
- publish the draft Go, Node.js 26+, operating-system, architecture, browser, and adapter support matrix used by release-candidate CI;
- establish evidence-based performance regression gates against measured baselines without introducing an absolute speed SLA.

### v1.0.0-rc.N: release candidate

- freeze the Go and TypeScript public APIs;
- freeze CLI output and exit status contracts;
- freeze the supported-version matrix before publishing release candidates;
- publish release candidates for Go, npm, the CLI, and official adapters;
- publish migration notes from every pre-v1 compatibility break;
- run the complete conformance, security, race, browser, packaging, and installation gates;
- allow only release-blocking fixes during the candidate period.

The candidate period lasts for at least 14 consecutive days with no unresolved release blocker.

### v1.0.0: stable release

- close every release-blocking discrepancy found during the candidate period;
- publish the Go and npm packages, CLI archives, and official adapter releases;
- begin the documented Semantic Versioning compatibility guarantees;
- publish the supported-version matrix and maintenance policy.

Publishing stable v1 artifacts remains an explicit project-owner approval gate.
The sequence is: complete issue #18's pre-publication checklist, obtain explicit project-owner approval, publish the stable artifacts, and then close issue #18.

## Dependency and support policy

Implementations prefer standard-library or platform APIs and small explicit abstractions.
Go may use `golang.org/x/*` packages as quasi-standard dependencies where they materially improve correctness.
The approved v1 runtime dependencies include `golang.org/x/net/html`, `parse5`, `re2js` for linear-time RE2-compatible TypeScript regular expressions, and Playwright in the separate TypeScript adapter package.
A new third-party runtime dependency outside these approved choices requires project-owner review; minimal build and test dependencies may be selected without review when they follow established ecosystem practice.

The support matrix is:

- Linux and macOS are supported; Windows remains out of scope;
- Node.js 26 is the minimum supported Node.js version;
- Chromium is supported for browser execution;
- Firefox and WebKit through Playwright are best effort;
- v1 has no absolute performance SLA, but measured regressions against checked-in or recorded baselines may block a release;
- resource limits, cancellation, URL policy, response-size bounds, and security defaults remain hard contracts.

## Tracking issues

### v0.1.0

- [#4 Adopt dated document, language, and IR compatibility identifiers](https://github.com/hsblabs/scrape-kdl/issues/4)
- [#5 Establish the executable examples acceptance harness](https://github.com/hsblabs/scrape-kdl/issues/5)
- [#6 Replace the Go HTTP parser with WHATWG tree construction](https://github.com/hsblabs/scrape-kdl/issues/6)

### v0.2.0

- [#8 Complete the 2026-07-15 language contract](https://github.com/hsblabs/scrape-kdl/issues/8)
- [#9 Complete the 2026-07-15 Validated IR and canonical JSON contract](https://github.com/hsblabs/scrape-kdl/issues/9)
- [#10 Define the cross-language conformance manifest and runners](https://github.com/hsblabs/scrape-kdl/issues/10)
- [#19 Define Go and TypeScript public API principles](https://github.com/hsblabs/scrape-kdl/issues/19)

### v0.3.0 and v0.4.0

- [#11 Publish the TypeScript package scaffold and CI](https://github.com/hsblabs/scrape-kdl/issues/11)
- [#12 Implement the TypeScript KDL parser, source loader, and imports](https://github.com/hsblabs/scrape-kdl/issues/12)
- [#13 Implement the TypeScript semantic compiler and diagnostic parity](https://github.com/hsblabs/scrape-kdl/issues/13)
- [#14 Implement the TypeScript HTTP runtime and extraction parity](https://github.com/hsblabs/scrape-kdl/issues/14)

### v0.5.0

- [#15 Complete the CLI human and automation contracts](https://github.com/hsblabs/scrape-kdl/issues/15)

### v0.6.0 through v1.0.0

- [#16 Implement the TypeScript browser runtime and official adapter](https://github.com/hsblabs/scrape-kdl/issues/16)
- [#17 Complete cross-runtime examples and hardening gates](https://github.com/hsblabs/scrape-kdl/issues/17)
- [#18 Freeze APIs, run the release candidate, and publish v1](https://github.com/hsblabs/scrape-kdl/issues/18)

## Agent execution policy

An implementation agent proceeds without further project-owner approval when a task stays inside this roadmap, the normative specifications, the IR schema, and the approved API, dependency, CLI, and support policies.
The normative specification remains the source of truth.
A Go and TypeScript mismatch MUST first be reduced to a fixture and then corrected as an implementation defect unless the fixture exposes a genuine specification ambiguity.
An observable difference inside the portable profile is a defect, not an implementation choice.

Project-owner approval is required only for:

- a semantic change that requires a new `language-version`;
- an IR semantic or schema change that requires a new `irVersion`;
- a new public contract outside the approved Go and TypeScript API principles;
- a new unapproved third-party runtime dependency;
- promotion of a best-effort target to supported status;
- creating a private release tag, GitHub Release, or restricted npm version;
- making the repository public;
- publishing any public pre-v1 release or release artifact;
- publishing stable v1 artifacts after issue #18's pre-publication checklist passes.

While one of these decisions is pending, agents continue independent roadmap work that does not depend on it.

## v1 release gate

The project is ready for `v1.0.0` when all of the following are true:

- Go and TypeScript compile the complete shared valid and invalid fixture suites;
- cross-language diagnostics and canonical IR match;
- HTTP and browser examples produce the expected outputs in both runtimes;
- the pinned HTML conformance and Chromium differential suites pass;
- the CLI completion gate passes on Linux and macOS;
- release-candidate packaging and installation pass for every target in the frozen support matrix;
- Go and npm consumers can install and run independent example applications;
- security defaults, cancellation, resource bounds, and URL policy remain enforced;
- the release candidate completes its compatibility period without an unresolved release blocker.
