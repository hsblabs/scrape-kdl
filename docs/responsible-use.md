# Responsible use

`scrape-kdl` is an extraction tool, not permission to access or reuse
third-party content. You are responsible for deciding whether each use is
authorized and appropriate. This document is operational guidance, not legal
advice.

## Check authorization before extraction

Before targeting a service:

- read its current terms of service and automation policy;
- inspect its `robots.txt` instructions;
- obtain permission when the service or applicable rules require it;
- use a documented API instead when it is the supported access method.

The [Robots Exclusion Protocol](https://www.rfc-editor.org/rfc/rfc9309) gives
service owners a standard way to express crawler preferences. A `robots.txt`
allowance does not grant permission, override site terms, or settle whether a
particular use is lawful.

Repository examples use `example.com` or checked-in local fixtures. They are
language and runtime demonstrations, not endorsements of extracting from a
similarly structured live service.

## Limit load on target services

The host application controls scheduling, concurrency, retries, and caching.
It should:

- pace requests conservatively and add jitter where appropriate;
- keep concurrency within a limit accepted by the target;
- cache responses and avoid fetching unchanged content repeatedly;
- stop or back off on `429`, `503`, timeouts, and other signs of overload;
- bound browser sessions, response sizes, retries, and total execution time.

Do not interpret the absence of a built-in global rate limiter as permission to
send unbounded traffic.

## Identify the client honestly

When automation is permitted, set an explicit User-Agent that identifies the
client and provides a useful contact or project URL when appropriate. Do not
impersonate another crawler or browser to evade a target's policy. The core and
go-rod CLIs expose `--user-agent`; library hosts can set the corresponding
runtime option.

## Handle extracted content carefully

Collect only what the application needs. Consider copyright, database rights,
privacy, confidentiality, contractual restrictions, retention, access control,
and secure deletion before storing or redistributing extracted content.

Requirements vary by jurisdiction and use case. Users in Japan should review
the current [Copyright Act](https://laws.e-gov.go.jp/document?lawid=345AC0000000048)
and [Act on the Protection of Personal Information](https://laws.e-gov.go.jp/document?lawid=415AC0000000057),
along with any other applicable rules, and obtain professional advice when
needed.

Never include live credentials, session cookies, authorization headers, or
private extracted content in issues, logs, fixtures, or examples.

## No anti-bot circumvention

This project does not provide or accept features whose purpose is to bypass
CAPTCHAs, access controls, paywalls, rate limits, bot detection, or account
restrictions. It also does not provide credential stuffing, browser fingerprint
spoofing, or stealth automation intended to conceal policy violations.

Security research or interoperability work must use systems the contributor is
authorized to test and must preserve the security boundaries in
[`SECURITY.md`](../SECURITY.md).
