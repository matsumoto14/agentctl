// Package core はタスク操作の方針を定義する。I/O の実装には依存しない（ADR 0001）。
package core

import (
	"errors"
	"fmt"
	"time"
)

// Agent はホスト上で起動する Coding Agent の種別。
type Agent string

const (
	AgentCodex  Agent = "codex"
	AgentClaude Agent = "claude"
)

func ParseAgent(s string) (Agent, error) {
	switch a := Agent(s); a {
	case AgentCodex, AgentClaude:
		return a, nil
	default:
		return "", fmt.Errorf("%w: --agent は codex か claude を指定する: %q", ErrInvalid, s)
	}
}

// エラー種別が終了コードへの対応を決める（cli パッケージの規約）。
var (
	// ErrInvalid は使い方エラー（exit 2）。
	ErrInvalid = errors.New("invalid argument")
	// ErrNotFound / ErrExists / ErrLocked は前提条件エラー（exit 3）。
	ErrNotFound = errors.New("not found")
	ErrExists   = errors.New("already exists")
	ErrLocked   = errors.New("locked")
)

// Session は状態機械ではなく事実の記録である（ADR 0002）。
type Session struct {
	ID         string    `json:"id"`
	Company    string    `json:"company"`
	Repo       string    `json:"repo"`
	RepoPath   string    `json:"repo_path"`
	Issue      int       `json:"issue"`
	IssueTitle string    `json:"issue_title"`
	Worktree   string    `json:"worktree"`
	Branch     string    `json:"branch"`
	LastAgent  Agent     `json:"last_agent,omitempty"`
	Runs       int       `json:"runs"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TaskID はリポジトリ名を含める。同じ会社に複数リポがある場合の Issue 番号衝突を防ぐ。
func TaskID(repo string, issue int) string { return fmt.Sprintf("%s-issue-%d", repo, issue) }

// ComposeProject はタスク毎に compose project 名を分離する（設計書 §9.3）。
func ComposeProject(taskID string) string { return "agentctl-" + taskID }

type Issue struct {
	Number int
	Title  string
	Body   string
	URL    string
}

// RepoConfig は基盤側設定（config/repos/<company>/<repo>.yaml）の core 表現。
type RepoConfig struct {
	Name    string
	Path    string
	GitHub  string
	Base    string
	Service string
	Checks  []CheckStep
}

type CheckStep struct {
	Name    string
	Command []string
}
