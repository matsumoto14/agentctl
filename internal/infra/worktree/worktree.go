// Package worktree は git worktree の作成と削除を実装する。
package worktree

import (
	"fmt"
	"os"
	"os/exec"
)

type Git struct {
	// テストから exec を差し替えるための間接化
	run func(dir string, args ...string) error
}

func New() *Git { return &Git{run: runGit} }

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git %v: %w: %s", args, err, out)
	}
	return nil
}

// Add は base から新しいブランチを切って worktree を作る。task rm 後に同じ Issue を
// やり直せるよう、同名ブランチが残っていても -B で base へ作り直す。
func (g *Git) Add(repoPath, worktreePath, branch, base string) error {
	return g.run(repoPath, "worktree", "add", "-B", branch, worktreePath, base)
}

func (g *Git) Remove(repoPath, worktreePath string) error {
	err := g.run(repoPath, "worktree", "remove", "--force", worktreePath)
	if err != nil {
		if _, statErr := os.Stat(worktreePath); os.IsNotExist(statErr) {
			// ディレクトリが手動で消されている場合は登録の掃除だけ行う
			return g.run(repoPath, "worktree", "prune")
		}
	}
	return err
}
