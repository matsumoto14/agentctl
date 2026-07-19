package core

import "fmt"

type PRDrafter struct {
	Store SessionStore
	PRs   PRCreator
}

// Draft は現在のブランチから Draft PR を作成する。本文は最小のテンプレートに
// とどめ、要約は人間または Agent が PR 上で追記する。実行ログは貼らない（C5）。
func (d *PRDrafter) Draft(id string, repo RepoConfig) (string, error) {
	s, err := d.Store.Load(id)
	if err != nil {
		return "", err
	}
	title := fmt.Sprintf("%s (#%d)", s.IssueTitle, s.Issue)
	body := fmt.Sprintf("Issue #%d の実装。\n\nCloses #%d\n", s.Issue, s.Issue)
	return d.PRs.CreateDraft(s.Worktree, repo.Base, title, body)
}
