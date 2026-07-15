# Official Playwright adapter

`@hsblabs/scrape-kdl-playwright` implements the TypeScript `BrowserAdapter` and `BrowserAdapterLease` contracts without adding Playwright to the core package. The adapter accepts a caller-owned Playwright `Browser`; it owns only the isolated contexts it creates.

Each `navigate` call closes the prior adapter context, creates a fresh context, installs the explicit session headers, cookies, and user agent, and navigates a new page. This prevents cookies, storage, page mutations, or failed work from one extraction from entering the next extraction. Closing the adapter never closes the caller's browser.

The adapter maps browser operations as follows:

- portable selectors use Playwright locators in document or current-element scope;
- text reads use descendant `textContent`, HTML reads use `innerHTML`, and attributes use DOM attribute values;
- workflow wait, click, fill, press, scroll, and configured network-idle operations run in source order;
- document JavaScript calls the source function without arguments; current JavaScript passes the current DOM element;
- JavaScript results cross the adapter boundary and are then checked by the core runtime for JSON compatibility and the declared `returns` type.

The adapter races operations against the public timeout and `AbortSignal`. For operations that could continue in the browser, timeout or cancellation first closes the isolated context. The runtime holds the adapter lease until this cleanup finishes. A later extraction creates a new context and can proceed normally.

Chromium is the blocking v1 target. The scheduled browser workflow also reports Firefox and WebKit results with non-blocking status. A support promotion requires a separate reviewed compatibility decision.
