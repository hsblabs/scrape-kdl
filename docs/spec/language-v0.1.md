# Scraping KDL Language Specification v0.1

Status: Working Draft

## Scope

Scraping KDL is an application DSL built on KDL 2.0.0. It defines resource acquisition, browser preparation, DOM selection, value extraction, transformation, validation, and language-neutral IR generation.

KDL 2.0.0 defines lexical syntax and the generic node data model. This specification defines the allowed application nodes, properties, arguments, ordering rules, type system, validation phases, and execution semantics.

A conforming implementation MUST reject a syntactically valid KDL document that violates this specification.

## Normative terms

The words MUST, MUST NOT, REQUIRED, SHOULD, SHOULD NOT, and MAY are normative.

## Base KDL restrictions

Scraping KDL v0.1 adds these restrictions:

- documents MUST be UTF-8;
- invalid UTF-8 MUST be rejected as `E_KDL_SYNTAX` at the zero-width start span before syntactic parsing;
- KDL type annotations MUST NOT be used;
- KDL slashdash (`/-`) suppression is supported and suppressed syntax is removed before Scraping KDL semantic validation;
- unknown nodes and properties MUST be rejected;
- a node MUST NOT contain duplicate property names, even though generic KDL permits rightmost-property override;
- node and property names defined by this specification use kebab-case;
- user-defined input, field, collection, transform, and import alias names MUST match `[a-z][a-z0-9_]*`;
- extractor and module names MUST match `[a-z][a-z0-9-]*`.

Canonical source layout uses two-space indentation, one node per line, KDL booleans `#true/#false`, KDL null `#null`, and raw multiline strings for JavaScript. This describes the preferred representation and the output of KDL Writing; it does not define a lossless formatter for existing source.

## Compatibility identifiers

The language contract defined by this document series is identified by the opaque calendar-date string `2026-07-15`.
Every extractor and transform module root MUST declare both `version` and `language-version` properties:

- `version` identifies that document revision and MUST be a real calendar date in the exact `YYYY-MM-DD` form;
- `language-version` selects the language contract and MUST be `2026-07-15`;
- implementations MUST reject missing, malformed, or unknown identifiers without ordering dates or assuming compatibility between them;
- a document `version` MAY advance independently without changing `language-version`.

The `v0.1` suffix in specification filenames identifies the document series; it is not an accepted serialized language identifier.

## Document kinds

A document MUST be exactly one of:

- extractor document;
- transform module document.

Both document kinds MAY contain `import` nodes before the root node.

### Extractor document

```kdl
import "./modules/common.kdl" as="common"

extractor "race-detail" version="2026-07-15" language-version="2026-07-15" {
  // ...
}
```

It MUST contain exactly one top-level `extractor` node and MUST NOT contain a top-level `module` node.

### Transform module document

```kdl
module "racing-common" version="2026-07-15" language-version="2026-07-15" {
  transform "extract_horse_id" input="string" output="string?" {
    pipeline {
      apply "regex-capture" pattern=#"/horse/([^/?#]+)"# group=1
    }
  }
}
```

It MUST contain exactly one top-level `module` node and MUST NOT contain a top-level `extractor` node.

All transforms declared directly in a module are exported. Re-export is not supported in v0.1.

## Imports

```kdl
import "./modules/common.kdl" as="common"
```

Rules:

- the path MUST be relative;
- `as` is REQUIRED and MUST be a valid unique alias;
- the target MUST be a transform module document;
- imports are resolved relative to the importing file;
- import loading is host-injected in the core TypeScript API and MUST NOT imply network access;
- cyclic imports MUST be rejected;
- remote URL imports MUST be rejected;
- imported transforms are referenced as `alias.transform_name`;
- local transforms are referenced as `transform_name`;
- import aliases are file-local and are not re-exported.

## Extractor

```kdl
extractor "race-detail" version="2026-07-15" language-version="2026-07-15" {
}
```

Rules:

- name MUST be non-empty and match the extractor name grammar;
- `version` and `language-version` MUST satisfy the compatibility identifier rules above;
- exactly one `source` child is REQUIRED;
- zero or more `input`, `transform`, `field`, and `collection` children are allowed;
- these child categories MAY appear in any source order;
- input names MUST be unique;
- local transform names MUST be unique and MUST NOT collide with built-in names;
- field and collection names share one output namespace and MUST be unique within their immediate parent object.

## Module

```kdl
module "common-transforms" version="2026-07-15" language-version="2026-07-15" {
}
```

Rules:

- name MUST match the module name grammar;
- `version` and `language-version` MUST satisfy the compatibility identifier rules above;
- only `transform` children are allowed;
- transform names MUST be unique and MUST NOT collide with built-in names.

