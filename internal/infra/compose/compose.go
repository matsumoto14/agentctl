// Package compose は対象リポジトリの既存 docker compose を無改変で利用する（C2）。
package compose

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

type Runner struct{}

// runArgs は -p でタスク毎に project 名を分離し、run --rm でポートを公開しない
// 実行形式に固定する（設計書 §9.3）。
func runArgs(project, service string, command []string) []string {
	return append([]string{"compose", "-p", project, "run", "--rm", service}, command...)
}

func downArgs(project string) []string {
	return []string{"compose", "-p", project, "down", "--remove-orphans"}
}

func (Runner) Run(dir, project, service string, command []string, out io.Writer) error {
	return execDocker(dir, out, runArgs(project, service, command))
}

func (Runner) Down(dir, project string, out io.Writer) error {
	// worktree が既に消えていても project 単位の down は成立する
	if _, err := os.Stat(dir); err != nil {
		dir = ""
	}
	return execDocker(dir, out, downArgs(project))
}

func execDocker(dir string, out io.Writer, args []string) error {
	cmd := exec.Command("docker", args...)
	cmd.Dir = dir
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker %v: %w", args, err)
	}
	return nil
}
