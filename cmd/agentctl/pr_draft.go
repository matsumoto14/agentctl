package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/matsumoto14/agentctl/internal/cli"
	"github.com/matsumoto14/agentctl/internal/core"
)

func newPRCmd(d *deps) *cobra.Command {
	c := &cobra.Command{
		Use:   "pr",
		Short: "Pull Request の操作",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitCodeError{cli.ExitUsage}
		},
	}
	c.AddCommand(newPRDraftCmd(d))
	return c
}

func newPRDraftCmd(d *deps) *cobra.Command {
	var (
		jsonOut bool
		company string
		taskID  string
	)
	c := &cobra.Command{
		Use:   "draft",
		Short: "タスクのブランチを push して Draft PR を作成する",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			const name = "pr draft"
			comp := d.company(company)
			id := taskID
			if id == "" {
				id = d.env("AGENTCTL_TASK")
			}
			if id == "" {
				return d.errorOut(name, jsonOut, fmt.Errorf("%w: --task に task id を指定する", core.ErrInvalid))
			}
			s, err := d.store(comp).Load(id)
			if err != nil {
				return d.errorOut(name, jsonOut, err)
			}
			repo, err := d.loadRepo(comp, s.Repo)
			if err != nil {
				return d.errorOutCode(name, jsonOut, err, cli.ExitPrecondition)
			}
			url, err := (&core.PRDrafter{PRs: d.prs}).Draft(s, repo)
			if err != nil {
				return d.errorOut(name, jsonOut, err)
			}
			if jsonOut {
				if err := cli.WriteJSON(d.stdout, map[string]string{"url": url}); err != nil {
					return d.errorOut(name, false, err)
				}
			} else {
				fmt.Fprintln(d.stdout, url)
			}
			return nil
		},
	}
	addCommonFlags(c, &jsonOut, &company)
	c.Flags().StringVar(&taskID, "task", "", "対象タスク（省略時は $AGENTCTL_TASK）")
	return c
}
