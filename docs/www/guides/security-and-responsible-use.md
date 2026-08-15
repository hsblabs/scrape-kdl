---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Security and Responsible Use
description: The trust model of Scraping KDL, the responsibilities of a host, the protections of the runtime, and the operational rules before you extract from a service that you do not operate.
hsblabs:
  sidebar:
    order: 29
---

Scraping KDL is an extraction tool. It does not give you permission to access or to re-use the content of a different person. You decide if each use is authorized and correct. This page is operational guidance. It is not legal advice.

The normative documents are [security-model.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/security-model.md) and [responsible-use.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/responsible-use.md).

## A specification is a trusted asset

A KDL document is executable configuration. There are three levels of authority:

| The document contains | The authority |
| --- | --- |
| An HTTP source, no external transform | It controls the outbound URL and the selectors. |
| A browser source | It also controls the interactions with the page. |
| An `evaluate-js` node | It is executable code inside the context of the page. |

Thus the runtime assumes that a specification is a trusted asset of your application. It does not supply a secure sandbox for untrusted KDL. Do not compile a document that a user of your service wrote.

## What the runtime supplies

- JavaScript is off until you give an explicit opt-in.
- The capability validation completes before each network operation and each browser operation.
- The response size and the operations have limits, and the limits obey a cancellation.
- A hook for a URL policy examines the initial target and each HTTP redirect. `PublicInternetURLPolicy` and `NewPublicInternetHTTPClient` are prepared for the public internet, and the Go CLI applies them by default.
- The runtime validates that a JavaScript result is JSON-compatible.
- Each regular expression uses RE2 and has a linear execution time.
- An adapter can hold a lease for the full extraction. Then the operations of two extractions cannot interleave on one page.
- A structured error code does not contain a session value.

The TypeScript HTTP runtime has the same sequence. Its preflight of the program, the selectors, the inputs, the session, the capabilities, and the external transforms completes before it calls `fetch`. It makes each redirect itself, thus the URL policy executes before each redirected request. It removes an authorization header and a host-only cookie header between the origins, and it limits the streamed body before the decode.

## What you must supply

The runtime is one layer. Your host must add:

- an allowlist of the outbound network, or an isolated network;
- a timeout for a request and for a browser;
- a limit for the size of a response;
- a browser process with low privileges and a separate context for each tenant;
- a redaction of the secrets in your logs;
- an explicit review before you enable JavaScript or an external transform.

An injected source loader is your authority boundary. It gets a lexically resolved path of an import. Limit those paths to the intended set of the sources, obey a cancellation, and do not put the content of a source or a credential in an error. The compiler gives a loader no access to the file system, the network, or a subprocess.

The functions `CompileFS` and `ValidateFS` limit the root and the names of the imports to the paths of `io/fs` and reject a lexical escape to a parent. Your `fs.FS` still defines the true authority. `os.DirFS` can follow a symbolic link outside of its directory. Use `os.Root.FS` when you need containment.

## The secrets

The CLI accepts a header and a cookie only from `--session-file FILE` or from `--session-file -`. It rejects a flag that carries a secret directly. A command argument can go into the history of a shell or become visible in a list of the processes.

```json
{
  "headers": {"Authorization": ["Bearer example"]},
  "cookies": [{"name": "session", "value": "example"}]
}
```

Make the file readable by the intended user only. Remove it or change it in agreement with your policy for the secrets.

The policy `session policy="none"` stops the explicit session only. It does not clear the cookie jar of your `http.Client` or the state of an existing browser context. For an execution without a credential, supply an isolated client or an isolated context.

Never put a live credential, a session cookie, an authorization header, or private extracted content in an issue, a log, a fixture, or an example.

## Before you use a live service

Do these operations before you make a service the target:

- Read its current terms of service and its policy for the automation.
- Examine its instructions in `robots.txt`.
- Get permission when the service or the applicable rules require it.
- Use a documented API when that is the supported method to get the data.

The [Robots Exclusion Protocol](https://www.rfc-editor.org/rfc/rfc9309) gives an owner a standard method to declare a preference. A permission in `robots.txt` does not give you permission, does not have priority over the terms of the site, and does not decide if a use is legal.

## Limit the load

Your application controls the schedule, the concurrency, the retries, and the cache. It must:

- send the requests slowly and add a variation of the delay where that is correct;
- keep the concurrency inside a limit that the target accepts;
- keep a response in a cache and not get the same unchanged content again;
- stop or wait after a status `429` or `503`, after a timeout, and after a different sign of an overload;
- limit the browser sessions, the sizes of the responses, the retries, and the total time.

Scraping KDL has no global rate limiter. The absence of one is not permission to send unlimited traffic.

## Identify your client

When the automation is permitted, set a User-Agent that identifies your client. Add a useful contact or a URL of the project where that is correct. Do not imitate a different crawler or a browser to escape a policy of the target. The two CLIs have the option `--user-agent`, and a library host can set the equivalent option.

## The extracted content

Collect only the data that your application needs. Examine the copyright, the database rights, the privacy, the confidentiality, the contractual limits, the retention, the access control, and the secure deletion before you keep or distribute the content.

The requirements are different in each jurisdiction. A user in Japan must read the current Copyright Act and the Act on the Protection of Personal Information, and also each other applicable rule, and must get professional advice when that is necessary.

## No circumvention

This project does not supply and does not accept a function whose purpose is to bypass a CAPTCHA, an access control, a paywall, a rate limit, a bot detection, or a limit of an account. It does not supply credential stuffing, a false browser fingerprint, or stealth automation that hides a violation of a policy.

Security research and interoperability work must use a system that you have permission to test.

## Next step

- [HTTP Execution](./http-execution.md) — the URL policy and the limits.
- [Patterns](./patterns.md) — the position of the pacing and the retry in your code.
