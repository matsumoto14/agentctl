package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/matsumoto14/agentctl/internal/cliio"
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
			if code != cliio.ExitUsage {
				t.Errorf("exit code = %d, want %d", code, cliio.ExitUsage)
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
	if code != cliio.ExitUsage {
		t.Errorf("exit code = %d, want %d", code, cliio.ExitUsage)
	}
}

func TestRunAllCommandsAreStubs(t *testing.T) {
	for _, c := range commands {
		name := strings.Join(c, " ")
		t.Run(name, func(t *testing.T) {
			code, stdout, stderr := runCapture(c)
			if code != cliio.ExitUnimplemented {
				t.Errorf("exit code = %d, want %d", code, cliio.ExitUnimplemented)
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
			if code != cliio.ExitUnimplemented {
				t.Errorf("exit code = %d, want %d", code, cliio.ExitUnimplemented)
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
