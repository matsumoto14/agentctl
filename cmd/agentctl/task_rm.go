package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newTaskRmCmd(d *deps) *cobra.Command {
	var (
		jsonOut bool
		company string
	)
	c := &cobra.Command{
		Use:   "rm <task-id>",
		Short: "worktree / compose project / session を一組として片付ける",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			const name = "task rm"
			comp := d.company(company)
			s, rmErr := d.tasks(comp).Remove(args[0], d.stderr)
			msg := fmt.Sprintf("task %s を片付けた（worktree / compose project / session）", s.ID)
			return d.taskResult(name, s, rmErr, jsonOut, msg)
		},
	}
	addCommonFlags(c, &jsonOut, &company)
	return c
}
