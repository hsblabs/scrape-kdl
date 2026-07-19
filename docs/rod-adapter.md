---
status: normative
updated: 2026-07-19
---

# go-rod adapter

The go-rod integration is an independent nested Go module:

```text
adapters/rod/
```

The main `scrape-kdl` module has no browser-library dependency. Applications choose the adapter explicitly and therefore control Chromium installation, launch options, sandboxing, network policy, and process lifecycle.

## Lifecycle

- `rodadapter.New(page)` wraps a caller-owned page.
- `rodadapter.NewBrowser(browser)` creates and owns one page.
- `Adapter.Close` closes only an owned page.
- The adapter never closes a caller-owned `*rod.Browser`.
- One adapter represents one mutable browser page.
- The adapter implements `BrowserAdapterLease`, serializing the complete navigation, workflow, and extraction lifecycle across concurrent calls.
- Parallel extraction should use separate adapters and pages.

## JavaScript

The core rejects JavaScript unless `AllowJavaScript` is true. The adapter executes document-scoped scripts through `Page.Evaluate` and current-scoped scripts through `Element.Evaluate`, passing the current element to the KDL function.

## Command line

`scrape-kdl-rod` compiles one extractor and executes it in browser mode:

```bash
scrape-kdl-rod --spec extractor.kdl --input race_id=202401010101 --json
scrape-kdl-rod --spec extractor.kdl --session-file session.json -o result.json
```

Supported options are:

- `--input NAME=VALUE`, repeated as needed and parsed according to the extractor declarations;
- `--session-file FILE|-` using the core CLI's JSON session schema;
- `--timeout`, `--user-agent`, `--headless`, and explicit `--allow-js`;
- `--json` for one-document machine-readable success and failure envelopes;
- `-o`/`--out FILE|-` for a bare extraction result;
- `--allow-private-hosts` to disable the default initial-target restriction;
- `-h`/`--help` and `--version`.

Direct `--header` and `--cookie` flags are rejected without echoing their values. Standard input is reserved for `--session-file -`; the KDL source is always named by `--spec`.

## Output and exit status

Without `--json`, success emits the bare formatted extraction result to standard output or the selected output file. Warnings and human-readable failures go to standard error. With `--json`, standard output contains exactly one of:

- `{"ok": true, "result": {...}}` on extraction success;
- `{"ok": false, "error": {...}}` on compilation, execution, I/O, or usage failure;
- `{"version": "...", "commit": "...", "built": "..."}` for `--version --json`.

`--json` cannot be combined with `--out FILE`; use `--out -` or omit it. Exit statuses are 0 for success, 1 for processing failure, 2 for usage error, 130 for `SIGINT`, and 143 for `SIGTERM`. The two signals cancel the active extraction context before the signal-specific exit.

## Sessions and URL policy

Session headers, cookies, User-Agent, runtime inputs, timeout, and JavaScript opt-in are passed through the same public `scrapekdl.Options` contract as library usage.

The CLI applies `PublicInternetURLPolicy` to the initial navigation target by default. It rejects schemes, credentials, and addresses that the IANA special-purpose registries do not mark globally reachable. `--allow-private-hosts` opts out.

This initial check is not a browser network sandbox. Redirects handled inside Chromium, subresources, service workers, and page-initiated traffic are outside the hook. Production hosts must enforce the required egress policy at the browser context, process, container, or network boundary.

## Verification

Contract tests use the local rod stub and do not download go-rod:

```bash
make test-rod-contract
```

The real dependency build and tests run with:

```bash
make test-rod
```

The build-tagged Chromium E2E suite runs with:

```bash
make test-rod-e2e
```

The verification scripts use temporary module files so the release-clean `adapters/rod/go.mod` and `go.sum` are not rewritten. The E2E suite requires a Chromium-compatible runtime; normal core and contract tests do not.
