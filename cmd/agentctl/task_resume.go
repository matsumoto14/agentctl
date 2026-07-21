package main

import (
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/matsumoto14/agentctl/internal/cli"
	"github.com/matsumoto14/agentctl/internal/core"
)

func newTaskResumeCmd(d *deps) *cobra.Command {
	var (
		jsonOut    bool
		company    string
		agentName  string
		maxMinutes int
	)
	c := &cobra.Command{
		Use:   "resume <task-id>",
		Short: "既存の worktree で Agent を再度 1 回起動する（git 操作なし）",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			const name = "task resume"
			if maxMinutes <= 0 {
				return d.errorOut(name, jsonOut, fmt.Errorf("%w: --max-minutes は 1 以上を指定する", core.ErrInvalid))
			}
			var ag core.Agent
			if agentName != "" {
				var err error
				if ag, err = core.ParseAgent(agentName); err != nil {
					return d.errorOut(name, jsonOut, err)
				}
			}
			comp := d.company(company)
			id := args[0]
			prev, err := d.store(comp).Load(id)
			if err != nil {
				return d.errorOut(name, jsonOut, err)
			}
			repo, err := d.loadRepo(comp, prev.Repo)
			if err != nil {
				return d.errorOutCode(name, jsonOut, err, cli.ExitPrecondition)
			}
			logw, err := d.logWriter(comp, id)
			if err != nil {
				return d.errorOut(name, jsonOut, err)
			}
			defer logw.Close()
			s, runErr := d.tasks(comp).Resume(d.ctx, core.ResumeParams{
				ID: id, Repo: repo, Agent: ag,
				Timeout:  time.Duration(maxMinutes) * time.Minute,
				AgentOut: io.MultiWriter(d.stderr, logw),
			})
			msg := fmt.Sprintf("task %s: agent %s の実行が完了（runs=%d）", s.ID, s.LastAgent, s.Runs)
			return d.taskResult(name, s, runErr, jsonOut, msg)
		},
	}
	addCommonFlags(c, &jsonOut, &company)
	c.Flags().StringVar(&agentName, "agent", "", "起動する Agent（省略時は前回と同じ）")
	c.Flags().IntVar(&maxMinutes, "max-minutes", defaultMaxMinutes, "Agent 実行時間の上限（分）")
	return c
}
