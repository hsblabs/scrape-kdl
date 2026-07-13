# scrape-kdl go-rod adapter

This nested Go module implements `scrapekdl.BrowserAdapter` with go-rod while keeping the core module free of browser dependencies.

```bash
go get github.com/hsblabs/scrape-kdl/adapters/rod@latest
```

```go
browser := rod.New().MustConnect()
defer browser.MustClose()

adapter, err := rodadapter.NewBrowser(browser)
if err != nil {
    return err
}
defer adapter.Close()

result, err := program.Extract(ctx, inputs, scrapekdl.Options{
    Browser:         adapter,
    AllowJavaScript: true,
})
```

`NewBrowser` creates and owns one page but does not own the browser. `New(page)` wraps a caller-owned page and closes neither page nor browser.

The adapter implements `BrowserAdapterLease`. Concurrent `Program.Extract` calls using the same adapter are serialized for the complete navigation/workflow/extraction lifecycle. Separate adapters/pages are recommended for parallel throughput.

## Verification

Contract verification without downloading go-rod:

```bash
make test-rod-contract
```

Real dependency verification:

```bash
make test-rod
```

Chromium E2E:

```bash
make test-rod-e2e
```

The module is versioned independently using tags such as `adapters/rod/v0.1.0`.
