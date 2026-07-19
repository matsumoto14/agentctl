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
	wt := filepath.Join(dir, "worktrees", "issue-1")
	if err := os.MkdirAll(wt, 0o755); err != nil {
		t.Fatal(err)
	}
	r := Run(Options{
		Binaries:       []string{"git", "docker"},
		StateDir:       dir,
		ConfigDir:      "/cfg",
		LoadConfig:     func() error { return nil },
		Sessions:       []core.Session{{ID: "issue-1", Worktree: wt}},
		WorktreesDir:   filepath.Join(dir, "worktrees"),
		LookPath:       lookPath,
		ComposeVersion: composeVersion,
	})
	if !r.OK {
		t.Errorf("report = %+v", r)
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
		Sessions:       []core.Session{{ID: "issue-9", Worktree: filepath.Join(dir, "worktrees", "gone")}},
		WorktreesDir:   filepath.Join(dir, "worktrees"),
		LookPath:       func(string) (string, error) { return "", errors.New("not found") },
		ComposeVersion: composeVersion,
	})
	if r.OK {
		t.Error("問題があるのに OK")
	}
	var names []string
	for _, c := range r.Checks {
		if !c.OK {
			names = append(names, c.Name)
		}
	}
	want := map[string]bool{"binary:codex": true, "config": true, "session:issue-9": true, "worktree:orphan": true}
	for n := range want {
		found := false
		for _, got := range names {
			if got == n {
				found = true
			}
		}
		if !found {
			t.Errorf("%s が報告されていない: %v", n, names)
		}
	}
}
