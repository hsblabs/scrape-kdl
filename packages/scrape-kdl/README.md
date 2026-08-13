# @hsblabs/scrape-kdl

ESM-only TypeScript implementation of the dated Scraping KDL contract for Node.js 22 or later and Bun 1.3 or later.

```ts
import { compile, validate } from "@hsblabs/scrape-kdl";
import { builtinCatalog, callBuiltin, write } from "@hsblabs/scrape-kdl/authoring";
import { compileFile } from "@hsblabs/scrape-kdl/node";
```

The root entry point has no ambient filesystem authority.
Use `@hsblabs/scrape-kdl/node` for filesystem conveniences.
Use `@hsblabs/scrape-kdl/authoring` to select an exact language-version catalog,
construct the bounded Authoring Document, and write deterministic KDL Source.
Compile that source normally to obtain diagnostics and Validated IR; the
authoring model is not mutable IR or a lossless formatter.
Browser integrations implement the exported `BrowserAdapter` interface and remain outside this core package. Static HTML execution uses the pinned `parse5` runtime dependency.

The implementation parses the complete documented KDL subset, preserves deterministic UTF-8 byte spans, and resolves extractor/module import graphs through an injected loader. It performs semantic validation, type checking, reference and capability resolution, and lowering to the frozen `2026-07-15` IR without Go or subprocess execution.

The release-blocking TypeScript conformance suite covers every shared valid and invalid compiler fixture, including exact diagnostics, canonical IR, and HTTP/offline-snapshot extraction parity. `Program.extract` enforces input and external-transform preflight before acquisition, URL policy on initial and redirect targets, bounded response reads, timeouts, cancellation, session scoping, charset decoding, recovery, and JSON-compatible results. `Program.extractSnapshot` executes the portable output subset against supplied HTML without fetch, navigation, workflow, or JavaScript.

Browser-mode programs use the exported browser-library-neutral `BrowserAdapter` and optional extraction-wide `BrowserAdapterLease`. JavaScript is disabled unless `allowJavaScript: true` is supplied for a trusted specification. The official implementation is `@hsblabs/scrape-kdl-playwright`; the core package does not depend on Playwright.

Release artifacts are versioned in isolated staging, checked for local
workspace dependencies, and installed into a clean consumer before publication.
Use the repository's
[responsible-use guidance](https://github.com/hsblabs/scrape-kdl/blob/main/docs/responsible-use.md)
before targeting a live service.

License: Apache-2.0.
