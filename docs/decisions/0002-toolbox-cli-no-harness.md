# 0002: agentctl は Agent を 1 回起動する道具箱とし、ハーネスにしない

- Status: accepted
- Date: 2026-07-19

## Context

設計検討の過程では、agentctl が implement ⇄ check の反復を統括する「ハーネス」案も検討した（設計書 Artifact v4）。具体的には次を含む。

- stage machine（INIT → IMPLEMENTING ⇄ CHECKING → REVIEWING → PR_DRAFTING → PR_CREATED、FAILED / PAUSED）
- `fail_count` の記録と、閾値超過による Codex → Claude Code への自動 escalate
- stage 遷移ごとの checkpoint commit と、`resume` = 「checkpoint へ `git reset --hard` して stage 頭から再実行」という規約
- `run codex` / `run claude` 動詞による Agent 起動の内部化

しかし実装開始前の再検討で、この案は現段階では過剰と判断した。

- まだ「Issue → Draft PR が 1 回通る」ことすら検証していない段階で、反復制御・escalate 規則・checkpoint 規約という 3 つの複雑な機構を先に固定することになる。
- 反復をいつ打ち切るか、いつ Claude へ切り替えるかの判断は、当面は人間が行った方が確実で、自動化の閾値（N / M）を決める材料もまだない。
- 「最終形を最初に作らない」（C8）という設計書自身の最優先原則に反する。

## Decision

agentctl は Agent の反復を統括しない。**決定的な道具の集合（道具箱）として実装し、進行判断は人間が行う。**

### コマンドセット（7 動詞）

```text
task start --issue N --agent codex|claude   # worktree + session 作成 → Agent を1回起動
task resume <id> --agent codex|claude       # 既存 worktree で Agent を再度1回起動
task status [--json]                        # session と worktree の事実を報告
task rm <id>                                # worktree / compose project / session の unwind
check                                       # compose 経由 lint/test/build
pr draft                                    # Draft PR 作成
doctor                                      # 環境の非破壊チェック
```

- `run codex` / `run claude` のような Agent 起動専用の動詞は設けない。Agent の選択は、Agent を起動するコマンド（`task start` / `task resume`）の `--agent` フラグで指定する。
- `task pause` は設けない。Agent 起動は同期・1 回実行であり、常駐する処理が存在しないため、中断すべき対象がない。
- escalate は人間の操作で表現する: `task resume <id> --agent claude`。

### 状態の扱い

- stage machine は実装しない。session JSON は状態機械ではなく**事実の記録**とする: task id、issue 番号、リポジトリ、worktree パス、ブランチ、最後に起動した Agent、起動回数、タイムスタンプ。
- checkpoint commit 規約は設けない。`resume` は git 操作を行わず、既存 worktree の現状のまま Agent を再起動する。作業内容のコミットは Agent と人間に委ねる。
- `fail_count` と自動 escalate は実装しない。check の成否は終了コードで報告し、次の一手は人間が決める。

### 採用するガード

- 暴走ガード: `task start` / `task resume` は `--max-minutes`（既定値つき）を持ち、超過で Agent プロセスを停止して失敗として報告する。`--max-iterations` はループ自体がないため不要。
- 逐次実行: state dir の lock により、同時に複数タスクの Agent を起動しない。
- Agent が呼べる agentctl 動詞は `{check, task status}` のみ、という境界。
- Gateway コントラクト（非対話・`--json`・安定終了コード・status ポーリング）。

## Consequences

### Positive

- 実装対象から stage machine・escalate 規則・checkpoint 規約が消え、MVP の実装量が大きく減る。
- `resume` の意味論が「Agent をもう一度起動する」だけになり、約束と実装のずれが生じにくい。
- 反復・escalate の自動化が本当に必要かを、道具箱を実際に使った経験から判断できる。

### Negative

- Issue → Draft PR の一気通貫自動実行はできず、各段階で人間の操作が必要になる。
- check の失敗回数などの履歴が機構として残らないため、Gateway 経由の非同期運用（Phase 1）で進行状況の把握が粗くなる。
- 将来ハーネスを導入する場合、session スキーマと `resume` の意味論を再設計する必要がある。

### ハーネス導入の条件

道具箱運用で「人間による反復判断が明確なボトルネック」と確認でき、打ち切り・escalate の閾値を経験則として言語化できた時点で、別の ADR としてハーネス導入を検討する。

## References

- 設計書 Artifact v5（§7 / §13 が本 ADR と対応）
- [0001-repo-layout.md](0001-repo-layout.md) — リポ構成は本判断の影響を受けない
