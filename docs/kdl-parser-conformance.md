# KDL parser conformance

The reference parser implements the KDL 2 features required by Scraping KDL v0.1 rather than exposing a general-purpose KDL data model.

Supported lexical features include:

- UTF-8 input;
- quoted strings and hash-delimited raw strings;
- raw multiline strings used by JavaScript blocks;
- decimal, hexadecimal, octal, and binary integers;
- finite decimal floating-point values;
- `#true`, `#false`, and `#null`;
- single-line and nested block comments;
- slashdash suppression for nodes, arguments, properties, and children blocks;
- semicolon and newline node termination.

Scraping KDL rejects KDL type annotations by language rule. The current parser does not promise support for every generic KDL 2 representation, including multiline escaped quoted strings, keyword non-finite numbers, arbitrary Unicode identifiers, or whitespace escapes. Specs should use the canonical formatting defined by the language specification.

Shared parser cases and fuzz seeds live under `fixtures/parser/`; both Go and TypeScript consume them directly. Language fixtures remain under `fixtures/`.
The normative section-to-evidence inventory is `docs/spec/conformance-coverage.json` and is drift-checked by `TestConformanceCoverage`.

The TypeScript parser under `packages/scrape-kdl` implements the same documented subset and consumes the same acceptance and diagnostic cases. It preserves UTF-8 byte offsets with one-based lines and columns, rejects invalid UTF-8 before parsing, and matches Go diagnostic ordering. Its source graph resolves relative imports lexically through the injected `SourceLoader`; remote and absolute import syntax is rejected before the loader is called.

## Fuzzing

`FuzzParseNeverPanics` continuously exercises arbitrary input using the shared seed inventory. TypeScript tests run the same seeds plus 2,048 deterministic mutations bounded to 511 UTF-16 code units per case. A scheduled GitHub Actions workflow runs the Go KDL parser and DOM parser/selector fuzz targets. Fuzzing guarantees crash resistance, not acceptance of syntax outside the documented subset.
