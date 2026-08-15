---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Reference
title: CLI
description: The contract of the scrape-kdl command line — the four commands, the options, the standard streams, the JSON envelopes, the exit statuses, and the handling of the secrets.
hsblabs:
  sidebar:
    order: 30
---

The Go binary `scrape-kdl` has four commands: `validate`, `compile`, `extract`, and `version`. It is the only command-line distribution of version 1. The TypeScript packages have no CLI.

The go-rod adapter has its own binary, `scrape-kdl-rod`, for browser mode. Refer to [go-rod Adapter](../golang/rod.md).

## The commands

```bash
scrape-kdl validate extractor.kdl
scrape-kdl compile extractor.kdl --out extractor.ir.json
scrape-kdl extract extractor.kdl --input id=42
scrape-kdl version
```

Each command and also the root accept `-h` and `--help`. The help goes to the standard output and the exit status is 0. `scrape-kdl help <command>` is equivalent.

An absent command or an absent argument writes a short usage text to the standard error and gives the exit status 2. The CLI never asks you for the absent input.

## `validate`

```text
scrape-kdl validate <file.kdl|-> [--json]
```

It parses the document, resolves the symbols, checks the types, and calculates the capabilities. It causes no network activity and no browser activity.

## `compile`

```text
scrape-kdl compile <file.kdl|-> [--json] [-o file.json|-]
```

| Option | Function |
| --- | --- |
| `-o`, `--out PATH` | Writes the bare IR to the path. Use `-` for the standard output. |
| `--json` | Writes one JSON document to the standard output. |
| `--emit-ir` | A compatibility spelling. `compile` always makes the IR. |

## `extract`

```text
scrape-kdl extract <file.kdl|-> [options]
```

| Option | Function |
| --- | --- |
| `--input NAME=VALUE` | A runtime input. Repeat the option for more than one input. |
| `--html PATH` | Uses the decoded HTML from the path. Use `-` for the standard input. |
| `--session-file PATH` | Reads the headers and the cookies from a JSON file. Use `-` for the standard input. |
| `--session` | Supplies an explicit empty session. |
| `--allow-private-hosts` | Permits a target that is not globally accessible, and gives the ordinary behavior of a proxy again. |
| `--timeout DURATION` | The timeout of the HTTP request. The default is 30 seconds. |
| `--max-body BYTES` | The maximum size of the body of the response. The default is 33554432. |
| `--user-agent VALUE` | The User-Agent of the HTTP request. |
| `--json` | Writes one JSON document to the standard output. |
| `-o`, `--out PATH` | Writes the bare result to the path. Use `-` for the standard output. |

With `--html`, the runtime does no acquisition. There is no URL expansion, no URL policy, no session, and no request. Thus the declared inputs are not necessary. Refer to [Offline Snapshots](../guides/offline-snapshots.md).

## The streams

The primary result goes to the standard output. A diagnostic, a warning, a confirmation of a write, and an error go to the standard error.

The commands `compile` and `extract` write the bare IR or the bare result as formatted JSON, by default. An invocation with a redirection or a pipe has no color escape sequence, no animation of the progress, no rendering with a carriage return, and no question.

## The standard input

The character `-` selects the standard input where one stream is not ambiguous:

```bash
cat extractor.kdl | scrape-kdl validate -
cat extractor.kdl | scrape-kdl compile - --out -
cat page.html | scrape-kdl extract extractor.kdl --html -
cat session.json | scrape-kdl extract extractor.kdl --session-file -
```

One invocation can give the standard input to the KDL source, to `--html -`, or to `--session-file -`. It cannot give it to two of them. An ambiguous combination is a usage error, and the CLI does not wait for more input. The option `--out -` selects the standard output and does not use the standard input.

## The JSON envelopes

The option `--json` writes exactly one JSON document to the standard output. It does this after a success, after a processing failure, and after a usage failure that occurs after the CLI recognized the flag. A diagnostic for a person stays on the standard error.

| Command | Envelope |
| --- | --- |
| `validate` | `{"ok": boolean, "diagnostics": [...]}` |
| `compile` | `{"ok": true, "diagnostics": [...], "ir": {...}}`, or `{"ok": false, "diagnostics": [...]}` |
| `extract` | `{"ok": true, "result": {...}}`, or `{"ok": false, "error": {...}}` |
| `version` | `{"version": "...", "commit": "...", "built": "..."}` |

You cannot use `--json` with `--out FILE`. Use `--out -`, or do not use `--out`.

In an automated procedure, use `--json` and examine the field `ok` and also the exit status.

## The exit statuses

| Status | Meaning |
| --- | --- |
| 0 | Success. |
| 1 | A failure of the validation, the compilation, the extraction, the input and output, or a different processing. |
| 2 | An error in the use of a command or a flag. |
| 130 | `SIGINT` stopped the process. |
| 143 | `SIGTERM` stopped the process. |

`SIGINT` and `SIGTERM` cancel the active context of the extraction. The HTTP request and the work of the runtime see the cancellation before the process stops with the status of the signal. The CLI writes no partial primary document after an interrupted extraction.

## The network policy

By default, `extract` rejects an address that the IANA special-purpose registries do not mark as globally accessible. This includes the loopback, private, link-local, carrier-grade NAT, documentation, benchmarking, multicast, unspecified, and reserved ranges. The CLI examines the declared host and also the address that it selects at connection time, and it examines each redirect again. A rejection gives `E_URL_POLICY`.

The guarded HTTP client makes a direct connection and does not use the proxy settings of the environment, because a proxy resolves the target itself and the client then cannot examine the selected address.

Use `--allow-private-hosts` for a local, an intranet, or an explicitly proxied extraction. An offline execution with `--html` causes no network activity, thus this option has no effect on it.

## The secrets

The CLI accepts a header and a cookie only from `--session-file FILE` or from `--session-file -`:

```json
{
  "headers": {"Authorization": ["Bearer example"]},
  "cookies": [{"name": "session", "value": "example"}]
}
```

The flags `--header` and `--cookie` were removed at the contract boundary of version 0.5, because a command argument can go into the history of a shell or become visible in a list of the processes. Put a repeated flag in the array `headers` or in an entry of `cookies`. The CLI rejects a removed flag and does not write its value.

## The compatibility

The contracts of the help, the streams, the JSON, the exit statuses, the signals, and the input of the secrets are frozen for version 1. A change follows Semantic Versioning and needs a black-box test and a compatibility note.

## Next step

- [Patterns](../guides/patterns.md) — a loop with `--json` and `jq`.
- [Diagnostics](../guides/diagnostics.md) — how to read the output of a failure.
