# 2026-07-15 conformance coverage

`conformance-coverage.json` is the reviewable inventory for the normative language, built-in, selector, and diagnostic contracts.
Each entry covers every normative statement in its cited section and names either automated fixture or test evidence, or an explicit reason that the section is not executable.

`go test ./scripts -run TestConformanceCoverage` enforces that:

- every level-two and level-three section in every normative document has an inventory entry;
- every inventory source anchor still exists;
- every automated evidence path still exists;
- each entry has automated evidence or a non-automated rationale, but not both;
- identifiers, source references, and normative documents are unique and complete.

The inventory is a coverage map, not a waiver mechanism.
A failing fixture is an implementation defect or specification ambiguity.
An executable rule must not be changed to a rationale merely to make the coverage check pass.

## Audit conclusion

The 2026-07-15 audit found one editorial grammar-summary drift: extractor and module roots were still summarized with integer `version` values and omitted `language-version`.
The normative language document, fixtures, compiler, diagnostics, and IR already used real `YYYY-MM-DD` identifiers, so correcting the non-normative EBNF summary does not alter accepted programs or observable behavior.

No accepted language-contract blocker remains after that correction and the independent TypeScript fixture cross-check.
Future wording fixes may remain on `2026-07-15` only when accepted programs, diagnostics, IR, and execution behavior do not change.
Any change to those observable contracts requires an approved new language date and compatibility notes.

## TypeScript cross-check boundary

`packages/scrape-kdl` is the future `@hsblabs/scrape-kdl` package boundary and requires Node.js 26 or later.
At this milestone it is deliberately a bounded, independent parser/compiler slice: it parses and lowers the representative HTTP fixture, rejects the shared dated-version fixtures, emits the shared diagnostic codes and paths, hashes source bytes, and matches the Go golden IR after canonical JSON normalization without invoking a Go binary.

The slice does not claim complete language support.
Issues #11 through #13 expand this package to the full parser, source loader, import resolver, semantic checker, and IR compiler.
Unsupported syntax in the bounded slice is an implementation limit, not an alternate interpretation of the 2026-07-15 contract.
