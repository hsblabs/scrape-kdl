# Built-in Transform Registry v0.1

This document is normative for Scraping KDL language version 0.1.

## Common rules

- Built-in names are reserved and cannot be shadowed.
- Unknown, duplicate, or type-incompatible call properties are errors.
- String indexes are Unicode scalar-value indexes, not UTF-8 byte offsets or UTF-16 code-unit offsets.
- All numeric outputs must be finite and within the target type range.
- Regex operations use RE2 syntax. Lookaround, backreferences, named capture groups, and conditional expressions are unsupported.
- Regex flags are passed with optional `flags` property. Allowed flags: `i`, `m`, `s`; each may appear at most once.
- Replacement strings use `$0` for the full match and `$1` through `$99` for numbered captures. `$$` emits a literal `$`.

## String operations

### `trim`

```text
string -> string
```

No properties. Removes leading and trailing Unicode White_Space characters.

### `normalize-whitespace`

```text
string -> string
```

No properties. Replaces each non-empty run of Unicode White_Space characters with U+0020 and trims leading/trailing whitespace.

### `lowercase`

```text
string -> string
```

No properties. Applies locale-independent Unicode default lowercase conversion.

### `uppercase`

```text
string -> string
```

No properties. Applies locale-independent Unicode default uppercase conversion.

### `replace`

```text
string -> string
```

Required properties:

- `old`: string
- `new`: string

Optional:

- `count`: non-negative integer; omitted means all occurrences; zero performs no replacement.

Literal, non-regex replacement from left to right with non-overlapping matches.

### `regex-replace`

```text
string -> string
```

Required:

- `pattern`: string
- `replacement`: string

Optional:

- `flags`: string
- `count`: non-negative integer; omitted means all matches.

### `regex-capture`

```text
string -> string?
```

Required:

- `pattern`: string

Optional:

- `group`: non-negative integer, default `0`
- `flags`: string

Returns null when the regex does not match or the requested capture did not participate. A group index greater than the pattern's capture count is a static error.

### `substring`

```text
string -> string
```

Required:

- `start`: integer

Optional:

- `end`: integer

Indexes are Unicode scalar-value indexes. Negative indexes count from the end. Bounds are clamped to `[0, length]`. If normalized end is less than normalized start, result is empty.

### `split`

```text
string -> string[]
```

Required:

- `separator`: string

Optional:

- `limit`: non-negative integer

A non-regex literal split. Empty separator splits into Unicode scalar values. Omitted limit means unlimited. `limit` is the maximum number of emitted elements; remaining input is discarded. Limit zero returns an empty array.

### `join`

```text
string[] -> string
```

Required:

- `separator`: string

### `prepend`

```text
string -> string
```

Required:

- `value`: string

### `append`

```text
string -> string
```

Required:

- `value`: string

## Conversion operations

### `parse-int`

```text
string -> TARGET_INTEGER
```

Required:

- `as`: one of `int`, `u8`, `u16`, `u32`, `u64`, `i8`, `i16`, `i32`, `i64`

Optional:

- `radix`: integer from 2 through 36, default 10

Rules:

- input is not implicitly trimmed;
- optional leading `+` or `-` is accepted for signed targets;
- unsigned targets reject `-`;
- underscore separators are rejected;
- the entire input must be consumed;
- overflow is an extraction error.

### `parse-float`

```text
string -> TARGET_FLOAT
```

Required:

- `as`: `float`, `f32`, or `f64`

Rules:

- input is not implicitly trimmed;
- decimal and exponent notation are supported;
- `NaN`, infinities, hexadecimal floats, and trailing characters are rejected;
- overflow to infinity is an extraction error.

### `parse-bool`

```text
string -> bool
```

Optional:

- `case-sensitive`: boolean, default `#false`
- `true`: string, default `"true"`
- `false`: string, default `"false"`

The entire input must equal one of the configured values.

### `to-string`

```text
string | bool | integer | float -> string
```

No properties. Uses canonical lowercase boolean and base-10 finite numeric representation.

### `empty-to-null`

```text
string -> string?
```

No properties. Returns null only when input length is zero.

### `coalesce`

```text
T? -> T
```

Required:

- `value`: scalar assignable to `T`

Returns input when non-null, otherwise configured value.

## URL operations

All URL operations use RFC 3986 parsing. Invalid URL input is an extraction error.

### `url-resolve`

```text
string -> string
```

Required:

- `base`: absolute URL string

Resolves input as a URI reference against base and emits an absolute URL.

### `url-query`

```text
string -> string?
```

Required:

- `name`: string

Optional:

- `index`: non-negative integer, default `0`

Returns the decoded query value at the requested occurrence or null when absent.

### `url-path`

```text
string -> string
```

No properties. Returns the percent-decoded URL path.

### `path-segment`

```text
string -> string?
```

Required:

- `index`: integer

Parses input as a URL or path, splits path by `/`, ignores empty leading/trailing segments, and returns the indexed decoded segment. Negative indexes count from the end. Out-of-range returns null.

## Validation operations

Validation operations return their input unchanged on success and produce an extraction error on failure.

### `assert-matches`

```text
string -> string
```

Required: `pattern`. Optional: `flags`.

### `assert-enum`

```text
T -> T
```

Required: one or more positional scalar arguments, all assignable to `T`.

### `assert-min`

```text
number -> number
```

Required: `value` numeric. Fails when input is less than value.

### `assert-max`

```text
number -> number
```

Required: `value` numeric. Fails when input is greater than value.
