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

## Charset decoding

Built in:

- UTF-8 / ASCII;
- ISO-8859-1;
- Windows-1252;
- UTF-16LE / UTF-16BE.

Charset is selected from HTTP `Content-Type`, then an early `<meta charset>` declaration, with UTF-8 as the default. A recognized UTF-8 or UTF-16 byte-order mark overrides the declared charset.

Other encodings are supported through `Options.CharsetDecoder`. This keeps encodings such as Shift_JIS and EUC-JP optional.

## HTML parser boundary

The current Go reference runtime uses an internal permissive tree parser to keep the core module small. It supports ordinary scraping documents and repository fixtures, but it is not a complete WHATWG HTML tree builder. Malformed table foster parenting, all implied-element cases, and foreign-content edge cases may differ from a browser DOM.

Use browser mode when exact live-DOM behavior is required. Replacing or supplementing the static parser with a fully conforming backend remains planned work.


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
