package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/matsumoto14/agentctl/internal/cliio"
)

// commands は実装対象の全コマンド。この 7 動詞の外に追加しない（CLAUDE.md / ADR 0002）。
var commands = [][]string{
	{"task", "start"},
	{"task", "resume"},
	{"task", "status"},
	{"task", "rm"},
	{"check"},
	{"pr", "draft"},
	{"doctor"},
}

// stubResult は未実装コマンドの --json 出力。
type stubResult struct {
	Command     string `json:"command"`
	Implemented bool   `json:"implemented"`
}

// run は引数を解釈してコマンドへディスパッチし、終了コードを返す。
// stdin は読まず、対話プロンプトを出さない。
func run(args []string, stdout, stderr io.Writer) int {
	name, rest, ok := match(args)
	if !ok {
		usage(stderr)
		return cliio.ExitUsage
	}

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "機械処理向けの JSON を stdout に出力する")
	if err := fs.Parse(rest); err != nil {
		return cliio.ExitUsage
	}

	return notImplemented(name, *jsonOut, stdout, stderr)
}

// match は args の先頭をコマンド定義と照合し、コマンド名と残りの引数を返す。
func match(args []string) (name string, rest []string, ok bool) {
	for _, c := range commands {
		if len(args) < len(c) {
			continue
		}
		matched := true
		for i, w := range c {
			if args[i] != w {
				matched = false
				break
			}
		}
		if matched {
			return strings.Join(c, " "), args[len(c):], true
		}
	}
	return "", nil, false
}

func notImplemented(name string, jsonOut bool, stdout, stderr io.Writer) int {
	if jsonOut {
		if err := cliio.WriteJSON(stdout, stubResult{Command: name, Implemented: false}); err != nil {
			fmt.Fprintf(stderr, "agentctl: %s: %v\n", name, err)
			return cliio.ExitFailure
		}
	}
	fmt.Fprintf(stderr, "agentctl: %s: not implemented\n", name)
	return cliio.ExitUnimplemented
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: agentctl <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	for _, c := range commands {
		fmt.Fprintf(w, "  %s\n", strings.Join(c, " "))
	}
}
