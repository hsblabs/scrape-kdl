---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: HTTP Execution
description: How the HTTP runtime gets a document — the execution order, the limits, the charset decoding, the sessions, the redirects, and the URL policy that protects a private network.
hsblabs:
  sidebar:
    order: 24
---

The HTTP runtime executes a program with `mode="http"`. It sends one GET request, decodes the body, parses the HTML into an internal DOM, and extracts the values. It does not execute JavaScript and does not start a browser.

## The execution order

```text
compiler validation
  -> runtime capability preflight
  -> input and default resolution
  -> validation of a required session
  -> URL expansion
  -> HTTP request
  -> validation of the response size
  -> charset decode
  -> DOM parse
  -> output extraction
```

The runtime checks the selectors, the availability of the external transforms, the browser-only value sources, and the fetch mode before it sends the request. Thus a program that cannot operate correctly does not cause traffic.

## The behavior of the request

| Property | Value |
| --- | --- |
| Method | GET |
| Accepted status | 200 to 299 |
| Default timeout | 30 seconds |
| Default body limit | 32 MiB |
| Default User-Agent | `scrape-kdl/1.0` |
| Redirects | The supplied `http.Client` controls them. |

A response that is too large fails with `E_HTTP_BODY_TOO_LARGE`. A status outside of the accepted range fails with `E_HTTP_STATUS`. The limits are not advisory. The runtime applies them before it reads the full body into the memory.

## The charset

The runtime selects the charset from the HTTP header `Content-Type`, then from an early declaration of `<meta charset>`, with UTF-8 as the default. A recognized byte-order mark of UTF-8 or UTF-16 has priority over the declared charset.

UTF-8, ASCII, ISO-8859-1, Windows-1252, UTF-16LE, and UTF-16BE are built in. The runtime resolves a different label through the WHATWG encoding index. Thus a legacy encoding such as Shift_JIS or EUC-JP decodes without configuration.

Two conditions cause an error:

- A bad byte sequence fails with `E_HTML_DECODE`. The Go runtime and the TypeScript runtime agree on this behavior.
- The replacement encoding, and a label outside of the WHATWG index, fail with `E_HTML_CHARSET_UNSUPPORTED`.

## The sessions

The property `policy` of the node `session` controls the behavior:

| Policy | Behavior |
| --- | --- |
| `none` (default) | The runtime ignores the session that you supply explicitly. |
| `optional` | The runtime uses the session when you supply one. |
| `required` | The extraction stops before the fetch when no session is available. |

The runtime adds the headers and the cookies of the session to the initial request only. It does not put them in a redirected request again. Thus the redirect rules of your `http.Client` control the propagation. By default the client copies a sensitive header such as `Authorization` or `Cookie` only to the same domain or to a subdomain. A configured cookie jar applies the scope of each cookie.

The policy `none` does not clear the ambient state of the host. Your `http.Client` and its cookie jar continue to operate, and a `Set-Cookie` response header continues to have an effect. For an execution without a state, supply an isolated client that has no jar and no ambient authentication.

## The URL policy

The hook `Options.URLPolicy` executes before the initial request and also before each HTTP redirect. An error from the hook stops the extraction with `E_URL_POLICY`.

`PublicInternetURLPolicy` is a prepared policy. It rejects a scheme that is not HTTP or HTTPS, a URL with userinfo, and an address that the IANA special-purpose registries do not mark as globally accessible. This includes the loopback, private, link-local, carrier-grade NAT, documentation, benchmarking, multicast, unspecified, and reserved ranges. It keeps the globally accessible exceptions of the registries.

A policy check occurs before the connection. Thus DNS rebinding can defeat the check alone. Use `NewPublicInternetHTTPClient` with the policy. Its dialer resolves the address again at connection time, examines it again, and reports a rejection as `E_URL_POLICY`. This guarded client makes a direct connection and does not use the proxy settings of the environment, because a proxy resolves the target itself and the client then cannot examine the selected address.

The CLI applies the policy and the guarded client together, by default. The option `--allow-private-hosts` disables them. The library has different defaults: it applies no policy until you configure one.

A `CheckRedirect` function of your own executes after the policy of Scraping KDL, not in the place of it.

The policy is not a substitute for a network-level egress control. It is one layer.

## The HTML parser

The Go runtime uses a pinned version of `golang.org/x/net/html` for the tree construction, after it decodes the bounded bytes to UTF-8. The internal DOM keeps the document order and supplies the portable selectors, the decoded text, a deterministic inner HTML, the attributes, and the missing-value behavior.

A compatibility manifest in the repository covers foster parenting in a bad table, the active formatting elements, the integration of foreign content, the raw text, the RCDATA, the optional end tags, and truncated input. It has no approved divergence.

The parser does not execute a script and does not calculate a layout. Use browser mode when you need a mutation by a script, a layout value, or a different behavior of a live DOM. Refer to [Browser Mode](./browser-mode.md).

## The error recovery

A missing selector or a missing attribute follows the properties `required` and `default` of the field. The node `on-error` does not control this condition.

The node `on-error` controls a transform error, a selector cardinality error, an output type error, and an external transform error. A collection with `on-row-error="skip"` drops only a row that contains an error that no policy recovered. Each dropped row adds the warning `W_ROW_SKIPPED` and sets `partial` to `true`.

## The cancellation

The runtime examines the context before it parses the HTML in the memory, and also before each output member and each collection row. A cancellation at one of these boundaries gives `E_EXECUTION_CANCELED` and keeps `context.Canceled` or `context.DeadlineExceeded` as the cause.

A field policy or a row policy cannot recover a cancellation. The runtime does not interrupt one parser call that is in progress. It sees the cancellation at the next boundary. The TypeScript runtime has the same boundaries with an `AbortSignal`.

## Next step

- [Offline Snapshots](./offline-snapshots.md) — extraction without an acquisition.
- [Browser Mode](./browser-mode.md) — a live page and a workflow.
- [Security and Responsible Use](./security-and-responsible-use.md) — the rules before a live target.
