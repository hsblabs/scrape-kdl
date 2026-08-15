---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: パターン
description: Scraping KDL で複数ページを抽出する方法。一覧と詳細の対、ページ送りの停止条件、そして呼び出し側に残る間隔制御と再試行。
hsblabs:
  sidebar:
    order: 28
---

1 回の抽出は 1 ドキュメントを扱う。
言語にループはない。
リンクをたどらず、リクエストを繰り返さず、クロールの終わりも判断しない。
その論理は呼び出し側のアプリケーションが制御し、ページごとに小さな抽出器を呼ぶ。

この境界は意図的なものである。
取得方針、間隔制御、再試行、重複の除去、チェックポイントを、目に見える自分のコードに残せる。
一覧の抽出器と詳細の抽出器を、それぞれ独立に開発しテストすることもできる。

Go と TypeScript のループを含む全体は [patterns.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/patterns.md) にある。

## 一覧と詳細

一覧のプログラムは項目ごとに二つの値を返す。
絶対 URL と、詳細のプログラムが必要とする安定した識別子である。

二つとも必要である。
URL があれば、対象を目で確認できる。
識別子は誤った操作を防ぐ。
完全な URL をテンプレートのプレースホルダに入れるとパーセントエンコードされ、同じページを指さなくなるからである。

次のドキュメントを `list.kdl` として保存する。

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

次のドキュメントを `detail.kdl` として保存する。

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

この対は、デコード済みパスの最後のセグメントが詳細ページを識別すること、そして詳細のプログラムのテンプレートが同じ対象を再構成することを前提にしている。
ホスト、クエリ、あるいは複数のパスセグメントが意味を持つ場合は、その部分を別々の input として宣言する。
ログに書いた URL と実際に取得する URL が食い違う状態は作らない。

## ループ

ループは二つのプログラムの外に置く。
`--json` オプションは `jq` で扱える安定したエンベロープを返す。
`--input` の引数は全体を引用符で囲む。
デコード済みの識別子には空白やシェルのメタ文字が入り得るからである。

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

この例は、詳細の結果ごとに JSON ドキュメントを一つ書き出す。
実運用の呼び出し側は、各ドキュメントを永続ストレージへ書き、ページと `detail_url` を記録してから次へ進む。

## 停止条件

対象に合う停止の契約を選ぶ。

- `min-items=0` のままにして、コレクションが空になったら止める。空のページが正常な終端の印である場合に使う。
- 任意の `next_url` を用意し、その値が `null` になったら止める。上のループはこの契約を使っている。対象が next リンクと安定したページ番号の両方を持つなら、数値の input `page` も併用できる。
- 空のページが正常でないなら `min-items=1` を設定する。最後のページはパス `output.items` で `E_COLLECTION_CARDINALITY` として失敗する。呼び出し側は、そのコードとパスの組を終端の印として使える。

三つ目の契約では、止まる前にコードを検査する。
どの失敗もページ送りの終わりとして扱ってはならない。

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

ページ数か項目数の上限は必ず設ける。
仕様が変わったサイトは、終わりのない同じ next リンクを返し続けることがある。
このドキュメントのループはいずれも 100 ページで止まる。
繰り返しを正常な完了とは見なさない。

## 間隔制御と再試行

Scraping KDL は遅延を入れず、失敗したページを再取得せず、重複 URL を除かず、クロールの進捗も保持しない。
これらの方針は呼び出し側のアプリケーションが供給する。
安全でない作業を繰り返さずに再開できるだけの状態も、呼び出し側が保持する。

再試行するのは、対象との取り決め上、一時的と分類されるエラーだけにする。
上限付きのバックオフを使い、キャンセルを保つ。
例に置いた 1 秒の遅延は、間隔制御を入れる位置を示しているにすぎない。
実運用の方針は、全リクエストを対象にし、サーバの指示に従い、並列度と遅延を対象に合わせて調整するものになる。

自分で運用していないサービスにこれらのパターンを使う前に、[セキュリティと責任ある利用](./security-and-responsible-use.md) を読むこと。

## 次に読むもの

- [オフラインスナップショット](./offline-snapshots.md)：ネットワークなしで二つのプログラムをテストする方法。
- [Go](../golang/compile-and-extract.md) または [TypeScript と Bun](../npm/compile-and-extract.md)：ライブラリで同じループを書く方法。
