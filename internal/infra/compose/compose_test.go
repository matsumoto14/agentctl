package compose

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestRunArgs(t *testing.T) {
	got := runArgs("agentctl-issue-12", "app", []string{"npm", "test"})
	want := []string{"compose", "-p", "agentctl-issue-12", "run", "--rm", "app", "npm", "test"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func TestDownArgs(t *testing.T) {
	got := downArgs("agentctl-issue-12")
	want := []string{"compose", "-p", "agentctl-issue-12", "down", "--remove-orphans"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func TestDownDirExisting(t *testing.T) {
	dir := t.TempDir()
	got, cleanup, err := downDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if got != dir {
		t.Errorf("dir = %q, want %q", got, dir)
	}
}

// worktree 消失後の down 再試行は、空ディレクトリからのラベルベース down になる。
func TestDownDirMissingUsesEmptyTempDir(t *testing.T) {
	got, cleanup, err := downDir(filepath.Join(t.TempDir(), "gone"))
	if err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(got)
	if err != nil || len(entries) != 0 {
		t.Errorf("空ディレクトリでない: %v, err=%v", entries, err)
	}
	cleanup()
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Error("cleanup でテンポラリディレクトリが消えていない")
	}
}
