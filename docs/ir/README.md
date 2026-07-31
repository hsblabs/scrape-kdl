# Validated IR 2026-07-15

The IR is the contract between parser/validator, interpreter, code generators, inspector, and runtime bindings.

## Boundary

The exported IR is a validated IR, not a lossless KDL AST.

Before IR emission:

- imports are resolved;
- aliases and transform references are resolved;
- built-in call arguments are validated;
- all intermediate types are computed;
- URL templates are tokenized;
- selector syntax/profile is validated;
- capabilities are derived and sorted;
- source spans are retained.

A lossless syntax tree may exist internally for formatting and editor support, but it is not this IR.
The public Authoring Document is also separate: it is a bounded semantic draft
that writes new KDL Source, which must pass through the compiler before this IR
can exist. See [`docs/authoring.md`](../authoring.md).

## Representations

- `2026-07-15/schema.json`: immutable publication layout for the initial schema
- `schema.json`: checked current-schema alias; CI requires byte-for-byte equality with the dated file
- `typescript/index.ts`: TypeScript type declarations
- `go/ir/types.go`: Go type declarations
- `example.ir.json`: example validated extractor

The dated schema's permanent publication identifier is `https://hsblabs.github.io/scrape-kdl/ir/2026-07-15/schema.json`. Publishing that static path remains part of the owner-gated release process; repository verification validates the exact artifact before publication.

## Stability

`irVersion` versions the serialized IR independently from `languageVersion`.

Within IR `2026-07-15`, property names, requiredness, union discriminators, enum values, number rules, and semantics are frozen. Unknown properties and unknown discriminators are rejected. Any observable field, schema, or semantic change requires a new approved `irVersion`; editorial corrections that do not change accepted documents or interpretation retain the current identifier. Implementations compare exact supported identifiers and never infer compatibility from date ordering.

## Canonical JSON

Canonical comparison:

- requires valid UTF-8 and rejects duplicate object member names or trailing data;
- emits no insignificant whitespace;
- emits the shortest JSON escape for quotation marks, reverse solidus, and control characters while leaving other Unicode scalar values unescaped;
- orders object member names by Unicode code point and preserves array order;
- preserves exact integers; finite decimal and exponent forms are emitted as an equivalent plain decimal with insignificant zeroes removed;
- rejects a decimal or exponent value outside finite IEEE-754 binary64 and normalizes every negative-zero spelling to `0`;
- preserves the distinction between an omitted optional property and a present property whose value is `null`.

Go's canonicalizer is exercised against every golden IR and the documented example; golden fixtures also round-trip through the typed Go compiler IR. Schema validation uses Draft 2020-12 with format assertions enabled. Declaration-shape tests compare required and optional JSON properties across the schema, internal Go IR, generated Go declarations, and TypeScript declarations.

## Design decisions

- Types are structural nodes, not encoded strings such as `string?`.
- Source spans are attached to executable and user-authored symbols.
- Resolved transform calls retain input/output types for codegen and inspection.
- Imported transforms use stable fully-qualified symbol IDs.
- Source files are ordered by path, include lowercase SHA-256 identities, and pair module names with document-version identifiers.
- Capabilities are the exact unique lexicographically sorted set derived from the executable IR.
- Output member order is retained.
- Field `successfulType` is separated from `effectiveType`; optional missing may lift the effective type to nullable.
- DOM value-source `rawType` describes a successful read. Selector/attribute absence is represented as missing control flow, not as nullable transform input.
- Millisecond durations are positive integers no greater than `9223372036854`, so every conforming value converts to Go `time.Duration` without overflow.
