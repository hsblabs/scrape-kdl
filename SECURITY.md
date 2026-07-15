# Security policy

## Supported versions

Before `v1.0.0`, security fixes are applied to the latest released minor version only.
After `v1.0.0`, the project will document an explicit support window here.

## Reporting a vulnerability

Use GitHub's private security advisory flow for `hsblabs/scrape-kdl`. Do not open a public issue for an unpatched vulnerability.

Include the affected version, execution mode (`http` or `browser`), a minimal KDL spec, expected impact, and reproduction steps that do not target third-party systems.

## Security boundaries

- `evaluate-js` executes arbitrary JavaScript in the configured browser page. It is disabled unless `AllowJavaScript` is explicitly enabled.
- KDL specs are executable configuration. Only run trusted specs, especially when imports, browser workflows, sessions, or external transforms are used.
- Browser sandboxing, process isolation, network policy, and credential storage are responsibilities of the host application and selected browser adapter.
- HTTP extraction can access URLs described by a spec. Hosts should apply outbound-network controls when specs are not fully trusted.
- Session cookies and headers may contain secrets. Diagnostics and logs must not print their values.
- The CLI accepts secret session values only through `--session-file PATH` or explicit standard input with `--session-file -`. Direct `--cookie` and `--header` values are rejected because shell history and process inspection can expose them.
