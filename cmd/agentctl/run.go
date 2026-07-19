package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/matsumoto14/agentctl/internal/cli"
)

// この 7 動詞の外に動詞を追加しない（CLAUDE.md / ADR 0002）。
var commands = [][]string{
	{"task", "start"},
	{"task", "resume"},
	{"task", "status"},
	{"task", "rm"},
	{"check"},
	{"pr", "draft"},
	{"doctor"},
}

type stubResult struct {
	Command     string `json:"command"`
	Implemented bool   `json:"implemented"`
}

// run は stdin を読まず、対話プロンプトも出さない（Gateway コントラクト: 非対話）。
func run(args []string, stdout, stderr io.Writer) int {
	name, rest, ok := match(args)
	if !ok {
		usage(stderr)
		return cli.ExitUsage
	}

	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	jsonOut := fs.Bool("json", false, "機械処理向けの JSON を stdout に出力する")
	pos, err := parseFlags(fs, rest)
	if err != nil {
		return cli.ExitUsage
	}
	// 位置引数を取る動詞はまだ実装していないため、余剰引数は使い方エラーにする。
	if len(pos) > 0 {
		fmt.Fprintf(stderr, "agentctl: %s: unexpected argument %q\n", name, pos[0])
		return cli.ExitUsage
	}

	return notImplemented(name, *jsonOut, stdout, stderr)
}

// parseFlags は位置引数の後ろに置かれたフラグも解析し、位置引数を返す。
// 標準の flag.FlagSet.Parse は最初の位置引数で解析を終了するため、
// `task resume <id> --agent claude` のような形式でフラグが黙って無視されてしまう。
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		args = fs.Args()
		if len(args) == 0 {
			return pos, nil
		}
		pos = append(pos, args[0])
		args = args[1:]
	}
}

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
		if err := cli.WriteJSON(stdout, stubResult{Command: name, Implemented: false}); err != nil {
			fmt.Fprintf(stderr, "agentctl: %s: %v\n", name, err)
			return cli.ExitFailure
		}
	}
	fmt.Fprintf(stderr, "agentctl: %s: not implemented\n", name)
	return cli.ExitUnimplemented
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: agentctl <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	for _, c := range commands {
		fmt.Fprintf(w, "  %s\n", strings.Join(c, " "))
	}
}
