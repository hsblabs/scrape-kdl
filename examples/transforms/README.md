# Transforms

This example composes portable built-ins inside a declared pipeline and dispatches a second declared transform through `match`.

```bash
go run ./cmd/scrape-kdl extract ./examples/transforms/extractor.kdl \
  --html ./examples/transforms/page.html
```

The reviewed IR shows resolved transform symbols and call types; `make examples` checks it together with the extraction result.
