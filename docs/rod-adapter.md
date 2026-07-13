# go-rod adapter

M4 adds a concrete browser adapter as an independent nested Go module:

```text
adapters/rod/
```

The main `scrape-kdl` module still has no browser-library dependency. Applications
choose the adapter explicitly and therefore control Chromium installation,
launch options, sandboxing, and process lifecycle.

## Lifecycle

- `rodadapter.New(page)` wraps a caller-owned page.
- `rodadapter.NewBrowser(browser)` creates and owns one page.
- `Adapter.Close` closes only an owned page.
- The adapter never closes a caller-owned `*rod.Browser`.
- One adapter represents one mutable browser page and is not safe for concurrent extractions.

## JavaScript

The core still rejects JavaScript unless `AllowJavaScript` is true. The adapter
executes document-scoped scripts through `Page.Evaluate` and current-scoped
scripts through `Element.Evaluate`, passing the current element to the KDL
function.

## E2E

The adapter module contains a build-tagged Chromium E2E test:

```bash
cd adapters/rod
go test -tags=e2e ./...
```

This requires network access for Go module resolution and a Chromium-compatible
runtime. Normal core tests do not require either.
