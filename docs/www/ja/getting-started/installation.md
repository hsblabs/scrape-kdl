---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: インストール
description: Scraping KDL の CLI、Go モジュール、npm パッケージのインストール手順と、Go、Node.js、Bun、OS のサポート対象バージョン。
hsblabs:
  sidebar:
    order: 10
---

CLI は `go install` で入れる。
ライブラリは `go get` または `npm install` で入れる。
現行リリースは Go モジュールが `v1.0.4`、npm パッケージが `1.0.4` である。
二つのエコシステムは同一のリリース列と同一のバージョン番号を共有する。

## サポート対象バージョン

| 構成要素 | サポート対象 |
| --- | --- |
| Go | 1.26 以降 |
| Node.js | 22 以降 |
| Bun | `@hsblabs/scrape-kdl` について 1.3 以降 |
| OS | Linux と macOS |
| リリース対象アーキテクチャ | amd64 と arm64 |
| Playwright のブラウザ | Chromium。Firefox と WebKit は best-effort。 |

Windows はサポート対象外である。
これは一時的な状態ではない。
本プロジェクトは Windows の CI ジョブ、Windows のリリース対象、Windows 互換コードのいずれも持たない。
Windows でコンパイルが通ったとしても、それは偶然であって契約ではない。

## コマンドライン

CLI は Go バイナリだけである。
npm パッケージに CLI は含まれない。

```bash
go install github.com/hsblabs/scrape-kdl/cmd/scrape-kdl@v1.0.4
```

go-rod によるブラウザモードを使う場合は、アダプタ側の CLI を別途入れる。

```bash
go install github.com/hsblabs/scrape-kdl/adapters/rod/cmd/scrape-kdl-rod@v1.0.4
```

インストールの確認には次のコマンドを使う。

```bash
scrape-kdl version
```

## Go モジュール

```bash
go get github.com/hsblabs/scrape-kdl@v1.0.4
```

go-rod アダプタは**独立したモジュール**である。
コアモジュール内のパッケージではない。

```bash
go get github.com/hsblabs/scrape-kdl/adapters/rod@v1.0.4
```

アダプタはコアモジュールに依存する。
コアモジュールはブラウザライブラリを import しない。
したがって HTTP だけを使うアプリケーションの依存グラフに、Chromium も CDP も go-rod も入らない。
詳細は [Go](../golang/index.md) を参照。

## Node.js

```bash
npm install @hsblabs/scrape-kdl@1.0.4
```

コアパッケージは ESM のみをサポートする。
エントリポイントは三つある。

- `@hsblabs/scrape-kdl`：コンパイラ、診断、IR、HTTP ランタイム、オフラインスナップショットランタイム、ブラウザアダプタの型。
- `@hsblabs/scrape-kdl/node`：`compileFile` と `validateFile`。これらの関数はコアパッケージの外に置いてある。したがってコアパッケージがファイルシステムへ自動的にアクセスすることはない。
- `@hsblabs/scrape-kdl/authoring`：制約付きのオーサリングモデルと、組み込みトランスフォームのカタログ。

公式の Playwright アダプタは別パッケージである。

```bash
npm install @hsblabs/scrape-kdl-playwright@1.0.4 playwright
npx playwright install chromium
```

詳細は [Playwright アダプタ](../npm/playwright.md) を参照。

## Bun

```bash
bun add @hsblabs/scrape-kdl@1.0.4
```

Bun 1.3 以降がコアパッケージをサポートする。
Playwright アダプタのテストは Node.js 22 以降を使う。

## 次に読むもの

[クイックスタート](./quick-start.md) に進む。
保存済みの HTML ファイルに対して抽出器を実行する手順であり、リクエストは送らない。
