# Security policy

## Supported versions

There is no supported public version before `v1.0.0`. For authorized private
users, only the latest `-private.N` version receives release-blocking fixes;
earlier private versions are superseded and unsupported.

After `v1.0.0`, the latest stable minor in the current major receives security fixes. The immediately preceding minor receives security fixes for 90 days after a new minor is published. Older minors and prereleases are unsupported. See `SUPPORT.md` for the complete maintenance policy.

## Reporting a vulnerability

Use GitHub's private security advisory flow for `hsblabs/scrape-kdl`. Do not open a public issue for an unpatched vulnerability.

Include the affected version, execution mode (`http` or `browser`), a minimal KDL spec, expected impact, and reproduction steps that do not target third-party systems.

## Security boundaries

- `evaluate-js` executes arbitrary JavaScript in the configured browser page. It is disabled unless `AllowJavaScript` is explicitly enabled.
- KDL specs are executable configuration. Only run trusted specs, especially when imports, browser workflows, sessions, or external transforms are used.
- Browser sandboxing, process isolation, network policy, and credential storage are responsibilities of the host application and selected browser adapter.
- HTTP extraction can access URLs described by a spec. Hosts should apply outbound-network controls when specs are not fully trusted.
- The core and go-rod CLIs reject non-public initial targets by default. The core CLI also re-checks HTTP redirects and dial-time DNS results through a direct, proxy-disabled guarded client. `--allow-private-hosts` deliberately removes those CLI safeguards.
- A browser adapter's URL policy check covers only the initial navigation target. Browser redirects, subresources, service workers, and page-initiated traffic require browser-context or host-level network controls.
- Session cookies and headers may contain secrets. Diagnostics and logs must not print their values.
- TypeScript regular-expression built-ins execute through the pinned RE2-compatible engine rather than the JavaScript backtracking engine. Keep the nested-repetition regression test when changing this boundary.
- Both CLIs accept secret session values only through `--session-file PATH` or explicit standard input with `--session-file -`. Direct `--cookie` and `--header` values are rejected because shell history and process inspection can expose them.
