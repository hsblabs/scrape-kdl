# Malformed HTML

This example keeps omitted `</li>` tags and a truncated document in source control. The expected output records the observable portable extraction result rather than an implementation-specific DOM dump.

```bash
go run ./cmd/scrape-kdl extract ./examples/malformed-html/extractor.kdl \
  --html ./examples/malformed-html/page.html
```

`make examples` compiles the source and compares both checked-in goldens without rewriting them.
