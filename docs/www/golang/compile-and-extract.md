---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Compile and Extract in Go
description: The Go execution API — the compile options, the injected loaders, the execution options, the three extraction entry points, the URL policy, and the typed decode of a result.
hsblabs:
  sidebar:
    order: 51
---

This page shows you the API of the compilation and the execution in the module `github.com/hsblabs/scrape-kdl`.

## Compile

```go
source := scrapekdl.Source{
    Path: "extractor.kdl",
    Data: data,
}

program, diagnostics, err := scrapekdl.Compile(ctx, source, scrapekdl.CompileOptions{})
if err != nil {
    return err
}
if diagnostics.HasErrors() {
    return fmt.Errorf("compilation failed: %v", diagnostics)
}
```

The field `Path` is a logical identity for the diagnostics, the source locations, the resolution of the imports, and the file identities in the IR. It does not have to name a file of the operating system.

The type `Diagnostics` has a deterministic order. It contains a warning also. Use `HasErrors()` to find an error.

## The imports

A source with an import needs a loader:

```go
options := scrapekdl.CompileOptions{
    Loader: func(ctx context.Context, path string) ([]byte, error) {
        if !allowed[path] {
            return nil, fmt.Errorf("refused: %s", path)
        }
        return os.ReadFile(path)
    },
}
```

The compiler resolves each import lexically, relative to the source that imports it, before it calls your loader. Your loader gives only the bytes. The compiler makes the parse, the validation, the detection of the cycles, the hash, and the deterministic order.

A compilation of a source with an import and without a loader fails before it gives an IR. Your loader is an authority boundary: limit the paths, obey the cancellation, and do not put the content of a source or a credential in an error.

For a file system of your application, use `CompileFS(ctx, fsys, path)`. It resolves each nested import inside the same file system and rejects a lexical escape to a parent. It examines the cancellation before and after each `fs.ReadFile`. The interface `fs.FS` cannot interrupt a read that is in progress.

## The three entry points of the extraction

```go
result, err := program.Extract(ctx, inputs, options)
result, err := program.ExtractHTML(ctx, html, options)
result, err := program.ExtractSnapshot(ctx, html, options)
```

| Method | Acquisition | Accepted modes |
| --- | --- | --- |
| `Extract` | Follows the mode of the source. | Each mode. A browser-mode program needs `Options.Browser`. |
| `ExtractHTML` | None. | HTTP mode only. |
| `ExtractSnapshot` | None. | Each mode, if the program is eligible for a snapshot. |

Refer to [Offline Snapshots](../guides/offline-snapshots.md).

## The execution options

```go
type Options struct {
    Browser            BrowserAdapter
    AllowJavaScript    bool
    HTTPClient         *http.Client
    Session            *Session
    ExternalTransforms map[string]ExternalTransform
    CharsetDecoder     CharsetDecoder
    RequestTimeout     time.Duration
    MaxResponseBytes   int64
    UserAgent          string
    URLPolicy          URLPolicy
}
```

Each `Options` value configures one extraction. The mutable state stays inside that extraction. Thus you can execute one immutable program more than one time, at the same time, with different options.

The field `AllowJavaScript` is off by default. A program with an `evaluate-js` node then fails with `E_JAVASCRIPT_DISABLED` before the navigation.

## The result

```go
type Result struct {
    Value    map[string]any `json:"value"`
    Warnings []Warning      `json:"warnings"`
    Partial  bool           `json:"partial"`
}
```

Use `Decode` for a typed value:

```go
var output struct {
    Title string `json:"title"`
    Items []struct {
        Value uint8 `json:"value"`
    } `json:"items"`
}
if err := result.Decode(&output); err != nil {
    return err
}
```

A `Warning` has a `Code`, a `Message`, an optional `Path`, and an optional `Row`. The flag `Partial` is `true` only after the runtime recovered an error or dropped a row.

A failure gives an `*ExecutionError` with a `Code`, a `Message`, a `Path`, and a `Cause`:

```go
var execErr *scrapekdl.ExecutionError
if errors.As(err, &execErr) && execErr.Code == "E_REQUIRED_VALUE_MISSING" {
    // handle the missing value
}
```

Examine the `Code`. The codes are stable. The messages are not. Refer to [Diagnostics](../guides/diagnostics.md).

## The URL policy

```go
options := scrapekdl.Options{
    URLPolicy:  scrapekdl.PublicInternetURLPolicy(),
    HTTPClient: scrapekdl.NewPublicInternetHTTPClient(),
}
```

Use the two together. The policy examines the initial target and each redirect. The guarded client resolves the address again at connection time and examines it again. Thus DNS rebinding cannot defeat the check.

The guarded client makes a direct connection and does not use the proxy settings of the environment, because a proxy resolves the target itself and the client then cannot examine the selected address.

The library applies no policy until you configure one. The CLI applies the two by default. Refer to [HTTP Execution](../guides/http-execution.md).

## The external transforms

```go
options := scrapekdl.Options{
    ExternalTransforms: map[string]scrapekdl.ExternalTransform{
        "decrypt_payload": func(ctx context.Context, input any) (any, error) {
            return decrypt(input)
        },
    },
}
```

If the registry does not have a symbol that the program needs, the validation fails before the fetch. After your function gives a result, the runtime immediately examines the result against the declared output type.

## The cancellation

The runtime propagates the context to the HTTP request, to the operations of the adapter, and to the traversal of the output. It examines the context before the parse of the HTML in the memory, and also before each output member and each collection row.

A cancellation gives `E_EXECUTION_CANCELED` and keeps `context.Canceled` or `context.DeadlineExceeded` as the cause. A field policy or a row policy cannot recover it.

## The compatibility

```go
scrapekdl.SupportedLanguageVersions()
scrapekdl.SupportedIRVersions()
```

The two functions give the exact versions that this build accepts.

## Next step

- [go-rod Adapter](./rod.md) — how to execute a browser-mode program.
- [Patterns](../guides/patterns.md) — a loop over more than one page.
