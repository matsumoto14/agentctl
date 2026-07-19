---
name: design-sync
description: 設計判断を変更・追加したとき、設計書 Artifact・CLAUDE.md・メモリを同期更新する手順。設計とコードの乖離（このプロジェクト最大のドリフト源）を防ぐ。
---

# design-sync

設計判断が変わったときに、正本群を同期させる手順。**コードだけ変えて設計書を放置しない**ためのスキル。

## 対象になる変更か判定する

以下に触れる変更は「設計判断」であり、このスキルの対象:

- MVP 動詞セットの増減・仕様変更（`--json` / 終了コード / `--agent` フラグ）
- タスクライフサイクル（Agent 1 回起動・session の記録内容・暴走ガード・ハーネス導入の是非 — ADR 0002）
- Agent 境界（呼べる動詞サブセット、ホスト直実行、permission 方針）
- SoT / 状態の置き場所、ディレクトリ規約、secrets の扱い
- 既存リポとの依存方向（C1）、compose 実行方式
- スコープ（§14 の「作らないリスト」への追加・削除）

純粋な実装詳細（関数分割、内部リファクタ）は対象外。迷ったら対象として扱う。

## 手順

1. **ADR を書く（判断の正本）** — `docs/decisions/NNNN-<slug>.md` に1判断1ファイルで記録する（Status / Date / Context / Decision / Consequences）。番号は連番。既存判断の変更なら、旧 ADR の Status を `superseded by NNNN` に更新する。
2. **設計書 Artifact を更新する（大改訂時のみ）** — 個別判断は ADR で足りる。アーキテクチャの骨格（タスクライフサイクル、動詞セット、境界、SoT）に及ぶ改訂のときだけ Artifact を更新する。
   - URL（固定）: `https://claude.ai/code/artifact/bc312b81-7c38-4561-99b3-c3b86e406334`
   - このセッションで publish していない場合: WebFetch で現内容を確認 → 新しい完全な HTML をファイルに書き、Artifact ツールに `url` パラメータでこの URL を渡して再公開（同一 URL が保たれる）。
   - 必ず反映するもの: §0 の改訂履歴 callout に `vN` の行を追加 / masthead の Rev チップ / フッターのバージョン表記 / 変更セクション内に「vN 変更」の明記。
   - `label` パラメータに短い版名（例: `v5-<要旨>`）を付ける。
3. **CLAUDE.md の「アーキテクチャ上の不変条件」を同期する** — 設計書・ADR と矛盾する記述を残さない。
4. **メモリを更新する** — `~/.claude/projects/-home-matsumoto-git-agentctl/memory/agentctl-design-doc.md` の主要判断リストを最新版に合わせる。
5. **要件に触れる変更なら scope-check を実行する** — C1〜C8 か §14 に関わる場合は、更新前に `scope-check` スキルで違反がないか確認する。

## 禁止事項

- 設計書を更新せずに CLAUDE.md だけ変える（またはその逆）。
- Artifact を `url` 指定なしで publish して新 URL を作ってしまう。
- 改訂履歴（§0）を省略する — 履歴が消えると過去の判断の根拠が追えなくなる。