## Source

```kdl
source "html" {
  fetch mode="http" url="https://example.com/items/{item_id}"
  session policy="optional"
}
```

`source` accepts only the argument `"html"` in v0.1.

It MUST contain exactly one `fetch` child, MAY contain one `session`, and MAY contain one `workflow` when fetch mode is `browser`.

## Fetch

```kdl
fetch mode="browser" url="https://example.com/race/{race_id}"
```

Properties:

- `mode`: REQUIRED, `http` or `browser`;
- `url`: REQUIRED, non-empty URL template.

### URL template

A placeholder is `{input_name}`. Placeholder names MUST reference declared inputs.

Expansion rules:

- input scalar values are converted to their canonical string form;
- strings are UTF-8 percent-encoded, preserving only RFC 3986 unreserved characters;
- booleans expand as `true` or `false`;
- integers and finite floats use base-10 canonical form;
- `{{` emits a literal `{` and `}}` emits a literal `}`;
- an omitted optional input referenced by the URL template is a runtime input error unless the input declares a default;
- URL expansion occurs before navigation or HTTP fetch;
- the expanded URL MUST be absolute and use the `http` or `https` scheme.

## Fetch modes

### HTTP mode

The runtime fetches the expanded URL, decodes the response using HTML encoding sniffing with HTTP charset metadata, parses the document with HTML tree-construction semantics, and evaluates all selectors against that static DOM.

The following are forbidden:

- `workflow`;
- `evaluate-js`;
- browser capabilities.

### Browser mode

The runtime MUST receive a browser adapter before extraction starts. The adapter owns navigation, workflow execution, selector queries, text/HTML/attribute reads, and JavaScript evaluation against the live DOM.

A browser-mode implementation MUST NOT serialize the page and then attempt to re-associate static parser nodes with browser element handles.

Missing adapter capability MUST fail during capability validation before navigation.

## Session

```kdl
session policy="optional"
```

`policy` MUST be one of:

- `none`;
- `optional`;
- `required`.

Default: `none`.

Semantics:

- `none`: the explicitly supplied runtime session input is ignored;
- `optional`: runtime session is used when supplied;
- `required`: missing runtime session fails before fetch/navigation.

The concrete cookie, storage, authentication, and header representation is runtime-specific. Host-owned ambient state, including an HTTP client cookie jar or an existing browser context, is not runtime session input and is not cleared by `none`. A host that requires credential-free execution MUST supply an isolated stateless client or browser context.

## Inputs

```kdl
input "race_id" type="string" required=#true
input "lang" type="string" required=#false default="ja"
```

Properties:

- `type`: REQUIRED primitive scalar type;
- `required`: optional, default `#true`;
- `default`: optional scalar compatible with `type`.

Rules:

- a required input MUST NOT declare `default`;
- an optional input MAY declare `default`;
- missing required input fails before fetch/navigation;
- defaults are applied before URL expansion.

Supported input types: `string`, `bool`, `int`, `float`.

## Workflow

A workflow executes after navigation and before output extraction. Steps execute sequentially in source order.

```kdl
workflow {
  wait-for ".content" state="visible" timeout-ms=5000
  click ".load-more" timeout-ms=3000
  wait-for-network-idle idle-ms=500 timeout-ms=5000
}
```

Supported steps:

```text
wait-for selector [state] [timeout-ms]
click selector [timeout-ms]
fill selector value [timeout-ms]
press selector key [timeout-ms]
scroll x y
wait-for-network-idle [idle-ms] [timeout-ms]
evaluate-js script [timeout-ms]
```

Detailed rules:

- `wait-for` state: `attached`, `visible`, `hidden`, `detached`; default `visible`;
- `scroll` uses numeric positional arguments `x` and `y` in CSS pixels and performs a window-relative scroll;
- `wait-for-network-idle` means no tracked HTTP request is active for `idle-ms`; WebSocket and EventSource connections are excluded; `idle-ms` defaults to 500;
- workflow `evaluate-js` script MUST evaluate to a callable function; async functions are allowed; resolved return value is discarded;
- `timeout-ms` and `idle-ms` MUST be between 1 and 9,223,372,036,854 milliseconds, inclusive;
- timeout expiry is an extraction error;
- workflow steps are browser-only.

## Output members

An output object contains fields and collections in source order. Generated serializers SHOULD preserve that order when the target format supports it.

## Fields

```kdl
field "horse_name" type="string" required=#true {
  select ".horse-name a" match="one"
  value "text"
  apply "normalize-whitespace"
}
```

