---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: Patterns
description: How to extract more than one page with Scraping KDL — the list-to-detail pair, the stop conditions of the pagination, and the pacing and the retry that stay in your application.
hsblabs:
  sidebar:
    order: 28
---

One extraction gets one document. The language has no loop. It does not follow a link, it does not repeat a request, and it does not decide when a crawl is complete. Your application controls that logic and calls a small extractor for each page.

This boundary is intentional. It keeps the acquisition policy, the pacing, the retry, the removal of the duplicates, and the checkpoint in your code, where you can see them. It also permits an independent development and an independent test of a list extractor and a detail extractor.

The full document, with the loops for Go and TypeScript, is in [patterns.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/patterns.md).

## The list and the detail

The list program gives two values for each item: an absolute URL, and the stable identifier that the detail program needs.

Two values are necessary. The URL makes the target visible for an examination. The identifier prevents a wrong operation: a full URL in a template placeholder becomes percent-encoded and then does not identify the same page.

Save this document as `list.kdl`:

```kdl
extractor "catalog-list" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/catalog?page={page}"
  }

  input "page" type="int" required=#true

  field "next_url" type="string?" required=#false {
    select "a.next" match="first"
    value "attr" name="href"
    apply "url-resolve" base="https://example.invalid/catalog"
  }

  collection "items" min-items=0 {
    select "article.item"

    field "detail_url" type="string" required=#true {
      select "a.detail" match="one"
      value "attr" name="href"
      apply "url-resolve" base="https://example.invalid/catalog"
    }

    field "detail_id" type="string" required=#true {
      select "a.detail" match="one"
      value "attr" name="href"
      apply "path-segment" index=-1
      apply "coalesce" value=""
      apply "assert-matches" pattern=".+"
    }
  }
}
```

Save this document as `detail.kdl`:

```kdl
extractor "catalog-detail" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/items/{item_id}"
  }

  input "item_id" type="string" required=#true

  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
    apply "normalize-whitespace"
  }
}
```

This pair assumes that the last decoded path segment identifies the detail page, and that the template of the detail program makes the same target again. If the host, the query, or more than one path segment is significant, declare those parts as separate inputs. Never write one URL in a log and then get a different URL.

## The loop

Keep the loop outside of the two programs. The option `--json` gives a stable envelope for `jq`. Put the full argument of `--input` in quotation marks, because a decoded identifier can contain a space or a metacharacter of the shell.

```bash
#!/usr/bin/env bash
set -euo pipefail

main() {
  local max_pages=100
  local page page_json next_url

  for ((page = 1; page <= max_pages; page++)); do
    sleep 1
    page_json="$(scrape-kdl extract ./list.kdl --input "page=$page" --json)"
    jq -e '.ok == true and (.result.value.items | type == "array")' >/dev/null <<<"$page_json"

    while IFS=$'\t' read -r detail_url detail_id; do
      printf 'extracting %s\n' "$detail_url" >&2
      sleep 1
      scrape-kdl extract ./detail.kdl --input "item_id=$detail_id" --json
    done < <(jq -r '.result.value.items[]? | [.detail_url, .detail_id] | @tsv' <<<"$page_json")

    next_url="$(jq -r '.result.value.next_url // empty' <<<"$page_json")"
    if [[ -z "$next_url" ]]; then
      return 0
    fi
  done

  printf 'pagination exceeded %d pages\n' "$max_pages" >&2
  return 1
}

main "$@"
```

This example writes one JSON document for each detail result. A production caller writes each document to a durable storage and records the page and the `detail_url` before it continues.

## The stop condition

Select a stop contract that agrees with the target:

- Keep `min-items=0` and stop when the collection is empty. Use this contract when an empty page is a normal end marker.
- Give an optional `next_url` and stop when its value is `null`. The loop above uses this contract. You can still use a numeric input `page` when the target has a next link and also stable page numbers.
- Set `min-items=1` when an empty page is not normal. The last page then fails with `E_COLLECTION_CARDINALITY` at the path `output.items`. Your caller can use that exact code and that exact path as the end marker.

For the third contract, examine the code before you stop. Do not treat each failure as the end of the pagination:

```bash
if ! page_json="$(scrape-kdl extract ./list.kdl --input "page=$page" --json)"; then
  if jq -e '.error.code == "E_COLLECTION_CARDINALITY" and .error.path == "output.items"' \
    >/dev/null <<<"$page_json"; then
    break
  fi
  printf '%s\n' "$page_json" >&2
  return 1
fi
```

Always set a maximum number of the pages or of the items. A site that changes can give the same next link without an end. Each loop in this document stops after 100 pages. It does not assume that a repetition is a successful completion.

## The pacing and the retry

Scraping KDL adds no delay, does not repeat a page that failed, does not remove a duplicate URL, and does not keep the progress of a crawl. Your application must supply these policies. It must also keep sufficient state to continue without a repetition of unsafe work.

Repeat only an error that your contract with the target classifies as temporary. Use a bounded backoff and keep the cancellation. The delay of one second in the example only shows you the position of the pacing. A production policy must cover each request, obey the guidance of the server, and adapt the concurrency and the delay to the target.

Refer to [Security and Responsible Use](./security-and-responsible-use.md) before you use these patterns on a service that you do not operate.

## Next step

- [Offline Snapshots](./offline-snapshots.md) — how to test the two programs without a network.
- [Go](../golang/compile-and-extract.md) or [TypeScript and Bun](../npm/compile-and-extract.md) — the same loop in a library.
