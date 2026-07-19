# CLAUDE.md

このファイルは、agentctl リポジトリで実装やレビューを行う際の作業ガイドである。

## プロジェクト概要

agentctl は、GitHub Issue を起点とする開発作業を支援する Go 製 CLI である。Worktree の作成、Coding Agent の起動、Docker Compose を使った検証、Draft PR の作成を、それぞれ独立した決定的な道具として提供する。工程間の進行判断は人間が行う（ADR 0002）。

agentctl 自身はコードを生成せず、タスクの反復も統括しない。曖昧な判断を LLM に委ねるのではなく、CLI の引数、設定、現在の状態に基づいて決定的に動作することを目指す。

## 開発環境

ビルド、テスト、vet の検証は、ホストの Go ではなくリポジトリ直下の `compose.yaml` の `dev` サービス（Go コンテナ）で実行する。

```sh
docker compose run --rm dev go build ./...
docker compose run --rm dev go test ./...
docker compose run --rm dev go vet ./...
```

特定のテストだけを実行する場合は、対象パッケージとテスト名を指定する。

```sh
docker compose run --rm dev go test ./internal/core/... -run TestName
```

## 実装方針

### CLI の責務

agentctl は、タスクの状態遷移を管理する CLI として実装する。

- すべてのコマンドは非対話で完了できるようにする。
- 人間向け出力に加えて、機械処理向けの `--json` を提供する。
- 終了コードはコマンド間で一貫させ、呼び出し側が結果を判定できるようにする。
- 状態確認はポーリング可能にし、常駐プロセスを前提としない。
- 環境診断は報告だけを行い、暗黙に修復しない。

実装対象のコマンドは次に限定する。

```text
task start
task resume
task status
task rm
check
pr draft
doctor
```

Agent（Codex / Claude Code）の選択は、Agent を起動するコマンド（`task start` / `task resume`）の `--agent codex|claude` フラグで指定する。

コマンドを追加する場合は、既存コマンドで表現できない理由と安全性への影響を ADR に記録する。

### タスクとセッション

agentctl はタスクの反復を統括しない。Agent の起動は 1 回ずつであり、続行・打ち切り・Agent の切り替えは人間が判断する（[ADR 0002](docs/decisions/0002-toolbox-cli-no-harness.md)）。

- `task start` は worktree とセッションを作成し、`--agent` で指定した Agent を 1 回起動する。
- `task resume` は既存の worktree で Agent を再度 1 回起動する。git 操作は行わない。
- セッションは状態機械ではなく事実の記録とする（issue、worktree、ブランチ、最後に起動した Agent、起動回数、タイムスタンプ）。
- `task rm` は Worktree、Compose project、セッションを一組として片付ける。
- 同じ実行環境では複数タスクの Agent を同時に起動しない。

`task start` と `task resume` は実行時間の上限（`--max-minutes`、既定値つき）を持つ。無制限の実行は実装しない。

### Coding Agent との境界

Codex と Claude Code はホスト上で実行する。コンテナ内実行や Docker socket の共有は前提にしない。

- Agent から実行できる agentctl コマンドは `check` と `task status` に限定する。
- テストや lint は Agent から Docker Compose を直接操作せず、`agentctl check` を経由する。
- Gateway や Agent にアクセストークンなどの秘密情報を渡さない。
- Agent ごとの差異は専用の分岐を広げず、必要な操作を表す小さな interface の実装として扱う。

### 対象リポジトリとの境界

agentctl の都合で、対象リポジトリの構成や Compose ファイルを変更しない。

- 対象リポジトリに基盤専用ファイルを追加しない。ただし、作業指示として既に使われる `AGENTS.md` と `CLAUDE.md` は許容する。
- lint、test、build の実行コマンドは agentctl 側の設定ファイルに記述する。
- Markdown からコマンドを推測または抽出しない。
- Compose 実行時は対象リポジトリの既存サービスを利用する。
- ポート公開を必要とする検証は初期スコープに含めない。

リポジトリごとの設定は次の場所を正本とする。

```text
config/repos/<company>/<repo>.yaml
```

### 状態と秘密情報

Issue と Pull Request には、作業の要約と最終的な判断を記録する。詳細な実行ログは貼り付けない。

ローカルの実行状態は、再作成可能な一時データとして扱う。

```text
$XDG_STATE_HOME/agentctl/companies/<company>/
```

秘密情報はリポジトリ管理対象の設定と分離する。

```text
$XDG_CONFIG_HOME/agentctl/secrets/<company>/
```

トークン、秘密鍵、認証情報を Git の管理対象へ追加してはならない。

## アーキテクチャ

実装コードは `cmd/` と `internal/` に配置する。

```text
cmd/agentctl/        CLI のエントリポイントと依存の組み立て
internal/core/       状態遷移、ユースケース、外部操作の interface
internal/infra/      ファイル、Git、Compose、GitHub、Agent などの実装
internal/config/     設定の読み込みと検証
internal/doctor/     環境の非破壊チェック
internal/cliio/      CLI 入出力、表示形式、終了コード
config/              リポジトリ管理対象の設定
docs/decisions/      Architecture Decision Record
test/                統合テストと E2E テスト
```

`core` は具体的な I/O 実装へ依存させない。外部操作の interface は、それを利用する `core` 側に定義し、`infra` が実装する。具象実装の組み立ては `cmd/agentctl` で行う。

構成と依存ルールの理由は [docs/decisions/0001-repo-layout.md](docs/decisions/0001-repo-layout.md) を参照する。

## スコープ

最初の実装は、個人利用、単一会社、単一リポジトリ、逐次実行を対象とする。

次の機能は、具体的な必要性と別の設計判断がない限り実装しない。

- merge、deploy、本番環境の操作、force-push
- secret の読み出しや配布
- Daemon、HTTP API、Web UI
- 複数タスクの並列実行
- プラグイン機構
- 複数 Gateway や会社別ランタイムの分離

将来必要になる可能性だけを理由に、抽象化、設定項目、ディレクトリを追加しない。

## 変更時のルール

- ブランチ名は `feat/`、`fix/`、`chore/` のいずれかを接頭辞にする（例: `feat/issue-1-cli-skeleton`）。機能追加は `feat`、不具合修正は `fix`、それ以外の整備は `chore`。

- 実装前に、変更が現在のコマンドとスコープに収まるか確認する。
- 設計上の判断を変更する場合は、該当する ADR を更新するか、新しい ADR を追加する。
- コード、ADR、この作業ガイドの記述が矛盾しないようにする。
- 新しい外部依存は、標準ライブラリで代替できない理由を確認してから追加する。
- 生成物や一時ファイル、秘密情報をコミットしない。
