# Diagnostics v0.1

Diagnostic codes and trigger conditions are normative. Human-readable messages are not.

## Shape

```json
{
  "code": "E_TRANSFORM_TYPE_MISMATCH",
  "severity": "error",
  "message": "normalize-whitespace requires string but received int",
  "file": "race-detail.kdl",
  "range": {
    "start": { "line": 14, "column": 5 },
    "end": { "line": 14, "column": 25 }
  },
  "path": "transforms.invalid.pipeline.calls[1]"
}
```

Line and column numbers are 1-based. End position is exclusive.

## Ordering

Static diagnostics MUST be ordered by:

1. import depth-first resolution order;
2. file path lexical order within equal resolution position;
3. source start offset;
4. diagnostic code lexical order.

Runtime warnings MUST be ordered by actual extraction execution order.

## Errors

- `E_KDL_SYNTAX`: base KDL parse failure.
- `E_DOCUMENT_ROOT`: invalid number or kind of root nodes.
- `E_DOCUMENT_VERSION_REQUIRED`: extractor or module root lacks `version`.
- `E_DOCUMENT_VERSION_INVALID`: document `version` is not a real calendar date in exact `YYYY-MM-DD` form.
- `E_LANGUAGE_VERSION_REQUIRED`: extractor or module root lacks `language-version`.
- `E_LANGUAGE_VERSION_INVALID`: `language-version` is not a real calendar date in exact `YYYY-MM-DD` form.
- `E_LANGUAGE_VERSION_UNSUPPORTED`: `language-version` is well formed but not in the implementation's explicit supported-version registry.
- `E_TYPE_ANNOTATION_UNSUPPORTED`: KDL type annotation is present.
- `E_UNKNOWN_NODE`: node is not allowed in its context.
- `E_UNKNOWN_PROPERTY`: property is not allowed on a node.
- `E_DUPLICATE_PROPERTY`: duplicate property on one node.
- `E_ARGUMENT_COUNT`: invalid positional argument count.
- `E_IDENTIFIER_INVALID`: user-defined name violates naming rules.
- `E_DUPLICATE_SYMBOL`: duplicate input/output/transform/import symbol.
- `E_IMPORT_ALIAS_REQUIRED`: import lacks alias.
- `E_IMPORT_CYCLE`: import graph contains a cycle.
- `E_IMPORT_KIND`: imported file is not a module document.
- `E_REMOTE_IMPORT_UNSUPPORTED`: import path is not relative.
- `E_INPUT_UNDECLARED`: URL template references unknown input.
- `E_INPUT_REQUIRED_DEFAULT`: required input declares default.
- `E_INPUT_MISSING`: required runtime input is absent.
- `E_TEMPLATE_INVALID`: malformed URL template or unmatched braces.
- `E_TEMPLATE_OPTIONAL_INPUT`: omitted optional input without default is required by URL template.
- `E_TYPE_UNKNOWN`: invalid type expression.
- `E_TYPE_MISMATCH`: value is not assignable to expected type.
- `E_NUMERIC_OVERFLOW`: numeric result exceeds target range.
- `E_NON_FINITE_NUMBER`: value is NaN or infinity.
- `E_TRANSFORM_UNKNOWN`: transform reference cannot be resolved.
- `E_TRANSFORM_SHADOWS_BUILTIN`: declaration collides with built-in.
- `E_TRANSFORM_TYPE_MISMATCH`: transform pipeline types are incompatible.
- `E_TRANSFORM_ARGUMENT`: call properties/arguments do not match signature.
- `E_MATCH_DEFAULT`: match has zero or multiple defaults.
- `E_MATCH_DUPLICATE_CASE`: duplicate match input value.
- `E_VALUE_SOURCE_MISSING`: field has no value source.
- `E_VALUE_SOURCE_MULTIPLE`: field has multiple value sources.
- `E_SELECTOR_REQUIRED`: value source requires select.
- `E_SELECTOR_FORBIDDEN`: select is invalid for chosen JS scope.
- `E_SELECTOR_INVALID`: malformed CSS selector.
- `E_SELECTOR_UNSUPPORTED`: selector lies outside portable profile.
- `E_SELECTOR_CARDINALITY`: `match="one"` selected more than one element.
- `E_CURRENT_SCOPE_UNAVAILABLE`: current JS scope has no element.
- `E_ATTRIBUTE_NAME_REQUIRED`: attr source lacks valid name.
- `E_FIELD_TYPE_REQUIRED`: field lacks output type.
- `E_DEFAULT_INVALID`: default conflicts with field type or required semantics.
- `E_ERROR_POLICY_INVALID`: error policy conflicts with type/default.
- `E_COLLECTION_EMPTY_SCHEMA`: collection has no output members.
- `E_COLLECTION_BOUNDS`: invalid min/max/required combination.
- `E_COLLECTION_CARDINALITY`: extracted row count violates bounds.
- `E_REQUIRED_VALUE_MISSING`: required field source is missing.
- `E_BROWSER_CAPABILITY_REQUIRED`: browser-only node used in HTTP mode.
- `E_BROWSER_RUNTIME_MISSING`: browser adapter absent.
- `E_BROWSER_CAPABILITY_MISSING`: adapter lacks required operation.
- `E_SESSION_REQUIRED`: required session absent.
- `E_JAVASCRIPT_DISABLED`: JS exists but runtime opt-in is false.
- `E_JAVASCRIPT_NOT_CALLABLE`: script does not evaluate to callable function.
- `E_JAVASCRIPT_RESULT_TYPE`: JS result is non-JSON or incompatible with `returns`.
- `E_TIMEOUT`: workflow or JS operation timed out.
- `E_EXTERNAL_TRANSFORM_MISSING`: host registry lacks symbol.
- `E_REGEX_INVALID`: regex is malformed or outside RE2 profile.

