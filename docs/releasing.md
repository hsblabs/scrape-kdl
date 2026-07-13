# Release process

## Core module

1. Update `CHANGELOG.md` and implementation status.
2. Run `make release-check`.
3. Create and push an annotated `vX.Y.Z` tag.
4. The core release workflow builds Linux and macOS CLI tar archives for amd64 and arm64, writes SHA-256 checksums, and creates the GitHub release.
5. Confirm that the module is visible through the Go module proxy before releasing adapters that depend on it.

## go-rod adapter

1. Update `adapters/rod/go.mod` to require the published core version.
2. Run `make test-rod` and `make test-rod-e2e`.
3. Create and push an annotated `adapters/rod/vX.Y.Z` tag.
4. The adapter release workflow verifies the module and creates a GitHub release.

Local and CI adapter verification temporarily replaces the core requirement with the repository checkout. Published module behavior is defined by each module's own `go.mod`, which must not contain a local `replace` directive.

## Supported release targets

- `linux/amd64`
- `linux/arm64`
- `darwin/amd64`
- `darwin/arm64`

Windows artifacts must not be added. The release script rejects `windows/*` explicitly.
