---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: go-rod Adapter
description: The official go-rod browser adapter for Go — the separate module, the ownership of a page, the lease that serializes an extraction, and the scrape-kdl-rod command line.
hsblabs:
  sidebar:
    order: 52
---

The go-rod integration is an independent nested Go module:

```text
adapters/rod/
```

```bash
go get github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.4
```

The main module has no dependency on a browser library. You select the adapter explicitly. Thus you control the installation of Chromium, the options of the launch, the sandbox, the network policy, and the lifecycle of the process.

## The lifecycle

| Function | Behavior |
| --- | --- |
| `rodadapter.New(page)` | Uses a page that you own. |
| `rodadapter.NewBrowser(browser)` | Makes one page and owns it. |
| `Adapter.Close` | Closes a page that the adapter owns. It closes nothing else. |

The adapter never closes a `*rod.Browser` that you own.

One adapter represents one mutable browser page. The adapter implements `BrowserAdapterLease`. Thus the lease puts the full sequence of the navigation, the workflow, and the extraction in a series, between the concurrent calls.

For a parallel extraction, use a separate adapter and a separate page for each thread of the work.

## JavaScript

The core rejects JavaScript until `AllowJavaScript` is `true`.

The adapter executes a script with the scope `document` through `Page.Evaluate`. It executes a script with the scope `current` through `Element.Evaluate` and gives the current element to your KDL function.

## The command line

The module has its own binary. It compiles one extractor and executes it in browser mode.

```bash
go install github.com/hsblabs/scrape-kdl/adapters/rod/cmd/scrape-kdl-rod@v1.0.4
```

```bash
scrape-kdl-rod --spec extractor.kdl --input race_id=202401010101 --json
scrape-kdl-rod --spec extractor.kdl --session-file session.json -o result.json
```

| Option | Function |
| --- | --- |
| `--spec FILE` | The KDL source. The CLI always uses this option for the source. |
| `--input NAME=VALUE` | A runtime input. Repeat the option for more than one input. |
| `--session-file FILE\|-` | The JSON session schema of the core CLI. |
| `--timeout` | The timeout of the operations. |
| `--user-agent` | The User-Agent. |
| `--headless` | The mode of the browser. |
| `--allow-js` | The explicit opt-in for JavaScript. |
| `--allow-private-hosts` | Disables the default limit of the initial target. |
| `--json` | One JSON document for a success or for a failure. |
| `-o`, `--out FILE\|-` | The bare result of the extraction. |

The CLI rejects the flags `--header` and `--cookie` and does not write their values. The standard input is reserved for `--session-file -`.

## The output and the exit status

Without `--json`, a success writes the bare formatted result to the standard output or to the selected file. A warning and a failure for a person go to the standard error.

With `--json`, the standard output has exactly one of these documents:

- `{"ok": true, "result": {...}}` after a successful extraction;
- `{"ok": false, "error": {...}}` after a failure of the compilation, the execution, the input and output, or the use;
- `{"version": "...", "commit": "...", "built": "..."}` for `--version --json`.

You cannot use `--json` with `--out FILE`. Use `--out -`, or do not use `--out`.

The exit status is 0 for a success, 1 for a processing failure, 2 for an error of the use, 130 for `SIGINT`, and 143 for `SIGTERM`. The two signals cancel the active context before the exit.

## The sessions and the URL policy

The headers and the cookies of the session, the User-Agent, the runtime inputs, the timeout, and the opt-in for JavaScript use the same public contract `scrapekdl.Options` as a library.

The CLI applies `PublicInternetURLPolicy` to the initial navigation target, by default. It rejects a scheme, a credential, and an address that the IANA special-purpose registries do not mark as globally accessible. The option `--allow-private-hosts` disables it.

This initial check is not a network sandbox for the browser. A redirect inside Chromium, a subresource, a service worker, and a request that the page starts are outside of the hook. A production host must apply the necessary egress policy at the boundary of the browser context, the process, the container, or the network.

## The verification

The contract tests use a local stub and do not download go-rod:

```bash
make test-rod-contract
```

The build with the true dependency and its tests use this command:

```bash
make test-rod
```

The end-to-end suite with Chromium uses this command:

```bash
make test-rod-e2e
```

The end-to-end suite needs a runtime that is compatible with Chromium. The core tests and the contract tests do not need one.

## Next step

- [Browser Mode](../guides/browser-mode.md) — the workflow steps and the rules of the JavaScript.
- [Compile and Extract in Go](./compile-and-extract.md) — the options of the execution.
