---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Overview
title: Scraping KDL
description: HTML 抽出のための宣言的言語とランタイム。抽出器を KDL で書き、ネットワーク操作の前にコンパイラが検査し、Go、Node.js、Bun から HTTP またはブラウザアダプタで実行する。
hsblabs:
  sidebar:
    order: 0
---

Scraping KDL は、HTML 抽出のための宣言的言語とランタイムである。
抽出器は [KDL](https://kdl.dev/) で書く。
コンパイラが import を解決し、型を検査し、言語非依存の **Validated IR** を生成する。
ランタイムはその IR を HTTP または実ブラウザで実行する。

対象は、抽出ルールをソース管理下に置いている開発者である。
コンパイラは、ランタイムがリクエストを送る前に誤りを見つける。

参照実装は Go、Node.js、Bun 向けに提供している。
ライセンスは Apache-2.0 である。

## 動作の流れ

```text
KDL source
  -> parser
  -> semantic validation and type checking
  -> Validated IR
  -> HTTP runtime or browser adapter
  -> structured result
```

コンパイラはリクエストを送らず、ブラウザを起動せず、外部トランスフォームも呼ばない。
これらが動くのは検証を通過したあとだけである。
不正なセレクタ、未知のトランスフォーム、型の不一致は、いずれもソース位置を伴う診断になる。

## 書くもの

```kdl
extractor "basic-http" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.invalid/{id}"
  }

  input "id" type="string" required=#true

  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
    apply "normalize-whitespace"
  }

  collection "items" min-items=1 {
    select "ul.items > li"
    field "value" type="u8" required=#true {
      select ".value" match="one"
      value "text"
      apply "trim"
      apply "parse-int" as="u8"
    }
  }
}
```

## 得られるもの

```json
{
  "value": {
    "items": [{ "value": 1 }, { "value": 2 }, { "value": 3 }],
    "title": "Scraping KDL Runtime"
  },
  "warnings": [],
  "partial": false
}
```

ランタイムは各値を宣言された型に照らして検査する。
`partial` フラグは、ランタイムがエラーから回復したかどうかを示す。
これにより、劣化した結果を正常な結果と取り違えることがない。

## 主な性質

- **コンパイラが先に動く**：セレクタ、トランスフォームのシグネチャ、ケイパビリティ、出力型を検査する。この段階を通過するまでプロセスからデータは出ない。
- **ブラウザはケイパビリティである**：ブラウザモードには利用者が渡すアダプタが必要になる。JavaScript はオプションを設定するまで無効のままである。
- **一つのプログラムを二つのランタイムで**：Go と TypeScript は同じドキュメントを受け付け、同じ診断と同じ値を返す。
- **可搬なセレクタ**：文書化された CSS の部分集合が、内部 DOM でも実ブラウザでも同じように動作する。
- **オフライン実行**：ワークフローも JavaScript も持たないプログラムは、保存済みの HTML ファイルに対して動作する。ネットワーク操作は発生しない。

## ここから始める

- [インストール](./getting-started/installation.md)：Go の CLI、Go モジュール、npm、Bun。
- [クイックスタート](./getting-started/quick-start.md)：保存済み HTML ファイルに対する validate、compile、extract。
- [動作の仕組み](./guides/how-it-operates.md)：コンパイラの各段階とその順序。
- [抽出器を書く](./guides/write-an-extractor.md)：抽出器ドキュメントの構造。

そのうえでランタイムを選ぶ。
[CLI](./cli/index.md)、[TypeScript と Bun](./npm/index.md)、[Go](./golang/index.md) のいずれかである。

## 実サービスに向ける前に

Scraping KDL は抽出ツールである。
他者のコンテンツにアクセスする権限や、それを再利用する権限を与えるものではない。
自分が運用していないサービスにリクエストを送る前に、[セキュリティと責任ある利用](./guides/security-and-responsible-use.md) を読むこと。

本プロジェクトは、アクセス制御、レート制限、ボット検知を回避する機能を提供しない。
