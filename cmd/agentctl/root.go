package main

import (
	"github.com/spf13/cobra"

	"github.com/matsumoto14/agentctl/internal/cli"
)

// newRootCmd は純粋なルーター。ロジックは各コマンドファイルと internal に置く。
func newRootCmd(d *deps) *cobra.Command {
	root := &cobra.Command{
		Use:           "agentctl",
		Short:         "GitHub Issue 起点の開発作業を支援する道具箱",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_ = cmd.Help()
			return exitCodeError{cli.ExitUsage}
		},
	}
	// stdout は機械可読な結果専用とし、help / usage は stderr へ出す
	root.SetOut(d.stderr)
	root.SetErr(d.stderr)
	// 7 動詞の外にコマンドを増やさない（completion / help サブコマンドも無効化）
	root.CompletionOptions.DisableDefaultCmd = true
	root.SetHelpCommand(&cobra.Command{Use: "no-help", Hidden: true})
	root.AddCommand(newTaskCmd(d), newCheckCmd(d), newPRCmd(d), newDoctorCmd(d))
	return root
}