Properties:

- `type`: REQUIRED successful output type;
- `required`: optional, default `#false`;
- `default`: optional scalar compatible with final output type.

Children:

- zero or one `select`;
- exactly one value source: `value` or `evaluate-js`;
- zero or more `apply`, executed in source order;
- zero or one `on-error`.

### Missing value semantics

A missing value means:

- selector `match="one"` or `match="first"` found zero elements;
- selected attribute does not exist;
- selected text/HTML source cannot be read because the selected element is absent.

Rules:

- `required=#true`: missing value is `E_REQUIRED_VALUE_MISSING`;
- `required=#false` with `default`: emit the default, no warning, `partial=#false`;
- `required=#false` without `default`: emit `null`, no warning, `partial=#false`;
- the effective generated type is nullable when an optional field has no default, even if the successful type is non-nullable.

`required` controls missing-value behavior only. Transform errors, JavaScript errors, type mismatches, and adapter failures are controlled by `on-error`.

### Select

```kdl
select ".horse-name a" match="one"
```

`match` MUST be:

- `one`: exactly one match is required; zero is missing, more than one is `E_SELECTOR_CARDINALITY`;
- `first`: first match in document order is selected; zero is missing.

Default: `one`.

A `select` child is REQUIRED for `value` sources.

For `evaluate-js`:

- with `scope="document"`, `select` MUST NOT be present;
- with `scope="current"`, `select` MAY be present;
- when `scope="current"` and `select` exists, the selected element is current;
- when `scope="current"` and `select` is absent, the enclosing collection row is current;
- top-level `scope="current"` without `select` is invalid.

## Collections

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

Properties:

- `required`: optional, default `#false`;
- `min-items`: optional non-negative integer;
- `max-items`: optional non-negative integer;
- `on-row-error`: `fail` or `skip`, default `fail`.

Rules:

- exactly one `select` is REQUIRED;
- all matching elements in document order become rows;
- at least one child field or collection is REQUIRED;
- nested collections are allowed;
- `max-items` MUST be greater than or equal to `min-items`;
- `required=#true` is equivalent to an effective minimum of at least one;
- cardinality validation occurs after row-error handling;
- `on-row-error="skip"` drops a row only when a child produces an unrecovered error; dropped rows emit a warning and set `partial=#true`.

## Value sources

### Text

```kdl
value "text"
```

Returns concatenated descendant text node content in DOM order. It does not trim or normalize whitespace. A `<br>` element does not implicitly insert a newline.

### HTML

```kdl
value "html"
```

Returns the selected element's inner HTML using the runtime's conforming HTML fragment serialization. Attribute ordering MUST NOT be relied upon by conformance tests.

### Attribute

```kdl
value "attr" name="href"
```

`name` is REQUIRED and non-empty. It returns the DOM attribute value, not a browser-resolved property. For an absolute link use `evaluate-js` or `url-resolve`.

### JavaScript evaluation

```kdl
evaluate-js #"""
  () => window.__INITIAL_STATE__?.race ?? null
  """# scope="document" returns="object?" timeout-ms=3000
```

Properties:

- `scope`: REQUIRED, `document` or `current`;
- `returns`: REQUIRED type expression for the raw JavaScript result;
- `timeout-ms`: optional integer from 1 through 9,223,372,036,854 milliseconds.

Invocation:

- `document`: callable receives no positional argument;
- `current`: callable receives the current browser element as its first argument.

The script MUST evaluate to a callable JavaScript function. Async functions are allowed.

The resolved return value MUST be JSON-compatible:

- `null`, boolean, string, finite number;
- arrays of JSON-compatible values;
- plain objects with string keys and JSON-compatible values.

`undefined`, `NaN`, infinities, bigint, symbol, function, DOM node, runtime handle, cyclic object, Map, Set, and Date are forbidden.

JavaScript execution requires explicit runtime opt-in. Capability validation MUST fail with `E_JAVASCRIPT_DISABLED` before navigation when opt-in is absent.

## Type system

### Primitive types

```text
string
bool
int
u8 u16 u32 u64
i8 i16 i32 i64
float f32 f64
object
unknown
```

`object` means a JSON object with string keys and `unknown` values. `unknown` means any JSON-compatible value.

### Container and nullable types

```text
T[]
T?
```

Examples: `string[]`, `u8?`, `object?`, `string?[]`, `(string[])?`.

Postfix operators bind from left to right. Therefore `string?[]` means an array of nullable strings, while `string[]?` means a nullable array. Parentheses MAY be used for clarity. Nested nullable wrappers such as `string??` are invalid and nullable lifting is idempotent.

