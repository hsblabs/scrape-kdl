# scrape-kdl

[![Go Reference](https://pkg.go.dev/badge/github.com/hsblabs/scrape-kdl.svg)](https://pkg.go.dev/github.com/hsblabs/scrape-kdl)
[![Go Reference: go-rod adapter](https://pkg.go.dev/badge/github.com/hsblabs/scrape-kdl/adapters/rod.svg)](https://pkg.go.dev/github.com/hsblabs/scrape-kdl/adapters/rod)
[![npm: @hsblabs/scrape-kdl](https://img.shields.io/npm/v/%40hsblabs%2Fscrape-kdl?logo=npm)](https://www.npmjs.com/package/@hsblabs/scrape-kdl)
[![npm: @hsblabs/scrape-kdl-playwright](https://img.shields.io/npm/v/%40hsblabs%2Fscrape-kdl-playwright?logo=npm)](https://www.npmjs.com/package/@hsblabs/scrape-kdl-playwright)

Declarative HTML extraction for Go, Node.js, and Bun. Write an extractor in
KDL, validate it before any network activity, and run it through HTTP or a
browser adapter.

Use `scrape-kdl` only where you are authorized to automate access. Read the
[responsible-use guidance](docs/responsible-use.md) before targeting a live
service.

## How it works

```text
KDL source
  -> parser
  -> semantic validation and type checking
  -> Validated IR
  -> HTTP runtime or browser adapter
  -> structured result
```

The same validated program can be executed by the Go and TypeScript runtimes.
The browser contract is library-neutral, so browser integrations stay outside
the core package.

## Highlights

- Declarative extractor and transform documents with relative imports.
- Deterministic diagnostics and Validated IR JSON.
- HTTP, sessions, response limits, charset decoding, and offline HTML snapshots.
- Portable CSS selectors with shared Go/TypeScript conformance checks.
- Explicit browser capability and opt-in JavaScript execution.
- Official adapters for Playwright and go-rod.
- CLI, Go API, TypeScript API, and bounded authoring APIs.

## Install v1.0.4

### Go CLI and go-rod CLI

```bash
go install github.com/hsblabs/scrape-kdl/cmd/scrape-kdl@v1.0.4
go install github.com/hsblabs/scrape-kdl/adapters/rod/cmd/scrape-kdl-rod@v1.0.4
```

### Go modules

```bash
go get github.com/hsblabs/scrape-kdl@v1.0.4
go get github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.4
```

The core module and the go-rod adapter are separate Go modules. The adapter
depends on the core module but the core module never imports a browser library.

### TypeScript and Bun

```bash
npm install @hsblabs/scrape-kdl@1.0.4 @hsblabs/scrape-kdl-playwright@1.0.4
```

The core package is ESM-only and supports Node.js 22 or later and Bun 1.3 or
later. The Playwright package is a separate adapter and is validated on
Node.js 22 or later.

## Quick start

Create `extractor.kdl`:

```kdl
extractor "basic-http" version="2026-07-15" language-version="2026-07-15" {
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

Validate and compile it:

```bash
scrape-kdl validate ./extractor.kdl
scrape-kdl compile ./extractor.kdl --emit-ir
```

For a saved HTML document, use the same extractor without making a network
request:

```bash
scrape-kdl extract ./extractor.kdl --html ./page.html
```

For HTTP execution, provide inputs and an optional JSON session file:

```bash
scrape-kdl extract ./extractor.kdl \
  --input id=123 \
  --session-file ./session.json
```

Session files contain headers and cookies. Protect them as secrets. Use
`--session-file -` to read the JSON document from standard input.

## Go API

```go
package main

import (
    "context"
    "time"

    scrapekdl "github.com/hsblabs/scrape-kdl"
)

func main() {
    ctx := context.Background()
    program, diagnostics, err := scrapekdl.CompileFile(ctx, "extractor.kdl")
    if err != nil {
        panic(err)
    }
    if diagnostics.HasErrors() {
        panic(diagnostics)
    }

    result, err := program.Extract(ctx, map[string]any{"id": "123"}, scrapekdl.Options{
        RequestTimeout: 15 * time.Second,
    })
    if err != nil {
        panic(err)
    }

    var output struct {
        Title string `json:"title"`
    }
    if err := result.Decode(&output); err != nil {
        panic(err)
    }
}
```

Compilation returns deterministic diagnostics. When compilation succeeds,
`Program.Extract` returns a structured result that can be decoded into typed Go
values. Use `URLPolicy` when the host application needs to restrict targets.

See [`docs/public-api-v1.md`](docs/public-api-v1.md) for the complete API
contract and intentional differences between the Go and TypeScript APIs.

## TypeScript API

```ts
import { readFile } from "node:fs/promises";
import { compile } from "@hsblabs/scrape-kdl";

const compiled = await compile({
  path: "extractor.kdl",
  data: await readFile("extractor.kdl", "utf8"),
});

if (!compiled.program) {
  throw new Error(JSON.stringify(compiled.diagnostics));
}

const result = await compiled.program.extract({ id: "123" });
console.log(result.value);
```

The core package exposes the compiler, diagnostics, IR, HTTP runtime, offline
snapshot runtime, and browser-neutral adapter types. Filesystem helpers are
available from `@hsblabs/scrape-kdl/node`, and bounded semantic authoring is
available from `@hsblabs/scrape-kdl/authoring`.

With Bun, install the core package with:

```bash
bun add @hsblabs/scrape-kdl@1.0.4
```

## Browser mode

Browser execution requires an application-supplied adapter. JavaScript is
disabled unless the host explicitly enables it for a trusted specification.

### Playwright

```bash
npm install @hsblabs/scrape-kdl-playwright@1.0.4 playwright
npx playwright install chromium
```

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

### go-rod

```bash
go get github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.4
```

See [`docs/browser-runtime.md`](docs/browser-runtime.md),
[`docs/playwright-adapter.md`](docs/playwright-adapter.md), and
[`docs/rod-adapter.md`](docs/rod-adapter.md) for adapter contracts and
lifecycle details.

## Responsible use and security

KDL is executable configuration. A specification containing `evaluate-js`
executes trusted JavaScript in the target page context. Never enable it for
untrusted specifications.

Respect the target service's terms and `robots.txt`, use reasonable request
rates and concurrency, identify the client honestly, and protect sessions and
extracted data. `scrape-kdl` does not support anti-bot or access-control
circumvention.

See [`SECURITY.md`](SECURITY.md) and [`docs/security-model.md`](docs/security-model.md).

## Compatibility

| Component | Supported versions |
| --- | --- |
| Go | 1.26 or later |
| Node.js | 22 or later |
| Bun | 1.3 or later for `@hsblabs/scrape-kdl` |
| Operating systems | Linux and macOS |
| Release architectures | amd64 and arm64 |
| Playwright browser | Chromium; Firefox and WebKit are best-effort |

Windows is explicitly unsupported. The language implements the subset defined
by the Scraping KDL v0.1 specification documents and uses the dated language
and IR identifiers `2026-07-15`.

## Development

```bash
npm ci
make verify
make test-rod
```

Run Chromium-backed tests with:

```bash
make test-rod-e2e
```

Run the TypeScript clean-consumer and package checks with:

```bash
npm run verify:typescript
```

Before a release, run:

```bash
make release-check
```

## Documentation

- [Language specification](docs/spec/language-v0.1.md)
- [CLI contract](docs/cli.md)
- [Authoring guide](docs/authoring.md)
- [HTTP runtime](docs/http-runtime.md)
- [Browser runtime](docs/browser-runtime.md)
- [Versioning](docs/versioning.md)
- [Releasing](docs/releasing.md)
- [Migration guide](docs/migrating-to-v1.md)

## License

Apache-2.0. See [`LICENSE`](LICENSE) and [`NOTICE`](NOTICE).
