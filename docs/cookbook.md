# Transform cookbook

Scraping KDL pipelines are statically typed. A transform consumes the exact
type produced by the previous step: `regex-capture` returns `string?`, while
`parse-int` and `parse-float` require `string`. The compiler does not insert a
coercion or choose what a missing match means.

The recipes below make that policy explicit. Every `kdl` block is compiled in
both reference implementations by `make examples`.

## Localized number to integer or float

Remove only separators and units whose meaning is known, then isolate the
numeric token. In this example a missing price match means zero because the
application has chosen `coalesce value="0"`; that is a product rule, not a
parser default.

```kdl
extractor "localized-numbers" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/summary"
  }

  field "price" type="float" required=#true {
    select ".price" match="one"
    value "text"
    apply "normalize-whitespace"
    apply "replace" old="," new=""
    apply "regex-capture" pattern="([0-9.]+)" group=1
    apply "coalesce" value="0"
    apply "parse-float" as="float"
  }

  field "quantity" type="int" required=#true {
    select ".quantity" match="one"
    value "text"
    apply "normalize-whitespace"
    apply "replace" old="," new=""
    apply "replace" old=" items" new=""
    apply "parse-int" as="int"
  }
}
```

For text such as `2,899.5 units`, the `price` pipeline produces `2899.5`. It
does not recognize every locale: full-width digits, locale-specific decimal
marks, and grouping rules need an explicit host transform or downstream parser.

If a missing numeric match must fail instead of becoming zero, coalesce to an
invalid sentinel and assert before parsing:

```text
apply "regex-capture" pattern="([0-9.]+)" group=1
apply "coalesce" value=""
apply "assert-matches" pattern=".+"
apply "parse-float" as="float"
```

## Optional ID from an attribute

An absent link is a normal `null`. If the link exists but its `href` does not
contain the expected ID shape, convert the nullable regex result to an explicit
failure; `on-error "warn"` then recovers that failure to `null` and records a
warning.

```kdl
extractor "optional-attribute-id" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/catalog"
  }

  field "item_id" type="string?" required=#false {
    select "a.detail" match="first"
    value "attr" name="href"
    apply "regex-capture" pattern="/items/([^/?#]+)" group=1
    apply "coalesce" value=""
    apply "assert-matches" pattern=".+"
    on-error "warn"
  }
}
```

Missing selectors and missing attributes follow optional-field semantics and
return `null` without a warning. `on-error` handles execution failures; it does
not turn an ordinary missing value into a warning.

## Relative link to absolute URL and path segment

Resolve a relative `href` before exposing it as a URL. Derive a nullable ID when
the final path segment is useful to the caller.

```kdl
extractor "portable-links" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/catalog/page/1"
  }

  field "detail_url" type="string" required=#true {
    select "a.detail" match="one"
    value "attr" name="href"
    apply "url-resolve" base="https://example.invalid/catalog/page/1"
  }

  field "detail_id" type="string?" required=#false {
    select "a.detail" match="first"
    value "attr" name="href"
    apply "url-resolve" base="https://example.invalid/catalog/page/1"
    apply "path-segment" index=-1
  }
}
```

`url-resolve` uses RFC 3986 rules; the `base` is an absolute literal known when
the document is authored. `path-segment` percent-decodes the selected segment
and returns `null` when the requested segment does not exist.

## Blank-cell handling

Normalize whitespace before deciding whether a cell is blank. A nullable field
can preserve blank as `null`; a required field can reject blank text explicitly.

```kdl
extractor "blank-cells" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/table"
  }

  field "note" type="string?" required=#false {
    select "td.note" match="first"
    value "text"
    apply "normalize-whitespace"
    apply "empty-to-null"
  }

  field "name" type="string" required=#true {
    select "td.name" match="one"
    value "text"
    apply "normalize-whitespace"
    apply "assert-matches" pattern=".+"
  }
}
```

`required=#true` requires the selected value to exist; it does not by itself
declare that an existing string must be non-empty. `empty-to-null` returns
`string?`, so its output belongs in a nullable field unless a later `coalesce`
turns it back into `string`.

## Nullable pipeline or coalesced value

Keep `T?` when absence has business meaning. Use `coalesce` only when the output
contract defines a concrete replacement value.

```kdl
extractor "nullable-policy" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/score"
  }

  field "label" type="string?" required=#false {
    select ".label" match="first"
    value "text"
    apply "normalize-whitespace"
    apply "empty-to-null"
  }

  field "score" type="int" required=#true {
    select ".score" match="one"
    value "text"
    apply "regex-capture" pattern="([0-9]+)" group=1
    apply "coalesce" value="0"
    apply "parse-int" as="int"
  }
}
```

Here `label: null` means “no label”, while `score: 0` means the application has
defined a missing numeric token as zero. If those states are not equivalent,
do not coalesce them merely to satisfy the next transform's input type.

## Date and time strings

Language version `2026-07-15` has no date, time, duration, or time-zone type.
Extract a normalized string, optionally reject an unexpected lexical shape,
then perform calendar and time-zone parsing downstream.

```kdl
extractor "date-strings" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/article"
  }

  field "published_at" type="string" required=#true {
    select "time.published" match="one"
    value "attr" name="datetime"
    apply "normalize-whitespace"
    apply "assert-matches" pattern="^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}(Z|[+-][0-9]{2}:[0-9]{2})$"
  }
}
```

The regular expression checks shape only. It does not prove that a date exists,
resolve daylight-saving transitions, or convert it to an instant. Use the
caller's date/time library for those operations.

## Choosing recovery deliberately

- Missing selector or attribute values use `required` and `default`; they do
  not pass through `on-error`.
- A nullable transform result such as `regex-capture` or `empty-to-null` is a
  successful value, not an execution failure.
- `coalesce` removes nullability by applying a declared scalar policy.
- `on-error "warn"` recovers an execution failure, appends
  `W_ERROR_RECOVERED`, and makes the whole result partial.

These distinctions are intentional. Do not add consumer-side coercions that
turn an unexpected null, malformed number, or overflow into a plausible value
without an explicit product rule.
