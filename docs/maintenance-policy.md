# v1 maintenance policy

After stable v1 publication, the Go modules, npm packages, CLI, and official adapters follow Semantic Versioning independently. Patch releases contain compatible fixes, documentation, and security hardening. Minor releases may add backward-compatible API or language support. Breaking public API changes require a new major module/package version; language or Validated IR semantic changes additionally require a newly approved dated contract identifier.

The latest stable v1 minor is the maintained feature line. Older v1 minors may receive critical security or data-integrity fixes when practical, but no long-term-support duration is promised. Supported runtime and browser tiers are those in `docs/support-matrix.json`; best-effort targets do not acquire compatibility guarantees through incidental success.

Security reports follow `SECURITY.md`. Maintainers may withhold exploit details until a fix is available. Security fixes do not weaken JavaScript opt-in, URL policy, response bounds, cancellation, session isolation, or browser lease requirements.

Diagnostic codes, CLI automation contracts, Go/TypeScript public APIs, and dated language/IR behavior are compatibility surfaces. Deprecations must include migration guidance before removal in a future major version. Release artifacts are reproducibly gated through `make release-gate` and `make rc-package-check` before publication.
