# xlsxdiff

2 つの Excel ファイル（.xlsx / .xlsm など）が、同じシート構成・同じセル内容を持つかを確認する CLI ツールです。
テストツール（runn の `exec` ステップ、CI のシェルスクリプトなど）から呼び出しやすいよう、結果を終了コードで返します。

## インストール

```sh
go install github.com/ramsesyok/xlsxdiff@latest
```

Go がない環境では、[Releases](https://github.com/ramsesyok/xlsxdiff/releases) から OS / アーキテクチャに合ったアーカイブ（Linux / macOS / Windows × amd64 / arm64）をダウンロードし、`xlsxdiff` を PATH の通った場所に置いてください。

## 使い方

```sh
xlsxdiff --expect expected.xlsx --actual actual.xlsx
```

| オプション | 説明 |
|---|---|
| `--expect <file>` | 期待値の Excel ファイル（必須） |
| `--actual <file>` | 実際の Excel ファイル（必須） |
| `--format text\|json` | 出力形式（既定 `text`） |
| `--raw` | 表示書式（数値書式・日付書式）を適用せず、生の値で比較する |
| `--ignore-sheet-order` | シートの並び順の違いを無視する |
| `-q, --quiet` | 何も出力せず終了コードのみ返す |
| `--version` | バージョン表示 |

### 終了コード

| コード | 意味 |
|---|---|
| `0` | 一致 |
| `1` | 差分あり |
| `2` | エラー（必須引数なし、ファイルが開けない など） |

## 比較内容

- **シート構成**: シート名の過不足と、並び順（両方に存在するシートで判定）
- **セル内容**: 両方に存在するシートの全セルの値。既定では Excel 上の表示値（書式適用後の文字列）で比較し、`--raw` で生の値で比較します
- 値が空のセルと存在しないセルは同じとみなします
- 数式そのもの、書式（フォント・色・罫線）、結合セル、列幅などは比較対象外です

## 出力例

text:

```
sheet "Summary": missing in actual
Sheet1!B3: expected "100", actual "120"
2 difference(s) found
```

json（`--format json`）:

```json
{
  "equal": false,
  "diffs": [
    { "kind": "sheet_missing", "sheet": "Summary" },
    { "kind": "cell_value", "sheet": "Sheet1", "cell": "B3", "expected": "100", "actual": "120" }
  ]
}
```

`kind` は `sheet_missing` / `sheet_extra` / `sheet_order` / `cell_value` のいずれかです。

## runn から使う例

```yaml
steps:
  compareReport:
    exec:
      command: xlsxdiff --expect testdata/expected.xlsx --actual out/report.xlsx
    test: current.exit_code == 0
```

## リリース

`v*` 形式のタグを push すると、GitHub Actions（`.github/workflows/release.yml`）が GoReleaser でビルドし、GitHub Releases に公開します。
Actions タブから `Release` ワークフローを手動実行（`tag` に `v0.1.0` などを指定）しても、タグ作成とリリースをまとめて行えます。

依存ライブラリを更新したら `go run scripts/licenses.go` で `THIRD_PARTY_NOTICES.md` を再生成してください（CI で差分をチェックしています）。

## ライセンス

自作部分は [MIT License](LICENSE)（Copyright (c) 2026 ramsesyok）です。
依存ライブラリの表示は [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md) を参照してください。
