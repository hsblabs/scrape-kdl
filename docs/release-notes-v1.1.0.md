---
status: prepared; not published
version: v1.1.0
prepared-date: 2026-09-24
---

# scrape-kdl v1.1.0 release notes

Collection fields can now read the current row directly. Omit `select` inside
a collection to read that row's text, HTML, or attribute:

```kdl
collection "links" min-items=1 {
  select "a.item"
  field "href" type="string" required=#true {
    value "attr" name="href"
  }
}
```

The same source works in the Go and TypeScript HTTP, snapshot, and browser
runtimes. An explicit `select` continues to search descendants of the current
row. Top-level value fields still require `select`; existing extractors need no
changes.

This release is prepared for review. Publication requires the protected public
release workflow after the preparation PR is merged.
