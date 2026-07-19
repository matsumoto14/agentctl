package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"
)

type Tasks struct {
	Store     SessionStore
	Lock      Locker
	Worktrees WorktreeManager
	Issues    IssueReader
	Agents    AgentRunner
	Compose   ComposeRunner
	Now       func() time.Time
}

type StartParams struct {
	Company      string
	Repo         RepoConfig
	Issue        int
	Agent        Agent
	Timeout      time.Duration
	WorktreePath string
	AgentOut     io.Writer
}

func (t *Tasks) Start(ctx context.Context, p StartParams) (Session, error) {
	if p.Issue <= 0 {
		return Session{}, fmt.Errorf("%w: --issue に正の Issue 番号を指定する", ErrInvalid)
	}
	release, err := t.Lock.Acquire()
	if err != nil {
		return Session{}, err
	}
	defer release()

	id := TaskID(p.Repo.Name, p.Issue)
	// 「読めない」と「存在しない」を区別する。壊れた session を未存在と扱うと、
	// 既存タスクの上に新しい worktree を作って作業を失う。
	if _, err := t.Store.Load(id); err == nil {
		return Session{}, fmt.Errorf("%w: task %s（task resume で再実行する）", ErrExists, id)
	} else if !errors.Is(err, ErrNotFound) {
		return Session{}, fmt.Errorf("既存 session の確認: %w", err)
	}
	issue, err := t.Issues.Get(p.Repo.GitHub, p.Issue)
	if err != nil {
		return Session{}, fmt.Errorf("issue #%d の取得: %w", p.Issue, err)
	}
	branch, err := t.pickBranch(p.Repo.Path, id)
	if err != nil {
		return Session{}, err
	}
	if err := t.Worktrees.Add(p.Repo.Path, p.WorktreePath, branch, p.Repo.Base); err != nil {
		return Session{}, fmt.Errorf("worktree の作成: %w", err)
	}
	now := t.Now()
	s := Session{
		ID: id, Company: p.Company, Repo: p.Repo.Name, RepoPath: p.Repo.Path,
		Issue: p.Issue, IssueTitle: issue.Title,
		Worktree: p.WorktreePath, Branch: branch,
		CreatedAt: now, UpdatedAt: now,
	}
	if err := t.Store.Save(s); err != nil {
		return s, err
	}
	return t.runAgent(ctx, s, p.Agent, p.Timeout, issue, p.AgentOut)
}

// pickBranch は未使用のブランチ名を選ぶ。過去の試行のブランチが local か origin に
// 残っていても、強制リセット（-B）や force-push に頼らず新しい名前で進める。
func (t *Tasks) pickBranch(repoPath, id string) (string, error) {
	const maxAttempts = 10
	for i := 1; i <= maxAttempts; i++ {
		name := "agentctl/" + id
		if i > 1 {
			name = fmt.Sprintf("agentctl/%s-%d", id, i)
		}
		exists, err := t.Worktrees.BranchExists(repoPath, name)
		if err != nil {
			return "", fmt.Errorf("ブランチの確認: %w", err)
		}
		if !exists {
			return name, nil
		}
	}
	return "", fmt.Errorf("agentctl/%s のブランチ候補が %d 個とも使用済み（不要な過去ブランチを削除する）", id, maxAttempts)
}

type ResumeParams struct {
	ID       string
	Repo     RepoConfig
	Agent    Agent // 空なら前回と同じ Agent
	Timeout  time.Duration
	AgentOut io.Writer
}

func (t *Tasks) Resume(ctx context.Context, p ResumeParams) (Session, error) {
	release, err := t.Lock.Acquire()
	if err != nil {
		return Session{}, err
	}
	defer release()

	s, err := t.Store.Load(p.ID)
	if err != nil {
		return Session{}, err
	}
	agent := p.Agent
	if agent == "" {
		agent = s.LastAgent
	}
	if agent == "" {
		agent = AgentCodex
	}
	issue, err := t.Issues.Get(p.Repo.GitHub, s.Issue)
	if err != nil {
		return s, fmt.Errorf("issue #%d の取得: %w", s.Issue, err)
	}
	return t.runAgent(ctx, s, agent, p.Timeout, issue, p.AgentOut)
}

// runAgent は Agent を 1 回だけ起動する。反復・打ち切り・Agent 切替の判断は
// 人間が行う（ADR 0002）。git 操作は行わず、worktree は現状のまま使う。
func (t *Tasks) runAgent(ctx context.Context, s Session, agent Agent, timeout time.Duration, issue Issue, out io.Writer) (Session, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	env := []string{"AGENTCTL_TASK=" + s.ID, "AGENTCTL_COMPANY=" + s.Company}
	runErr := t.Agents.Run(ctx, agent, s.Worktree, Prompt(issue), env, out)
	if runErr != nil && errors.Is(ctx.Err(), context.DeadlineExceeded) {
		runErr = fmt.Errorf("--max-minutes の上限で Agent を停止した: %w", runErr)
	}
	s.LastAgent = agent
	s.Runs++
	s.UpdatedAt = t.Now()
	if err := t.Store.Save(s); err != nil && runErr == nil {
		runErr = err
	}
	return s, runErr
}

// Remove は worktree / compose project / session を一組として片付ける（unwind）。
// Agent 実行中の破壊を防ぐため lock を取り、session の削除は worktree の片付けが
// 完了した場合に限る — 途中失敗で session だけ消えると CLI から復旧できなくなる。
func (t *Tasks) Remove(id string, out io.Writer) (Session, error) {
	release, err := t.Lock.Acquire()
	if err != nil {
		return Session{}, err
	}
	defer release()

	s, err := t.Store.Load(id)
	if err != nil {
		return Session{}, err
	}
	var errs []error
	if err := t.Compose.Down(s.Worktree, ComposeProject(s.ID), out); err != nil {
		errs = append(errs, fmt.Errorf("compose down: %w", err))
	}
	if err := t.Worktrees.Remove(s.RepoPath, s.Worktree); err != nil {
		errs = append(errs, fmt.Errorf("worktree remove: %w", err))
	}
	if len(errs) > 0 {
		return s, fmt.Errorf("%w（session は残した — 再実行で片付け直す）", errors.Join(errs...))
	}
	if err := t.Store.Delete(id); err != nil {
		return s, fmt.Errorf("session delete: %w", err)
	}
	return s, nil
}
