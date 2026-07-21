package doctor

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/matsumoto14/agentctl/internal/core"
)

func fakeExec() (func(string) (string, error), func() (string, error)) {
	return func(name string) (string, error) { return "/usr/bin/" + name, nil },
		func() (string, error) { return "v9.9.9", nil }
}

func TestRunAllOK(t *testing.T) {
	lookPath, composeVersion := fakeExec()
	dir := t.TempDir()
	wt := filepath.Join(dir, "worktrees", "myapp-issue-1")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	r := Run(Options{
		Binaries:       []string{"git", "docker"},
		StateDir:       dir,
		ConfigDir:      "/cfg",
		LoadConfig:     func() error { return nil },
		Sessions:       []core.Session{{ID: "myapp-issue-1", Worktree: wt}},
		WorktreesDir:   filepath.Join(dir, "worktrees"),
		LookPath:       lookPath,
		ComposeVersion: composeVersion,
	})
	if !r.OK {
		t.Errorf("report = %+v", r)
	}
}

// doctor は report-only であり、state dir の未作成を異常とせず、作成もしない。
func TestRunDoesNotCreateStateDir(t *testing.T) {
	lookPath, composeVersion := fakeExec()
	missing := filepath.Join(t.TempDir(), "not-yet")
	r := Run(Options{
		StateDir:       missing,
		LookPath:       lookPath,
		ComposeVersion: composeVersion,
	})
	if !r.OK {
		t.Errorf("report = %+v", r)
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Error("doctor が state dir を作成した（report-only 違反）")
	}
}

func TestRunReportsProblems(t *testing.T) {
	_, composeVersion := fakeExec()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "worktrees", "orphan"), 0o755); err != nil {
		t.Fatal(err)
	}
	r := Run(Options{
		Binaries:       []string{"codex"},
		StateDir:       dir,
		LoadConfig:     func() error { return errors.New("設定がない") },
		Sessions:       []core.Session{{ID: "myapp-issue-9", Worktree: filepath.Join(dir, "worktrees", "gone")}},
		BrokenSessions: []string{"myapp-issue-8"},
		WorktreesDir:   filepath.Join(dir, "worktrees"),
		LookPath:       func(string) (string, error) { return "", errors.New("not found") },
		ComposeVersion: composeVersion,
	})
	if r.OK {
		t.Error("問題があるのに OK")
	}
	want := map[string]bool{
		"binary:codex":          false,
		"config":                false,
		"session:myapp-issue-9": false,
		"session:myapp-issue-8": false,
		"worktree:orphan":       false,
	}
	for _, c := range r.Checks {
		if _, target := want[c.Name]; target {
			want[c.Name] = true
		}
	}
	for n, found := range want {
		if !found {
			t.Errorf("%s が報告されていない: %+v", n, r.Checks)
		}
	}
}
