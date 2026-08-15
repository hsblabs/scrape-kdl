---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Write an Extractor
description: The structure of an extractor document — the source, the inputs, the fields, the collections, the transforms, and the difference between a missing value and an error.
hsblabs:
  sidebar:
    order: 21
---

An extractor document declares what to get and what shape the result has. This page shows you the parts of the document and the rules that control them. The full grammar is in [language-v0.1.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/language-v0.1.md).

## The document

A file contains one `extractor` node or one `module` node. It cannot contain both.

```kdl
extractor "race-detail" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.com/race/{race_id}"
  }

  input "race_id" type="string" required=#true

  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
    apply "normalize-whitespace"
  }
}
```

Both properties of the root node are necessary:

- `version` is your revision identifier. It must be a real calendar date in the form `YYYY-MM-DD`.
- `language-version` selects the language contract. Its value must be `2026-07-15`.

The children `input`, `transform`, `field`, and `collection` can be in any sequence. The extractor needs exactly one `source`.

## The source

```kdl
source "html" {
  fetch mode="http" url="https://example.com/items/{item_id}"
  session policy="optional"
}
```

The node `source` accepts only the argument `"html"`. It needs exactly one `fetch` child. It can have one `session` child. It can have one `workflow` child, but only when the fetch mode is `browser`.

The property `mode` is `http` or `browser`. In HTTP mode the nodes `workflow` and `evaluate-js` are forbidden. The compiler rejects them with `E_BROWSER_CAPABILITY_REQUIRED`. Refer to [Browser Mode](./browser-mode.md).

The property `policy` of the node `session` is `none`, `optional`, or `required`. The default is `none`. With `required`, the runtime stops before the fetch when the host supplies no session.

## The inputs

```kdl
input "race_id" type="string" required=#true
input "lang" type="string" required=#false default="ja"
```

An input has one of these types: `string`, `bool`, `int`, or `float`. The property `required` has the default `#true`. A necessary input cannot have a default value.

The URL template uses an input with the syntax `{input_name}`. The runtime expands the template before the fetch. It percent-encodes a string and keeps only the unreserved characters of RFC 3986. To write a literal brace, use `{{` or `}}`.

If a required input is absent, the extraction stops before the fetch.

## The fields

```kdl
field "horse_name" type="string" required=#true {
  select ".horse-name a" match="one"
  value "text"
  apply "normalize-whitespace"
}
```

A field has one type and one value source. The children are:

- zero or one `select`;
- exactly one value source, either `value` or `evaluate-js`;
- zero or more `apply` nodes, in source order;
- zero or one `on-error`.

The property `match` of the node `select` is `one` or `first`. The default is `one`. With `one`, two or more matches cause `E_SELECTOR_CARDINALITY`. With `first`, the runtime uses the first match in document order. Refer to [Selectors](./selectors.md).

There are three value sources for a static DOM:

| Source | Result |
| --- | --- |
| `value "text"` | The text of the descendant nodes, in DOM order. The runtime does not remove the unwanted spaces. |
| `value "html"` | The inner HTML of the selected element. |
| `value "attr" name="href"` | The attribute of the DOM, not the resolved property of a browser. |

For an absolute link, apply the transform `url-resolve` to the attribute. The attribute itself keeps its relative form.

## A missing value is not an error

This distinction is important. The two conditions have different controls.

A value is **missing** when the selector found zero elements, or when the attribute does not exist. The property `required` controls this condition:

| Declaration | Result of a missing value |
| --- | --- |
| `required=#true` | The error `E_REQUIRED_VALUE_MISSING`. |
| `required=#false` with a `default` | The default value. No warning. `partial` stays `false`. |
| `required=#false` without a `default` | The value `null`. No warning. `partial` stays `false`. |

An **error** is a different condition. A transform failure, a type mismatch, a JavaScript error, or an adapter failure is an error. The node `on-error` controls this condition:

```kdl
on-error "warn"
```

| Policy | Result |
| --- | --- |
| `fail` | The runtime propagates the error. |
| `null` | The runtime gives `null` and sets `partial` to `true`. |
| `warn` | The runtime gives `null`, adds a warning, and sets `partial` to `true`. |
| `default` | The runtime gives the default of the field and sets `partial` to `true`. |

The default policy is `fail` for a necessary field and `null` for an optional field. The policies `null` and `warn` need an output type that permits null. The policy `default` needs a default value.

The node `on-error` does not control a missing selector or a missing attribute. Use `required` for that condition.

## The collections

```kdl
collection "entries" min-items=1 on-row-error="skip" {
  select "table.entries tbody tr"
  field "number" type="u8" required=#true {
    select ".number"
    value "text"
    apply "parse-int" as="u8"
  }
}
```

A collection needs exactly one `select` and a minimum of one child field or collection. Each element that agrees with the selector becomes a row, in document order. A collection can contain another collection.

The properties `min-items` and `max-items` limit the number of the rows. The value of `max-items` must be equal to or larger than `min-items`. The property `required=#true` gives an effective minimum of one row.

The property `on-row-error` is `fail` or `skip`. The default is `fail`. With `skip`, the runtime drops a row when a child has an error that no policy recovered. Each dropped row adds a warning and sets `partial` to `true`. The runtime examines the limits after it drops the rows.

## The types

The primitive types are `string`, `bool`, `int`, the unsigned integers from `u8` to `u64`, the signed integers from `i8` to `i64`, `float`, `f32`, `f64`, `object`, and `unknown`.

Add `[]` for an array and `?` for a nullable type. The operators bind from the left to the right. Thus `string?[]` is an array of nullable strings, and `string[]?` is a nullable array. Use parentheses to make the intent clear.

There is no implicit conversion. A string does not become a number without the transform `parse-int` or `parse-float`. An integer overflow is an extraction error, not a truncation. Each float must be finite.

## The transforms

Each `apply` node executes in source order. The output type of one call must agree with the input type of the next call. The compiler examines the full pipeline and rejects a bad sequence with `E_TRANSFORM_TYPE_MISMATCH`.

You can declare your own transform in the extractor or in a module:

```kdl
transform "extract_horse_id" input="string" output="string?" {
  pipeline {
    apply "regex-capture" pattern=#"/horse/([^/?#]+)"# group=1
  }
}
```

A declared transform has exactly one body: `pipeline`, `match`, or `external`. Refer to [Transforms](./transforms.md).

## The modules

Put the shared transforms in a module document and import it:

```kdl
import "./modules/common.kdl" as="common"

extractor "race-detail" version="2026-07-15" language-version="2026-07-15" {
  // ...
  field "horse_id" type="string?" {
    select "a.horse" match="first"
    value "attr" name="href"
    apply "common.extract_horse_id"
  }
}
```

The rules of an import are strict:

- the path must be relative. A remote URL is an error.
- the property `as` is necessary and each alias must be unique.
- the target must be a module document.
- a cycle in the import graph is an error.
- you must write an imported transform with its alias, in the form `alias.name`.

A module exports each transform that it declares directly. Version 0.1 has no re-export.

## The result

An extraction gives three parts:

```text
value       the object from the fields and the collections
warnings    the warnings, in the sequence of the execution
partial     true when the runtime recovered an error or dropped a row
```

The flag `partial` becomes `true` only after a recovery or a dropped row. An expected optional value that is absent does not set the flag. Thus you can trust a result that has `partial: false`.

## Next step

- [Selectors](./selectors.md) — the portable subset of CSS.
- [Transforms](./transforms.md) — the pipeline, the match, and the external transforms.
- [Patterns](./patterns.md) — the usual document shapes.
