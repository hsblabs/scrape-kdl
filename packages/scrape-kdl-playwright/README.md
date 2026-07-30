# @hsblabs/scrape-kdl-playwright

Official Playwright adapter for `@hsblabs/scrape-kdl`. It requires Node.js 26 or later and pins Playwright 1.61.1. Chromium is supported; Firefox and WebKit are exercised as non-blocking best-effort targets.

Release artifacts use the exact matching core candidate and are installed
together in a clean consumer before publication. Use the repository's
[responsible-use guidance](https://github.com/hsblabs/scrape-kdl/blob/main/docs/responsible-use.md)
before targeting a live service.

```ts
import { chromium } from "playwright";
import { compile } from "@hsblabs/scrape-kdl";
import { PlaywrightAdapter } from "@hsblabs/scrape-kdl-playwright";

const browser = await chromium.launch({ headless: true });
const adapter = new PlaywrightAdapter(browser);
try {
  const { program, diagnostics } = await compile(source);
  if (!program) throw new Error(JSON.stringify(diagnostics));
  const result = await program.extract(inputs, {
    browser: adapter,
    allowJavaScript: true, // only for trusted specifications
  });
  // use result
} finally {
  await adapter.close();
  await browser.close();
}
```

The adapter creates a fresh isolated `BrowserContext` for every navigation and implements the extraction-wide lease. Session headers, cookies, and user agent apply only to that context. Timeout or cancellation invalidates the context before the lease is released, so an interrupted Playwright operation cannot continue into the next extraction. `close()` closes adapter-owned contexts; the caller continues to own the `Browser`.

See `docs/playwright-adapter.md` for concrete behavior and `docs/browser-runtime.md` for the browser-library-neutral author contract.

License: Apache-2.0.
