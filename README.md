# scrape-kdl

`scrape-kdl` is a Go reference implementation for declaring HTML extraction in KDL, validating it into a language-neutral IR, and executing it through HTTP or a live browser adapter.

Status: pre-release, language specification v0.1 working draft, implementation milestone M5.

```text
KDL source
  -> parser
  -> semantic validation and type checking
  -> Validated IR
  -> HTTP runtime or browser adapter
  -> structured extraction result
```

The full source tree is being initialized from the M5 release-hardening bundle.
