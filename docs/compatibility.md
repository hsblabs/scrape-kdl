# Compatibility matrix

| Area | Minimum | CI target |
|---|---:|---:|
| Go | 1.26 | 1.26.x |
| Node.js | 26 | 26.x |
| TypeScript package format | ESM-only | npm pack and clean Node.js consumer |
| TypeScript HTML parser | parse5 8.0.1 | 8.0.1 |
| KDL lexical base | KDL 2.0 concepts | Scraping KDL supported subset |
| Scraping KDL language | `2026-07-15` | `2026-07-15` |
| Validated IR | `2026-07-15` | `2026-07-15` |
| go-rod adapter | go-rod v0.116.2 | v0.116.2 |
| Playwright adapter | Playwright 1.61.1 | Chromium (blocking); Firefox and WebKit (best effort) |
| Operating system | Linux or macOS | Linux and macOS |

The Go and TypeScript core packages intentionally have no browser-library dependency. The TypeScript core pins the roadmap-approved `parse5` dependency for WHATWG HTML tree construction; its transitive `entities` dependency is lockfile-pinned. Browser integrations are separate modules or packages. The official TypeScript adapter pins Playwright 1.61.1. Chromium is the supported v1 browser target; scheduled Firefox and WebKit results remain best effort and do not block v1.

The Go and TypeScript libraries publish their exact accepted sets through `SupportedLanguageVersions` / `SupportedIRVersions` and `supportedLanguageVersions` / `supportedIRVersions`. These values are opaque identifiers; an earlier date is not implicitly compatible with a later date.

## Migration from the untagged working draft

- Replace integer root properties such as `version=1` with a real document revision such as `version="2026-07-15"`.
- Add `language-version="2026-07-15"` to every extractor and transform module root.
- Regenerate Validated IR with `languageVersion` and `irVersion` set to `2026-07-15`; serialized `0.1` IR is rejected before acquisition.
- Update Go callers for `Program.Version()` returning `string` instead of `int`.

The former identifiers are not accepted as aliases because doing so would obscure the selected language and IR contracts.

## v0.1 contract decisions

The project is pre-release, and these M5 changes intentionally close previously ambiguous v0.1 behavior:

- explicit `-h` and `--help` on every subcommand print help to standard output and exit with status 0; malformed or incomplete arguments still exit with status 2;
- `--session-file PATH` and `--session-file -` are the supported secret-input paths; direct `--header` and `--cookie` values remain accepted but are deprecated for a future minor-version removal;
- browser adapters use the concrete result representations documented in `docs/browser-runtime.md`; typed containers and custom marshalers are not accepted implicitly;
- external-transform result mismatches and runtime-managed cancellation use the new stable codes `E_EXTERNAL_TRANSFORM_RESULT_TYPE` and `E_EXECUTION_CANCELED`;
- `session policy="none"` ignores only the explicit runtime session and does not clear ambient client or browser state;
- millisecond workflow durations are limited to 9,223,372,036,854, the largest whole-millisecond value representable by Go `time.Duration`.

These changes may affect scripts built against an untagged working draft. After a release, patch versions remain backward compatible within their minor version under `docs/versioning.md`.

The HTTP reference runtime uses an internal permissive parser with raw-text/RCDATA protection, truncated-document recovery, and common optional-end-tag handling. It is not yet a complete WHATWG HTML tree builder. Browser mode operates on the browser's live DOM and does not share that limitation.

## Platform policy

Supported targets are `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`. Windows is explicitly outside the support scope: there is no Windows CI, binary distribution, compatibility guarantee, or Windows-specific bug support.
