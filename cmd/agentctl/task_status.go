package main

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/matsumoto14/agentctl/internal/cli"
	"github.com/matsumoto14/agentctl/internal/core"
)

type statusEntry struct {
	core.Session
	WorktreeExists bool `json:"worktree_exists"`
}

func newTaskStatusCmd(d *deps) *cobra.Command {
	var (
		jsonOut bool
		company string
	)
	c := &cobra.Command{
		Use:   "status [task-id]",
		Short: "session と worktree の事実を報告する（引数なしで全タスク列挙）",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			const name = "task status"
			comp := d.company(company)
			var sessions []core.Session
			if len(args) == 1 {
				s, err := d.store(comp).Load(args[0])
				if err != nil {
					return d.errorOut(name, jsonOut, err)
				}
				sessions = []core.Session{s}
			} else {
				var err error
				if sessions, err = d.store(comp).List(); err != nil {
					return d.errorOut(name, jsonOut, err)
				}
			}
			entries := make([]statusEntry, 0, len(sessions))
			for _, s := range sessions {
				_, statErr := os.Stat(s.Worktree)
				entries = append(entries, statusEntry{Session: s, WorktreeExists: statErr == nil})
			}
			if jsonOut {
				if err := cli.WriteJSON(d.stdout, map[string]any{"tasks": entries}); err != nil {
					return d.errorOut(name, false, err)
				}
				return nil
			}
			if len(entries) == 0 {
				fmt.Fprintln(d.stdout, "タスクなし")
				return nil
			}
			for _, e := range entries {
				note := ""
				if !e.WorktreeExists {
					note = "（worktree 欠落 — task rm で片付ける）"
				}
				fmt.Fprintf(d.stdout, "%s  repo=%s issue=#%d agent=%s runs=%d updated=%s %s\n",
					e.ID, e.Repo, e.Issue, e.LastAgent, e.Runs, e.UpdatedAt.Format(time.RFC3339), note)
			}
			return nil
		},
	}
	addCommonFlags(c, &jsonOut, &company)
	return c
}
