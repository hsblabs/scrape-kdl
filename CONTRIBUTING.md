# Contributing

Contributions are accepted through GitHub issues and pull requests.

## Development requirements

- Go 1.26 or later
- Node.js 26 or later and npm for TypeScript contract checks
- GNU Make or compatible shell commands
- Chromium only for go-rod E2E tests

Run the offline verification suite before opening a pull request:

```bash
npm ci
make verify
```

TypeScript package changes can run the complete focused gate directly:

```bash
npm run verify:typescript
```

This gate typechecks and lints source, runs tests and coverage thresholds, inspects the packed file set, and installs the tarball into a clean Node.js consumer. Do not commit generated `dist/` files or packed archives.

When network access and Chromium are available, also run:

```bash
make test-rod
make test-rod-e2e
```

## Change expectations

Language changes must include:

- a specification update;
- valid and invalid KDL fixtures;
- stable diagnostics where applicable;
- expected IR changes;
- runtime tests when execution semantics change.

Public Go or TypeScript API changes must explain compatibility impact and update both independent consumer contract checks. Generated or golden files must be updated by the same pull request.

Examples and fixtures must use documentation-safe domains or local test
servers. Do not contribute examples that target a real service without
documented authorization, or whose terms prohibit the demonstrated automation.
Features intended to bypass anti-bot controls, access controls, rate limits, or
account restrictions are not accepted. See `docs/responsible-use.md`.

## Commit and pull request scope

Keep changes focused. Describe why the change is needed, the compatibility effect, and the commands used for verification. No contributor license agreement is required; submitted contributions are accepted under the repository's Apache-2.0 license.
