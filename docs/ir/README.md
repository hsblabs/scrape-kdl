# Validated IR v0.1

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

## Representations

- `schema.json`: canonical JSON representation
- `typescript/index.ts`: TypeScript type declarations
- `go/ir/types.go`: Go type declarations
- `example.ir.json`: example validated extractor

## Stability

`irVersion` versions the serialized IR independently from `languageVersion`.

Within IR 0.1:

- enum strings and JSON property names are stable;
- new optional properties MAY be added;
- required property removal or semantic change requires a new IR version.

## Design decisions

- Types are structural nodes, not encoded strings such as `string?`.
- Source spans are attached to executable and user-authored symbols.
- Resolved transform calls retain input/output types for codegen and inspection.
- Imported transforms use stable fully-qualified symbol IDs.
- Output member order is retained.
- Field `successfulType` is separated from `effectiveType`; optional missing may lift the effective type to nullable.
- DOM value-source `rawType` describes a successful read. Selector/attribute absence is represented as missing control flow, not as nullable transform input.
