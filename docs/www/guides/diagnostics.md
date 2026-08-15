---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Diagnostics
description: How to read a Scraping KDL diagnostic — the stable codes, the deterministic order, the severities, the warnings, and the most frequent compile and runtime errors.
hsblabs:
  sidebar:
    order: 27
---

A diagnostic tells you what is wrong, where it is, and which part of the output it affects. The codes are a public compatibility surface. A code keeps the same meaning between the releases. Thus you can write a test or an alert that examines a code.

The full list is in [diagnostics.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/diagnostics.md).

## The format

The CLI writes one line for each diagnostic:

```text
extractor.kdl:9:5: error E_SELECTOR_UNSUPPORTED: selector byte 9: unsupported pseudo-class "has" [output.title.selection]
```

The line has five parts:

| Part | Meaning |
| --- | --- |
| `extractor.kdl:9:5` | The file, the line, and the column. The numbers start at 1. |
| `error` | The severity. |
| `E_SELECTOR_UNSUPPORTED` | The stable code. |
| The text after the code | The message for a person. It is not stable. |
| `[output.title.selection]` | The path in the output that the error affects. |

Examine the code, not the message. The codes and their conditions are normative. The messages are not, and they can change.

## The machine-readable form

The option `--json` gives one JSON document:

```bash
scrape-kdl validate ./extractor.kdl --json
```

```json
{
  "ok": false,
  "diagnostics": [
    {
      "code": "E_SELECTOR_UNSUPPORTED",
      "severity": "error",
      "message": "selector byte 13: unsupported pseudo-class \"has\"",
      "span": {
        "file": "extractor.kdl",
        "start": { "offset": 197, "line": 7, "column": 5 },
        "end": { "offset": 219, "line": 7, "column": 27 }
      },
      "path": "output.title.selection"
    }
  ]
}
```

The `span` uses the same definition as the Validated IR. The line and the column start at 1. The `offset` is a 0-based UTF-8 byte offset. The end position is exclusive. The `path` is absent when the diagnostic affects no output member.

The envelope is different for each command:

- `validate`: `{"ok": boolean, "diagnostics": [...]}`;
- `compile`: `{"ok": true, "diagnostics": [...], "ir": {...}}`, or `{"ok": false, "diagnostics": [...]}`;
- `extract`: `{"ok": true, "result": {...}}`, or `{"ok": false, "error": {...}}`.

## The order

The compiler orders the static diagnostics by:

1. the depth-first resolution order of the imports;
2. the lexical order of the file paths, for an equal position;
3. the start offset in the source;
4. the lexical order of the codes.

The runtime orders the warnings by the sequence of the execution.

The order is deterministic. Two executions of the same program on the same input give the same sequence. Thus you can compare the full output of a diagnostic in a golden file.

## The severities

There are two severities:

- `error` — the process stops. The compiler makes no IR, or the runtime gives no result.
- `warning` — the extraction continues. The result has the warning in its array `warnings`.

A warning frequently sets the flag `partial` to `true`. Then you know that the result is not complete.

## The frequent compile errors

| Code | Cause | Correction |
| --- | --- | --- |
| `E_SELECTOR_UNSUPPORTED` | The selector is outside of the portable profile, for example `:has()`. | Refer to [Selectors](./selectors.md). |
| `E_TRANSFORM_TYPE_MISMATCH` | The output type of one transform does not agree with the input type of the next transform. | Add a conversion, for example `parse-int`. |
| `E_TRANSFORM_UNKNOWN` | The name is not a built-in, a local transform, or a qualified imported transform. | Examine the spelling and the alias of the import. |
| `E_BROWSER_CAPABILITY_REQUIRED` | A browser-only node is in an HTTP-mode program. | Change the mode to `browser`, or remove the node. |
| `E_LANGUAGE_VERSION_UNSUPPORTED` | The value of `language-version` is well formed but not supported. | Use `2026-07-15`. |
| `E_DUPLICATE_PROPERTY` | One node has the same property two times. | Remove one property. The compiler does not select one for you. |
| `E_IMPORT_CYCLE` | The import graph has a cycle. | Divide the shared transforms into a module without an import. |
| `E_REMOTE_IMPORT_UNSUPPORTED` | The path of the import is not relative. | Copy the module into your repository. |

## The frequent runtime errors

| Code | Cause |
| --- | --- |
| `E_REQUIRED_VALUE_MISSING` | A field with `required=#true` found no value. |
| `E_SELECTOR_CARDINALITY` | A selector with `match="one"` found more than one element. |
| `E_URL_POLICY` | The URL policy rejected the initial target or a redirect. |
| `E_HTTP_STATUS` | The status of the response is outside of the range 200 to 299. |
| `E_HTTP_BODY_TOO_LARGE` | The response is larger than the limit of the body. |
| `E_JAVASCRIPT_DISABLED` | The program has JavaScript, but you gave no opt-in. |
| `E_SNAPSHOT_UNSUPPORTED` | A snapshot execution was requested for a program with a workflow or with JavaScript. |
| `E_BROWSER_RUNTIME_MISSING` | A browser-mode program has no adapter. |
| `E_EXECUTION_CANCELED` | The context or the `AbortSignal` stopped the execution. |

The error `E_JAVASCRIPT_DISABLED`, the error `E_BROWSER_RUNTIME_MISSING`, and the error `E_SNAPSHOT_UNSUPPORTED` occur before the acquisition. Thus a bad configuration does not cause traffic.

## The warnings

| Code | Meaning |
| --- | --- |
| `W_ROW_SKIPPED` | The policy `on-row-error="skip"` dropped a collection row. |
| `W_ERROR_RECOVERED` | The policy `on-error="warn"` recovered an error. |
| `W_PARTIAL_EXTRACTION` | A summary warning for a partial result. |
| `W_JAVASCRIPT_PRESENT` | A static examination found the trusted-code capability. |

## The exit statuses

| Status | Meaning |
| --- | --- |
| 0 | Success. |
| 1 | A failure of the validation, the compilation, the extraction, or the input and output. |
| 2 | An error in the use of a command or a flag. |
| 130 | `SIGINT` stopped the process. |
| 143 | `SIGTERM` stopped the process. |

In an automated procedure, examine the exit status and also the field `ok` of the envelope `--json`.

## Next step

- [Patterns](./patterns.md) — the shapes that prevent the frequent errors.
- [CLI](../cli/index.md) — the full contract of the commands.
