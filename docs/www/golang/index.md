---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Go
description: The Go modules of Scraping KDL — the core module, the separate go-rod adapter module, the reason the core never imports a browser library, and the first compilation.
hsblabs:
  sidebar:
    order: 50
---

The core module gives you the compiler, the diagnostics, the IR, the HTTP runtime, the offline snapshot runtime, and the interface of the browser adapter. It needs Go 1.26 or later.

```bash
go get github.com/hsblabs/scrape-kdl@v1.0.4
```

## The two modules

The go-rod adapter is a **separate module**. It is not a package inside the core module.

```bash
go get github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.4
```

The core module must not import go-rod. This is an invariant of the project, not a preference. A browser library belongs in an adapter.

Thus an application with HTTP only does not get Chromium, CDP, or go-rod in its dependency graph. Add the adapter module only when you need browser mode. Refer to [go-rod Adapter](./rod.md).

## The first compilation

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

The compilation gives three values: a program, the ordered diagnostics, and an operational error. Examine `diagnostics.HasErrors()` before you use the program.

An operational error is different from a diagnostic. A cancellation, a failure of the file system, and a failure of an injected loader are operational errors. The functions keep the cause for `errors.Is` and `errors.As`. Refer to [Compile and Extract in Go](./compile-and-extract.md).

## The functions of the compilation

| Function | Source |
| --- | --- |
| `Compile(ctx, Source, CompileOptions)` | A source in the memory. |
| `CompileFile(ctx, path)` | A file of the operating system. |
| `CompileFS(ctx, fsys, path)` | An `fs.FS` of your application. |

`Validate`, `ValidateFile`, and `ValidateFS` are the equivalents that give only the diagnostics.

The functions `CompileFS` and `ValidateFS` limit the root and each import to a valid path of `io/fs` and reject a lexical escape to a parent. Your `fs.FS` still defines the true authority. `os.DirFS` can follow a symbolic link outside of its directory. Use `os.Root.FS` when you need containment.

## The metadata of a program

```go
metadata := program.Metadata()
metadata.Capabilities  // the exact capabilities that the program needs
metadata.LanguageVersion
metadata.IRVersion
metadata.Files         // each source file, with its SHA-256

descriptor := program.Descriptor()
descriptor.Source.FetchMode      // "http" or "browser"
descriptor.Source.SessionPolicy  // "none", "optional", or "required"
```

A program is immutable and has a reusable execution plan. Use the capabilities to permit or to refuse a program in your host. Use the descriptor for a decision about the acquisition, without a decode of the full IR.

`Program.IRJSON()` gives the full Validated IR for an interchange or a tool.

## The concurrency

A program is safe for a concurrent extraction, if your adapters obey the documented contract of the ownership. The mutable state stays inside one extraction.

A browser adapter that controls one page is the exception. It must implement `BrowserAdapterLease`, and the lease then prevents an interleaved operation. Use more than one page or more than one adapter for a true parallelism.

## The command line

The core module also has the CLI:

```bash
go install github.com/hsblabs/scrape-kdl/cmd/scrape-kdl@v1.0.4
```

Refer to [CLI](../cli/index.md).

## Next step

- [Compile and Extract in Go](./compile-and-extract.md) — the full API of the execution.
- [go-rod Adapter](./rod.md) — browser mode with an official adapter.
