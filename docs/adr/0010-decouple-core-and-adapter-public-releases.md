---
status: accepted
date: 2026-08-13T12:45:00+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0010: Decouple core and adapter public release phases

## Context

The unified public workflow published the core Git tag and GitHub Release, then
waited up to five minutes for the public Go proxy before publishing either npm
package or the go-rod adapter. A proxy negative cache therefore left the core
release partially published and skipped unrelated npm publication. The same
failure recurred whenever the proxy took longer than the bounded wait.

The core Go/npm release and the Playwright/go-rod adapter release have a real
dependency boundary: adapters must consume the published core, while the core
package does not need adapter publication to be useful. They should therefore
not share a failure boundary.

## Decision

Keep one manually dispatched, protected `release.yml` workflow and one
`release-publish` approval, but divide its publish job into two ordered phases:

1. Publish or verify the core annotated tag and GitHub Release, then publish or
   verify `@hsblabs/scrape-kdl` immediately through npm trusted publishing.
2. Wait for the core Go module to resolve through `proxy.golang.org` for up to
   30 minutes, then wait until 30 minutes have elapsed since the core GitHub
   Release was published. Only after both conditions publish or verify the
   Playwright npm package and the go-rod tag, Release, and module.

The npm publisher accepts an explicit `core` or `playwright` selection and
remains resumable and integrity-checking. Public Go proxy waits use 180 ten-
second attempts. A failed adapter phase can be rerun after the core phase has
already completed; matching core tags, Releases, assets, and npm versions are
verified and skipped.

## Consequences

- A delayed Go proxy no longer prevents the core npm package from publishing.
- Playwright and go-rod are released at least 30 minutes after the core
  GitHub Release, and never before the core Go module is publicly resolvable.
- The protected publish runner remains occupied during the release window, but
  the workflow keeps one approval and one caller-visible release command.
- The adapter phase may fail independently after the core phase succeeds;
  recovery reruns the same immutable version and fails closed on mismatches.
- The core GitHub Release still carries the inspected Playwright archive as
  part of the complete release bundle; registry publication is delayed until
  the adapter phase.
