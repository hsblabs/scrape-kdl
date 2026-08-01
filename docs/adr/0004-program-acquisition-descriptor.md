---
status: accepted
date: 2026-08-01T03:08:45+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0004: Expose a small acquisition descriptor from Program

## Context

Hosts that acquire HTML outside scrape-kdl need the compiled fetch mode, raw URL
template, and session policy. The release-candidate Go interface exposed those
facts only inside the language-neutral Validated IR JSON. A host therefore had
to serialize the complete IR and decode a private partial projection. That
coupled ordinary embedding code to an interchange schema it did not otherwise
need.

## Decision

Each compiled program exposes an immutable acquisition descriptor. Go provides
`Program.Descriptor()` returning a value composed only of public types.
TypeScript provides a recursively frozen `Program.descriptor`. Both contain the
validated fetch mode, raw URL template, and explicit-session policy.

The descriptor does not include inputs, browser workflow, output shape,
transforms, source spans, or file metadata. `Program.Metadata` remains the small
source and compatibility summary. `IRJSON` in Go and `program.ir` in TypeScript
remain the complete language-neutral representation.

## Consequences

- Hosts can make acquisition decisions without decoding or depending on the
  complete IR schema.
- The descriptor cannot mutate compiler or execution state.
- Fetch mode and session policy use named Go types with validated constants and
  string unions in TypeScript rather than unlabelled strings.
- Additional host-facing facts require an explicit additive interface decision;
  they are not copied from internal IR opportunistically.
