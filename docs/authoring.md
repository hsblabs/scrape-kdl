---
title: Authoring API
status: v1 release candidate
---

# Authoring API

The authoring API lets an application construct one extractor without copying
the built-in transform registry or implementing KDL string escaping. Go exposes
it as `github.com/hsblabs/scrape-kdl/authoring`. TypeScript exposes the matching
`@hsblabs/scrape-kdl/authoring` package entry point.

The boundary is deliberately one-way:

```text
Authoring Document
  -> deterministic KDL Writing
  -> KDL Source
  -> ordinary compiler validation
  -> Validated IR
  -> execution
```

An Authoring Document is not a syntax tree or mutable IR. Writing does not prove
that selectors, types, transforms, and output relationships are semantically
valid. Always compile the emitted KDL and handle its structured diagnostics
before execution.

## Supported authoring model

The first authoring surface supports:

- one extractor with an explicit document and language version;
- an HTTP or browser fetch source and session policy;
- required or optional scalar inputs;
- text, HTML, and attribute fields;
- nested collections and their cardinality and row-error policies;
- field match and error policies;
- calls selected from the versioned built-in catalog.

It does not model imports, transform modules, declared transforms, browser
workflow, JavaScript value sources, input or field defaults, comments, or
arbitrary KDL nodes. Author those features as KDL Source. Expanding this public
model is a compatibility decision, not a reason to expose compiler syntax or IR
internals.

## Versioned built-in catalog

Catalog lookup always requires an exact language version. There is no `latest`
alias:

```go
catalog, err := authoring.BuiltinCatalog("2026-07-15")
normalize, ok := catalog.Lookup("normalize-whitespace")
```

```ts
const catalog = builtinCatalog("2026-07-15");
const normalize = catalog.builtins.find(
  ({ name }) => name === "normalize-whitespace",
);
```

Each definition provides its input and output constraints, nullability effect,
positional arity, named argument constraints, required arguments, and defaults.
Go returns an independent catalog value. TypeScript returns a recursively frozen
catalog. A writer accepts a call only when its name and arguments belong to the
catalog selected by the document's `languageVersion`.

Call scalars cover KDL strings, booleans, integers, finite floats, and null. Go
uses `authoring.String`, `Bool`, `Int`, `Float`, and `Null`; TypeScript uses the
corresponding native values, including `null`.

The normative machine-readable catalog is
[`docs/spec/builtins-v0.1.authoring.json`](spec/builtins-v0.1.authoring.json).
Repository checks compare it with both compiler registries and both public
catalog implementations.

## Go example

```go
catalog, err := authoring.BuiltinCatalog("2026-07-15")
if err != nil {
    return err
}
normalize, ok := catalog.Lookup("normalize-whitespace")
if !ok {
    return errors.New("normalize-whitespace is unavailable")
}

source, err := authoring.Write(authoring.Document{
    LanguageVersion: "2026-07-15",
    Extractor: authoring.Extractor{
        Name:    "article",
        Version: "2026-08-01",
        Source: authoring.Source{
            FetchMode:     scrapekdl.FetchModeHTTP,
            URLTemplate:   "https://example.invalid/articles/{id}",
            SessionPolicy: scrapekdl.SessionPolicyNone,
        },
        Inputs: []authoring.Input{{
            Name: "id", Type: authoring.PrimitiveString, Required: true,
        }},
        Members: []authoring.Member{authoring.Field{
            Name: "title", Type: "string", Required: true,
            Selector: "h1", Match: authoring.MatchOne,
            Value: authoring.TextValue{},
            Transforms: []authoring.BuiltinCall{normalize.Call(nil, nil)},
            OnError: authoring.ErrorFail,
        }},
    },
})
if err != nil {
    return err
}
program, diagnostics, err := scrapekdl.Compile(
    ctx,
    scrapekdl.Source{Path: "article.kdl", Data: source},
    scrapekdl.CompileOptions{},
)
```

## TypeScript example

```ts
import { compile } from "@hsblabs/scrape-kdl";
import {
  builtinCatalog,
  callBuiltin,
  write,
  type AuthoringDocument,
} from "@hsblabs/scrape-kdl/authoring";

const catalog = builtinCatalog("2026-07-15");
const normalize = catalog.builtins.find(
  ({ name }) => name === "normalize-whitespace",
);
if (normalize === undefined) throw new Error("built-in is unavailable");

const document: AuthoringDocument = {
  languageVersion: "2026-07-15",
  extractor: {
    name: "article",
    version: "2026-08-01",
    source: {
      fetchMode: "http",
      urlTemplate: "https://example.invalid/articles/{id}",
      sessionPolicy: "none",
    },
    inputs: [{ name: "id", type: "string", required: true }],
    members: [{
      kind: "field",
      name: "title",
      type: "string",
      required: true,
      selector: "h1",
      match: "one",
      value: { kind: "text" },
      transforms: [callBuiltin(normalize)],
      onError: "fail",
    }],
  },
};

const source = write(document);
const { program, diagnostics } = await compile({
  path: "article.kdl",
  data: source,
});
```

## Writing versus formatting

Writing creates new canonical KDL Source with two-space indentation, stable
member and call order, catalog-defined named-argument order, and KDL 2 string
escaping. Equal Authoring Documents produce byte-identical output.

Formatting starts from existing KDL Source and must retain syntax-level material
such as comments and lexical choices. This repository does not expose a public
lossless formatter. Do not parse existing source into an Authoring Document and
write it back when preservation matters.

The shared tracer output is checked in at
[`fixtures/authoring/tracer.kdl`](../fixtures/authoring/tracer.kdl). Both public
writers must produce those exact bytes, and both compilers must accept them.
