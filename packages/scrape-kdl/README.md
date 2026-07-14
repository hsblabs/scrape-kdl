# @hsblabs/scrape-kdl

ESM-only TypeScript implementation of the dated Scraping KDL contract for Node.js 26 and later.

```ts
import { compile, validate } from "@hsblabs/scrape-kdl";
import { compileFile } from "@hsblabs/scrape-kdl/node";
```

The root entry point has no ambient filesystem authority.
Use `@hsblabs/scrape-kdl/node` for filesystem conveniences.
Browser integrations implement the exported `BrowserAdapter` interface and remain outside this core package.

The implementation parses the complete documented KDL subset, preserves deterministic UTF-8 byte spans, and resolves extractor/module import graphs through an injected loader. It performs semantic validation, type checking, reference and capability resolution, and lowering to the frozen `2026-07-15` IR without Go or subprocess execution.

The release-blocking TypeScript conformance suite covers every shared valid and invalid compiler fixture, including exact diagnostics and canonical IR parity. Issue #14 completes HTTP execution.

The package is not published by this pull request.
Local `npm pack` plus a clean-consumer smoke test verifies the future artifact.

License: Apache-2.0.
