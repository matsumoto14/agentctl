package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/matsumoto14/agentctl/internal/cli"
	"github.com/matsumoto14/agentctl/internal/core"
	"github.com/matsumoto14/agentctl/internal/doctor"
)

const defaultMaxMinutes = 30

// deps は具象実装の束。組み立ては main が行い、テストはフェイクを注入する（ADR 0001）。
type deps struct {
	ctx    context.Context
	stdout io.Writer
	stderr io.Writer
	env    func(string) string
	now    func() time.Time

	loadRepo     func(company, name string) (core.RepoConfig, error)
	store        func(company string) core.SessionStore
	locker       func(company string) core.Locker
	logWriter    func(company, id string) (io.WriteCloser, error)
	worktreePath func(company, id string) string

	worktrees core.WorktreeManager
	issues    core.IssueReader
	agents    core.AgentRunner
	compose   core.ComposeRunner
	prs       core.PRCreator

	doctorReport func(company string) doctor.Report
}

func (d *deps) tasks(company string) *core.Tasks {
	return &core.Tasks{
		Store: d.store(company), Lock: d.locker(company),
		Worktrees: d.worktrees, Issues: d.issues, Agents: d.agents, Compose: d.compose,
		Now: d.now,
	}
}

func exitFor(err error) int {
	switch {
	case err == nil:
		return cli.ExitOK
	case errors.Is(err, core.ErrInvalid):
		return cli.ExitUsage
	case errors.Is(err, core.ErrNotFound), errors.Is(err, core.ErrExists), errors.Is(err, core.ErrLocked):
		return cli.ExitPrecondition
	default:
		return cli.ExitFailure
	}
}

func (d *deps) fail(name string, err error) int {
	fmt.Fprintf(d.stderr, "agentctl: %s: %v\n", name, err)
	return exitFor(err)
}

// 設定が読めないのは環境の問題であり、使い方エラーとは区別する。
func (d *deps) failConfig(name string, err error) int {
	fmt.Fprintf(d.stderr, "agentctl: %s: %v\n", name, err)
	return cli.ExitPrecondition
}

func (d *deps) unexpected(name string, pos []string) int {
	fmt.Fprintf(d.stderr, "agentctl: %s: unexpected argument %q\n", name, pos[0])
	return cli.ExitUsage
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

type taskJSON struct {
	core.Session
	OK    bool   `json:"ok"`
	Error string `json:"error,omitempty"`
}

func (d *deps) taskResult(name string, s core.Session, err error, jsonOut bool) int {
	if jsonOut {
		cli.WriteJSON(d.stdout, taskJSON{Session: s, OK: err == nil, Error: errString(err)})
	} else if err == nil {
		fmt.Fprintf(d.stdout, "task %s: agent %s の実行が完了（runs=%d）\n", s.ID, s.LastAgent, s.Runs)
	}
	if err != nil {
		fmt.Fprintf(d.stderr, "agentctl: %s: %v\n", name, err)
		return exitFor(err)
	}
	return cli.ExitOK
}

func (d *deps) taskStart(name string, args []string) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	jsonOut := fs.Bool("json", false, "機械処理向けの JSON を stdout に出力する")
	company := fs.String("company", "personal", "会社プロファイル")
	repoName := fs.String("repo", "", "設定名（config/repos/<company>/ に 1 つだけなら省略可）")
	issueNo := fs.Int("issue", 0, "GitHub Issue 番号")
	agentName := fs.String("agent", string(core.AgentCodex), "起動する Agent（codex|claude）")
	maxMinutes := fs.Int("max-minutes", defaultMaxMinutes, "Agent 実行時間の上限（分）")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return cli.ExitUsage
	}
	if len(pos) > 0 {
		return d.unexpected(name, pos)
	}
	if *issueNo <= 0 {
		return d.fail(name, fmt.Errorf("%w: --issue に Issue 番号を指定する", core.ErrInvalid))
	}
	ag, err := core.ParseAgent(*agentName)
	if err != nil {
		return d.fail(name, err)
	}
	repo, err := d.loadRepo(*company, *repoName)
	if err != nil {
		return d.failConfig(name, err)
	}
	id := core.TaskID(*issueNo)
	logw, err := d.logWriter(*company, id)
	if err != nil {
		return d.fail(name, err)
	}
	defer logw.Close()
	s, runErr := d.tasks(*company).Start(d.ctx, core.StartParams{
		Company: *company, Repo: repo, Issue: *issueNo, Agent: ag,
		Timeout:      time.Duration(*maxMinutes) * time.Minute,
		WorktreePath: d.worktreePath(*company, id),
		AgentOut:     io.MultiWriter(d.stderr, logw),
	})
	return d.taskResult(name, s, runErr, *jsonOut)
}

