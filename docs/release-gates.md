# v1 release gates

`conformance/release-matrix.json` is the executable map from roadmap example categories to fixtures, runtimes, and blocking commands. It covers static HTTP, malformed HTML, charset decoding, session and redirect policy, transforms, partial results, imports, and browser workflow with trusted JavaScript. Every repository example runs in both Go and TypeScript; the shared browser fixture runs through go-rod and Playwright.

Run the complete supported-target gate from a checkout with Chromium installed:

```bash
make release-gate
```

This command verifies formatting, diagnostics, goldens, examples, conformance, clean packed consumers, measured performance, security and resource contracts, race behavior, module metadata, go-rod with its real dependency, both browser E2E paths, and the Chromium WPT differential. It does not create a tag, publish an artifact, change repository visibility, or merge a pull request.

Firefox and WebKit are visible scheduled best-effort jobs. Their failures do not weaken or replace the blocking Chromium gate. External live-site validation is intentionally opt-in through `.github/workflows/live-site-smoke.yml`; it accepts no credentials, bounds time and response bytes, and rejects cross-origin redirects.

The support contract is machine-readable in `docs/support-matrix.json`. Pull-request CI executes all four CLI package targets and runs native tests on Linux and macOS with Go 1.26 and Node.js 26.

Before any public tag, build the exact private candidate bundle:

```bash
make release-dist VERSION=v1.0.0-rc.1 OUT=dist
```

The bundle gate checks release-only npm metadata and peer ranges, clean npm installation, all four CLI archives, required license and notice files, and SHA-256 checksums. It does not publish the artifacts. `.github/workflows/release-rehearsal.yml` runs the same process and stores only a private Actions artifact.
