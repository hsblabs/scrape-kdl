---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Reference
title: References
description: The canonical documents of Scraping KDL — the normative language specifications, the IR schema, the type declarations, the compatibility policy, and the external standards.
hsblabs:
  sidebar:
    order: 60
---

The pages of this site tell you how to use Scraping KDL. The documents below are the canon. When a page here and a document below do not agree, the document below is correct.

## The normative specification

| Document | Content |
| --- | --- |
| [Language v0.1](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/language-v0.1.md) | The full grammar, the semantics, and the validation rules. |
| [Built-ins v0.1](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/builtins-v0.1.md) | Each built-in transform, with its signature and its behavior. |
| [Selectors v0.1](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/selectors-v0.1.md) | The portable subset of CSS and the rejected constructions. |
| [Diagnostics](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/diagnostics.md) | Each diagnostic code, its severity, and its condition. |
| [Grammar summary](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/grammar-summary.ebnf) | The EBNF summary of the syntax. |

Machine-readable data for a tool:

- [`builtins-v0.1.contract.json`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/builtins-v0.1.contract.json) — the signatures of the transforms.
- [`builtins-v0.1.authoring.json`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/builtins-v0.1.authoring.json) — the data for an editor and for a completion.
- [`conformance-coverage.json`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/conformance-coverage.json) — the coverage of the fixtures against the specification.

## The Validated IR

| Document | Content |
| --- | --- |
| [IR schema](https://github.com/hsblabs/scrape-kdl/blob/main/docs/ir/schema.json) | The JSON Schema of the Validated IR. |
| [IR README](https://github.com/hsblabs/scrape-kdl/blob/main/docs/ir/README.md) | The version policy and the layout of the directory. |
| [Example IR](https://github.com/hsblabs/scrape-kdl/blob/main/docs/ir/example.ir.json) | A complete IR document. |

The IR is the boundary between the compiler and a runtime. Use the schema when you write a tool that reads a program or that makes one.

## The API declarations

- [TypeScript `index.d.ts`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/api/typescript/index.d.ts) — the portable entry point.
- [TypeScript `node.d.ts`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/api/typescript/node.d.ts) — the entry point for the file system.
- [TypeScript `authoring.d.ts`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/api/typescript/authoring.d.ts) — the data for an editor tool.
- [Public API v1](https://github.com/hsblabs/scrape-kdl/blob/main/docs/public-api-v1.md) — the stable surface of the two runtimes.

For the Go API, use `go doc`:

```bash
go doc github.com/hsblabs/scrape-kdl
```

## The runtime documents

| Document | Content |
| --- | --- |
| [Compiler pipeline](https://github.com/hsblabs/scrape-kdl/blob/main/docs/compiler-pipeline.md) | The seven stages of the validation. |
| [HTTP runtime](https://github.com/hsblabs/scrape-kdl/blob/main/docs/http-runtime.md) | The behavior of the request and of the recovery. |
| [Browser runtime](https://github.com/hsblabs/scrape-kdl/blob/main/docs/browser-runtime.md) | The contract of the adapter and the workflow. |
| [Playwright adapter](https://github.com/hsblabs/scrape-kdl/blob/main/docs/playwright-adapter.md) | The details of the TypeScript adapter. |
| [go-rod adapter](https://github.com/hsblabs/scrape-kdl/blob/main/docs/rod-adapter.md) | The details of the Go adapter. |
| [HTML compatibility](https://github.com/hsblabs/scrape-kdl/blob/main/docs/html-compatibility.md) | The parse of the HTML and the differences from a browser. |
| [Performance](https://github.com/hsblabs/scrape-kdl/blob/main/docs/performance.md) | The measured behavior and the limits. |

## The compatibility and the security

| Document | Content |
| --- | --- |
| [Compatibility](https://github.com/hsblabs/scrape-kdl/blob/main/docs/compatibility.md) | The supported versions of Go, of Node.js, and of Bun. |
| [Versioning](https://github.com/hsblabs/scrape-kdl/blob/main/docs/versioning.md) | The version of the language, the version of the IR, and the version of the release. |
| [Migrate to v1](https://github.com/hsblabs/scrape-kdl/blob/main/docs/migrating-to-v1.md) | The changes from an earlier version. |
| [Changelog](https://github.com/hsblabs/scrape-kdl/blob/main/CHANGELOG.md) | The history of the releases. |
| [Security model](https://github.com/hsblabs/scrape-kdl/blob/main/docs/security-model.md) | The trust levels and the protections of the runtime. |
| [Security policy](https://github.com/hsblabs/scrape-kdl/blob/main/SECURITY.md) | How to report a vulnerability. |
| [Responsible use](https://github.com/hsblabs/scrape-kdl/blob/main/docs/responsible-use.md) | The obligations of an operator. |

## The project

- [Repository](https://github.com/hsblabs/scrape-kdl) — the source, the fixtures, and the tests.
- [Support](https://github.com/hsblabs/scrape-kdl/blob/main/SUPPORT.md) — where to ask a question.
- [Contributing](https://github.com/hsblabs/scrape-kdl/blob/main/CONTRIBUTING.md) — how to make a change.
- [Code of conduct](https://github.com/hsblabs/scrape-kdl/blob/main/CODE_OF_CONDUCT.md)
- [Decision records](https://github.com/hsblabs/scrape-kdl/tree/main/docs/adr) — the reason for each architectural decision.

## The external standards

- [KDL](https://kdl.dev/) — the document language of the syntax. Scraping KDL accepts a documented subset.
- [RE2 syntax](https://github.com/google/re2/wiki/Syntax) — the profile of the regular expressions. A lookaround, a backreference, and a named group are not available.
- [WHATWG HTML](https://html.spec.whatwg.org/multipage/parsing.html) — the standard of the parse of the HTML.
- [CSS Selectors Level 4](https://www.w3.org/TR/selectors-4/) — the full language. The portable profile is a small subset of it.
- [JSON Schema](https://json-schema.org/) — the schema language of the IR document.
