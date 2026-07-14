# Roadmap to v1.0.0

Status: Approved on 2026-07-14.

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
- CLI commands, exit status classes, standard streams, and machine-readable output;
- supported Linux and macOS distributions.

Type generation, a language server, an inspector UI, and a browser authoring extension are not v1 release requirements.
The stable core contract is the browser-library-neutral adapter interface.
The v1 distribution MUST also include at least one official adapter for each language so the browser contract can be tested and used without a downstream adapter project.
The planned adapters are go-rod for Go and Playwright for TypeScript, but the TypeScript implementation choice remains open until the browser milestone begins.

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

Implementations MUST treat these dates as exact identifiers rather than assuming that an earlier date is compatible with a later date.
Each implementation MUST publish the set of language and IR versions it accepts.
An implementation MUST reject a malformed document version or an unknown or malformed language or IR version before network or browser activity.
The migration to `2026-07-15` replaces the untagged working-draft identifiers `version=1` and `0.1`; they do not remain supported identifiers unless a later compatibility decision explicitly restores them.
Editorial corrections do not create a new dated version, but a semantic change requires a new date and compatibility notes.

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

The completion gate includes:

- `-h` and `--help` on the root command and every subcommand;
- useful concise help for missing arguments and complete help when requested;
- primary results on standard output and diagnostics, warnings, and progress on standard error;
- stable JSON output that contains no human-oriented messages;
- `-` where a command accepts a streamable input or output;
- non-interactive behavior that never waits for a prompt in CI;
- documented exit status classes for usage, validation, compilation, execution, and cancellation failures;
- secret input through protected files or standard input rather than required command-line values;
- context cancellation and predictable `SIGINT` and `SIGTERM` behavior;
- a deprecation and removal decision for direct `--header` and `--cookie` values;
- install and smoke tests for released binaries.

New CLI dependencies are optional.
The public behavior matters more than the argument-parsing implementation.
Between `v0.5.0` and `v0.9.0`, a contract candidate may change only with tests, compatibility notes, and a migration path.
The contract freezes at `v0.9.0` and receives Semantic Versioning guarantees at `v1.0.0`.

## Release milestones

### v0.1.0: public preview

- adopt dated document, language, and IR identifiers;
- establish a Go-backed `examples/` harness with provisional goldens and an extension point for TypeScript;
- validate the Go HTML parser replacement against the initial checked-in HTML compatibility manifest;
- publish the repository and the first core and go-rod releases;
- verify installation through the Go module proxy and release archives.

### v0.2.0: specification freeze

- publish the `2026-07-15` language, built-in, selector, diagnostic, and IR contracts;
- establish the minimal TypeScript package scaffold and runtime policy needed for an independent implementation slice;
- pass that TypeScript parser and compiler slice over representative valid and invalid fixtures before freezing the contracts;
- close every accepted specification blocker for the initial contract;
- define canonical IR comparison and the cross-language conformance manifest;
- require a new dated language or IR version for later semantic changes.

After this milestone, the `2026-07-15` contracts receive errata and clarifications that do not alter observable behavior.
An ambiguity discovered by later parity work requires a new dated version when resolving it changes accepted programs, diagnostics, IR, or execution semantics.

### v0.3.0: TypeScript compiler foundation

- expand the pre-freeze TypeScript scaffold into the publishable package and finalize its supported-runtime policy;
- expand the pre-freeze TypeScript slice into the complete KDL subset parser and source loader;
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
- remove or explicitly retain every pre-v0.5 deprecated behavior.

### v0.6.0 through v0.8.0: browser parity and field validation

- implement the TypeScript browser adapter contract and trusted JavaScript opt-in;
- ship and test at least one official TypeScript browser adapter;
- complete browser workflow, lease, timeout, cancellation, and result-normalization parity;
- expand examples across HTTP and browser modes;
- complete HTML differential testing and the scheduled upstream conformance suite;
- establish performance, resource-limit, fuzzing, and security regression gates;
- validate Go and TypeScript use from independent example applications.
- publish the draft Go, Node.js, operating-system, architecture, browser, and adapter support matrix used by release-candidate CI.

### v0.9.0: release candidate

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

## Tracking issues

### v0.1.0

- [#4 Adopt dated document, language, and IR compatibility identifiers](https://github.com/hsblabs/scrape-kdl/issues/4)
- [#5 Establish the executable examples acceptance harness](https://github.com/hsblabs/scrape-kdl/issues/5)
- [#6 Replace the Go HTTP parser with WHATWG tree construction](https://github.com/hsblabs/scrape-kdl/issues/6)
- [#7 Publish the public preview and first core and rod releases](https://github.com/hsblabs/scrape-kdl/issues/7)

### v0.2.0

- [#8 Freeze the 2026-07-15 language contract](https://github.com/hsblabs/scrape-kdl/issues/8)
- [#9 Freeze the 2026-07-15 Validated IR and canonical JSON contract](https://github.com/hsblabs/scrape-kdl/issues/9)
- [#10 Define the cross-language conformance manifest and runners](https://github.com/hsblabs/scrape-kdl/issues/10)

### v0.3.0 and v0.4.0

- [#11 Publish the TypeScript library scaffold, public API, and CI](https://github.com/hsblabs/scrape-kdl/issues/11)
- [#12 Implement the TypeScript KDL parser, source loader, and imports](https://github.com/hsblabs/scrape-kdl/issues/12)
- [#13 Implement the TypeScript semantic compiler and diagnostic parity](https://github.com/hsblabs/scrape-kdl/issues/13)
- [#14 Implement the TypeScript HTTP runtime and extraction parity](https://github.com/hsblabs/scrape-kdl/issues/14)

### v0.5.0

- [#15 Complete the CLI human and automation contracts](https://github.com/hsblabs/scrape-kdl/issues/15)

### v0.6.0 through v1.0.0

- [#16 Implement the TypeScript browser runtime and official adapter](https://github.com/hsblabs/scrape-kdl/issues/16)
- [#17 Complete cross-runtime examples and hardening gates](https://github.com/hsblabs/scrape-kdl/issues/17)
- [#18 Freeze APIs, run the release candidate, and publish v1](https://github.com/hsblabs/scrape-kdl/issues/18)

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
