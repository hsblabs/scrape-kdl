# Command-line contract

The Go `scrape-kdl` binary provides `validate`, `compile`, `extract`, and `version`. It is the only v1 command-line distribution; the TypeScript packages do not ship a CLI.

## Help and command discovery

`-h` and `--help` print full help to standard output and exit 0 on the root and every subcommand. `scrape-kdl help <command>` is equivalent. A missing required command or argument prints concise usage to standard error and exits 2; the CLI never prompts to repair missing input.

```bash
scrape-kdl --help
scrape-kdl help extract
scrape-kdl compile --help
```

## Standard streams

Primary results go to standard output. Diagnostics, warnings, write confirmations, and errors go to standard error. `compile` and `extract` emit their bare IR or extraction result as formatted JSON by default. A redirected or piped invocation contains no color escape sequences, progress animation, carriage-return rendering, or prompts.

`-` is supported where a single stream is unambiguous:

```bash
cat extractor.kdl | scrape-kdl validate -
cat extractor.kdl | scrape-kdl compile - --out -
cat page.html | scrape-kdl extract extractor.kdl --html -
cat session.json | scrape-kdl extract extractor.kdl --session-file -
```

One invocation may assign standard input to only one of the KDL source, `--html -`, or `--session-file -`. Ambiguous combinations are usage errors and never wait for additional input. `--out -` explicitly selects standard output and does not consume standard input.

## Machine-readable output

`--json` emits exactly one JSON document on standard output on success, processing failure, or usage failure after the flag has been recognized. Human diagnostics and errors remain on standard error. `--json` cannot be combined with `--out FILE`; use `--out -` or omit `--out`.

The command-specific envelopes are:

- `validate`: `{"ok": boolean, "diagnostics": [...]}`;
- `compile`: `{"ok": true, "diagnostics": [...], "ir": {...}}` on success, or `{"ok": false, "diagnostics": [...]}` for compiler rejection;
- `extract`: `{"ok": true, "result": {...}}` on success, or `{"ok": false, "error": {...}}` on failure;
- `version`: `{"version": "...", "commit": "...", "built": "..."}`.

The default bare `compile` and `extract` documents remain convenient for direct piping. Automation that needs an explicit success discriminator should use `--json` and check the process exit status as well.

## Exit statuses and signals

| Status | Meaning |
|---:|---|
| 0 | success |
| 1 | validation, compilation, extraction, I/O, or other processing failure |
| 2 | command or flag usage error |
| 130 | interrupted by `SIGINT` |
| 143 | terminated by `SIGTERM` |

`SIGINT` and `SIGTERM` cancel the active extraction context. HTTP requests and runtime work observe that cancellation before the process exits with the signal-specific status. No partial primary document is written after an interrupted extraction.

## Session secrets

Session headers and cookies are accepted only through `--session-file FILE` or explicit `--session-file -` standard input. The JSON shape is:

```json
{
  "headers": {"Authorization": ["Bearer example"]},
  "cookies": [{"name": "session", "value": "example"}]
}
```

The former `--header` and `--cookie` flags were removed at the v0.5 contract boundary because command arguments can leak through shell history and process inspection. Migrate repeated flags into the corresponding `headers` array or `cookies` entry. The CLI rejects removed flags without echoing their values.

## Compatibility window

This command contract is frozen for the v1 release candidate as of 2026-07-15. A change requires black-box tests, compatibility notes, migration guidance, a release-blocker record, and a restarted 14-day candidate window. After stable publication, the help, stream, JSON, exit-status, signal, and secret-input contracts follow Semantic Versioning.
