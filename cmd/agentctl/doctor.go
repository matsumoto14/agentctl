package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/matsumoto14/agentctl/internal/cli"
)

func newDoctorCmd(d *deps) *cobra.Command {
	var (
		jsonOut bool
		company string
	)
	c := &cobra.Command{
		Use:   "doctor",
		Short: "環境の非破壊チェック（報告のみ、修復しない）",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			const name = "doctor"
			rep := d.doctorReport(d.company(company))
			if jsonOut {
				if err := cli.WriteJSON(d.stdout, rep); err != nil {
					return d.errorOut(name, false, err)
				}
			} else {
				for _, c := range rep.Checks {
					mark := "ok"
					if !c.OK {
						mark = "NG"
					}
					fmt.Fprintf(d.stdout, "%s  %-20s %s\n", mark, c.Name, c.Detail)
				}
			}
			if !rep.OK {
				return exitCodeError{cli.ExitFailure}
			}
			return nil
		},
	}
	addCommonFlags(c, &jsonOut, &company)
	return c
}
