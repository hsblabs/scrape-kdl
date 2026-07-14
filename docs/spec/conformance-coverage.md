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

## TypeScript compiler parity

`packages/scrape-kdl` is the future `@hsblabs/scrape-kdl` package boundary and requires Node.js 26 or later.
The package contains the complete documented KDL subset parser, injectable import graph, semantic validator, type checker, capability and reference resolver, and dated IR lowerer. Shared parser and import cases require exact Go/TypeScript acceptance, diagnostics, UTF-8 byte spans, and ordering. The release-blocking TypeScript compiler suite runs every shared valid and invalid language fixture without network access, Go invocation, or subprocess execution.

For valid fixtures with golden IR, TypeScript output is compared canonically against the same frozen-schema artifact as Go. For invalid fixtures, code, severity, message, span, path, and ordering must match exactly. Compiler errors never return executable IR, and the cross-language check is part of the pull-request verification gate.
