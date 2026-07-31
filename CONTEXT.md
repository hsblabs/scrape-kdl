# Scraping KDL

Scraping KDL defines portable extraction documents and the boundaries through
which applications author, validate, inspect, and execute them.

## Language

**Authoring Document**:
An application-owned semantic draft used to construct one extractor before
compiler validation.
_Avoid_: Draft IR, mutable IR, syntax tree

**KDL Source**:
The textual Scraping KDL document accepted by the parser and compiler.
_Avoid_: IR, compiled spec

**KDL Writing**:
Deterministic serialization of an Authoring Document into new KDL Source.
_Avoid_: Formatting, pretty-printing

**Formatting**:
Rewriting existing KDL Source while retaining syntax-level material such as
comments and unsupported-but-parseable lexical choices.
_Avoid_: KDL Writing, compilation

**Validated IR**:
The language-neutral, fully resolved representation emitted only after parsing,
semantic validation, type checking, and capability resolution succeed.
_Avoid_: Authoring Document, syntax tree, draft IR

**Built-in Catalog**:
The machine-readable transform call surface selected for one explicit language
version, including type constraints, arguments, defaults, and nullability.
_Avoid_: Latest built-ins, transform name list

**Execution**:
Evaluation of Validated IR through HTTP, offline snapshot, or browser runtime
semantics to produce a structured extraction result.
_Avoid_: Compilation, authoring
