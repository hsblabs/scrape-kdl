---
status: accepted
date: 2026-08-01T03:14:32+09:00
decision-makers:
  - project owner
agent: OpenAI Codex (GPT-5)
---

# ADR 0005: Decode Go extraction results strictly and atomically

## Context

The Go runtime exposes the validated extraction value as `map[string]any`.
Typed hosts repeated shape assertions and numeric conversion switches, often
falling back to plausible zero values when extraction output drifted. Encoding
the map to JSON and decoding it again would route integers through a wire format,
weaken control over missing and null values, and make strict overflow behavior
harder to state.

## Decision

`Result.Decode(destination)` performs a reflection-based, in-process conversion
over a completed result. The destination must be a non-nil pointer to a struct
or string-keyed map. Conversion occurs into a fresh value and the caller's
destination is assigned only after every field succeeds.

Struct field matching uses an explicit `json` tag name or the exact exported Go
field name. Missing non-nullable fields, nullability mismatches, unknown source
fields, unsupported destination types, and duplicate destination names are
errors. Nested structs, maps, slices, arrays, and pointers are recursive. The
decoder does not invoke custom unmarshaling methods.

Integer conversion uses exact signed or unsigned values and arbitrary-precision
comparison before assignment. It rejects sign changes, overflow, fractional
numbers, and float-to-integer conversion. Float conversion accepts only finite
floating sources, and a `float32` destination rejects precision loss.

Decoding is independent of extraction status. `Warnings` and `Partial` remain on
the original result and are never cleared or converted into zero values.

## Consequences

- Consumers can decode nested typed output without private shape helpers.
- Destination values never contain a partially decoded prefix after an error.
- Struct decoding is intentionally strict about both source and destination
  fields; callers use a map when they need presence-sensitive dynamic keys.
- TypeScript does not receive a mechanically identical helper because its native
  schema and type-narrowing ecosystem is different. Observable extraction values
  remain portable.
