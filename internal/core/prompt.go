package core

import "fmt"

// Prompt は Agent への作業指示を組み立てる。Issue 本文（untrusted text）を含むため、
// 危険操作の抑止はここではなく Agent 側 permission と人間レビューが担う（設計書 §11）。
func Prompt(issue Issue) string {
	return fmt.Sprintf(`GitHub Issue #%d「%s」を、この作業ディレクトリで実装してください。

--- Issue 本文 ---
%s
------------------

ルール:
- lint / test / build の検証は agentctl check を実行する（docker compose を直接操作しない）
- 状態確認は agentctl task status を使う
- このディレクトリの外のファイルへ書き込まない
- 変更は意味のある単位で git commit する
`, issue.Number, issue.Title, issue.Body)
}
