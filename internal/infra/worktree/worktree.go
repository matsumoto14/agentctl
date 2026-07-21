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

// Add は base から新しいブランチを切って worktree を作る。既存ブランチの
// 強制リセット（-B）は行わない — 未使用名の選択は core 側の責務（pickBranch）。
func (g *Git) Add(repoPath, worktreePath, branch, base string) error {
	return g.run(repoPath, "worktree", "add", "-b", branch, worktreePath, base)
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

// BranchExists は local と origin remote-tracking のどちらかにブランチがあるかを返す。
// rev-parse の失敗は「存在しない」と同義に扱う（リポ自体の異常は直後の Add が報告する）。
func (g *Git) BranchExists(repoPath, branch string) (bool, error) {
	for _, ref := range []string{"refs/heads/" + branch, "refs/remotes/origin/" + branch} {
		if err := g.run(repoPath, "rev-parse", "--verify", "--quiet", ref); err == nil {
			return true, nil
		}
	}
	return false, nil
}
