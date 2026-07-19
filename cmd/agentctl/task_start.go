package main

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/matsumoto14/agentctl/internal/cli"
	"github.com/matsumoto14/agentctl/internal/core"
)

func newTaskStartCmd(d *deps) *cobra.Command {
	var (
		jsonOut    bool
		company    string
		repoName   string
		agentName  string
		issueNo    int
		maxMinutes int
	)
	c := &cobra.Command{
		Use:   "start",
		Short: "worktree と session を作成し、Agent を 1 回起動する",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			const name = "task start"
			if issueNo <= 0 {
				return d.errorOut(name, jsonOut, fmt.Errorf("%w: --issue に Issue 番号を指定する", core.ErrInvalid))
			}
			if maxMinutes <= 0 {
				return d.errorOut(name, jsonOut, fmt.Errorf("%w: --max-minutes は 1 以上を指定する", core.ErrInvalid))
			}
			ag, err := core.ParseAgent(agentName)
			if err != nil {
				return d.errorOut(name, jsonOut, err)
			}
			comp := d.company(company)
			repo, err := d.loadRepo(comp, repoName)
			if err != nil {
				return d.errorOutCode(name, jsonOut, err, cli.ExitPrecondition)
			}
			id := core.TaskID(repo.Name, issueNo)
			logw, err := d.logWriter(comp, id)
			if err != nil {
				return d.errorOut(name, jsonOut, err)
			}
			defer logw.Close()
			s, runErr := d.tasks(comp).Start(d.ctx, core.StartParams{
				Company: comp, Repo: repo, Issue: issueNo, Agent: ag,
				Timeout:      time.Duration(maxMinutes) * time.Minute,
				WorktreePath: d.worktreePath(comp, id),
				AgentOut:     io.MultiWriter(d.stderr, logw),
			})
			msg := fmt.Sprintf("task %s: agent %s の実行が完了（runs=%d）", s.ID, s.LastAgent, s.Runs)
			return d.taskResult(name, s, runErr, jsonOut, msg)
		},
	}
	addCommonFlags(c, &jsonOut, &company)
	c.Flags().StringVar(&repoName, "repo", "", "設定名（config/repos/<company>/ に 1 つだけなら省略可）")
	c.Flags().IntVar(&issueNo, "issue", 0, "GitHub Issue 番号")
	c.Flags().StringVar(&agentName, "agent", string(core.AgentCodex), "起動する Agent（codex|claude）")
	c.Flags().IntVar(&maxMinutes, "max-minutes", defaultMaxMinutes, "Agent 実行時間の上限（分）")
	return c
}
