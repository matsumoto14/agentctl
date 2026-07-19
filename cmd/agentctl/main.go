package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/matsumoto14/agentctl/internal/cli"
	"github.com/matsumoto14/agentctl/internal/config"
	"github.com/matsumoto14/agentctl/internal/core"
	"github.com/matsumoto14/agentctl/internal/doctor"
	"github.com/matsumoto14/agentctl/internal/infra/agent"
	"github.com/matsumoto14/agentctl/internal/infra/compose"
	"github.com/matsumoto14/agentctl/internal/infra/githubx"
	"github.com/matsumoto14/agentctl/internal/infra/state"
	"github.com/matsumoto14/agentctl/internal/infra/worktree"
)

func main() {
	// Ctrl-C / SIGTERM で実行中の Agent を止め、session の保存まで進める
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	os.Exit(execute(os.Args[1:], realDeps(ctx)))
}

// execute は cobra の結果を終了コード規約（0/1/2/3/10）へ写像する。
func execute(args []string, d *deps) int {
	root := newRootCmd(d)
	root.SetArgs(args)
	err := root.Execute()
	if err == nil {
		return cli.ExitOK
	}
	var ec exitCodeError
	if errors.As(err, &ec) {
		return ec.code
	}
	// cobra 由来（未知コマンド・未知フラグ・引数の数）は使い方エラー
	fmt.Fprintf(d.stderr, "agentctl: %v\n", err)
	return cli.ExitUsage
}

func realDeps(ctx context.Context) *deps {
	stateRoot := filepath.Join(xdgStateHome(), "agentctl")
	configDir := os.Getenv("AGENTCTL_CONFIG_DIR")
	if configDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			configDir = filepath.Join(home, "git", "agentctl", "config")
		}
	}
	companyDir := func(c string) string { return filepath.Join(stateRoot, "companies", c) }
	storeFor := func(c string) *state.Store { return state.NewStore(companyDir(c)) }
	gh := githubx.New()

	return &deps{
		ctx:    ctx,
		stdout: os.Stdout,
		stderr: os.Stderr,
		env:    os.Getenv,
		now:    time.Now,
		loadRepo: func(company, name string) (core.RepoConfig, error) {
			r, err := config.LoadRepo(configDir, company, name)
			if err != nil {
				return core.RepoConfig{}, err
			}
			return toCoreRepo(r), nil
		},
		store: func(c string) core.SessionStore { return storeFor(c) },
		// 「同じ実行環境で Agent を同時に起動しない」ため、lock は会社を跨いだ
		// ホスト単位に置く（会社毎だと --company 違いで並走できてしまう）
		lock:         state.NewLock(stateRoot),
		logWriter:    func(c, id string) (io.WriteCloser, error) { return storeFor(c).LogWriter(id) },
		worktreePath: func(c, id string) string { return storeFor(c).WorktreePath(id) },
		worktrees:    worktree.New(),
		issues:       gh,
		agents:       agent.Runner{},
		compose:      compose.Runner{},
		prs:          gh,
		doctorReport: func(company string) doctor.Report {
			st := storeFor(company)
			sessions, listErr := st.List()
			opts := doctor.Options{
				Binaries:  []string{"git", "docker", "gh", "codex", "claude"},
				StateDir:  companyDir(company),
				ConfigDir: configDir,
				LoadConfig: func() error {
					_, err := config.LoadAll(configDir, company)
					return err
				},
				Sessions:       sessions,
				BrokenSessions: st.ListBroken(),
				WorktreesDir:   st.WorktreesDir(),
			}
			if listErr != nil {
				opts.SessionsError = listErr.Error()
			}
			return doctor.Run(opts)
		},
	}
}

func toCoreRepo(r config.Repo) core.RepoConfig {
	rc := core.RepoConfig{
		Name: r.Name, Path: r.Path, GitHub: r.GitHub, Base: r.Base,
		Service: r.Compose.Service,
	}
	for _, c := range r.Checks {
		rc.Checks = append(rc.Checks, core.CheckStep{Name: c.Name, Command: c.Run})
	}
	return rc
}

func xdgStateHome() string {
	if v := os.Getenv("XDG_STATE_HOME"); v != "" {
		return v
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state")
}
