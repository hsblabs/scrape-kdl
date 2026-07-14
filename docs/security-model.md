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

`session policy="none"` suppresses only the explicit runtime `Session`. It does not clear an `http.Client` cookie jar or an existing browser context. Hosts that require credential-free execution must provide isolated stateless clients or contexts.

CLI users should pass cookies and sensitive headers through `--session-file` or `--session-file -` rather than command-line values. Files should be readable only by the intended user and removed or rotated according to the host application's secret-management policy.

## Runtime safeguards

The reference runtime provides:

- JavaScript disabled by default;
- capability validation before network or browser activity;
- response-size and timeout limits;
- a host-supplied URL policy for initial targets and HTTP redirects;
- JSON-compatible JavaScript result validation;
- optional adapter-wide extraction leases to prevent page-operation interleaving;
- structured error codes that avoid embedding session values.
