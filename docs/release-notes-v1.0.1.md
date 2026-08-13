---
status: prepared; not published
version: v1.0.1
prepared-date: 2026-08-13
---

# scrape-kdl v1.0.1 release notes

`v1.0.1` is a compatibility-focused patch release prepared from the stable
`v1.0.0` line. It does not change the language, IR, public API, extraction
semantics, diagnostics, or security defaults.

## Supported environment

- Linux and macOS on amd64 and arm64.
- Go 1.26.
- Node.js 22 or later for the TypeScript packages.
- Bun 1.3 or later for `@hsblabs/scrape-kdl`.
- Chromium supported. Playwright Firefox and WebKit are best effort.
- Windows is not supported.

## Changes

- Lowered the TypeScript package Node.js minimum from 26 to 22.
- Declared Bun 1.3 or later for the core TypeScript package.
- Updated package metadata, lockfile, compatibility documentation, release
  checks, and pull-request CI to match the supported runtimes.

The immutable `v1.0.0` tags and artifacts remain unchanged.
