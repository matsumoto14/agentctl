package main

import (
	"github.com/spf13/cobra"

	"github.com/matsumoto14/agentctl/internal/cli"
)

func newTaskCmd(d *deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "task",
		Short: "タスク（worktree + session）の操作",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitCodeError{cli.ExitUsage}
		},
	}
	c.AddCommand(newTaskStartCmd(d), newTaskResumeCmd(d), newTaskStatusCmd(d), newTaskRmCmd(d))
	return c
}
