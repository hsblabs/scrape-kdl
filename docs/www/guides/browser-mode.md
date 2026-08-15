---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Browser Mode
description: How browser mode operates in Scraping KDL — the adapter contract, the workflow steps, the JavaScript opt-in, the extraction lease, and the parts that are common to each adapter.
hsblabs:
  sidebar:
    order: 25
---

Browser mode is a capability, not a different method to fetch a page. A program with `mode="browser"` needs an adapter that you supply. The core module does not depend on Playwright, Puppeteer, go-rod, or chromedp. If you do not supply an adapter, the extraction fails with `E_BROWSER_RUNTIME_MISSING` before the navigation.

This page describes the contract that is common to each adapter. For an installation and a lifecycle, refer to [Playwright Adapter](../npm/playwright.md) or [go-rod Adapter](../golang/rod.md).

## When you need it

Use browser mode only for a condition that a static DOM cannot give you:

- a script writes the content after the load;
- the content becomes visible only after a click, an input, or a scroll;
- the data is in the memory of the page and not in the markup.

For each other condition, use HTTP mode. It is faster, it needs no Chromium, and it has a smaller attack surface.

## The execution order

```text
capability and output preflight
  -> input resolution and URL expansion
  -> session policy
  -> optional lease acquisition
  -> navigation
  -> workflow steps, in source order
  -> extraction from the live DOM
  -> validation of the JavaScript results and the transforms
  -> lease release, after a success or a failure
```

The runtime checks the availability of the external transforms, the kinds and the selectors of the workflow steps, the portable output selectors, the kinds of the output members, and the kinds of the value sources before it acquires the lease and before the navigation. Thus a program with an error does not start a browser.

## The workflow

A workflow executes after the navigation and before the extraction. The steps execute in source order.

```kdl
source "html" {
  fetch mode="browser" url="https://example.com/race/{race_id}"
  workflow {
    wait-for ".content" state="visible" timeout-ms=5000
    click ".load-more" timeout-ms=3000
    wait-for-network-idle idle-ms=500 timeout-ms=5000
  }
}
```

| Step | Function |
| --- | --- |
| `wait-for selector [state] [timeout-ms]` | Waits for a state: `attached`, `visible`, `hidden`, or `detached`. The default is `visible`. |
| `click selector [timeout-ms]` | Clicks the element. |
| `fill selector value [timeout-ms]` | Puts a value in the element. |
| `press selector key [timeout-ms]` | Sends a key to the element. |
| `scroll x y` | Scrolls the window. The two numbers are CSS pixels. |
| `wait-for-network-idle [idle-ms] [timeout-ms]` | Waits until no tracked HTTP request is active for `idle-ms`. The default is 500 ms. |
| `evaluate-js script [timeout-ms]` | Executes a function. The runtime discards the result. |

A workflow step is available in browser mode only. In HTTP mode the compiler rejects the node `workflow` with `E_BROWSER_CAPABILITY_REQUIRED`.

The values `timeout-ms` and `idle-ms` must be from 1 to 9,223,372,036,854 milliseconds. An expired timeout is an extraction error. A WebSocket connection and an EventSource connection are not tracked requests for `wait-for-network-idle`.

## JavaScript

JavaScript is off by default. You must give an explicit opt-in: `AllowJavaScript: true` in Go, or `allowJavaScript: true` in TypeScript. Without the opt-in, a program with JavaScript fails with `E_JAVASCRIPT_DISABLED` before the navigation.

```kdl
field "race" type="object?" {
  evaluate-js #"""
    () => window.__INITIAL_STATE__?.race ?? null
    """# scope="document" returns="object?" timeout-ms=3000
}
```

The rules are strict:

- The script must give a callable function. An async function is permitted.
- The property `scope` is `document` or `current`. With `document`, the function gets no argument. With `current`, it gets the current element as its first argument.
- The property `returns` declares the type of the raw result.
- The result must be JSON-compatible: `null`, a boolean, a string, a finite number, an array, or a plain object with string keys.
- The values `undefined`, `NaN`, an infinity, a bigint, a symbol, a function, a DOM node, a handle of the runtime, a cyclic object, a `Map`, a `Set`, and a `Date` are forbidden. The result fails with `E_JAVASCRIPT_RESULT_TYPE`.

Treat `evaluate-js` as trusted code of the specification. It executes with the full permissions of the page. It is also an intentional escape from the portable behavior: an offline snapshot cannot reproduce it.

## The adapter contract

An adapter implements a small interface. The Go interface has these operations:

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

The TypeScript contract has the same operations with promises, timeout fields in milliseconds, and an optional `AbortSignal`.

A `BrowserElement` is an opaque handle. A Playwright adapter can keep a Locator or an ElementHandle. A go-rod adapter can keep a `*rod.Element`. The core never examines the content of a handle.

If the adapter does not have an operation that the program needs, the validation fails with `E_BROWSER_CAPABILITY_MISSING` before the navigation.

## The lease

An adapter that controls one mutable page must implement a lease:

```go
type BrowserAdapterLease interface {
    Acquire(context.Context) (release func(), err error)
}
```

The runtime holds the lease during the navigation, the workflow, and each output read. Thus the operations of two concurrent extractions cannot interleave on one page.

A lease does not give parallelism. For a parallel execution, use more than one page or more than one adapter.

## The scope of a query

A nil element for `QueryAll` means the scope of the document. For `evaluate-js`, the scope `document` gives a nil scope, and the scope `current` gives the selected element of the field or the row of the collection.

An adapter can also implement a bounded query, `BrowserAdapterQueryLimit` in Go or `queryLimit` in TypeScript. The runtime uses it for `match="first"` and `match="one"`, where one or two handles are sufficient. An adapter without this function uses `QueryAll`.

## The sessions and the URL policy

With `session policy="none"`, the runtime gives no explicit session to `Navigate`. It does not clear the cookies, the storage, or the authentication that the browser context already has. For an execution without a state, supply an isolated context.

The hook `Options.URLPolicy` executes before the lease acquisition and before the navigation. A rejection gives `E_URL_POLICY` and does not use the browser.

The policy controls the initial target only. A redirect of the browser, a subresource, a service worker, and a request that the page starts are outside of this hook. Control them with the browser context or with a network policy of the host.

## Next step

- [Playwright Adapter](../npm/playwright.md) — the official adapter for TypeScript and Node.js.
- [go-rod Adapter](../golang/rod.md) — the official adapter for Go.
- [Offline Snapshots](./offline-snapshots.md) — how to test a browser-mode program without a browser.
