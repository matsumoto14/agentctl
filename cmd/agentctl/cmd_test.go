package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/matsumoto14/agentctl/internal/cli"
	"github.com/matsumoto14/agentctl/internal/core"
	"github.com/matsumoto14/agentctl/internal/doctor"
)

type fakeStore struct{ m map[string]core.Session }

func (f *fakeStore) Save(s core.Session) error { f.m[s.ID] = s; return nil }
func (f *fakeStore) Load(id string) (core.Session, error) {
	s, ok := f.m[id]
	if !ok {
		return core.Session{}, core.ErrNotFound
	}
	return s, nil
}
func (f *fakeStore) List() ([]core.Session, error) {
	var out []core.Session
	for _, s := range f.m {
		out = append(out, s)
	}
	return out, nil
}
func (f *fakeStore) Delete(id string) error { delete(f.m, id); return nil }

type fakeLock struct{ err error }

func (f *fakeLock) Acquire() (func(), error) {
	if f.err != nil {
		return nil, f.err
	}
	return func() {}, nil
}

type fakeWT struct{ adds, removes int }

func (f *fakeWT) Add(repo, path, branch, base string) error      { f.adds++; return nil }
func (f *fakeWT) Remove(repo, path string) error                 { f.removes++; return nil }
func (f *fakeWT) BranchExists(repo, branch string) (bool, error) { return false, nil }

type fakeIssues struct{}

func (fakeIssues) Get(repo string, n int) (core.Issue, error) {
	return core.Issue{Number: n, Title: "タイトル", Body: "本文"}, nil
}

type fakeAgents struct {
	agents []core.Agent
	envs   [][]string
	err    error
}

func (f *fakeAgents) Run(ctx context.Context, a core.Agent, dir, prompt string, env []string, out io.Writer) error {
	f.agents = append(f.agents, a)
	f.envs = append(f.envs, env)
	return f.err
}

type fakeCompose struct {
	failStep string
	downs    int
}

func (f *fakeCompose) Run(dir, project, service string, command []string, out io.Writer) error {
	if len(command) > 0 && command[0] == f.failStep {
		return errors.New("failed")
	}
	return nil
}
func (f *fakeCompose) Down(dir, project string, out io.Writer) error { f.downs++; return nil }

type fakePRs struct{ url string }

func (f *fakePRs) CreateDraft(dir, base, title, body string) (string, error) { return f.url, nil }

type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }

type testEnv struct {
	d      *deps
	stdout *bytes.Buffer
	stderr *bytes.Buffer
	store  *fakeStore
	lock   *fakeLock
	wt     *fakeWT
	agents *fakeAgents
	comp   *fakeCompose
	report doctor.Report
	envs   map[string]string
}

func newTestEnv() *testEnv {
	e := &testEnv{
		stdout: &bytes.Buffer{},
		stderr: &bytes.Buffer{},
		store:  &fakeStore{m: map[string]core.Session{}},
		lock:   &fakeLock{},
		wt:     &fakeWT{},
		agents: &fakeAgents{},
		comp:   &fakeCompose{},
		envs:   map[string]string{},
	}
	repo := core.RepoConfig{
		Name: "myapp", Path: "/repo/myapp", GitHub: "owner/myapp", Base: "main",
		Service: "app",
		Checks:  []core.CheckStep{{Name: "lint", Command: []string{"lint"}}, {Name: "test", Command: []string{"test"}}},
	}
	e.d = &deps{
		ctx:    context.Background(),
		stdout: e.stdout,
		stderr: e.stderr,
		env:    func(k string) string { return e.envs[k] },
		now:    func() time.Time { return time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC) },
		loadRepo: func(company, name string) (core.RepoConfig, error) {
			return repo, nil
		},
		store:        func(string) core.SessionStore { return e.store },
		lock:         e.lock,
		logWriter:    func(string, string) (io.WriteCloser, error) { return nopCloser{io.Discard}, nil },
		worktreePath: func(c, id string) string { return "/state/worktrees/" + id },
		worktrees:    e.wt,
		issues:       fakeIssues{},
		agents:       e.agents,
		compose:      e.comp,
		prs:          &fakePRs{url: "https://github.com/owner/myapp/pull/9"},
		doctorReport: func(string) doctor.Report { return e.report },
	}
	return e
}

func (e *testEnv) run(args ...string) int { return execute(args, e.d) }

func TestUsageErrors(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{"引数なし", nil},
		{"未知コマンド", []string{"foo"}},
		{"task のみ", []string{"task"}},
		{"未知サブコマンド", []string{"task", "unknown"}},
		{"pr のみ", []string{"pr"}},
		{"未知フラグ", []string{"check", "--nope"}},
		{"start の余剰引数", []string{"task", "start", "extra"}},
		{"resume の引数なし", []string{"task", "resume"}},
		{"rm の引数過多", []string{"task", "rm", "a", "b"}},
		{"doctor の余剰引数", []string{"doctor", "unexpected"}},
		{"max-minutes 0", []string{"task", "start", "--issue", "12", "--max-minutes", "0"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEnv()
			if code := e.run(tt.args...); code != cli.ExitUsage {
				t.Errorf("exit code = %d, want %d (stderr=%q)", code, cli.ExitUsage, e.stderr.String())
			}
			if e.stdout.String() != "" && !strings.Contains(strings.Join(tt.args, " "), "--json") {
				t.Errorf("stdout は空であるべき: %q", e.stdout.String())
			}
		})
	}
}

func TestBareInvocationShowsHelpOnStderr(t *testing.T) {
	e := newTestEnv()
	if code := e.run(); code != cli.ExitUsage {
		t.Fatalf("exit = %d", code)
	}
	for _, verb := range []string{"task", "check", "pr", "doctor"} {
		if !strings.Contains(e.stderr.String(), verb) {
			t.Errorf("help に %s がない: %q", verb, e.stderr.String())
		}
	}
}

// 実行時エラー（exit 1/3）でも --json 時は stdout に単一の JSON オブジェクトを返す。
func TestJSONOnErrorPaths(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"status: 不在タスク", []string{"task", "status", "myapp-issue-99", "--json"}, cli.ExitPrecondition},
		{"resume: 不在タスク", []string{"task", "resume", "myapp-issue-99", "--json"}, cli.ExitPrecondition},
		{"pr draft: task 未指定", []string{"pr", "draft", "--json"}, cli.ExitUsage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newTestEnv()
			if code := e.run(tt.args...); code != tt.wantCode {
				t.Fatalf("exit = %d, want %d", code, tt.wantCode)
			}
			var v map[string]any
			if err := json.Unmarshal(e.stdout.Bytes(), &v); err != nil {
				t.Fatalf("stdout が JSON でない: %q: %v", e.stdout.String(), err)
			}
			if ok, _ := v["ok"].(bool); ok {
				t.Errorf("ok = true: %v", v)
			}
		})
	}
}
