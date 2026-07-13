# Contributing

Contributions are accepted through GitHub issues and pull requests.

## Development requirements

- Go 1.26 or later
- GNU Make or compatible shell commands
- Chromium only for go-rod E2E tests

Run the offline verification suite before opening a pull request:

```bash
make verify
```

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

Public Go API changes must explain compatibility impact. Generated or golden files must be updated by the same pull request.

## Commit and pull request scope

Keep changes focused. Describe why the change is needed, the compatibility effect, and the commands used for verification. No contributor license agreement is required; submitted contributions are accepted under the repository's Apache-2.0 license.
