// Package githubx は GitHub 操作を gh CLI 経由で実装する。トークンは gh が保持し、
// agentctl から Gateway や Agent へ生のクレデンシャルを渡さない（設計書 §8.3）。
package githubx

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/matsumoto14/agentctl/internal/core"
)

type Client struct {
	// テストから exec を差し替えるための間接化
	run func(dir, name string, args ...string) (string, error)
}

func New() *Client { return &Client{run: runCmd} }

func runCmd(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	var out, errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s %v: %w: %s", name, args, err, strings.TrimSpace(errBuf.String()))
	}
	return out.String(), nil
}

func (c *Client) Get(repo string, number int) (core.Issue, error) {
	out, err := c.run("", "gh", "issue", "view", fmt.Sprint(number),
		"--repo", repo, "--json", "number,title,body,url")
	if err != nil {
		return core.Issue{}, err
	}
	var v struct {
		Number int    `json:"number"`
		Title  string `json:"title"`
		Body   string `json:"body"`
		URL    string `json:"url"`
	}
	if err := json.Unmarshal([]byte(out), &v); err != nil {
		return core.Issue{}, fmt.Errorf("gh issue view の出力: %w", err)
	}
	return core.Issue{Number: v.Number, Title: v.Title, Body: v.Body, URL: v.URL}, nil
}

func (c *Client) CreateDraft(dir, base, title, body string) (string, error) {
	if _, err := c.run(dir, "git", "push", "-u", "origin", "HEAD"); err != nil {
		return "", err
	}
	out, err := c.run(dir, "gh", "pr", "create", "--draft",
		"--base", base, "--title", title, "--body", body)
	if err != nil {
		return "", err
	}
	// gh は最終行に PR URL を出力する
	lines := strings.Split(strings.TrimSpace(out), "\n")
	return strings.TrimSpace(lines[len(lines)-1]), nil
}
