# HTML compatibility

The Go HTTP runtime uses the pinned `golang.org/x/net/html` dependency for WHATWG tree construction after bounded response bytes have been decoded to UTF-8. The dependency is internal and does not appear in the public Go API.

The versioned manifest is `fixtures/html-compat/manifest.json`. It pins parser mode, decoded encoding, input bytes, selector observations, text, inner HTML, attributes, normalization, source identity, and approved divergences. Both the Go DOM tests and the TypeScript `parse5` tests consume the same file; a difference inside this portable surface fails pull-request verification.

The pull-request corpus covers foster parenting, active formatting elements, foreign content integration, raw text, RCDATA, entities, optional end tags, truncated input, document order, portable selector combinators, attributes, `nth-*`, and `:not`. It contains no approved divergences. Missing attributes are observed as `null`; element and result arrays retain document order; text is returned decoded without whitespace normalization; inner HTML uses deterministic attribute ordering.

The manifest also records the Web Platform Tests repository and exact revision used by the scheduled expansion. Issue #17 owns populating and running that broader pinned subset and Chromium differential observations. Updating `x/net/html`, the upstream revision, or any expected observation requires a reviewed compatibility diff rather than implicit golden regeneration.

Run the local compatibility gates with:

```bash
GOTOOLCHAIN=local go test ./internal/dom -run TestPinnedHTMLCompatibilityManifest
npm run test:typescript
```

HTTP decoding occurs before parsing. The TypeScript runtime supports the same built-in labels as the Go runtime, honors BOM precedence, bounds meta-charset sniffing to the first 4 KiB, and reports unsupported or malformed encodings with stable execution codes.

Static HTTP extraction does not include JavaScript mutations or other live-browser state. Namespace-sensitive selectors, fragment parsing, scripting-enabled parser state, and DOM APIs outside the documented portable selector and extraction surface are not promoted by this corpus.
