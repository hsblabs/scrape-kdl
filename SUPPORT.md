# Support

## Version support

There is no supported public version before the first public release. Private release candidates are verification artifacts and are superseded by the next candidate.

Starting with `v1.0.0`:

- the latest stable minor release in the current major receives bug and security fixes;
- the immediately preceding minor receives security fixes for 90 days after a new minor is published;
- older minors and prereleases are unsupported;
- deprecations are announced in a minor release and removals wait for the next major release;
- release binaries are built with the repository-pinned, security-patched Go toolchain, while library users must stay on a security-supported Go patch release.

The release notes identify the currently supported minor and any exceptional backport decision. There is no separate long-term-support line.

## Supported platforms

Linux and macOS are supported. Windows is explicitly unsupported: no Windows CI, release artifacts, compatibility guarantees, or Windows-only defect investigation are provided.

Use GitHub Discussions for design questions and usage help when available. Use issues for reproducible defects and accepted feature work.

Before reporting a problem, run `scrape-kdl validate` and include:

- scrape-kdl version;
- Go version;
- operating system and architecture;
- fetch mode;
- a minimal spec and sanitized fixture;
- complete diagnostics or execution error codes.

Do not include live credentials, session cookies, authorization headers, or private HTML.
