package core

import (
	"context"
	"io"
)

// port は利用側の core に定義し、infra が実装する（ADR 0001）。
// 実際に利用するメソッドだけを含める。

type SessionStore interface {
	Save(Session) error
	Load(id string) (Session, error)
	List() ([]Session, error)
	Delete(id string) error
}

// Locker は複数タスクの同時実行を機構で防ぐ（設計書 §10）。
// Acquire は非ブロッキングで、取得できない場合は ErrLocked を返す。
type Locker interface {
	Acquire() (release func(), err error)
}

type WorktreeManager interface {
	Add(repoPath, worktreePath, branch, base string) error
	Remove(repoPath, worktreePath string) error
	// BranchExists は local と origin のどちらかにブランチがあるかを返す。
	BranchExists(repoPath, branch string) (bool, error)
}

type IssueReader interface {
	Get(repo string, number int) (Issue, error)
}

type PRCreator interface {
	CreateDraft(dir, base, title, body string) (url string, err error)
}

type AgentRunner interface {
	Run(ctx context.Context, agent Agent, dir, prompt string, env []string, out io.Writer) error
}

type ComposeRunner interface {
	Run(dir, project, service string, command []string, out io.Writer) error
	Down(dir, project string, out io.Writer) error
}
