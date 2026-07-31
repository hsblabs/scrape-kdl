---
status: accepted
date: 2026-08-01T03:06:48+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0003: Compile application-owned filesystems through fs.FS

## Context

Go hosts commonly keep extraction specifications in `embed.FS` or another
application-owned filesystem. The injected source loader could represent these
files, but each caller had to read the root, resolve imports, reject escapes,
propagate cancellation, and preserve filesystem error identity. Those concerns
are language and compiler behavior rather than application behavior.

## Decision

The Go package exposes `CompileFS` and `ValidateFS`. Both accept one `fs.FS` and
one slash-separated `io/fs` root path. The implementation reads the root and all
nested imports from the same filesystem, while the compiler remains responsible
for relative resolution, cycle detection, hashing, diagnostics, and metadata.

The root must be a non-directory `fs.ValidPath`. A resolved import that escapes
the logical filesystem root is rejected with an operational error wrapping
`fs.ErrInvalid`. Absolute and remote import syntax remains a deterministic
document diagnostic. Cancellation is checked immediately before and after every
filesystem read; `fs.FS` itself does not provide an interruptible read method.

Lexical path validation does not change the authority of the supplied
filesystem. `os.DirFS` may follow symlinks outside its directory. Hosts that
require symlink containment must supply `os.Root.FS` or another filesystem with
equivalent guarantees.

## Consequences

- `embed.FS`, `fstest.MapFS`, and other `fs.FS` implementations share the same
  compilation behavior without consumer-owned loader glue.
- Filesystem and cancellation failures retain their identity through the
  operational error interface established by ADR 0002.
- The Go-specific convenience does not require a matching TypeScript function;
  TypeScript continues to use its Node entry point or an injected loader.
- The interface adds no filesystem or network authority to KDL. The host chooses
  and owns the supplied filesystem.
