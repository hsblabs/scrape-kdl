# Compatibility matrix

| Area | Minimum | CI target |
|---|---:|---:|
| Go | 1.26 | 1.26.x |
| KDL lexical base | KDL 2.0 concepts | Scraping KDL supported subset |
| Scraping KDL language | v0.1 working draft | v0.1 |
| go-rod adapter | go-rod v0.116.2 | v0.116.2 |
| Operating system | Linux or macOS | Linux and macOS |

The core module intentionally has no browser-library dependency. Browser integrations are separate modules.

The HTTP reference runtime uses an internal permissive parser with raw-text/RCDATA protection, truncated-document recovery, and common optional-end-tag handling. It is not yet a complete WHATWG HTML tree builder. Browser mode operates on the browser's live DOM and does not share that limitation.

## Platform policy

Supported targets are `linux/amd64`, `linux/arm64`, `darwin/amd64`, and `darwin/arm64`. Windows is explicitly outside the support scope: there is no Windows CI, binary distribution, compatibility guarantee, or Windows-specific bug support.
