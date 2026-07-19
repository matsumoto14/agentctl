# 0003: cobra の採用とコマンド別ファイル構成

- Status: accepted
- Date: 2026-07-19

## Context

CLI 骨格（Issue #1）は標準ライブラリの `flag` と手書きディスパッチで実装した。7 動詞の実処理（Issue #4）を進めた結果、次の問題が出た。

- `flag.FlagSet.Parse` は最初の位置引数で解析を終了するため、`task resume <id> --agent claude` 形式を扱うのに自前の反復解析（parseFlags）が必要になった。位置引数の数の検証も各ハンドラの手書きだった。
- 全ハンドラが 1 つの `handlers.go` に集まり、コマンドを跨いだ見通しが悪くなった。
- 参考実装 nr（コマンド毎 1 ファイル、root は純粋なルーター、cmd と internal の 1:1 対応）と比べ、構成の意図が読み取りにくい。

## Decision

`github.com/spf13/cobra` を採用し、`cmd/agentctl` をコマンド毎のファイルに分割する。

```text
cmd/agentctl/
├── main.go          # エントリポイントと依存の組み立て、終了コードへの写像
├── deps.go          # 依存の束（deps）と共通ヘルパー（出力・エラー畳み込み）
├── root.go          # 純粋なルーター（ロジックなし）
├── task.go          # task 親コマンド（ルーターのみ）
├── task_start.go / task_resume.go / task_status.go / task_rm.go
├── check.go / pr_draft.go / doctor.go
```

- 標準ライブラリで代替できない理由: 位置引数と混在するフラグの解析、引数個数の検証、サブコマンドの usage 生成は `flag` に無く、自前実装が既に綻んでいた。cobra（+ pflag）はこれらを提供し、手書きディスパッチと parseFlags を丸ごと置き換える。
- 7 動詞規律の維持: cobra が自動追加する `completion` / `help` サブコマンドは無効化する。動詞を増やさない規律は変わらない（ADR 0002）。
- Gateway コントラクトの維持: 終了コード 0/1/2/3/10 は `main.execute` が cobra の結果から写像する。stdout は機械可読な結果専用とし、help / usage / 診断は stderr に出す。
- `--json` の保証範囲: 成功時と実行時エラー（exit 1 / 3）では stdout に単一の JSON オブジェクトを返す。使い方エラー（exit 2）のうち cobra のフラグ解析前に失敗するもの（未知フラグ・引数個数）は JSON を保証しない — 呼び出し側の組み立てバグであり、終了コード 2 で判別できる。

## Consequences

### Positive

- コマンド追加・変更の差分が 1 ファイルに閉じ、nr と同じ感覚で読める。
- 位置引数まわりの自前解析が消え、`task resume <id> --agent claude` が自然に動く。
- 依存注入（deps）はそのままなので、テストはフェイク注入の形を維持できる。

### Negative

- 外部依存が cobra / pflag / mousetrap の 3 モジュール増える。
- cobra の既定動作（自動コマンド・usage 出力先）を規律に合わせて明示的に抑える設定が root.go に必要になる。

## References

- [0001-repo-layout.md](0001-repo-layout.md) — パッケージ構成と依存方向は不変
- [0002-toolbox-cli-no-harness.md](0002-toolbox-cli-no-harness.md) — 7 動詞規律は不変
