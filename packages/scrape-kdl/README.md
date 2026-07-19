# @hsblabs/scrape-kdl

ESM-only TypeScript implementation of the dated Scraping KDL contract for Node.js 26 and later.

```ts
import { compile, validate } from "@hsblabs/scrape-kdl";
import { compileFile } from "@hsblabs/scrape-kdl/node";
```

The root entry point has no ambient filesystem authority.
Use `@hsblabs/scrape-kdl/node` for filesystem conveniences.
Browser integrations implement the exported `BrowserAdapter` interface and remain outside this core package. Static HTML execution uses the pinned `parse5` runtime dependency.

The implementation parses the complete documented KDL subset, preserves deterministic UTF-8 byte spans, and resolves extractor/module import graphs through an injected loader. It performs semantic validation, type checking, reference and capability resolution, and lowering to the frozen `2026-07-15` IR without Go or subprocess execution.

The release-blocking TypeScript conformance suite covers every shared valid and invalid compiler fixture, including exact diagnostics, canonical IR, and HTTP/offline-HTML extraction parity. `Program.extract` enforces input and external-transform preflight before acquisition, URL policy on initial and redirect targets, bounded response reads, timeouts, cancellation, session scoping, charset decoding, recovery, and JSON-compatible results.

Browser-mode programs use the exported browser-library-neutral `BrowserAdapter` and optional extraction-wide `BrowserAdapterLease`. JavaScript is disabled unless `allowJavaScript: true` is supplied for a trusted specification. The official implementation is `@hsblabs/scrape-kdl-playwright`; the core package does not depend on Playwright.

The workspace manifest remains unpublished at `0.0.0-development` until the project-owner release gate. `make release-dist VERSION=vX.Y.Z OUT=dist` creates release-versioned archives in temporary staging and verifies them with a clean-consumer smoke test without publishing anything.

License: Apache-2.0.
