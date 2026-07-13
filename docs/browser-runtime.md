# Browser runtime

Browser-mode extractors run through an application-supplied `BrowserAdapter`. The core module does not depend on Playwright, Puppeteer, go-rod, or chromedp.

`Program.Extract` performs:

1. runtime capability and output-IR preflight;
2. input resolution and URL expansion;
3. session-policy enforcement;
4. optional adapter lease acquisition;
5. browser navigation;
6. workflow execution in source order;
7. live-DOM field and collection extraction;
8. JavaScript result validation and transform execution;
9. lease release on success or failure.

External-transform availability, portable output selectors, output member kinds, and value-source kinds are checked before the adapter lease is acquired or navigation begins.

JavaScript is disabled by default. Trusted specs must opt in with `AllowJavaScript: true`.

## Adapter boundary

```go
type BrowserAdapter interface {
    Navigate(context.Context, string, BrowserNavigateOptions) error
    WaitFor(context.Context, string, string, time.Duration) error
    Click(context.Context, string, time.Duration) error
    Fill(context.Context, string, string, time.Duration) error
    Press(context.Context, string, string, time.Duration) error
    Scroll(context.Context, float64, float64) error
    WaitForNetworkIdle(context.Context, time.Duration, time.Duration) error
    Evaluate(context.Context, string, BrowserEvaluateOptions) (any, error)
    QueryAll(context.Context, BrowserElement, string) ([]BrowserElement, error)
    Text(context.Context, BrowserElement) (string, error)
    HTML(context.Context, BrowserElement) (string, error)
    Attribute(context.Context, BrowserElement, string) (string, bool, error)
}
```

`BrowserElement` is opaque. A Playwright adapter can store a Locator or ElementHandle; a go-rod adapter can store `*rod.Element`.

## Optional extraction lease

Adapters wrapping a single mutable page should implement:

```go
type BrowserAdapterLease interface {
    Acquire(context.Context) (release func(), err error)
}
```

The runtime holds the lease across navigation, workflow, and all output reads. `release` must be non-nil and safe to call exactly once; implementations may make it idempotent. Acquisition honors context cancellation.

This contract prevents operations from two concurrent extractions from interleaving on one page. It does not create parallelism; use multiple pages/adapters for that.

## Scope

A nil element passed to `QueryAll` means document scope. For `evaluate-js`:

- `scope="document"` passes a nil scope;
- `scope="current"` passes the selected field element or collection row.

Adapters must return only JSON-compatible values from `Evaluate`.

## URL policy

`Options.URLPolicy` runs before acquiring the adapter lease and before navigation. A rejection returns `E_URL_POLICY` without using the browser. Browser redirects and page-initiated requests remain the responsibility of the adapter, browser context, or host network policy.
