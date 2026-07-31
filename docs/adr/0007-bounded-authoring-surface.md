---
status: accepted
date: 2026-08-01T03:48:22+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0007: Keep authoring semantic, bounded, and separate from formatting and IR

## Context

Embedding applications need to construct valid Scraping KDL without copying
the built-in registry or implementing string escaping. Exposing compiler syntax
nodes would make parser internals public, while accepting Validated IR as an
authoring model would invert the required source-to-validation pipeline. A
generic KDL node builder would avoid those leaks but still force every consumer
to duplicate the language grammar and node relationships.

## Decision

The v1 authoring surface is a separate semantic model for one extractor. Its
first bounded scope includes a source and session policy, scalar inputs, text,
HTML, and attribute fields, nested collections, error policies, and calls
selected from a versioned built-in catalog. Imports, modules, browser workflow,
JavaScript, declared transforms, input defaults, field defaults, and arbitrary
KDL nodes remain outside this first tracer.

The authoring writer creates deterministic KDL Source and owns KDL 2 string
escaping. It is not a lossless formatter and does not preserve comments or
lexical choices. The compiler remains the only boundary that can emit Validated
IR or establish semantic validity. Catalog lookup always names an explicit
language version; no moving `latest` alias is provided.

Go exposes this boundary as the `authoring` package. TypeScript exposes the
matching `@hsblabs/scrape-kdl/authoring` entry point. Both derive catalog names
and call arguments from the same registries their compilers use, and both keep
compiler-internal syntax and IR types private.

## Consequences

- Authoring tools can render prompts and serialize the tracer without private
  language tables or escaping code.
- Drafts may still be semantically incomplete; callers compile writer output to
  obtain the ordinary structured diagnostics.
- Existing KDL formatting remains a distinct, currently unsupported concern.
- Expanding the authoring model requires an intentional public-surface decision
  rather than exposing every internal parser or IR feature by accident.
