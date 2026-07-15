# HTTP runtime

## Execution order

```text
compiler validation
  -> runtime capability preflight
  -> input/default resolution
  -> required session validation
  -> URL expansion
  -> HTTP request
  -> response-size validation
  -> charset decoding
  -> DOM parse
  -> output extraction
```

Selector parsing, external-transform availability, browser-only value sources, and fetch mode are checked before an HTTP request is sent.

## HTTP behavior

- method: GET;
- accepted status: 200 through 299;
- default timeout: 30 seconds;
- default body limit: 32 MiB;
- default User-Agent: `scrape-kdl/0.1`;
- redirect behavior: delegated to the supplied `http.Client`.

The runtime adds `Session` headers and cookies only to the initial request; it does not manually re-inject them after redirects. Go's `http.Client` redirect rules therefore control propagation. By default, sensitive headers such as `Authorization` and `Cookie` are copied only to the same domain or its subdomains, while a configured cookie jar applies each cookie's own scope. A custom `CheckRedirect` may further restrict or mutate the redirected request.

For `session policy="none"`, the explicit `Options.Session` is ignored. The supplied `http.Client` and its cookie jar are preserved, so ambient cookies and response `Set-Cookie` updates remain active. Hosts that require stateless execution must supply an isolated client with no jar or other ambient authentication behavior.

## Charset decoding

Built in:

- UTF-8 / ASCII;
- ISO-8859-1;
- Windows-1252;
- UTF-16LE / UTF-16BE.

Charset is selected from HTTP `Content-Type`, then an early `<meta charset>` declaration, with UTF-8 as the default. A recognized UTF-8 or UTF-16 byte-order mark overrides the declared charset.

Other encodings are supported through `Options.CharsetDecoder`. This keeps encodings such as Shift_JIS and EUC-JP optional.

## HTML parser boundary

The Go reference runtime delegates document tree construction to pinned `golang.org/x/net/html` after bounded response bytes have been decoded to UTF-8. The internal DOM preserves document order and exposes portable selectors, decoded text, deterministic inner HTML, attributes, and missing-value behavior to the executor.

The checked-in compatibility manifest covers malformed table foster parenting, active formatting elements, foreign content integration, raw text, RCDATA, optional end tags, truncated input, and the portable selector surface with zero approved divergences. See `docs/html-compatibility.md` for normalization and exclusions. Use browser mode when script mutations, layout, browser APIs, or other live-DOM behavior is required.


## URL policy

`Options.URLPolicy` runs before the initial request and before every HTTP redirect. Returning an error stops extraction with `E_URL_POLICY`. This hook is intended for host allowlists and private-network restrictions. It does not replace network-level egress controls.

A custom `http.Client.CheckRedirect` still runs after the scrape-kdl policy.

## Error recovery

Missing selector or attribute follows field `required` and `default` semantics. It is not handled by `on-error`.

Transform, selector cardinality, output type, and external-transform errors use field `on-error`:

- `fail`;
- `null`;
- `warn`;
- `default`.

A collection with `on-row-error="skip"` drops only rows containing an unrecovered child error. Dropped rows append `W_ROW_SKIPPED` and set `partial=true`.

## Offline HTML cancellation

`Program.ExtractHTML` checks its context before in-memory HTML parsing and before each output member and collection row. Cancellation at these runtime-managed boundaries returns `E_EXECUTION_CANCELED`, preserves `context.Canceled` or `context.DeadlineExceeded` as the cause, and is not recoverable through field or row error policies. One in-progress parser call is not interrupted; cancellation is observed at the next boundary after it returns.
