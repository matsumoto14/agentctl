---
name: scope-check
description: 変更・提案を要件 C1〜C8、§14「作らないリスト」、7動詞規律に照らして検査するチェックリスト。過剰設計と権限拡大を機構的に止める。実装前・PR 前・設計変更前に実行する。
---

# scope-check

agentctl への変更・提案が設計規律に違反していないかを検査する。**最大のリスクは「最終形を最初に作る」誘惑**（設計書 §18）であり、このスキルはそれを機構化する。

## チェックリスト

各項目を変更内容と突き合わせ、違反を列挙する。

### スコープ（C8）
- [ ] MVP 動詞セット（`task start/resume/status/rm`, `check`, `pr draft`, `doctor` の 7 つ。Agent 選択は `task start/resume` の `--agent` フラグ — ADR 0002）の**外**に動詞を追加していないか。廃止済みの `run codex/claude` / `task pause` を復活させていないか。追加が必要なら先に設計書改訂（design-sync）。
- [ ] §14「作らないリスト」に踏み込んでいないか: Daemon / HTTP API / Plugin 機構 / Firestore 連携 / 並列タスク実行 / 複数 Gateway 同時接続 / 会社別ランタイム分離 / 高度な Memory・Skills 基盤 / Agent 同士の自律ハンドオフ / 自動自己更新。

### 権限・セキュリティ（C7）
- [ ] 危険動詞（merge / deploy / prod 操作 / force-push / secret 読出し / 破壊的 DB 操作）を実装していないか。**これらは「承認つきで実装」ではなく「実装しない」が方針**。
- [ ] Agent が呼べる agentctl 動詞が `{check, status}` を超えていないか。
- [ ] Gateway・Agent に生トークン・生 Shell・Docker socket を渡す経路を作っていないか。
- [ ] secrets を Git 管理下・ログ・Issue/PR 本文に置いていないか。

### 依存方向（C1）と実行環境（C2）
- [ ] 対象リポジトリへ AI 基盤専用ファイルを追加していないか（許容は `AGENTS.md` / `CLAUDE.md` のみ。機械可読な正規コマンドは基盤側 `config/repos/<company>/<repo>.yaml`）。
- [ ] compose を `-p agentctl-<task>` + `run --rm` 以外の形で叩いていないか。compose ファイルを改変していないか。

### 状態（C5）
- [ ] ローカル状態（session / state dir）に永続的な記録を持たせていないか。永続記録は GitHub Issue/PR へ、ローカルは一時実行状態のみ。
- [ ] 生ログ・生トランスクリプトを Issue/PR に貼っていないか（貼るのは要約と結論）。

### 実装規約
- [ ] 新しい処理が `internal/core` に置かれ、`cmd/` が薄いままか。
- [ ] 全動詞が非対話で完走し、`--json` と安定した終了コードを持つか（Gateway コントラクト）。
- [ ] ハーネスを持ち込んでいないか: agentctl 側の反復制御・自動 escalate・stage machine・checkpoint commit は実装しない（ADR 0002）。Agent 起動は常に 1 回、進行判断は人間。
- [ ] Agent 1 回起動・session = 事実の記録・flock 排他・`--max-minutes` 暴走ガードの規約（ADR 0002 / §7 / §10）と整合するか。
- [ ] `doctor` に修復動作を足していないか（report-only を維持）。

## 判定

- **違反なし** → その旨を報告して続行。
- **違反あり** → 実装を始めず、(a) 変更を規律内に収める案、(b) 設計書改訂が正当と考える場合はその理由、の両方を提示して人間の判断を仰ぐ。設計書改訂となった場合は design-sync を実行する。
