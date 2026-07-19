package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"strings"
	"testing"

	"github.com/matsumoto14/agentctl/internal/cli"
)

func runCapture(args []string) (code int, stdout, stderr string) {
	var out, errBuf bytes.Buffer
	code = run(args, &out, &errBuf)
	return code, out.String(), errBuf.String()
}

func TestRunUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"引数なし", nil},
		{"未知コマンド", []string{"foo"}},
		{"task のみ", []string{"task"}},
		{"未知サブコマンド", []string{"task", "unknown"}},
		{"pr のみ", []string{"pr"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, stderr := runCapture(tt.args)
			if code != cli.ExitUsage {
				t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
			}
			if !strings.Contains(stderr, "usage: agentctl") {
				t.Errorf("stderr に usage がない: %q", stderr)
			}
			if stdout != "" {
				t.Errorf("stdout は空であるべき: %q", stdout)
			}
		})
	}
}

func TestRunUnknownFlag(t *testing.T) {
	code, _, _ := runCapture([]string{"check", "--nope"})
	if code != cli.ExitUsage {
		t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
	}
}

// 位置引数を取る動詞は未実装のため、余剰の位置引数は使い方エラーになる。
// 位置引数の後ろのフラグが黙って無視されないことも保証する。
func TestRunUnexpectedArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"余剰引数", []string{"doctor", "unexpected"}},
		{"位置引数の後ろの既知フラグ", []string{"task", "resume", "abc", "--json"}},
		{"位置引数の後ろの未知フラグ", []string{"task", "resume", "abc", "--agent", "claude"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, stdout, _ := runCapture(tt.args)
			if code != cli.ExitUsage {
				t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
			}
			if strings.Contains(stdout, "implemented") {
				t.Errorf("未実装スタブとして処理された: %q", stdout)
			}
		})
	}
}

func TestParseFlagsAfterPositionals(t *testing.T) {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	jsonOut := fs.Bool("json", false, "")
	agent := fs.String("agent", "", "")
	pos, err := parseFlags(fs, []string{"abc", "--agent", "claude", "--json"})
	if err != nil {
		t.Fatal(err)
	}
	if len(pos) != 1 || pos[0] != "abc" {
		t.Errorf("pos = %v, want [abc]", pos)
	}
	if !*jsonOut || *agent != "claude" {
		t.Errorf("json = %v, agent = %q; 位置引数の後ろのフラグが解析されていない", *jsonOut, *agent)
	}
}

func TestRunAllCommandsAreStubs(t *testing.T) {
	for _, c := range commands {
		name := strings.Join(c, " ")
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := runCapture(c)
			if code != cli.ExitUnimplemented {
				t.Errorf("exit code = %d, want %d", code, cli.ExitUnimplemented)
			}
			want := "agentctl: " + name + ": not implemented\n"
			if stderr != want {
				t.Errorf("stderr = %q, want %q", stderr, want)
			}
			if stdout != "" {
				t.Errorf("--json なしで stdout に出力がある: %q", stdout)
			}
		})
	}
}

func TestRunAllCommandsAcceptJSONFlag(t *testing.T) {
	for _, c := range commands {
		name := strings.Join(c, " ")
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := runCapture(append(append([]string{}, c...), "--json"))
			if code != cli.ExitUnimplemented {
				t.Errorf("exit code = %d, want %d", code, cli.ExitUnimplemented)
			}
			var got stubResult
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("stdout が JSON として解釈できない: %q: %v", stdout, err)
			}
			if got.Command != name || got.Implemented {
				t.Errorf("JSON = %+v, want {Command: %q, Implemented: false}", got, name)
			}
			if !strings.Contains(stderr, "not implemented") {
				t.Errorf("stderr = %q", stderr)
			}
		})
	}
}