func (d *deps) taskResume(name string, args []string) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	jsonOut := fs.Bool("json", false, "機械処理向けの JSON を stdout に出力する")
	company := fs.String("company", "personal", "会社プロファイル")
	agentName := fs.String("agent", "", "起動する Agent（省略時は前回と同じ）")
	maxMinutes := fs.Int("max-minutes", defaultMaxMinutes, "Agent 実行時間の上限（分）")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return cli.ExitUsage
	}
	if len(pos) != 1 {
		fmt.Fprintf(d.stderr, "agentctl: %s: task id を 1 つ指定する\n", name)
		return cli.ExitUsage
	}
	var ag core.Agent
	if *agentName != "" {
		if ag, err = core.ParseAgent(*agentName); err != nil {
			return d.fail(name, err)
		}
	}
	id := pos[0]
	prev, err := d.store(*company).Load(id)
	if err != nil {
		return d.fail(name, err)
	}
	repo, err := d.loadRepo(*company, prev.Repo)
	if err != nil {
		return d.failConfig(name, err)
	}
	logw, err := d.logWriter(*company, id)
	if err != nil {
		return d.fail(name, err)
	}
	defer logw.Close()
	s, runErr := d.tasks(*company).Resume(d.ctx, core.ResumeParams{
		ID: id, Repo: repo, Agent: ag,
		Timeout:  time.Duration(*maxMinutes) * time.Minute,
		AgentOut: io.MultiWriter(d.stderr, logw),
	})
	return d.taskResult(name, s, runErr, *jsonOut)
}

type statusEntry struct {
	core.Session
	WorktreeExists bool `json:"worktree_exists"`
}

func (d *deps) taskStatus(name string, args []string) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	jsonOut := fs.Bool("json", false, "機械処理向けの JSON を stdout に出力する")
	company := fs.String("company", "personal", "会社プロファイル")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return cli.ExitUsage
	}
	if len(pos) > 1 {
		return d.unexpected(name, pos[1:])
	}
	var sessions []core.Session
	if len(pos) == 1 {
		s, err := d.store(*company).Load(pos[0])
		if err != nil {
			return d.fail(name, err)
		}
		sessions = []core.Session{s}
	} else {
		if sessions, err = d.store(*company).List(); err != nil {
			return d.fail(name, err)
		}
	}
	entries := make([]statusEntry, 0, len(sessions))
	for _, s := range sessions {
		_, statErr := os.Stat(s.Worktree)
		entries = append(entries, statusEntry{Session: s, WorktreeExists: statErr == nil})
	}
	if *jsonOut {
		cli.WriteJSON(d.stdout, map[string]any{"tasks": entries})
		return cli.ExitOK
	}
	if len(entries) == 0 {
		fmt.Fprintln(d.stdout, "タスクなし")
		return cli.ExitOK
	}
	for _, e := range entries {
		note := ""
		if !e.WorktreeExists {
			note = "（worktree 欠落 — task rm で片付ける）"
		}
		fmt.Fprintf(d.stdout, "%s  repo=%s issue=#%d agent=%s runs=%d updated=%s %s\n",
			e.ID, e.Repo, e.Issue, e.LastAgent, e.Runs, e.UpdatedAt.Format(time.RFC3339), note)
	}
	return cli.ExitOK
}

