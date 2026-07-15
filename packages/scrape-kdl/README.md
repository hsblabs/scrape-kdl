# @hsblabs/scrape-kdl

ESM-only TypeScript implementation of the dated Scraping KDL contract for Node.js 26 and later.

```ts
import { compile, validate } from "@hsblabs/scrape-kdl";
import { compileFile } from "@hsblabs/scrape-kdl/node";
```

The root entry point has no ambient filesystem authority.
Use `@hsblabs/scrape-kdl/node` for filesystem conveniences.
Browser integrations implement the exported `BrowserAdapter` interface and remain outside this core package.

The current v0.3 scaffold preserves the independently tested v0.2 parser/compiler slice.
It compiles the representative `basic-http` fixture and the shared dated-version invalid fixtures without Go or subprocess execution.
Issues #12 and #13 complete parsing, loading, imports, semantics, and IR lowering; Issue #14 completes HTTP execution.

The package is not published by this pull request.
Local `npm pack` plus a clean-consumer smoke test verifies the future artifact.

License: Apache-2.0.
