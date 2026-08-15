---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Quick Start
description: Write your first Scraping KDL extractor, then run validate, compile, and extract against a saved HTML file. This procedure sends no network request.
hsblabs:
  sidebar:
    order: 11
---

This procedure starts with an empty directory and ends with a structured result. It uses only the CLI. It sends **no network request**, because the extractor operates on a saved HTML file. Test each new extractor in this manner before you point it at a live service.

You must have the `scrape-kdl` binary. Refer to [Installation](./installation.md).

## 1. Save an HTML file

Save this text as `page.html`. The spaces in the heading are intentional. They show you the function of a transform.

```html
<!doctype html>
<html>
  <body>
    <h1>  Scraping   KDL Runtime  </h1>
    <ul class="items">
      <li><span class="value">1</span></li>
      <li><span class="value">2</span></li>
      <li><span class="value">3</span></li>
    </ul>
  </body>
</html>
```

## 2. Write the extractor

Save this text as `extractor.kdl`.

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

Read the document from the top to the bottom:

- `version` identifies this revision of the document. `language-version` selects the language contract. Its value must be `2026-07-15`. Both properties are necessary.
- `source` declares the method to get the document. The value `mode="http"` selects the static HTTP runtime. The runtime puts a declared input in the position of `{id}`.
- The node `field "title"` selects one `h1`, reads its text, and removes the unwanted spaces.
- The node `collection "items"` makes one row from each `li` that agrees with the selector. It also requires a minimum of one row.
- The property `type="u8"` is a true constraint. The runtime parses the text into an 8-bit unsigned integer. A value that is too large causes an extraction error. The runtime does not truncate the value.

## 3. Validate

```bash
scrape-kdl validate ./extractor.kdl
```

```text
valid: ./extractor.kdl
```

Validation is analysis only. It parses the document, resolves the symbols, checks the types, and calculates the capabilities. It does not open a socket. The exit status is `0` for a correct document and `1` when the diagnostics contain an error.

Now cause an error. Change the selector of the title to `h1:has(a)` and validate the document again:

```text
extractor.kdl:9:5: error E_SELECTOR_UNSUPPORTED: selector byte 9: unsupported pseudo-class "has" [output.title.selection]
```

The pseudo-class `:has()` is outside of the portable selector profile. Thus the compiler rejects it and gives you a source location and an output path. You get this error immediately, not at the twentieth page of a crawl. Change the selector to `h1` again before you continue. Refer to [Diagnostics](../guides/diagnostics.md).

## 4. Compile

```bash
scrape-kdl compile ./extractor.kdl --out ./extractor.ir.json
```

```text
wrote: ./extractor.ir.json
```

The Validated IR is the language-neutral contract between the compiler and each runtime. If you do not give `--out`, the CLI writes the IR to the standard output. Examine these fields first:

```json
{
  "irVersion": "2026-07-15",
  "languageVersion": "2026-07-15",
  "capabilities": ["http.fetch"]
}
```

The array `capabilities` contains the exact set of the capabilities that this program needs. This program only fetches with HTTP. If you add a browser workflow or an `evaluate-js` field, the set becomes larger. Thus a host can decide what to permit before it executes the program. Refer to [How It Operates](../guides/how-it-operates.md).

## 5. Extract from the saved HTML

```bash
scrape-kdl extract ./extractor.kdl --html ./page.html
```

```json
{
  "value": {
    "items": [
      {
        "value": 1
      },
      {
        "value": 2
      },
      {
        "value": 3
      }
    ],
    "title": "Scraping KDL Runtime"
  },
  "warnings": [],
  "partial": false
}
```

The heading has no unwanted spaces. The item values are numbers, not strings. The flag `partial: false` shows you that the runtime recovered no error.

The option `--html` does no acquisition. There is no URL expansion, no URL policy, no session, and no HTTP request. Thus you do not have to supply the necessary `id` input. Refer to [Offline Snapshots](../guides/offline-snapshots.md).

## 6. Get a machine-readable envelope

For a script, the option `--json` puts the result in an envelope with an explicit success flag:

```bash
scrape-kdl extract ./extractor.kdl --html ./page.html --json
```

```json
{
  "ok": true,
  "result": {
    "value": {
      "items": [
        {
          "value": 1
        },
        {
          "value": 2
        },
        {
          "value": 3
        }
      ],
      "title": "Scraping KDL Runtime"
    },
    "warnings": [],
    "partial": false
  }
}
```

In an automated procedure, examine `ok` and also the exit status of the process. Refer to [CLI](../cli/index.md).

## Execute against a live URL

The same extractor operates on a live URL. Supply the declared input in the place of `--html`:

```bash
scrape-kdl extract ./extractor.kdl --input id=123
```

Read [Security and Responsible Use](../guides/security-and-responsible-use.md) before you do this against a real service. By default the CLI rejects a target that is not globally accessible. It accepts a session only from `--session-file`. Thus your credentials do not go into the history of your shell.

## Next step

- [Write an Extractor](../guides/write-an-extractor.md) — the fields, the collections, the inputs, and the error policy.
- [HTTP Execution](../guides/http-execution.md) — the sessions, the redirects, the limits, and the URL policy.
- [TypeScript and Bun](../npm/index.md) or [Go](../golang/index.md) — execute the same program from a library.
