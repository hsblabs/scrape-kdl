# HTML compatibility

The Go HTTP runtime uses the pinned `golang.org/x/net/html` dependency for WHATWG tree construction after bounded response bytes have been decoded to UTF-8. The dependency is internal and does not appear in the public Go API.

The versioned manifest is `fixtures/html-compat/manifest.json`. It pins parser mode, decoded encoding, input bytes, selector observations, text, deterministic inner HTML, attributes, normalization, source identity, and approved divergences. Both the Go DOM tests and TypeScript `parse5` tests consume it; a difference inside this portable surface fails pull-request verification.

The pull-request corpus covers foster parenting, active formatting elements, foreign content integration, raw text, RCDATA, entities, optional end tags, truncated input, document order, portable selector combinators, attributes, `nth-*`, and `:not`. It contains no approved divergences. Missing attributes are observed as `null`; element and result arrays retain document order; text is returned decoded without whitespace normalization; inner HTML uses deterministic attribute ordering.

The manifest also records the Web Platform Tests repository, exact revision, and three selected document-parsing reductions for ambiguous ampersands, SVG foreign content, and HTML integration points. Go, parse5, and blocking Chromium consume the same observations. Fragment parsing is explicitly outside the v1 portable HTTP profile; the manifest records the excluded adoption-agency fragment test and its reason. Updating `x/net/html`, parse5, the upstream revision, the selected subset, or any expected observation requires a reviewed compatibility diff rather than implicit golden regeneration.

Run the pull-request compatibility gate with:

```bash
GOTOOLCHAIN=local go test ./internal/dom -run TestPinnedHTMLCompatibilityManifest
npm run test:typescript
make test-playwright-e2e
```

Static HTTP extraction does not include JavaScript mutations or other live-browser state. Namespace-sensitive selectors, fragment parsing, scripting-enabled parser state, and DOM APIs outside the documented portable selector and extraction surface are not promoted by this corpus.

HTTP decoding occurs before parsing. The TypeScript runtime supports the same built-in labels as the Go runtime, honors BOM precedence, bounds meta-charset sniffing to the first 4 KiB, and reports unsupported or malformed encodings with stable execution codes.
