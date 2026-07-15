# Basic HTTP

This example extracts a normalized title and a numeric collection. The acceptance runner serves `page.html` from a local HTTP server and rewrites requests inside an isolated test transport, so it never contacts `example.invalid`.

From the repository root, run the saved page offline:

```bash
go run ./cmd/scrape-kdl extract ./examples/basic-http/extractor.kdl \
  --input id=fixture --html ./examples/basic-http/page.html
```

Run every executable example and compare its reviewed IR and output:

```bash
make examples
```
