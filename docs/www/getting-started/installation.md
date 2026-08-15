---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Installation
description: Install the Scraping KDL CLI, the Go modules, or the npm packages, and check that your Go, Node.js, Bun, and operating system versions are supported.
hsblabs:
  sidebar:
    order: 10
---

Install the CLI with `go install`. Install the libraries with `go get` or with `npm install`. The current release is `v1.0.4` for the Go modules and `1.0.4` for the npm packages. The two ecosystems have one release train and one version number.

## Supported versions

| Component | Supported |
| --- | --- |
| Go | 1.26 or later |
| Node.js | 22 or later |
| Bun | 1.3 or later, for `@hsblabs/scrape-kdl` |
| Operating systems | Linux and macOS |
| Release architectures | amd64 and arm64 |
| Playwright browser | Chromium. Firefox and WebKit have best-effort status. |

Windows is not supported. This is not a temporary condition. The project has no Windows CI jobs, no Windows release targets, and no Windows compatibility code. A successful compilation on Windows is accidental and is not a contract.

## Command line

The Go binary is the only CLI. The npm packages do not contain a CLI.

```bash
go install github.com/hsblabs/scrape-kdl/cmd/scrape-kdl@v1.0.4
```

For browser mode with go-rod, install the separate CLI of the adapter:

```bash
go install github.com/hsblabs/scrape-kdl/adapters/rod/cmd/scrape-kdl-rod@v1.0.4
```

To confirm the installation, use this command:

```bash
scrape-kdl version
```

## Go modules

```bash
go get github.com/hsblabs/scrape-kdl@v1.0.4
```

The go-rod adapter is a **separate module**. It is not a package in the core module.

```bash
go get github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.4
```

The adapter depends on the core module. The core module never imports a browser library. Thus an HTTP-only application does not get Chromium, CDP, or go-rod in its dependency graph. For more data, refer to [Go](../golang/index.md).

## Node.js

```bash
npm install @hsblabs/scrape-kdl@1.0.4
```

The core package supports only ESM. It has three entry points:

- `@hsblabs/scrape-kdl` — the compiler, the diagnostics, the IR, the HTTP runtime, the offline snapshot runtime, and the browser adapter types;
- `@hsblabs/scrape-kdl/node` — `compileFile` and `validateFile`. These functions stay outside the core package. Thus the core package gets no automatic access to the file system.
- `@hsblabs/scrape-kdl/authoring` — the bounded authoring model and the catalog of the built-in transforms.

The official Playwright adapter is a separate package:

```bash
npm install @hsblabs/scrape-kdl-playwright@1.0.4 playwright
npx playwright install chromium
```

For more data, refer to [Playwright Adapter](../npm/playwright.md).

## Bun

```bash
bun add @hsblabs/scrape-kdl@1.0.4
```

Bun 1.3 or later supports the core package. The tests of the Playwright adapter use Node.js 22 or later.

## Next step

Continue with the [Quick Start](./quick-start.md). It executes an extractor against a saved HTML file and sends no request.
