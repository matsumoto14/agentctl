package main

import (
	"context"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

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
	os.Exit(run(os.Args[1:], realDeps(ctx)))
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
		store:        func(c string) core.SessionStore { return storeFor(c) },
		locker:       func(c string) core.Locker { return state.NewLock(companyDir(c)) },
		logWriter:    func(c, id string) (io.WriteCloser, error) { return storeFor(c).LogWriter(id) },
		worktreePath: func(c, id string) string { return storeFor(c).WorktreePath(id) },
		worktrees:    worktree.New(),
		issues:       gh,
		agents:       agent.Runner{},
		compose:      compose.Runner{},
		prs:          gh,
		doctorReport: func(company string) doctor.Report {
			st := storeFor(company)
			sessions, _ := st.List()
			return doctor.Run(doctor.Options{
				Binaries:  []string{"git", "docker", "gh", "codex", "claude"},
				StateDir:  companyDir(company),
				ConfigDir: configDir,
				LoadConfig: func() error {
					_, err := config.LoadAll(configDir, company)
					return err
				},
				Sessions:     sessions,
				WorktreesDir: st.WorktreesDir(),
			})
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
