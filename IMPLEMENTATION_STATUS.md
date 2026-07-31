# Implementation status

## M1: compiler

Implemented:

- KDL parser;
- extractor and transform-module documents;
- relative imports and cycle detection;
- semantic validation;
- transform type checking;
- capability derivation;
- stable diagnostics;
- Validated IR generation and golden fixtures.

## M2: HTTP runtime

Implemented:

- public Go API and CLI extraction;
- runtime inputs and URL expansion;
- HTTP sessions, timeout, response limits, and charset decoding;
- offline HTML extraction;
- internal DOM and portable selector engine;
- declared, built-in, match, and external transform execution;
- field recovery, row recovery, warnings, and partial results.

## M3: browser runtime

Implemented:

- browser-library-neutral adapter contract;
- navigation and session forwarding;
- ordered workflow execution;
- live-DOM query/read operations;
- document/current JavaScript scope;
- trusted-spec JavaScript opt-in;
- browser collection extraction;
- optional adapter lease for extraction-wide serialization.
- optional bounded browser queries for first/one cardinality.

## M4: go-rod adapter

Implemented as the independent module `github.com/hsblabs/scrape-kdl/adapters/rod`:

- navigation, headers, cookies, and User-Agent;
- wait, click, fill, press, scroll, and network-idle operations;
- JavaScript evaluation;
- scoped selectors and element reads;
- page ownership and close behavior;
- extraction-wide lease;
- example CLI;
- contract tests and build-tagged Chromium E2E.

## M5: release hardening

Implemented:

- KDL slashdash suppression;
- hexadecimal, octal, and binary integer parsing;
- browser lease and failure-path tests;
- release-clean nested module configuration;
- isolated root and go-rod modules with temporary verification workspaces;
- CI, browser E2E, and release workflows;
- Apache-2.0 licensing;
- security, contribution, support, compatibility, and versioning policies;
- release archive generation and checksum script;
- module-path, formatting, and golden-IR checks;
- diagnostic registry drift checks and stable partial-result warnings;
- dated document, language, and Validated IR compatibility identifiers with explicit supported-version registries;
- release tag validation and scheduled parser fuzzing;
- host URL policy for initial targets and HTTP redirects;
- raw-text/RCDATA, truncated HTML, and common optional-end-tag recovery;
- immutable prepared execution plans and a shared HTTP/browser output walker;
- machine-readable built-in, public API, exact IR, and diagnostic contract gates;
- RE2-compatible TypeScript regular-expression execution and scaling probes.

## v1 contract completion

Implemented:

- section-complete conformance coverage inventory for the language, built-in, selector, and diagnostic contracts;
- exact dated-version grammar and shared Go/TypeScript diagnostic fixtures;
- a Node.js 26 npm workspace containing the complete independent TypeScript compiler and runtime;
- canonical Go/TypeScript IR equality for the representative static HTTP fixture;
- a machine-readable fixture manifest, dated result schema, focused suite selection, Go runner, and exact Go/TypeScript comparator;
- CI rejection of unregistered fixtures, missing expected artifacts, schema drift, and unapproved implementation differences;
- ESM-only npm workspace boundaries for `@hsblabs/scrape-kdl` and the reserved `@hsblabs/scrape-kdl-playwright` adapter package;
- the approved TypeScript root and Node.js entry points, full readonly IR declarations, and direct package consumer typechecking;
- Node.js 26 typecheck, formatting, source lint, tests, coverage thresholds, package-content inspection, and packed clean-consumer smoke gates on Linux and macOS;
- the complete documented TypeScript KDL subset with exact UTF-8 byte spans, stable invalid-UTF-8 rejection, and shared Go/TypeScript parser cases and fuzz seeds;
- injectable lexical import loading for extractor and module documents, including exact shared diagnostics for missing sources, aliases, remote paths, wrong kinds, import order, and cycles;
- explicit filesystem import loading only through the TypeScript Node.js entry point;
- bounded Go and TypeScript Authoring Documents, deterministic KDL writers, and
  an exact-version built-in catalog checked against both compiler registries.

The TypeScript semantic compiler, HTTP runtime, and official Playwright adapter now implement the complete v1 product boundary and share conformance, packaging, browser, and performance gates with Go.

## Remaining limitations

- Real go-rod and Chromium E2E require a network-enabled environment and browser availability.
- The reference KDL parser implements the Scraping KDL v0.1 subset, not every generic KDL representation.
- Go/TypeScript type and standalone extractor generation are not implemented.
- Inspector, language server, and browser authoring extension are not implemented.
