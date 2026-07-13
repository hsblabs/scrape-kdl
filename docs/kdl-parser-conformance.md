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

Parser conformance fixtures belong under `internal/kdl` and language fixtures under `fixtures/`.

## Fuzzing

`FuzzParseNeverPanics` continuously exercises arbitrary input. A scheduled GitHub Actions workflow runs the KDL parser and DOM parser/selector fuzz targets. Fuzzing guarantees crash resistance, not acceptance of syntax outside the documented subset.
