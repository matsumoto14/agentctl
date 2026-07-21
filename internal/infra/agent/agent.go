// Package agent は Coding Agent CLI のホスト直実行を実装する。
// コンテナ内実行や Docker socket の共有は行わない（設計書 §9.2）。
package agent

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"github.com/matsumoto14/agentctl/internal/core"
)

type Runner struct{}

// commandFor は各 Agent のヘッドレス起動形式を返す（実仕様の確認は設計書 §16-2）。
func commandFor(a core.Agent, prompt string) []string {
	switch a {
	case core.AgentCodex:
		return []string{"codex", "exec", prompt}
	case core.AgentClaude:
		return []string{"claude", "-p", prompt}
	default:
		return nil
	}
}

func (Runner) Run(ctx context.Context, a core.Agent, dir, prompt string, env []string, out io.Writer) error {
	argv := commandFor(a, prompt)
	if argv == nil {
		return fmt.Errorf("%w: unknown agent %q", core.ErrInvalid, a)
	}
	cmd := exec.CommandContext(ctx, argv[0], argv[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdout = out
	cmd.Stderr = out
	// タイムアウト時は Agent が起動した子プロセスごと止める
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", argv[0], err)
	}
	return nil
}
