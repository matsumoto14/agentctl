package main

import (
	"flag"
	"fmt"
	"io"
	"strings"

	"github.com/matsumoto14/agentctl/internal/cli"
)

type command struct {
	tokens []string
	run    func(d *deps, name string, args []string) int
}

// この 7 動詞の外に動詞を追加しない（CLAUDE.md / ADR 0002）。
var commands = []command{
	{[]string{"task", "start"}, (*deps).taskStart},
	{[]string{"task", "resume"}, (*deps).taskResume},
	{[]string{"task", "status"}, (*deps).taskStatus},
	{[]string{"task", "rm"}, (*deps).taskRm},
	{[]string{"check"}, (*deps).check},
	{[]string{"pr", "draft"}, (*deps).prDraft},
	{[]string{"doctor"}, (*deps).doctor},
}

// run は stdin を読まず、対話プロンプトも出さない（Gateway コントラクト: 非対話）。
func run(args []string, d *deps) int {
	name, rest, c, ok := match(args)
	if !ok {
		usage(d.stderr)
		return cli.ExitUsage
	}
	return c.run(d, name, rest)
}

func match(args []string) (name string, rest []string, c command, ok bool) {
	for _, cand := range commands {
		if len(args) < len(cand.tokens) {
			continue
		}
		matched := true
		for i, w := range cand.tokens {
			if args[i] != w {
				matched = false
				break
			}
		}
		if matched {
			return strings.Join(cand.tokens, " "), args[len(cand.tokens):], cand, true
		}
	}
	return "", nil, command{}, false
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

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: agentctl <command> [flags]")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "commands:")
	for _, c := range commands {
		fmt.Fprintf(w, "  %s\n", strings.Join(c.tokens, " "))
	}
}
