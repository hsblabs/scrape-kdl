# @hsblabs/scrape-kdl

Declarative HTML extraction for Node.js and Bun. Write an extractor in KDL,
compile it into a validated program, then run it over HTTP, over a saved HTML
snapshot, or through a browser adapter.

- ESM-only. Node.js 22+ / Bun 1.3+.
- Nothing hits the network until compilation succeeds.
- Deterministic diagnostics and Validated IR (`2026-07-15`), shared with the Go implementation.

Use `scrape-kdl` only where you are authorized to automate access. Read the
[responsible-use guidance](https://github.com/hsblabs/scrape-kdl/blob/main/docs/responsible-use.md)
before targeting a live service.

## Install

```bash
npm install @hsblabs/scrape-kdl
```

```bash
bun add @hsblabs/scrape-kdl
```

## Quick start

`extractor.kdl`:

```kdl
extractor "article" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/articles/{id}"
  }

  input "id" type="string" required=#true

  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
    apply "normalize-whitespace"
  }
}
```

```ts
import { compileFile } from "@hsblabs/scrape-kdl/node";

const { program, diagnostics } = await compileFile("extractor.kdl");
if (!program) throw new Error(JSON.stringify(diagnostics, null, 2));

const result = await program.extract({ id: "123" });
console.log(result.value); // { title: "..." }
```

`compile` never throws on invalid KDL. It returns diagnostics and no program,
so failures are inspectable instead of stringly-typed.

## Entry points

| Import | Use it for |
| --- | --- |
| `@hsblabs/scrape-kdl` | Compiler, runtime, diagnostics, IR, `BrowserAdapter` types. No ambient filesystem authority. |
| `@hsblabs/scrape-kdl/node` | `compileFile` / `validateFile` and relative-import loading from disk. |
| `@hsblabs/scrape-kdl/authoring` | Build KDL Source programmatically: `builtinCatalog`, `callBuiltin`, `write`. |

The root entry point reads no files by itself — supply a `loader` to resolve
imports however your host allows.

## Running without a network

`extractSnapshot` runs the portable output subset against HTML you already
have. No fetch, no navigation, no workflow, no JavaScript.

```ts
const result = await program.extractSnapshot(await readFile("page.html", "utf8"));
```

Useful for tests, golden fixtures, and re-processing archived pages.

## Execution options

```ts
await program.extract(
  { id: "123" },
  {
    requestTimeoutMs: 15_000,
    maxResponseBytes: 4 * 1024 * 1024,
    session: { headers: { authorization: "..." } },
    urlPolicy: (_context, url) => {
      if (url.hostname !== "example.invalid") throw new Error(`blocked: ${url}`);
    },
    signal: AbortSignal.timeout(30_000),
  },
);
```

`urlPolicy` is checked for the initial target, every redirect, and browser
navigation. Timeouts, response-size bounds, charset decoding, session scoping,
and cancellation are enforced by the runtime, not by the extractor author.

## Browser mode

Browser-mode programs need an adapter implementing the exported,
library-neutral `BrowserAdapter`. The core package depends on no browser
library. The official adapter is
[`@hsblabs/scrape-kdl-playwright`](https://www.npmjs.com/package/@hsblabs/scrape-kdl-playwright).

```ts
await program.extract(inputs, {
  browser: adapter,
  allowJavaScript: true, // only for a specification you trust
});
```

JavaScript execution is off unless you pass `allowJavaScript: true`. Adapters
may also expose an extraction-wide `BrowserAdapterLease` so a mutable page is
never shared concurrently.

## Authoring API

`@hsblabs/scrape-kdl/authoring` selects an exact language-version catalog,
builds a bounded Authoring Document, and writes deterministic KDL Source. That
source is then compiled normally. It is not mutable IR and not a lossless
formatter.

## Compatibility

The TypeScript implementation parses the complete documented KDL subset,
preserves UTF-8 byte spans, and lowers to the same frozen IR as the Go
implementation. The release-blocking conformance suite covers every shared
valid and invalid fixture — exact diagnostics, canonical IR, and HTTP /
offline-snapshot extraction parity. Static HTML execution uses the pinned
`parse5` runtime dependency.

## Documentation

- [Language spec](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/language-v0.1.md)
- [Public API contract](https://github.com/hsblabs/scrape-kdl/blob/main/docs/public-api-v1.md)
- [Browser runtime contract](https://github.com/hsblabs/scrape-kdl/blob/main/docs/browser-runtime.md)
- [Cookbook](https://github.com/hsblabs/scrape-kdl/blob/main/docs/cookbook.md)
- [Security model](https://github.com/hsblabs/scrape-kdl/blob/main/docs/security-model.md)

License: Apache-2.0.
