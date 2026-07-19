package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/matsumoto14/agentctl/internal/cli"
	"github.com/matsumoto14/agentctl/internal/core"
)

func newCheckCmd(d *deps) *cobra.Command {
	var (
		jsonOut  bool
		company  string
		repoName string
		taskID   string
	)
	c := &cobra.Command{
		Use:   "check",
		Short: "設定された lint / test / build を compose 経由で順に実行する",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			const name = "check"
			comp := d.company(company)
			id := taskID
			if id == "" {
				id = d.env("AGENTCTL_TASK")
			}
			var repo core.RepoConfig
			var dir, project string
			if id != "" {
				s, err := d.store(comp).Load(id)
				if err != nil {
					return d.errorOut(name, jsonOut, err)
				}
				if repo, err = d.loadRepo(comp, s.Repo); err != nil {
					return d.errorOutCode(name, jsonOut, err, cli.ExitPrecondition)
				}
				dir, project = s.Worktree, core.ComposeProject(s.ID)
			} else {
				var err error
				if repo, err = d.loadRepo(comp, repoName); err != nil {
					return d.errorOutCode(name, jsonOut, err, cli.ExitPrecondition)
				}
				dir, project = repo.Path, "agentctl-"+repo.Name
			}
			res := (&core.Checker{Compose: d.compose}).Run(repo, dir, project, d.stderr)
			if jsonOut {
				if err := cli.WriteJSON(d.stdout, res); err != nil {
					return d.errorOut(name, false, err)
				}
			} else {
				for _, s := range res.Steps {
					mark := "ok"
					if !s.Passed {
						mark = "NG"
					}
					fmt.Fprintf(d.stdout, "%s  %s\n", mark, s.Name)
				}
			}
			if !res.Passed {
				return exitCodeError{cli.ExitFailure}
			}
			return nil
		},
	}
	addCommonFlags(c, &jsonOut, &company)
	c.Flags().StringVar(&repoName, "repo", "", "設定名（--task 指定時は不要）")
	c.Flags().StringVar(&taskID, "task", "", "対象タスク（省略時は $AGENTCTL_TASK）")
	return c
}
