---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Transforms
description: How a value pipeline operates in Scraping KDL — the built-in registry, the declared transforms, the match tables, the external host functions, and the RE2 regular expression profile.
hsblabs:
  sidebar:
    order: 23
---

A value source gives you a string. A transform makes that string into the value that you declared. Each `apply` node executes in source order. The compiler checks the types of the full sequence before the execution.

The normative registry is in [builtins-v0.1.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/builtins-v0.1.md). Compiled examples are in the [transform cookbook](https://github.com/hsblabs/scrape-kdl/blob/main/docs/cookbook.md).

## The pipeline

```kdl
field "price" type="u32" required=#true {
  select ".price" match="one"
  value "text"
  apply "trim"
  apply "replace" old="," new=""
  apply "parse-int" as="u32"
}
```

Each call gets the output of the previous call. The output type of one call must agree with the input type of the next call. A bad sequence gives `E_TRANSFORM_TYPE_MISMATCH` at compile time, not at execution time.

The final output must be assignable to the `type` of the field. There is no implicit conversion. A `string` does not become a `u32` without `parse-int`.

## The built-in registry

The built-in transforms have four groups:

| Group | Function | Examples |
| --- | --- | --- |
| String | Change the shape of the text. | `trim`, `normalize-whitespace`, `replace`, `regex-capture`, `split` |
| Conversion | Make a typed value from the text. | `parse-int`, `parse-float`, `parse-bool`, `empty-to-null`, `coalesce` |
| URL | Read or resolve a URL with RFC 3986. | `url-resolve`, `url-query`, `url-path`, `path-segment` |
| Validation | Give the input again, or cause an error. | `assert-matches`, `assert-enum`, `assert-min`, `assert-max` |

Refer to the registry for the full list and for each signature. These rules apply to each built-in:

- A built-in name is reserved. You cannot shadow it.
- An unknown, a duplicate, or a type-incompatible call property is an error.
- A string index is an index of the Unicode scalar values. It is not an offset of the UTF-8 bytes or of the UTF-16 code units.
- Each numeric output must be finite and inside the range of the target type.

The transform `parse-int` does not remove the unwanted spaces. Apply `trim` first. It must also consume the full input. Thus the text `12 kg` causes an extraction error and does not give the value `12`. This behavior is intentional. A silent partial parse hides a change of the page.

## The regular expressions

Each regular expression uses the RE2 syntax. RE2 does not have lookaround, backreference, named capture group, or conditional expression. These absent functions are the cost of a predictable execution time.

You can give the flags `i`, `m`, and `s` with the property `flags`. Each flag can be present one time only.

In a replacement string, `$0` is the full match and `$1` to `$99` are the numbered captures. Write `$$` for a literal dollar sign.

The syntax of JavaScript for a regular expression is not the syntax of the language. A pattern that operates in `RegExp` can still be an error with `E_REGEX_INVALID`.

## The declared transforms

Give a name to a sequence that you use more than one time:

```kdl
transform "extract_horse_id" input="string" output="string?" {
  pipeline {
    apply "regex-capture" pattern=#"/horse/([^/?#]+)"# group=1
  }
}
```

A declared transform has exactly one body: `pipeline`, `match`, or `external`. In version 0.1 a declared transform takes no call argument and no call property. Make a second transform when you need different parameters.

## The match tables

Use `match` for a table of the scalar values:

```kdl
transform "normalize_sex" input="string" output="string" {
  match {
    case "牡" "male"
    case "牝" "female"
    case "セ" "gelding"
    default "unknown"
  }
}
```

The runtime compares the cases in source order with exact equality. Exactly one `default` is necessary. Two cases with the same input value are an error. The input type and the output type must be scalar or nullable scalar.

## The external transforms

An external transform is a function of the host. Use it when the logic cannot be in the language, for example a decryption or a call to an internal service.

```kdl
transform "decrypt_payload" input="string" output="object" {
  external symbol="decrypt_payload"
}
```

The host supplies the symbol from a registry. The compiler adds the capability `transform.external:decrypt_payload` to the IR. If the registry does not have the symbol, the validation fails before the fetch. Thus you do not find the absent function at the middle of an extraction.

After the function gives a result, the runtime immediately checks the result against the declared output type. A mismatch fails with `E_EXTERNAL_TRANSFORM_RESULT_TYPE`, before each subsequent transform.

An external transform is an intentional escape from the portable behavior. The same program then needs the same registry on each runtime.

## How a name is resolved

The compiler resolves the argument of `apply` in this sequence:

1. an exact built-in name;
2. an exact local transform name;
3. a qualified imported name, in the form `alias.name`.

You must write an imported transform with its alias. An unqualified reference to an imported transform is an error. A built-in name cannot be shadowed. Thus the meaning of `apply "trim"` never changes.

## Next step

- [HTTP Execution](./http-execution.md) — how the runtime gets the document.
- [Diagnostics](./diagnostics.md) — how to read a compile error.
- [Patterns](./patterns.md) — the usual combinations.
