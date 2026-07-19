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
	d, cleanup, err := downDir(dir)
	if err != nil {
		return err
	}
	defer cleanup()
	return execDocker(d, out, downArgs(project))
}

// downDir は down の実行ディレクトリを決める。worktree が既に消えていても down を
// 再試行できるようにする（rm の unwind が途中失敗した後の回収経路）。空ディレクトリ
// から実行すると Compose v2.21+ は compose ファイルなしで project ラベルから対象を
// 解決する。cwd の無関係な compose ファイルを誤読しないよう、必ず空ディレクトリを使う。
func downDir(dir string) (string, func(), error) {
	if _, err := os.Stat(dir); err == nil {
		return dir, func() {}, nil
	}
	empty, err := os.MkdirTemp("", "agentctl-down-*")
	if err != nil {
		return "", nil, err
	}
	return empty, func() { os.RemoveAll(empty) }, nil
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
