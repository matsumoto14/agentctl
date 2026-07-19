package worktree

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"
)

func TestAddArgs(t *testing.T) {
	var gotDir string
	var gotArgs []string
	g := &Git{run: func(dir string, args ...string) error {
		gotDir = dir
		gotArgs = args
		return nil
	}}
	if err := g.Add("/repo", "/wt/myapp-issue-1", "agentctl/myapp-issue-1", "main"); err != nil {
		t.Fatal(err)
	}
	if gotDir != "/repo" {
		t.Errorf("dir = %q", gotDir)
	}
	// -B（強制リセット）ではなく -b。既存ブランチ回避は core.pickBranch の責務
	want := []string{"worktree", "add", "-b", "agentctl/myapp-issue-1", "/wt/myapp-issue-1", "main"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("args = %v, want %v", gotArgs, want)
	}
}

func TestRemoveFallsBackToPrune(t *testing.T) {
	var calls [][]string
	g := &Git{run: func(dir string, args ...string) error {
		calls = append(calls, args)
		if args[0] == "worktree" && args[1] == "remove" {
			return errors.New("not a working tree")
		}
		return nil
	}}
	// 存在しないパス → remove 失敗後に prune へフォールバックする
	missing := filepath.Join(t.TempDir(), "gone")
	if err := g.Remove("/repo", missing); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[1][1] != "prune" {
		t.Errorf("calls = %v", calls)
	}
}

func TestBranchExists(t *testing.T) {
	g := &Git{run: func(dir string, args ...string) error {
		if args[len(args)-1] == "refs/remotes/origin/agentctl/x" {
			return nil // origin にだけ存在
		}
		return errors.New("missing")
	}}
	exists, err := g.BranchExists("/repo", "agentctl/x")
	if err != nil || !exists {
		t.Errorf("exists = %v, err = %v（origin 側のブランチも既存扱い）", exists, err)
	}
	g2 := &Git{run: func(dir string, args ...string) error { return errors.New("missing") }}
	exists, err = g2.BranchExists("/repo", "agentctl/y")
	if err != nil || exists {
		t.Errorf("exists = %v, err = %v", exists, err)
	}
}
