# @hsblabs/scrape-kdl

ESM-only TypeScript implementation of the dated Scraping KDL contract for Node.js 26 and later.

```ts
import { compile, validate } from "@hsblabs/scrape-kdl";
import { compileFile } from "@hsblabs/scrape-kdl/node";
```

The root entry point has no ambient filesystem authority.
Use `@hsblabs/scrape-kdl/node` for filesystem conveniences.
Browser integrations implement the exported `BrowserAdapter` interface and remain outside this core package.

The v0.3 implementation parses the complete documented KDL subset, preserves deterministic UTF-8 byte spans, and resolves extractor/module import graphs through an injected loader. Shared Go/TypeScript cases cover raw and quoted strings, numbers, comments, slashdash, unsupported representations, invalid UTF-8, loader failures, aliases, document kinds, and cycles without Go or subprocess execution.

Semantic validation and IR lowering currently retain the representative `basic-http` and dated-version slice. Issue #13 completes semantic and diagnostic parity; Issue #14 completes HTTP execution.

The package is not published by this pull request.
Local `npm pack` plus a clean-consumer smoke test verifies the future artifact.

License: Apache-2.0.
