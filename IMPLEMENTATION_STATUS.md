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
- Go workspace for monorepo development;
- CI, browser E2E, and release workflows;
- Apache-2.0 licensing;
- security, contribution, support, compatibility, and versioning policies;
- release archive generation and checksum script;
- module-path, formatting, and golden-IR checks.
- diagnostic registry drift checks and stable partial-result warnings;
- dated document, language, and Validated IR compatibility identifiers with explicit supported-version registries;
- release tag validation and scheduled parser fuzzing;
- host URL policy for initial targets and HTTP redirects;
- raw-text/RCDATA, truncated HTML, and common optional-end-tag recovery.

## v1 contract completion

Implemented:

- section-complete conformance coverage inventory for the language, built-in, selector, and diagnostic contracts;
- exact dated-version grammar and shared Go/TypeScript diagnostic fixtures;
- a private Node.js 26 npm workspace containing the bounded independent TypeScript parser/compiler cross-check;
- canonical Go/TypeScript IR equality for the representative static HTTP fixture.

The TypeScript slice deliberately covers the v0.2 contract cross-check only.
The complete source loader, import resolver, semantic compiler, HTTP runtime, and Playwright adapter remain later roadmap work.

## Remaining limitations

- Real go-rod and Chromium E2E require a network-enabled environment and browser availability.
- The HTTP HTML parser is not a complete WHATWG tree builder.
- The reference KDL parser implements the Scraping KDL v0.1 subset, not every generic KDL representation.
- The complete TypeScript compiler, runtime, and Playwright adapter are not implemented.
- Go/TypeScript type and standalone extractor generation are not implemented.
- Inspector, language server, and browser authoring extension are not implemented.
