---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: How It Operates
description: The seven compiler stages of Scraping KDL, the capability set, the Validated IR, and the rule that no network or browser operation occurs before the validation is correct.
hsblabs:
  sidebar:
    order: 20
---

Scraping KDL divides the work into two parts. The compiler reads your document and makes a Validated IR. A runtime executes that IR. The two parts are fully separate. Thus you can examine, store, or transmit a program before it touches a network.

## The stages

The compiler executes these stages in this sequence:

1. KDL syntax parse;
2. base restriction validation;
3. application grammar validation;
4. import graph resolution;
5. symbol resolution;
6. type check;
7. capability derivation and validation.

An error in one stage stops the program before the subsequent stages. Each error becomes a diagnostic with a code and a source location.

A conforming implementation must complete the stages 1 to 7 before it sends a network request, starts a browser, navigates a page, changes a session, or calls an external transform. This is a rule of the language, not a property of one implementation.

## What the compiler does not do

The compiler package contains no network client, no browser control, and no external transform call. It reads the entry KDL file and the relative modules that the file imports. It does nothing more.

The TypeScript core has no automatic access to the file system and no automatic network loader. It resolves a relative path lexically and asks a `SourceLoader` for the bytes. The `@hsblabs/scrape-kdl/node` entry point supplies the loader for the file system. Thus the core package cannot read a file that you did not permit.

The parser keeps the sequence of the properties and also the duplicates. The semantic stage then rejects a duplicate property with `E_DUPLICATE_PROPERTY`. The compiler does not silently use the last property, although KDL permits this behavior.

## Capabilities

The Validated IR contains a sorted set of the capabilities that the program needs. The compiler calculates this set from the content of the document.

| Capability | Cause |
| --- | --- |
| `http.fetch` | An HTTP source. |
| `browser.navigate` | A browser source. |
| `browser.query` | A selection in browser mode. |
| `browser.read-text`, `browser.read-html`, `browser.read-attr` | A value source in browser mode. |
| `browser.wait`, `browser.input`, `browser.scroll`, `browser.network-idle` | A workflow step. |
| `browser.evaluate-js` | An `evaluate-js` value source. |
| `transform.external:<symbol>` | An external transform. |

The set is a contract. A host can read it and refuse a program before the execution. For example, a host can permit `http.fetch` and refuse each `browser.*` capability. The exact list is in [language-v0.1.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/language-v0.1.md).

## The Validated IR

The IR is the language-neutral contract between the compiler and each runtime. The Go runtime and the TypeScript runtime read the same IR and give the same values.

The IR contains three version fields with a related function:

- `irVersion` — the format of the IR document. The current value is `2026-07-15`.
- `languageVersion` — the language contract that the document selected with `language-version`. The current value is `2026-07-15`.
- The `version` property of your document — your revision identifier. The compiler does not interpret it, but the value must be a real calendar date in the form `YYYY-MM-DD`.

The schema of the IR is in [schema.json](https://github.com/hsblabs/scrape-kdl/blob/main/docs/ir/schema.json).

## The runtimes

One IR has three execution boundaries:

- **HTTP** — the runtime sends a request, decodes the body, parses the HTML, and extracts the values. Refer to [HTTP Execution](./http-execution.md).
- **Browser** — the runtime uses an adapter that you supply to navigate a live page, execute the workflow, and read the values. Refer to [Browser Mode](./browser-mode.md).
- **Offline snapshot** — the runtime extracts from HTML that you supply and does no acquisition. Refer to [Offline Snapshots](./offline-snapshots.md).

For an extraction without JavaScript, the HTTP runtime and the browser runtime must agree. The same conformance fixtures test the two runtimes. A difference between them is a defect, not a permitted variation.

## Deterministic diagnostics

The compiler orders the static diagnostics by the resolution sequence of the imports, then by the path of the file, then by the position in the source, then by the code. The runtime orders the warnings by the sequence of the execution.

Thus two executions of the same program on the same input give the same output. You can compare the diagnostics in a test or in a CI job. Refer to [Diagnostics](./diagnostics.md).

## Next step

- [Write an Extractor](./write-an-extractor.md) — the structure of a document.
- [Selectors](./selectors.md) — the portable subset of CSS.
- [Transforms](./transforms.md) — the value pipeline and the types.
