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
	if err := g.Add("/repo", "/wt/issue-1", "agentctl/issue-1", "main"); err != nil {
		t.Fatal(err)
	}
	if gotDir != "/repo" {
		t.Errorf("dir = %q", gotDir)
	}
	want := []string{"worktree", "add", "-B", "agentctl/issue-1", "/wt/issue-1", "main"}
	if !reflect.DeepEqual(gotArgs, want) {
		t.Errorf("args = %v, want %v", gotArgs, want)
	}
}

func TestRemoveFallsBackToPrune(t *testing.T) {
	var calls [][]string
	g := &Git{run: func(dir string, args ...string) error {
		calls = append(calls, args)
		if args[1] == "remove" {
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
