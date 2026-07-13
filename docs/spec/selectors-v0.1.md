# Portable CSS Selector Profile v0.1

Scraping KDL selectors are a portability subset intended to work consistently in browser DOM APIs and non-browser Go/TypeScript HTML runtimes.

## Supported

- universal selector: `*`
- type selector: `div`
- ID selector: `#main`
- class selector: `.entry`
- attribute presence: `[href]`
- attribute operators: `=`, `~=`, `|=`, `^=`, `$=`, `*=`
- combinators: descendant, child `>`, adjacent sibling `+`, general sibling `~`
- selector lists separated by comma
- pseudo-classes:
  - `:first-child`
  - `:last-child`
  - `:only-child`
  - `:empty`
  - `:first-of-type`
  - `:last-of-type`
  - `:only-of-type`
  - `:nth-child(An+B)`
  - `:nth-last-child(An+B)`
  - `:nth-of-type(An+B)`
  - `:nth-last-of-type(An+B)`
  - `:not(compound-selector)`

## Unsupported in v0.1

- pseudo-elements
- `:has()`
- `:is()`
- `:where()`
- `:scope`
- shadow DOM selectors
- browser-vendor pseudo selectors
- namespace selectors
- CSS escape sequences in identifiers or string tokens
- attribute selector case-sensitivity flags (`i` / `s`)
- stateful UI pseudo-classes such as `:hover`, `:focus`, `:visited`, and `:checked`

## Semantics

- matching follows DOM tree order;
- selector lists are de-duplicated and returned in document order;
- element and attribute names follow HTML ASCII case-insensitive rules for HTML documents;
- attribute values are compared according to CSS selector rules;
- unsupported syntax MUST fail static validation with `E_SELECTOR_UNSUPPORTED`;
- syntactically malformed selectors MUST fail with `E_SELECTOR_INVALID`.

Implementations MAY use a broader selector engine internally, but MUST reject selectors outside this profile under language version 0.1.
