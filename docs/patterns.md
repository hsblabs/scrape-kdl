# Multi-page extraction patterns

One scrape-kdl extraction acquires one document. The language does not loop,
follow links, retry requests, or decide when a crawl is complete. The caller
owns that control flow and invokes small extractors for each page.

This boundary keeps acquisition policy, pacing, retry, deduplication, and
checkpointing visible in application code. It also lets a list extractor and a
detail extractor evolve and test independently.

## List to detail

The list program emits both an absolute detail URL and the stable path segment
needed by the detail program. Emitting the URL makes the discovered target
auditable; passing the ID avoids treating a complete URL as a URL-template
placeholder, whose value would be percent-encoded.

This particular pair assumes that the final decoded path segment uniquely
identifies a detail page and that the detail template reconstructs the same
target as `detail_url`. If host, query, or multiple path segments are
significant, model those stable components as separate inputs or use a
different application-owned target mapping. Never log one URL and silently
fetch a different one.

Save this as `list.kdl`:

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

Save this as `detail.kdl`:

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

The CLI can keep the loop outside both programs. `--json` gives `jq` a stable
success envelope, and quoting the entire `--input` argument preserves spaces or
shell metacharacters in decoded IDs.

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

This example prints one JSON document per detail result. A production caller
normally writes each document to durable storage and records its page and
`detail_url` before continuing.

## Pagination stop conditions

Choose a stop contract that matches the target:

- Keep `min-items=0` and stop when the returned collection is empty. This is
  suitable when an empty page is a normal end marker.
- Emit an optional `next_url` and stop when it is `null`. The caller can still
  use a numeric `page` input when the target exposes both a next link and stable
  page numbers. The CLI and API loops in this document choose this contract.
- Set the collection to `min-items=1` when an empty page is exceptional. The
  final page then fails with `E_COLLECTION_CARDINALITY` at `output.items`; a
  caller may treat that exact code and path as its explicit end marker. Do not
  swallow other execution errors as pagination completion.

For the last variant, a CLI loop distinguishes the declared end marker from
every other failure before deciding to stop:

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

The caller must also set a maximum page or item count. A changing site can
otherwise repeat a next link indefinitely. Every loop below fails after 100
pages instead of assuming that repetition means successful completion.

## Go API loop

After compiling `list.kdl` and `detail.kdl` into `listProgram` and
`detailProgram`, keep pagination and typed result validation in the host:

```go
type listPage struct {
	NextURL *string `json:"next_url"`
	Items   []struct {
		DetailURL string `json:"detail_url"`
		DetailID  string `json:"detail_id"`
	} `json:"items"`
}

waitForRequest := func() error {
	timer := time.NewTimer(time.Second)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

completed := false
for page := int64(1); page <= 100; page++ {
	if err := waitForRequest(); err != nil {
		return err
	}
	result, err := listProgram.Extract(ctx, map[string]any{"page": page}, options)
	if err != nil {
		return err
	}
	var decoded listPage
	if err := result.Decode(&decoded); err != nil {
		return err
	}
	for _, item := range decoded.Items {
		log.Printf("extracting %s", item.DetailURL)
		if err := waitForRequest(); err != nil {
			return err
		}
		if _, err := detailProgram.Extract(ctx, map[string]any{"item_id": item.DetailID}, options); err != nil {
			return err
		}
	}
	if decoded.NextURL == nil {
		completed = true
		break
	}
}
if !completed {
	return errors.New("pagination exceeded 100 pages")
}
```

## TypeScript API loop

TypeScript callers should validate the dynamic result with their application
schema before using it. Here `decodeListPage` denotes that schema boundary:

```ts
const waitForRequest = (signal?: AbortSignal): Promise<void> =>
  new Promise((resolve, reject) => {
    if (signal?.aborted) {
      reject(signal.reason);
      return;
    }
    const onAbort = () => {
      clearTimeout(timer);
      reject(signal?.reason);
    };
    const timer = setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, 1_000);
    signal?.addEventListener("abort", onAbort, { once: true });
  });
let completed = false;

for (let page = 1; page <= 100; page++) {
  await waitForRequest(options.signal);
  const result = await listProgram.extract({ page }, options);
  const decoded = decodeListPage(result.value);

  for (const item of decoded.items) {
    console.error(`extracting ${item.detail_url}`);
    await waitForRequest(options.signal);
    await detailProgram.extract({ item_id: item.detail_id }, options);
  }

  if (decoded.next_url === null) {
    completed = true;
    break;
  }
}
if (!completed) throw new Error("pagination exceeded 100 pages");
```

Do not replace `decodeListPage` with an unchecked type assertion at an external
data boundary. The extractor validates its declared result types; the host
still owns the correspondence between dynamic field names and its TypeScript
model.

## Pacing, retry, and resumption

scrape-kdl does not add delays, retry failed pages, deduplicate discovered URLs,
or persist crawl progress. The caller must implement those policies and retain
enough state to resume without repeating unsafe work. Retry only errors that
the target and application contract classify as transient, use bounded backoff,
and preserve cancellation. The fixed one-second spacing above only demonstrates
where caller-owned pacing belongs; production policy should cover every request,
honor server guidance, and adapt concurrency and delay to the target.

Follow the project's [responsible-use guidance](responsible-use.md): confirm
authorization, honor target policies, identify the client honestly, and limit
request rate and concurrency.
