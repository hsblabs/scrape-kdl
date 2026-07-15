# Partial result

This example recovers a field failure with `on-error "warn"`, skips one invalid collection row, and returns a deterministic warning list with `partial: true`.

```bash
go run ./cmd/scrape-kdl extract ./examples/partial-result/extractor.kdl \
  --html ./examples/partial-result/page.html
```

Use `make examples` to compare the warning paths, skipped row index, values, and reviewed IR.