func (d *deps) taskRm(name string, args []string) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	jsonOut := fs.Bool("json", false, "機械処理向けの JSON を stdout に出力する")
	company := fs.String("company", "personal", "会社プロファイル")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return cli.ExitUsage
	}
	if len(pos) != 1 {
		fmt.Fprintf(d.stderr, "agentctl: %s: task id を 1 つ指定する\n", name)
		return cli.ExitUsage
	}
	s, rmErr := d.tasks(*company).Remove(pos[0], d.stderr)
	if *jsonOut {
		cli.WriteJSON(d.stdout, taskJSON{Session: s, OK: rmErr == nil, Error: errString(rmErr)})
	} else if rmErr == nil {
		fmt.Fprintf(d.stdout, "task %s を片付けた（worktree / compose project / session）\n", s.ID)
	}
	if rmErr != nil {
		fmt.Fprintf(d.stderr, "agentctl: %s: %v\n", name, rmErr)
		return exitFor(rmErr)
	}
	return cli.ExitOK
}

func (d *deps) check(name string, args []string) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	jsonOut := fs.Bool("json", false, "機械処理向けの JSON を stdout に出力する")
	company := fs.String("company", "personal", "会社プロファイル")
	repoName := fs.String("repo", "", "設定名（--task 指定時は不要）")
	taskID := fs.String("task", "", "対象タスク（省略時は AGENTCTL_TASK）")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return cli.ExitUsage
	}
	if len(pos) > 0 {
		return d.unexpected(name, pos)
	}
	id := *taskID
	if id == "" {
		id = d.env("AGENTCTL_TASK")
	}
	var repo core.RepoConfig
	var dir, project string
	if id != "" {
		s, err := d.store(*company).Load(id)
		if err != nil {
			return d.fail(name, err)
		}
		if repo, err = d.loadRepo(*company, s.Repo); err != nil {
			return d.failConfig(name, err)
		}
		dir, project = s.Worktree, core.ComposeProject(s.ID)
	} else {
		if repo, err = d.loadRepo(*company, *repoName); err != nil {
			return d.failConfig(name, err)
		}
		dir, project = repo.Path, "agentctl-"+repo.Name
	}
	res := (&core.Checker{Compose: d.compose}).Run(repo, dir, project, d.stderr)
	if *jsonOut {
		cli.WriteJSON(d.stdout, res)
	} else {
		for _, s := range res.Steps {
			mark := "ok"
			if !s.Passed {
				mark = "NG"
			}
			fmt.Fprintf(d.stdout, "%s  %s\n", mark, s.Name)
		}
	}
	if !res.Passed {
		return cli.ExitFailure
	}
	return cli.ExitOK
}

func (d *deps) prDraft(name string, args []string) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	jsonOut := fs.Bool("json", false, "機械処理向けの JSON を stdout に出力する")
	company := fs.String("company", "personal", "会社プロファイル")
	taskID := fs.String("task", "", "対象タスク（省略時は AGENTCTL_TASK）")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return cli.ExitUsage
	}
	if len(pos) > 0 {
		return d.unexpected(name, pos)
	}
	id := *taskID
	if id == "" {
		id = d.env("AGENTCTL_TASK")
	}
	if id == "" {
		return d.fail(name, fmt.Errorf("%w: --task に task id を指定する", core.ErrInvalid))
	}
	st := d.store(*company)
	s, err := st.Load(id)
	if err != nil {
		return d.fail(name, err)
	}
	repo, err := d.loadRepo(*company, s.Repo)
	if err != nil {
		return d.failConfig(name, err)
	}
	url, err := (&core.PRDrafter{Store: st, PRs: d.prs}).Draft(id, repo)
	if err != nil {
		return d.fail(name, err)
	}
	if *jsonOut {
		cli.WriteJSON(d.stdout, map[string]string{"url": url})
	} else {
		fmt.Fprintln(d.stdout, url)
	}
	return cli.ExitOK
}

func (d *deps) doctor(name string, args []string) int {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(d.stderr)
	jsonOut := fs.Bool("json", false, "機械処理向けの JSON を stdout に出力する")
	company := fs.String("company", "personal", "会社プロファイル")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return cli.ExitUsage
	}
	if len(pos) > 0 {
		return d.unexpected(name, pos)
	}
	rep := d.doctorReport(*company)
	if *jsonOut {
		cli.WriteJSON(d.stdout, rep)
	} else {
		for _, c := range rep.Checks {
			mark := "ok"
			if !c.OK {
				mark = "NG"
			}
			fmt.Fprintf(d.stdout, "%s  %-20s %s\n", mark, c.Name, c.Detail)
		}
	}
	if !rep.OK {
		return cli.ExitFailure
	}
	return cli.ExitOK
}
