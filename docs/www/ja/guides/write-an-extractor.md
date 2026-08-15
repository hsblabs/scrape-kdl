---
schema_version: "2026-08-15"
okf_version: "0.2"
type: Guide
title: 抽出器を書く
description: 抽出器ドキュメントの構造。ソース、input、フィールド、コレクション、トランスフォーム、そして「値がない」ことと「エラー」の区別。
hsblabs:
  sidebar:
    order: 21
---

抽出器ドキュメントは、何を取得し、結果をどんな形にするかを宣言する。
このページではドキュメントの各部分と、それを制御する規則を示す。
文法の全体は [language-v0.1.md](https://github.com/hsblabs/scrape-kdl/blob/main/docs/spec/language-v0.1.md) にある。

## ドキュメント

一つのファイルには `extractor` ノードか `module` ノードのどちらか一つを置く。
両方を含めることはできない。

```kdl
extractor "race-detail" version="2026-07-15" language-version="2026-07-15" {
  source "html" {
    fetch mode="http" url="https://example.com/race/{race_id}"
  }

  input "race_id" type="string" required=#true

  field "title" type="string" required=#true {
    select "h1" match="one"
    value "text"
    apply "normalize-whitespace"
  }
}
```

ルートノードの二つのプロパティはどちらも必須である。

- `version` は利用者側の版識別子である。`YYYY-MM-DD` 形式の実在する暦日でなければならない。
- `language-version` は言語契約を選ぶ。値は `2026-07-15` でなければならない。

子ノードの `input`、`transform`、`field`、`collection` の順序は自由である。
`source` はちょうど一つ必要になる。

## ソース

```kdl
source "html" {
  fetch mode="http" url="https://example.com/items/{item_id}"
  session policy="optional"
}
```

`source` ノードは引数として `"html"` だけを受け付ける。
子に `fetch` をちょうど一つ必要とする。
`session` は一つまで置ける。
`workflow` も一つまで置けるが、fetch のモードが `browser` のときに限る。

`mode` プロパティは `http` か `browser` である。
HTTP モードでは `workflow` ノードと `evaluate-js` ノードを書けない。
コンパイラはこれらを `E_BROWSER_CAPABILITY_REQUIRED` として拒否する。
詳細は [ブラウザモード](./browser-mode.md) を参照。

`session` ノードの `policy` プロパティは `none`、`optional`、`required` のいずれかで、既定値は `none` である。
`required` の場合、ホストがセッションを渡さなければ、ランタイムは fetch の前に停止する。

## input

```kdl
input "race_id" type="string" required=#true
input "lang" type="string" required=#false default="ja"
```

input の型は `string`、`bool`、`int`、`float` のいずれかである。
`required` プロパティの既定値は `#true` である。
必須の input に既定値は持たせられない。

URL テンプレートは `{input_name}` の構文で input を参照する。
ランタイムは fetch の前にテンプレートを展開する。
文字列はパーセントエンコードし、RFC 3986 の非予約文字だけを残す。
波括弧そのものを書くには `{{` または `}}` を使う。

必須の input が渡されなければ、抽出は fetch の前に停止する。

## フィールド

```kdl
field "horse_name" type="string" required=#true {
  select ".horse-name a" match="one"
  value "text"
  apply "normalize-whitespace"
}
```

フィールドは一つの型と一つの値ソースを持つ。
子ノードは次のとおりである。

- `select` を 0 個または 1 個
- 値ソースをちょうど 1 個（`value` または `evaluate-js`）
- `apply` ノードを 0 個以上、ソース順に
- `on-error` を 0 個または 1 個

`select` ノードの `match` プロパティは `one` か `first` で、既定値は `one` である。
`one` では、2 個以上一致すると `E_SELECTOR_CARDINALITY` になる。
`first` では、ドキュメント順で最初に一致した要素を使う。
詳細は [セレクタ](./selectors.md) を参照。

静的 DOM に対する値ソースは三つある。

| ソース | 結果 |
| --- | --- |
| `value "text"` | 子孫ノードのテキストを DOM 順に連結したもの。余分な空白は除去されない。 |
| `value "html"` | 選択した要素の innerHTML。 |
| `value "attr" name="href"` | DOM の属性値。ブラウザが解決したプロパティ値ではない。 |

絶対リンクが必要なら、属性値に `url-resolve` トランスフォームを適用する。
属性そのものは相対形式のままである。

## 「値がない」ことはエラーではない

この区別は重要である。
二つの状態は、それぞれ別の仕組みで制御する。

セレクタが 0 個の要素にしか一致しなかったとき、あるいは属性が存在しないとき、値は**欠落**している。
この状態を制御するのは `required` プロパティである。

| 宣言 | 値が欠落したときの結果 |
| --- | --- |
| `required=#true` | エラー `E_REQUIRED_VALUE_MISSING`。 |
| `required=#false` かつ `default` あり | 既定値。警告なし。`partial` は `false` のまま。 |
| `required=#false` かつ `default` なし | 値 `null`。警告なし。`partial` は `false` のまま。 |

**エラー**はこれとは別の状態である。
トランスフォームの失敗、型の不一致、JavaScript のエラー、アダプタの失敗はエラーである。
この状態を制御するのは `on-error` ノードである。

```kdl
on-error "warn"
```

| ポリシー | 結果 |
| --- | --- |
| `fail` | ランタイムはエラーを伝播させる。 |
| `null` | `null` を返し、`partial` を `true` にする。 |
| `warn` | `null` を返し、警告を追加し、`partial` を `true` にする。 |
| `default` | フィールドの既定値を返し、`partial` を `true` にする。 |

既定のポリシーは、必須フィールドでは `fail`、任意フィールドでは `null` である。
`null` と `warn` は、null を許す出力型を必要とする。
`default` は既定値を必要とする。

`on-error` ノードは、セレクタの不一致や属性の欠落を制御しない。
その場合は `required` を使う。

## コレクション

```kdl
collection "entries" min-items=1 on-row-error="skip" {
  select "table.entries tbody tr"
  field "number" type="u8" required=#true {
    select ".number"
    value "text"
    apply "parse-int" as="u8"
  }
}
```

コレクションは `select` をちょうど一つと、フィールドまたはコレクションの子を最低一つ必要とする。
セレクタに一致した要素がそれぞれ 1 行になり、順序はドキュメント順である。
コレクションは別のコレクションを含められる。

`min-items` と `max-items` は行数を制限する。
`max-items` の値は `min-items` 以上でなければならない。
`required=#true` は、実質的に行数の下限 1 を意味する。

`on-row-error` プロパティは `fail` か `skip` で、既定値は `fail` である。
`skip` の場合、子のエラーがどのポリシーでも回復されなかった行を、ランタイムは捨てる。
捨てた行ごとに警告が追加され、`partial` が `true` になる。
行数の上限と下限は、行を捨てたあとに検査する。

## 型

プリミティブ型は `string`、`bool`、`int`、`u8` から `u64` までの符号なし整数、`i8` から `i64` までの符号付き整数、`float`、`f32`、`f64`、`object`、`unknown` である。

配列には `[]`、null 許容には `?` を付ける。
演算子は左から右へ結合する。
つまり `string?[]` は「null 許容文字列の配列」、`string[]?` は「null 許容の配列」である。
意図を明示するには括弧を使う。

暗黙の変換はない。
文字列は `parse-int` や `parse-float` を経ずに数値にはならない。
整数のオーバーフローは切り詰めではなく抽出エラーになる。
浮動小数点数は有限値でなければならない。

## トランスフォーム

`apply` ノードはソース順に実行する。
ある呼び出しの出力型は、次の呼び出しの入力型と一致しなければならない。
コンパイラはパイプライン全体を検査し、不整合な並びを `E_TRANSFORM_TYPE_MISMATCH` として拒否する。

抽出器やモジュールの中で、自分のトランスフォームを宣言できる。

```kdl
transform "extract_horse_id" input="string" output="string?" {
  pipeline {
    apply "regex-capture" pattern=#"/horse/([^/?#]+)"# group=1
  }
}
```

宣言されたトランスフォームは、`pipeline`、`match`、`external` のうちちょうど一つの本体を持つ。
詳細は [トランスフォーム](./transforms.md) を参照。

## モジュール

共有するトランスフォームはモジュールドキュメントに置き、import する。

```kdl
import "./modules/common.kdl" as="common"

extractor "race-detail" version="2026-07-15" language-version="2026-07-15" {
  // ...
  field "horse_id" type="string?" {
    select "a.horse" match="first"
    value "attr" name="href"
    apply "common.extract_horse_id"
  }
}
```

import の規則は厳格である。

- パスは相対でなければならない。リモート URL はエラーになる。
- `as` プロパティは必須であり、別名はそれぞれ一意でなければならない。
- 参照先はモジュールドキュメントでなければならない。
- import グラフの循環はエラーになる。
- import したトランスフォームは `alias.name` の形で書かなければならない。

モジュールは、自身が直接宣言したトランスフォームをエクスポートする。
v0.1 に再エクスポートはない。

## 結果

抽出は三つの部分を返す。

```text
value       フィールドとコレクションから作られたオブジェクト
warnings    実行順に並んだ警告
partial     エラーから回復したか、行を捨てたときに true
```

`partial` フラグが `true` になるのは、回復か行の破棄が起きたときだけである。
想定どおり任意の値が存在しなかった場合には立たない。
そのため `partial: false` の結果は信頼できる。

## 次に読むもの

- [セレクタ](./selectors.md)：可搬な CSS の部分集合。
- [トランスフォーム](./transforms.md)：パイプライン、match、外部トランスフォーム。
- [パターン](./patterns.md)：よく使うドキュメントの形。