## Runtime errors

The following codes are emitted by an interpreter or browser adapter after static validation succeeds:

- `E_IR_INVALID`: validated IR is malformed or internally inconsistent.
- `E_INPUT_REQUIRED`: required runtime input is absent.
- `E_INPUT_UNKNOWN`: runtime input is not declared.
- `E_INPUT_TYPE`: runtime input does not match its declared type.
- `E_INPUT_DEFAULT`: encoded input default cannot be decoded.
- `E_URL_INVALID`: expanded source URL is invalid.
- `E_URL_POLICY`: host URL policy rejected the initial target or an HTTP redirect.
- `E_SESSION_REQUIRED`: required runtime session is absent.
- `E_HTTP_REQUEST`: HTTP request construction failed.
- `E_HTTP_FETCH`: HTTP transport failed.
- `E_HTTP_STATUS`: response status is outside the accepted range.
- `E_HTTP_READ`: response body read failed.
- `E_HTTP_BODY_TOO_LARGE`: response exceeds configured body limit.
- `E_HTML_CHARSET_UNSUPPORTED`: response charset has no configured decoder.
- `E_HTML_DECODE`: response decoding failed.
- `E_HTML_PARSE`: HTML parsing failed.
- `E_BROWSER_MODE_REQUIRED`: browser execution was requested for a non-browser source.
- `E_BROWSER_ACQUIRE`: browser adapter lease acquisition failed.
- `E_BROWSER_NAVIGATE`: navigation failed.
- `E_BROWSER_WORKFLOW`: a workflow operation failed.
- `E_BROWSER_QUERY`: browser DOM query failed.
- `E_BROWSER_READ`: browser DOM value read failed.
- `E_JAVASCRIPT_EVALUATION`: JavaScript evaluation failed or rejected.
- `E_FIELD_EXECUTION`: field execution failed without a more specific code.
- `E_TRANSFORM_MISSING`: referenced declared transform is absent from IR.
- `E_TRANSFORM_RECURSION`: declared transform recursion was detected.
- `E_TRANSFORM`: transform execution failed without a more specific code.
- `E_EXTERNAL_TRANSFORM`: external transform returned an error.
- `E_EXTERNAL_TRANSFORM_RESULT_TYPE`: external transform returned a value incompatible with its declared output type.
- `E_EXECUTION_CANCELED`: execution was canceled before or during fetch, navigation, workflow, query, read, JavaScript evaluation, parsing, transform execution, or output traversal.
- `E_OUTPUT_TYPE`: runtime value does not match the field output type.

## Warnings

- `W_JAVASCRIPT_PRESENT`: static inspection notes trusted-code capability.
- `W_ROW_SKIPPED`: collection row dropped by `on-row-error="skip"`.
- `W_ERROR_RECOVERED`: `on-error="warn"` recovered an execution error.
- `W_PARTIAL_EXTRACTION`: summary warning when partial result is returned.
- `W_SELECTOR_MATCHES_ZERO`: dynamic inspector-only warning for optional source.
- `W_SELECTOR_MATCHES_MULTIPLE`: dynamic inspector-only warning when `match="first"` sees multiple elements.
