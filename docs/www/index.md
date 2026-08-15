---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Overview
title: Scraping KDL
description: A declarative language and runtime for HTML extraction. Write an extractor in KDL, let the compiler check it before any network operation, then execute it with HTTP or a browser adapter from Go, Node.js, or Bun.
hsblabs:
  sidebar:
    order: 0
---

Scraping KDL is a declarative language and a runtime for HTML extraction. You write an extractor in [KDL](https://kdl.dev/). The compiler resolves the imports, checks the types, and makes a language-neutral Validated IR. A runtime then executes the IR with HTTP or a live browser.

Scraping KDL is for developers who keep their extraction rules in source control. The compiler finds the errors before the runtime sends a request.

The reference implementation is available for Go, Node.js, and Bun. The license is Apache-2.0.

## How it operates

```text
KDL source
  -> parser
  -> semantic validation and type checking
  -> Validated IR
  -> HTTP runtime or browser adapter
  -> structured result
```

The compiler does not send a request, start a browser, or call an external transform. These operations occur only after the validation is correct. A bad selector, an unknown transform, or a wrong type becomes a diagnostic with a source location.

## What you write

```kdl
extractor "basic-http" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/{id}"
  }

  input "id" type="string" required=#true

  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
    apply "normalize-whitespace"
  }

  collection "items" min-items=1 {
    select "ul.items > li"
    field "value" type="u8" required=#true {
      select ".value" match="one"
      value "text"
      apply "trim"
      apply "parse-int" as="u8"
    }
  }
}
```

## What you get

```json
{
  "value": {
    "items": [{ "value": 1 }, { "value": 2 }, { "value": 3 }],
    "title": "Scraping KDL Runtime"
  },
  "warnings": [],
  "partial": false
}
```

The runtime checks each value against its declared type. The `partial` flag tells you if the runtime recovered an error. Thus you cannot mistake a degraded result for a correct result.

## Main properties

- **The compiler operates first.** It checks the selectors, the transform signatures, the capabilities, and the output types. No data leaves the process before this step.
- **The browser is a capability.** Browser mode needs an adapter that you supply. JavaScript stays off until you set the option.
- **One program, two runtimes.** Go and TypeScript accept the same documents. They give the same diagnostics and the same values.
- **Portable selectors.** A documented subset of CSS operates in the same manner in the internal DOM and in a live browser.
- **Offline execution.** A program without a workflow and without JavaScript can operate on a saved HTML file. No network operation occurs.

## Start here

- [Installation](./getting-started/installation.md) — the Go CLI, the Go modules, npm, and Bun.
- [Quick Start](./getting-started/quick-start.md) — validate, compile, and extract from a saved HTML file.
- [How It Operates](./guides/how-it-operates.md) — the compiler stages and their sequence.
- [Write an Extractor](./guides/write-an-extractor.md) — the structure of an extractor document.

Then select your runtime: [CLI](./cli/index.md), [TypeScript and Bun](./npm/index.md), or [Go](./golang/index.md).

## Before you use a live service

Scraping KDL is an extraction tool. It does not give you permission to access or to re-use content of other persons. Read [Security and Responsible Use](./guides/security-and-responsible-use.md) before you send a request to a service that you do not operate.

The project does not supply functions that bypass access controls, rate limits, or bot detection.
