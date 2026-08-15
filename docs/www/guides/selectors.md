---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Selectors
description: The portable CSS selector profile of Scraping KDL — what the compiler accepts, what it rejects, and why the same selector gives the same elements in HTTP mode and in browser mode.
hsblabs:
  sidebar:
    order: 22
---

Scraping KDL accepts a subset of CSS. The subset operates in the same manner in the internal DOM and in a live browser. The compiler rejects a selector outside of the subset. Thus you do not find the difference at execution time on one runtime only.

The full profile is in [selectors-v0.1.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/selectors-v0.1.md).

## What you can use

The profile contains the parts of CSS that each engine implements in the same manner:

- the universal selector `*`, a type selector such as `div`, an ID selector such as `#main`, and a class selector such as `.entry`;
- the attribute selectors: the presence form `[href]` and the operators `=`, `~=`, `|=`, `^=`, `$=`, and `*=`;
- the combinators: descendant, child `>`, adjacent sibling `+`, and general sibling `~`;
- a selector list with commas;
- the structural pseudo-classes: `:first-child`, `:last-child`, `:only-child`, `:empty`, `:first-of-type`, `:last-of-type`, `:only-of-type`, `:nth-child(An+B)`, `:nth-last-child(An+B)`, `:nth-of-type(An+B)`, `:nth-last-of-type(An+B)`, and `:not(compound-selector)`.

```kdl
select "table.entries tbody tr:not(.header)"
```

## What the compiler rejects

The compiler rejects these constructions with the diagnostic `E_SELECTOR_UNSUPPORTED`:

- each pseudo-element;
- `:has()`, `:is()`, `:where()`, and `:scope`;
- a shadow DOM selector and a vendor pseudo-selector;
- a namespace selector;
- a CSS escape sequence in an identifier or a string token;
- the case-sensitivity flags `i` and `s` in an attribute selector;
- a pseudo-class of the user interface state, such as `:hover`, `:focus`, `:visited`, and `:checked`.

A selector with bad syntax gets the different diagnostic `E_SELECTOR_INVALID`.

The rejection occurs at compile time, with the position of the character in the selector:

```text
extractor.kdl:9:5: error E_SELECTOR_UNSUPPORTED: selector byte 9: unsupported pseudo-class "has" [output.title.selection]
```

An implementation can use a larger selector engine internally. It must still reject each selector outside of the profile under the language version `2026-07-15`. Thus a program that compiles on one runtime also compiles on the other runtime.

## Why the profile is small

A pseudo-class of the user interface state, such as `:hover`, has a value only in a live browser. A structural pseudo-class gives the same result in each engine.

If the two classes were in one profile, a program could operate correctly in browser mode and fail in HTTP mode. The profile prevents this condition. Each accepted selector has the same meaning in the two modes.

## Alternatives to `:has()`

The pseudo-class `:has()` is the most frequent absent function. Use one of these methods:

- Select the parent element with a collection, then use a field in the row to find the child. A row without the child gives a missing value, and you control that condition with `required`.
- Select the child element directly when you need only its value. The parent element is frequently not necessary.
- In browser mode only, use `evaluate-js` with `scope="current"`. This is an intentional escape from the portable profile. It needs the capability `browser.evaluate-js` and an explicit opt-in for JavaScript.

## Cardinality

The property `match` of the node `select` controls the number of the elements:

| Value | Behavior |
| --- | --- |
| `one` (default) | Exactly one element. Zero elements is a missing value. Two or more elements causes `E_SELECTOR_CARDINALITY`. |
| `first` | The first element in document order. Zero elements is a missing value. |

A collection is different. Its `select` gives each element that agrees with the selector, in document order. One row comes from each element.

Use `match="one"` when the page must have exactly one element. The runtime then tells you when the structure of the page changed. Use `match="first"` only when more than one element is correct and you want the first element.

## Semantics

- The matches follow the tree order of the DOM.
- A selector list gives the elements in document order and has no duplicates.
- The names of the elements and the attributes follow the ASCII case-insensitive rules of HTML.

## Next step

- [Transforms](./transforms.md) — how to make a value from the selected text.
- [Browser Mode](./browser-mode.md) — the selector behavior with a live page.
