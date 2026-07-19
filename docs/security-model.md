# Execution security model

## Trust levels

A plain HTTP spec without external transforms still controls outbound URLs and selectors. A browser spec additionally controls page interactions. A spec containing `evaluate-js` is equivalent to executable code within the browser page context.

The runtime therefore assumes specs are trusted application assets. It does not provide a secure sandbox for untrusted KDL.

## Host responsibilities

Hosts should apply:

- outbound network allowlists or isolated networks;
- request and browser timeouts;
- response-size limits;
- dedicated low-privilege browser processes;
- secret redaction in logs;
- separate browser contexts for unrelated tenants;
- explicit review before enabling JavaScript or external transforms.

Injected source loaders are host-owned authority boundaries. They receive lexically resolved import paths and should constrain those paths to the intended source set, honor cancellation, and avoid embedding source contents or credentials in returned errors. The compiler performs parsing, cycle detection, hashing, and validation after loading; it does not grant an injected loader filesystem, network, or subprocess access.

`session policy="none"` suppresses only the explicit runtime `Session`. It does not clear an `http.Client` cookie jar or an existing browser context. Hosts that require credential-free execution must provide isolated stateless clients or contexts.

Core and go-rod CLI users must pass cookies and sensitive headers through `--session-file` or explicit `--session-file -` standard input; direct secret-bearing header and cookie flags are rejected. Files should be readable only by the intended user and removed or rotated according to the host application's secret-management policy.

## Runtime safeguards

The reference runtime provides:

- JavaScript disabled by default;
- capability validation before network or browser activity;
- response-size and timeout limits;
- a host-supplied URL policy for initial targets and HTTP redirects, with a ready-made public-internet policy (`PublicInternetURLPolicy` plus `NewPublicInternetHTTPClient` for dial-time re-checks against DNS rebinding) that the Go CLI applies by default;
- JSON-compatible JavaScript result validation;
- RE2-compatible, linear-time regular-expression execution in both reference runtimes;
- optional adapter-wide extraction leases to prevent page-operation interleaving;
- structured error codes that avoid embedding session values.

The guarded Go HTTP client resolves and checks the address selected for each direct connection. It deliberately ignores environment proxy settings: proxy-side target resolution would prevent the client from verifying that address. Custom library clients remain host-owned; network-level egress controls are still required when the trust boundary extends beyond this safeguard.

The TypeScript HTTP runtime applies the same ordering: program, selector, input, session, capability, and external-transform preflight completes before `fetch` is invoked. It performs redirects manually so `URLPolicy` runs before every redirected request, strips authorization and host-only cookie headers across origins, bounds streamed bodies before decoding, and propagates parent cancellation separately from request timeouts. `parse5` receives only decoded HTML after these acquisition limits succeed.

The browser runtimes reject missing adapters and disabled JavaScript before lease acquisition or navigation. They apply the URL policy only to the initial target before acquiring the extraction-wide lease and pass cancellation to adapter operations. The official Playwright adapter creates an isolated context for each extraction; timeout or cancellation closes that context before the lease is released. The go-rod adapter serializes extraction-wide use of its mutable page. Neither adapter promotes the initial-target hook into a complete browser network policy: redirects, subresources, service workers, and page-initiated requests remain subject to adapter, browser-context, and host network controls described in `docs/browser-runtime.md` and `docs/rod-adapter.md`.