Rules:

- null is assignable only to nullable types and `unknown`;
- arrays preserve order;
- integer overflow is an extraction error;
- all floats MUST be finite;
- field `type` is the successful final type before optional-missing lifting;
- optional-missing lifting wraps the successful type in nullable only when it is not already nullable;
- final pipeline output MUST be assignable to field `type`;
- no implicit string-to-number or numeric-width conversion occurs.

## Transforms

### Declared transform

```kdl
transform "extract_horse_id" input="string" output="string?" {
  pipeline {
    apply "regex-capture" pattern=#"/horse/([^/?#]+)"# group=1
  }
}
```

A transform MUST contain exactly one body node:

- `pipeline`;
- `match`;
- `external`.

### Pipeline

```kdl
pipeline {
  apply "trim"
  apply "parse-int" as="u16"
}
```

A pipeline MUST contain one or more `apply` nodes. Calls execute in source order. Each call's input type MUST accept the previous call's output type.

### Apply and symbol resolution

```kdl
apply "normalize-whitespace"
apply "common.extract_horse_id"
```

Resolution order:

- exact built-in name;
- exact local transform name;
- qualified imported transform name.

Unqualified imported transform references are forbidden. Built-in names cannot be shadowed.

Call positional arguments and properties are validated against the target signature. In v0.1, declared and external transforms take no call arguments or properties; parameterized behavior belongs to built-ins or a separate declared transform.

### Match transform

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

Rules:

- input and output MUST be scalar or nullable scalar types;
- cases are evaluated in source order with exact scalar equality;
- case input values MUST be assignable to transform input type;
- case result and default values MUST be assignable to transform output type;
- exactly one `default` is REQUIRED;
- duplicate case input values are forbidden.

### External transform

```kdl
transform "decrypt_payload" input="string" output="object" {
  external symbol="decrypt_payload"
}
```

External transforms are host functions resolved from a runtime registry. They do not receive browser handles in v0.1. A missing symbol fails capability validation before fetch/navigation. After a callback returns successfully, its result MUST be checked immediately against the transform's declared output type. A mismatch fails with `E_EXTERNAL_TRANSFORM_RESULT_TYPE` before any downstream transform runs.

## Built-in transforms

The normative built-in registry is defined in `builtins-v0.1.md`. Implementations MUST NOT change signatures or semantics under language version `2026-07-15`.

Regex-taking built-ins MUST use the portable RE2 profile defined there. Native JavaScript RegExp semantics are not the language semantics.

## Error recovery

```kdl
on-error "warn"
```

Policies:

- `fail`: propagate the error;
- `null`: emit null and set `partial=#true`;
- `warn`: emit null, append a warning, and set `partial=#true`;
- `default`: emit field default and set `partial=#true`.

Defaults:

- required field: `fail`;
- optional field: `null`.

Constraints:

- `null` and `warn` require an effective nullable output;
- `default` requires a field default;
- `on-error` does not handle missing selectors or missing attributes;
- collection row dropping is controlled only by `on-row-error`.

## Extraction result

Logical result:

```text
value       object from fields and collections
warnings    ordered warning diagnostics
partial     boolean
```

`partial` is true only when:

- an execution error was recovered by `null`, `warn`, or `default`;
- a collection row was dropped.

Expected optional missing does not set partial.

Warnings MUST be ordered by execution order, then source location for diagnostics produced before execution.

## Validation phases

A conforming implementation performs:

1. KDL syntax parsing;
2. base restriction validation;
3. application grammar validation;
4. import graph resolution;
5. symbol resolution;
6. type checking;
7. capability derivation and validation;
8. optional dynamic validation against HTML or browser page.

Phases 1–7 MUST complete before any network request, browser launch, navigation, session mutation, or external transform invocation.

## Capability model

Validated IR contains a deterministic sorted set of required capabilities.

Initial capability identifiers:

```text
http.fetch
browser.navigate
browser.query
browser.read-text
browser.read-html
browser.read-attr
browser.wait
browser.input
browser.scroll
browser.network-idle
browser.evaluate-js
transform.external:<symbol>
```

Code generators MUST preserve these requirements in generated bindings or manifests.

## Portability

- selectors MUST conform to `selectors-v0.1.md`;
- regex MUST conform to the RE2 profile in `builtins-v0.1.md`;
- browser JavaScript and external transforms are explicit portability escape hatches;
- HTTP and browser implementations MUST pass the same static DOM conformance fixtures for non-JavaScript extraction;
- browser-only behavior requires separate integration fixtures.

## Diagnostics

Stable diagnostic codes and ordering are defined in `diagnostics.md`.
