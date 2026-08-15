---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Playwright Adapter
description: The official Playwright browser adapter for TypeScript — the ownership of the browser, the isolation of each context, the cleanup after a timeout, and the supported browsers.
hsblabs:
  sidebar:
    order: 42
---

The package `@hsblabs/scrape-kdl-playwright` implements the contracts `BrowserAdapter` and `BrowserAdapterLease`. It is a separate package. Thus the core package does not get Playwright in its dependency graph.

```bash
npm install @hsblabs/scrape-kdl-playwright@1.0.4 playwright
npx playwright install chromium
```

## The use

```ts
import { chromium } from "playwright";
import { PlaywrightAdapter } from "@hsblabs/scrape-kdl-playwright";

const browser = await chromium.launch({ headless: true });
const adapter = new PlaywrightAdapter(browser);
try {
  const result = await compiled.program.extract(
    { id: "123" },
    { browser: adapter, allowJavaScript: true },
  );
  console.log(result.value);
} finally {
  await adapter.close();
  await browser.close();
}
```

The option `allowJavaScript: true` is necessary only when your program has an `evaluate-js` node. Do not set it for a program that does not need it.

## The ownership

You own the `Browser`. The adapter owns only the isolated contexts that it makes.

The method `adapter.close()` closes the contexts of the adapter. It never closes your browser. Close the browser yourself, as the example shows.

Thus you can use one browser for more than one adapter, or keep one browser during the full life of your process.

## The isolation

Each call of `navigate` does these operations in this sequence:

1. It closes the previous context of the adapter.
2. It makes a new context.
3. It installs the explicit headers of the session, the cookies, and the User-Agent.
4. It navigates a new page.

Thus a cookie, a storage value, a mutation of a page, or a failed operation from one extraction cannot go into the next extraction.

## The mapping of the operations

- A portable selector becomes a Playwright locator, in the scope of the document or of the current element.
- A read of the text uses the `textContent` of the descendants. A read of the HTML uses `innerHTML`. An attribute gives the value of the attribute of the DOM.
- The workflow steps `wait`, `click`, `fill`, `press`, `scroll`, and the configured network-idle operation execute in source order.
- With `scope="document"`, the JavaScript function gets no argument. With `scope="current"`, it gets the current DOM element.
- A JavaScript result crosses the boundary of the adapter. The core runtime then examines it for the JSON compatibility and for the declared type `returns`.

The adapter also limits a query for `match="first"` and `match="one"`. It does not make the full set of the matches.

## The timeout and the cancellation

The adapter puts each operation in a race against the public timeout and the `AbortSignal`.

For an operation that can continue inside the browser, a timeout or a cancellation first closes the isolated context. The runtime holds the lease of the adapter until this cleanup is complete. Thus no operation continues after the release of the lease. A later extraction makes a new context and operates correctly.

## The supported browsers

Chromium is the blocking target of version 1. The scheduled workflow of the browser also reports the results of Firefox and WebKit, with a non-blocking status.

Use Chromium for a production extraction. A promotion of the support of a different browser needs a separate compatibility decision.

## Next step

- [Browser Mode](../guides/browser-mode.md) — the workflow steps and the rules of the JavaScript.
- [Security and Responsible Use](../guides/security-and-responsible-use.md) — the isolation of the contexts and the control of the network.
