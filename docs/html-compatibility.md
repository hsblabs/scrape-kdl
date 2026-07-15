# HTML compatibility

The versioned manifest is `fixtures/html-compat/manifest.json`. It pins parser mode, decoded encoding, input bytes, selector observations, text, inner HTML, attributes, normalization, source identity, and approved divergences. Both the Go DOM tests and the TypeScript `parse5` tests consume the same file; a difference inside this portable surface fails pull-request verification.

The pull-request corpus covers raw-text and entity handling, optional end tags and table construction, document order, selector combinators, attributes, `nth-*`, and `:not`. It contains no approved divergences.

The manifest also records the Web Platform Tests repository and exact revision used by the scheduled expansion. Issue #17 owns populating and running that broader pinned subset and Chromium differential observations. Updating the parser or upstream revision requires a reviewed manifest diff rather than implicit golden regeneration.

Run the local compatibility gates with:

```bash
GOTOOLCHAIN=local go test ./internal/dom -run TestPinnedHTMLCompatibilityManifest
npm run test:typescript
```

HTTP decoding occurs before parsing. The TypeScript runtime supports the same built-in labels as the Go runtime, honors BOM precedence, bounds meta-charset sniffing to the first 4 KiB, and reports unsupported or malformed encodings with stable execution codes.
