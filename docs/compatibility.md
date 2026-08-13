# Compatibility matrix

| Area | Minimum | CI target |
|---|---:|---:|
| Go | 1.26 | 1.26.x |
| Node.js | 22 | 22.x (minimum compatibility gate) |
| Bun (core TypeScript package) | 1.3 | 1.3.x (smoke-tested) |
| TypeScript package format | ESM-only | npm pack and clean Node.js consumer |
| TypeScript HTML parser | parse5 8.0.1 | 8.0.1 |
| TypeScript regular expressions | re2js 2.8.6 (RE2-compatible) | 2.8.6 |
| KDL lexical base | KDL 2.0 concepts | Scraping KDL supported subset |
| Scraping KDL language | `2026-07-15` | `2026-07-15` |
| Validated IR | `2026-07-15` | `2026-07-15` |
| Go HTML parser | `golang.org/x/net/html` v0.57.0 | v0.57.0 |
| go-rod adapter | go-rod v0.116.2 | v0.116.2 |
| Playwright adapter | Playwright 1.61.1 | Chromium (blocking); Firefox and WebKit (best effort) |
| Operating system | Linux or macOS | Linux and macOS |

The executable source for this table is [`support-matrix.json`](./support-matrix.json). Pull-request CI cross-builds all four supported OS/architecture package targets and runs the native Go, Node.js, and clean-consumer gates on Linux and macOS. Bun support applies to the core TypeScript package; the Playwright adapter remains validated as a Node.js package.

The Go and TypeScript core packages intentionally have no browser-library dependency. The TypeScript core pins the roadmap-approved `parse5` dependency for WHATWG HTML tree construction and `re2js` for RE2-compatible, linear-time regular expressions; transitive dependencies are lockfile-pinned. Browser integrations are separate modules or packages. The official TypeScript adapter pins Playwright 1.61.1. Chromium is the supported v1 browser target; scheduled Firefox and WebKit results remain best effort and do not block v1.

The Go and TypeScript libraries publish their exact accepted sets through `SupportedLanguageVersions` / `SupportedIRVersions` and `supportedLanguageVersions` / `supportedIRVersions`. These values are opaque identifiers; an earlier date is not implicitly compatible with a later date.

## Migration from the untagged working draft

- Replace integer root properties such as `version=1` with a real document revision such as `version="2026-07-15"`.
- Add `language-version="2026-07-15"` to every extractor and transform module root.
- Regenerate Validated IR with `languageVersion` and `irVersion` set to `2026-07-15`; serialized `0.1` IR is rejected before acquisition.
- Update Go callers for `Program.Version()` returning `string` instead of `int`.
- Rename workspace automation from `conformance:typescript-slice` to `conformance:typescript` and from `test:contract-slice` to `test:typescript-contract`.
- TypeScript consumers importing internal `compileContractSlice` or `validateContractSlice` helpers must use the public `compile` and `validate` APIs; the internal result is now named `CompilerResult`.

The former identifiers are not accepted as aliases because doing so would obscure the selected language and IR contracts.

## Working-draft contract decisions frozen for v1

These decisions close previously ambiguous working-draft behavior and are frozen for v1:

- explicit `-h` and `--help` on every subcommand print help to standard output and exit with status 0; malformed or incomplete arguments still exit with status 2;
- `--session-file PATH` and `--session-file -` are the supported secret-input paths; direct `--header` and `--cookie` values were removed at the v0.5 CLI boundary with migration guidance in `docs/cli.md`;
- browser adapters use the concrete result representations documented in `docs/browser-runtime.md`; typed containers and custom marshalers are not accepted implicitly;
- external-transform result mismatches and runtime-managed cancellation use the new stable codes `E_EXTERNAL_TRANSFORM_RESULT_TYPE` and `E_EXECUTION_CANCELED`;
- `session policy="none"` ignores only the explicit runtime session and does not clear ambient client or browser state;
- millisecond workflow durations are limited to 9,223,372,036,854, the largest whole-millisecond value representable by Go `time.Duration`.
- the default Go and TypeScript HTTP User-Agent is `scrape-kdl/1.0`.

These changes may affect scripts built against an untagged working draft; the complete migration is in `docs/migrating-to-v1.md`. After a release, patch versions remain backward compatible within their minor version under `docs/versioning.md`.

The frozen v1 CLI contract defines help, standard streams, one-document `--json`
envelopes, explicit `-` streams, exit statuses 0/1/2/130/143, non-interactive
behavior, and signal cancellation. Changes follow the Semantic Versioning policy
in `docs/versioning.md`.

The HTTP reference runtime uses pinned `golang.org/x/net/html` WHATWG document tree construction after charset decoding. The portable guarantee and explicit exclusions are recorded in `docs/html-compatibility.md`; browser mode continues to operate on the browser's mutable live DOM.

Both HTTP runtimes decode WHATWG-labelled charsets (including Shift_JIS and EUC-JP) without configuration: TypeScript through `TextDecoder`, Go through `golang.org/x/text/encoding/htmlindex`. Extractions against such pages previously failed in Go with `E_HTML_CHARSET_UNSUPPORTED` unless `Options.CharsetDecoder` was set; that code now indicates a label outside the WHATWG index. Both runtimes reject malformed byte sequences with `E_HTML_DECODE`, checked through the shared charset compatibility manifest.

## Platform policy

Supported targets are `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`. Windows is explicitly outside the support scope: there is no Windows CI, binary distribution, compatibility guarantee, or Windows-specific bug support.
