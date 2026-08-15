---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Reference
title: 参照資料
description: Scraping KDL の正典。規範的な言語仕様、IR スキーマ、型宣言、互換性方針、外部標準。
hsblabs:
  sidebar:
    order: 60
---

このサイトのページは、Scraping KDL の使い方を伝える。
以下のドキュメントが正典である。
このサイトのページと以下のドキュメントが食い違う場合、正しいのは以下のドキュメントである。

## 規範仕様

| ドキュメント | 内容 |
| --- | --- |
| [Language v0.1](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/language-v0.1.md) | 文法、意味論、検証規則の全体。 |
| [Built-ins v0.1](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/builtins-v0.1.md) | 各組み込みトランスフォームのシグネチャと挙動。 |
| [Selectors v0.1](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/selectors-v0.1.md) | CSS の可搬な部分集合と、拒否される構文。 |
| [Diagnostics](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/diagnostics.md) | 各診断コードの重大度と条件。 |
| [Grammar summary](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/grammar-summary.ebnf) | 構文の EBNF 要約。 |

ツール向けの機械可読データ。

- [`builtins-v0.1.contract.json`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/builtins-v0.1.contract.json)：トランスフォームのシグネチャ。
- [`builtins-v0.1.authoring.json`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/builtins-v0.1.authoring.json)：エディタと補完のためのデータ。
- [`conformance-coverage.json`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/conformance-coverage.json)：仕様に対するフィクスチャの網羅状況。

## Validated IR

| ドキュメント | 内容 |
| --- | --- |
| [IR schema](https://github.com/hsblabs/scrape-kdl/blob/main/docs/ir/schema.json) | Validated IR の JSON Schema。 |
| [IR README](https://github.com/hsblabs/scrape-kdl/blob/main/docs/ir/README.md) | バージョン方針とディレクトリの構成。 |
| [Example IR](https://github.com/hsblabs/scrape-kdl/blob/main/docs/ir/example.ir.json) | 完全な IR ドキュメントの例。 |

IR はコンパイラとランタイムの境界である。
プログラムを読むツールや作るツールを書くときは、このスキーマを使う。

## API 宣言

- [TypeScript `index.d.ts`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/api/typescript/index.d.ts)：可搬なエントリポイント。
- [TypeScript `node.d.ts`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/api/typescript/node.d.ts)：ファイルシステム用のエントリポイント。
- [TypeScript `authoring.d.ts`](https://github.com/hsblabs/scrape-kdl/blob/main/docs/api/typescript/authoring.d.ts)：エディタツール向けのデータ。
- [Public API v1](https://github.com/hsblabs/scrape-kdl/blob/main/docs/public-api-v1.md)：二つのランタイムの安定した公開面。

Go の API には `go doc` を使う。

```bash
go doc github.com/hsblabs/scrape-kdl
```

## ランタイムのドキュメント

| ドキュメント | 内容 |
| --- | --- |
| [Compiler pipeline](https://github.com/hsblabs/scrape-kdl/blob/main/docs/compiler-pipeline.md) | 検証の七つの段階。 |
| [HTTP runtime](https://github.com/hsblabs/scrape-kdl/blob/main/docs/http-runtime.md) | リクエストとエラー回復の挙動。 |
| [Browser runtime](https://github.com/hsblabs/scrape-kdl/blob/main/docs/browser-runtime.md) | アダプタの契約とワークフロー。 |
| [Playwright adapter](https://github.com/hsblabs/scrape-kdl/blob/main/docs/playwright-adapter.md) | TypeScript アダプタの詳細。 |
| [go-rod adapter](https://github.com/hsblabs/scrape-kdl/blob/main/docs/rod-adapter.md) | Go アダプタの詳細。 |
| [HTML compatibility](https://github.com/hsblabs/scrape-kdl/blob/main/docs/html-compatibility.md) | HTML の解析と、ブラウザとの差異。 |
| [Performance](https://github.com/hsblabs/scrape-kdl/blob/main/docs/performance.md) | 計測された挙動と限界。 |

## 互換性とセキュリティ

| ドキュメント | 内容 |
| --- | --- |
| [Compatibility](https://github.com/hsblabs/scrape-kdl/blob/main/docs/compatibility.md) | 対応する Go、Node.js、Bun のバージョン。 |
| [Versioning](https://github.com/hsblabs/scrape-kdl/blob/main/docs/versioning.md) | 言語、IR、リリースの各バージョン。 |
| [Migrate to v1](https://github.com/hsblabs/scrape-kdl/blob/main/docs/migrating-to-v1.md) | 以前のバージョンからの変更点。 |
| [Changelog](https://github.com/hsblabs/scrape-kdl/blob/main/CHANGELOG.md) | リリースの履歴。 |
| [Security model](https://github.com/hsblabs/scrape-kdl/blob/main/docs/security-model.md) | 信頼の段階とランタイムの保護。 |
| [Security policy](https://github.com/hsblabs/scrape-kdl/blob/main/SECURITY.md) | 脆弱性の報告方法。 |
| [Responsible use](https://github.com/hsblabs/scrape-kdl/blob/main/docs/responsible-use.md) | 運用者の責務。 |

## プロジェクト

- [Repository](https://github.com/hsblabs/scrape-kdl)：ソース、フィクスチャ、テスト。
- [Support](https://github.com/hsblabs/scrape-kdl/blob/main/SUPPORT.md)：質問の場所。
- [Contributing](https://github.com/hsblabs/scrape-kdl/blob/main/CONTRIBUTING.md)：変更の出し方。
- [Code of conduct](https://github.com/hsblabs/scrape-kdl/blob/main/CODE_OF_CONDUCT.md)
- [Decision records](https://github.com/hsblabs/scrape-kdl/tree/main/docs/adr)：各アーキテクチャ上の決定の理由。

## 外部標準

- [KDL](https://kdl.dev/)：構文の基となるドキュメント言語。Scraping KDL は文書化された部分集合を受け付ける。
- [RE2 syntax](https://github.com/google/re2/wiki/Syntax)：正規表現のプロファイル。後読み、後方参照、名前付きグループは使えない。
- [WHATWG HTML](https://html.spec.whatwg.org/multipage/parsing.html)：HTML の解析の標準。
- [CSS Selectors Level 4](https://www.w3.org/TR/selectors-4/)：言語の全体。可搬プロファイルはその小さな部分集合である。
- [JSON Schema](https://json-schema.org/)：IR ドキュメントのスキーマ言語。
