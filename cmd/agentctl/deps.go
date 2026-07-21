package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

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
	lock         core.Locker // 実行環境（ホスト）単位。会社毎に分けると Agent が並走できてしまう
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
		Store: d.store(company), Lock: d.lock,
		Worktrees: d.worktrees, Issues: d.issues, Agents: d.agents, Compose: d.compose,
		Now: d.now,
	}
}

// company は --company > AGENTCTL_COMPANY > personal の順で決める。
// Agent 起動時に AGENTCTL_COMPANY を渡してあるため（core.runAgent）、Agent が
// worktree 内から呼ぶ check / task status も正しい会社プロファイルを引ける。
func (d *deps) company(flagVal string) string {
	if flagVal != "" {
		return flagVal
	}
	if v := d.env("AGENTCTL_COMPANY"); v != "" {
		return v
	}
	return "personal"
}

// 全コマンド共通のフラグ。
func addCommonFlags(c *cobra.Command, jsonOut *bool, company *string) {
	c.Flags().BoolVar(jsonOut, "json", false, "機械処理向けの JSON を stdout に出力する")
	c.Flags().StringVar(company, "company", "", "会社プロファイル（既定: $AGENTCTL_COMPANY か personal）")
}

// exitCodeError は「メッセージ報告済み・終了コード確定」を表す。
type exitCodeError struct{ code int }

func (e exitCodeError) Error() string { return fmt.Sprintf("exit status %d", e.code) }

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

// errorOut はエラーを報告して exitCodeError に畳む。--json 時は stdout にも
// 機械可読な {ok:false} を出す（実行時エラーでも JSON を返す Gateway コントラクト）。
func (d *deps) errorOut(name string, jsonOut bool, err error) error {
	return d.errorOutCode(name, jsonOut, err, exitFor(err))
}

func (d *deps) errorOutCode(name string, jsonOut bool, err error, code int) error {
	if jsonOut {
		_ = cli.WriteJSON(d.stdout, map[string]any{"ok": false, "error": err.Error()})
	}
	fmt.Fprintf(d.stderr, "agentctl: %s: %v\n", name, err)
	return exitCodeError{code}
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

// taskResult は session を伴うコマンド（start / resume / rm）の共通出力。
func (d *deps) taskResult(name string, s core.Session, err error, jsonOut bool, successMsg string) error {
	if jsonOut {
		if werr := cli.WriteJSON(d.stdout, taskJSON{Session: s, OK: err == nil, Error: errString(err)}); werr != nil && err == nil {
			err = werr
		}
	} else if err == nil {
		fmt.Fprintln(d.stdout, successMsg)
	}
	if err != nil {
		fmt.Fprintf(d.stderr, "agentctl: %s: %v\n", name, err)
		return exitCodeError{exitFor(err)}
	}
	return nil
}
