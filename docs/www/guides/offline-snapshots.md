---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Offline Snapshots
description: Execute an extractor against saved HTML with no network operation and no browser — the three execution boundaries, the rules of eligibility, and how to make a test that is stable.
hsblabs:
  sidebar:
    order: 26
---

An offline snapshot executes the extraction against HTML that you supply. It gets nothing. There is no URL expansion, no URL policy, no session, no HTTP request, no browser lease, and no JavaScript.

Use a snapshot for a test in CI, for the development of a selector, and for a regression test after a change of a page.

## The three execution boundaries

One compiled program has three entry points. Their differences are important:

| Entry point | Acquisition | Accepted modes |
| --- | --- | --- |
| `Program.Extract`, `program.extract` | Follows the mode of the source. HTTP, or a browser adapter. | Each mode. |
| `Program.ExtractHTML` | None. | HTTP mode only. Go only. |
| `Program.ExtractSnapshot`, `program.extractSnapshot` | None. | Each mode, if the program is eligible. |

`Program.ExtractHTML` is the original entry point of Go for saved HTML. It accepts an HTTP-mode program only.

`ExtractSnapshot` is the general form. It accepts a browser-mode program also, but only when the program is eligible.

## The eligibility

The runtime calculates the eligibility for a snapshot from the full program, when it prepares the immutable program.

A program that has a `workflow` node or an `evaluate-js` field value source **is not eligible**. It fails with `E_SNAPSHOT_UNSUPPORTED`.

The runtime never ignores these operations. Static HTML cannot reproduce a mutation of a browser or a result of JavaScript. Thus a false success is not possible. If your browser-mode program needs a snapshot test, keep the JavaScript in a small number of fields and test the other fields offline.

## What a snapshot keeps

An eligible snapshot keeps the full behavior of the extraction:

- the selectors and their cardinality;
- the transforms, and also the external transforms;
- the error recovery with `on-error` and `on-row-error`;
- the warnings and the flag `partial`;
- the cancellation.

Thus a snapshot test examines the true behavior. It is not an approximation.

## The CLI

Give the HTML with the option `--html`:

```bash
scrape-kdl extract ./extractor.kdl --html ./page.html
```

The CLI also accepts the standard input:

```bash
cat page.html | scrape-kdl extract ./extractor.kdl --html -
```

The declared inputs are not necessary here, because the runtime does no URL expansion. An offline execution causes no network activity. The option `--allow-private-hosts` has no effect on it.

## A stable test

Save the HTML in your repository with the extractor. Then a test tells you about a change of your code, not about a change of the network.

1. Save the page one time. Use a page that has the conditions that you must control: an absent optional value, a collection with more than one row, and a bad row.
2. Put the file in the version control with the extractor.
3. Execute `extract` with `--html` in your CI job.
4. Compare the JSON result with a golden file.

The result is deterministic. The same HTML and the same program always give the same value, the same warnings, and the same flag `partial`.

When the page changes, get a new snapshot and examine the difference in the golden file. The difference tells you exactly what changed.

## The cancellation

The runtime examines the context before it parses the HTML in the memory, and also before each output member and each collection row. A cancellation gives `E_EXECUTION_CANCELED` and keeps the cause. A field policy or a row policy cannot recover it. The TypeScript runtime uses an `AbortSignal` at the same boundaries.

## Next step

- [Diagnostics](./diagnostics.md) — how to read the result of a failure.
- [Go](../golang/compile-and-extract.md) or [TypeScript and Bun](../npm/compile-and-extract.md) — how to call a snapshot from a program.
