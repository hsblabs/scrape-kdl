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

External-transform availability, workflow step kinds and selectors, portable output selectors, output member kinds, and value-source kinds are checked before the adapter lease is acquired or navigation begins.

JavaScript is disabled by default. Trusted specs must opt in with `AllowJavaScript: true`.

`Program.ExtractSnapshot` and TypeScript `program.extractSnapshot` are separate from this live-browser sequence. They can evaluate an otherwise portable browser-mode program against supplied static HTML without an adapter or outbound I/O. A program containing any workflow or JavaScript field value source fails with `E_SNAPSHOT_UNSUPPORTED`; snapshot execution never pretends that static HTML reproduces browser mutation or JavaScript. See `docs/http-runtime.md` for the complete three-way execution boundary.

## Adapter boundary

Adapter authors must treat every element as an opaque handle, honor operation cancellation, apply the supplied timeout, avoid returning browser-library objects as JavaScript results, and keep all mutable page operations inside an extraction-wide lease. Timeout or cancellation must not leave work running after the lease is released.

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

## Optional bounded query

Adapters may implement `BrowserAdapterQueryLimit` with `QueryLimit(context.Context, BrowserElement, string, int)`. The TypeScript equivalent exposes `queryLimit(scope, selector, limit, options)`. The runtime uses this capability for `match="first"` and `match="one"`, where at most one or two handles are needed, and falls back to `QueryAll` for existing adapters. The core passes only positive limits; adapters should return no more than that many handles in document order.

The official Playwright adapter bounds the returned locator handles without materializing the complete match set. The go-rod adapter currently preserves compatibility by truncating its query result; a future adapter-native bounded query can improve transport cost without changing the core contract.

## Optional extraction lease

Adapters wrapping a single mutable page should implement:

```go
type BrowserAdapterLease interface {
    Acquire(context.Context) (release func(), err error)
}
```

The runtime holds the lease across navigation, workflow, and all output reads. `release` must be non-nil and safe to call exactly once; implementations may make it idempotent. Acquisition honors context cancellation.

This contract prevents operations from two concurrent extractions from interleaving on one page. It does not create parallelism; use multiple pages/adapters for that.

The TypeScript contract mirrors these operations with promises, millisecond timeout fields, and optional `AbortSignal` values. `BrowserAdapterLease.acquire(signal)` resolves to an idempotent release callback. The official implementation and its concrete lifecycle are documented separately in `docs/playwright-adapter.md`.

Compiled programs prepare immutable selectors, transforms, regular expressions, and output preflight state once. Per-extraction inputs, external-transform bindings, cancellation, session state, URL policy, and browser leases remain dynamic and are never cached in that prepared plan.

## Scope

A nil element passed to `QueryAll` means document scope. For `evaluate-js`:

- `scope="document"` passes a nil scope;
- `scope="current"` passes the selected field element or collection row.

Adapters must return only these concrete Go representations from `Evaluate`:

- `nil`, `bool`, `string`;
- signed or unsigned built-in integer types;
- finite `float32` or `float64`, or a valid finite `json.Number`;
- `[]string` or `[]any`, recursively containing accepted values;
- `map[string]any`, recursively containing accepted values.

Typed containers such as `[]map[string]any` and `map[string]string`, structs, and custom JSON marshalers are not part of the adapter contract. Adapters can call `scrapekdl.NormalizeBrowserResult` before returning a value; it validates the contract, copies recursive containers, and converts `[]string` to `[]any` without reflection or `json.Marshaler` invocation. The runtime then normalizes numbers to the declared `returns` type.

For `session policy="none"`, the runtime passes no explicit `Session` to `Navigate`. It does not clear cookies, storage, or authentication already held by the adapter's browser context. Hosts that require stateless execution must provide an isolated context.

## URL policy

`Options.URLPolicy` runs before acquiring the adapter lease and before navigation. A rejection returns `E_URL_POLICY` without using the browser. Browser redirects and page-initiated requests remain the responsibility of the adapter, browser context, or host network policy.
