package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
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

func (f *fakeWT) Add(repo, path, branch, base string) error { f.adds++; return nil }
func (f *fakeWT) Remove(repo, path string) error            { f.removes++; return nil }

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
		locker:       func(string) core.Locker { return e.lock },
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

func (e *testEnv) run(args ...string) int { return run(args, e.d) }

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
			e := newTestEnv()
			if code := e.run(tt.args...); code != cli.ExitUsage {
				t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
			}
			if !strings.Contains(e.stderr.String(), "usage: agentctl") {
				t.Errorf("stderr に usage がない: %q", e.stderr.String())
			}
		})
	}
}

func TestRunUnknownFlagAndUnexpectedArgs(t *testing.T) {
	tests := [][]string{
		{"check", "--nope"},
		{"doctor", "unexpected"},
		{"task", "start", "extra"},
		{"task", "resume"},
		{"task", "rm", "a", "b"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			e := newTestEnv()
			if code := e.run(args...); code != cli.ExitUsage {
				t.Errorf("exit code = %d, want %d", code, cli.ExitUsage)
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

func TestTaskStart(t *testing.T) {
	e := newTestEnv()
	code := e.run("task", "start", "--issue", "12", "--json")
	if code != cli.ExitOK {
		t.Fatalf("exit = %d, stderr = %s", code, e.stderr.String())
	}
	var got taskJSON
	if err := json.Unmarshal(e.stdout.Bytes(), &got); err != nil {
		t.Fatalf("stdout = %q: %v", e.stdout.String(), err)
	}
	if !got.OK || got.ID != "issue-12" || got.Runs != 1 || got.LastAgent != core.AgentCodex {
		t.Errorf("json = %+v", got)
	}
	if e.wt.adds != 1 {
		t.Errorf("worktree adds = %d", e.wt.adds)
	}
	if len(e.agents.envs) != 1 || !contains(e.agents.envs[0], "AGENTCTL_TASK=issue-12") {
		t.Errorf("agent env = %v", e.agents.envs)
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}

func TestTaskStartRequiresIssue(t *testing.T) {
	e := newTestEnv()
	if code := e.run("task", "start"); code != cli.ExitUsage {
		t.Errorf("exit = %d, want %d", code, cli.ExitUsage)
	}
}

func TestTaskStartInvalidAgent(t *testing.T) {
	e := newTestEnv()
	if code := e.run("task", "start", "--issue", "1", "--agent", "gpt"); code != cli.ExitUsage {
		t.Errorf("exit = %d, want %d", code, cli.ExitUsage)
	}
}

func TestTaskStartLocked(t *testing.T) {
	e := newTestEnv()
	e.lock.err = core.ErrLocked
	if code := e.run("task", "start", "--issue", "12"); code != cli.ExitPrecondition {
		t.Errorf("exit = %d, want %d", code, cli.ExitPrecondition)
	}
}

func TestTaskStartAgentFailure(t *testing.T) {
	e := newTestEnv()
	e.agents.err = errors.New("agent died")
	if code := e.run("task", "start", "--issue", "12"); code != cli.ExitFailure {
		t.Errorf("exit = %d, want %d", code, cli.ExitFailure)
	}
	// 失敗した起動も事実として記録される
	if s := e.store.m["issue-12"]; s.Runs != 1 {
		t.Errorf("Runs = %d", s.Runs)
	}
}

func TestTaskResumeSwitchesAgent(t *testing.T) {
	e := newTestEnv()
	e.store.m["issue-12"] = core.Session{ID: "issue-12", Issue: 12, Repo: "myapp", Worktree: "/wt", LastAgent: core.AgentCodex, Runs: 1}
	code := e.run("task", "resume", "issue-12", "--agent", "claude")
	if code != cli.ExitOK {
		t.Fatalf("exit = %d, stderr = %s", code, e.stderr.String())
	}
	if len(e.agents.agents) != 1 || e.agents.agents[0] != core.AgentClaude {
		t.Errorf("agents = %v", e.agents.agents)
	}
	if s := e.store.m["issue-12"]; s.Runs != 2 || s.LastAgent != core.AgentClaude {
		t.Errorf("session = %+v", s)
	}
}

func TestTaskResumeNotFound(t *testing.T) {
	e := newTestEnv()
	if code := e.run("task", "resume", "issue-99"); code != cli.ExitPrecondition {
		t.Errorf("exit = %d, want %d", code, cli.ExitPrecondition)
	}
}

func TestTaskStatusEmptyJSON(t *testing.T) {
	e := newTestEnv()
	if code := e.run("task", "status", "--json"); code != cli.ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if got := strings.TrimSpace(e.stdout.String()); got != `{"tasks":[]}` {
		t.Errorf("stdout = %q", got)
	}
}

func TestTaskStatusList(t *testing.T) {
	e := newTestEnv()
	e.store.m["issue-12"] = core.Session{ID: "issue-12", Issue: 12, Repo: "myapp", Worktree: "/no/such/dir"}
	if code := e.run("task", "status"); code != cli.ExitOK {
		t.Fatalf("exit = %d", code)
	}
	out := e.stdout.String()
	if !strings.Contains(out, "issue-12") || !strings.Contains(out, "worktree 欠落") {
		t.Errorf("stdout = %q", out)
	}
}

func TestTaskRm(t *testing.T) {
	e := newTestEnv()
	e.store.m["issue-12"] = core.Session{ID: "issue-12", RepoPath: "/repo/myapp", Worktree: "/wt"}
	if code := e.run("task", "rm", "issue-12"); code != cli.ExitOK {
		t.Fatalf("exit = %d, stderr = %s", code, e.stderr.String())
	}
	if _, ok := e.store.m["issue-12"]; ok {
		t.Error("session が残っている")
	}
	if e.comp.downs != 1 || e.wt.removes != 1 {
		t.Errorf("downs = %d, removes = %d", e.comp.downs, e.wt.removes)
	}
}

func TestCheckPassAndFail(t *testing.T) {
	e := newTestEnv()
	e.store.m["issue-12"] = core.Session{ID: "issue-12", Repo: "myapp", Worktree: "/wt"}
	if code := e.run("check", "--task", "issue-12", "--json"); code != cli.ExitOK {
		t.Fatalf("exit = %d", code)
	}
	var res core.CheckResult
	if err := json.Unmarshal(e.stdout.Bytes(), &res); err != nil {
		t.Fatal(err)
	}
	if !res.Passed || len(res.Steps) != 2 {
		t.Errorf("result = %+v", res)
	}

	e2 := newTestEnv()
	e2.store.m["issue-12"] = core.Session{ID: "issue-12", Repo: "myapp", Worktree: "/wt"}
	e2.comp.failStep = "lint"
	if code := e2.run("check", "--task", "issue-12"); code != cli.ExitFailure {
		t.Errorf("exit = %d, want %d", code, cli.ExitFailure)
	}
}

func TestCheckUsesEnvTask(t *testing.T) {
	e := newTestEnv()
	e.store.m["issue-7"] = core.Session{ID: "issue-7", Repo: "myapp", Worktree: "/wt"}
	e.envs["AGENTCTL_TASK"] = "issue-7"
	if code := e.run("check"); code != cli.ExitOK {
		t.Errorf("exit = %d, stderr = %s", code, e.stderr.String())
	}
}

func TestPRDraft(t *testing.T) {
	e := newTestEnv()
	e.store.m["issue-12"] = core.Session{ID: "issue-12", Issue: 12, Repo: "myapp", IssueTitle: "タイトル", Worktree: "/wt"}
	if code := e.run("pr", "draft", "--task", "issue-12"); code != cli.ExitOK {
		t.Fatalf("exit = %d, stderr = %s", code, e.stderr.String())
	}
	if !strings.Contains(e.stdout.String(), "/pull/9") {
		t.Errorf("stdout = %q", e.stdout.String())
	}
}

func TestPRDraftRequiresTask(t *testing.T) {
	e := newTestEnv()
	if code := e.run("pr", "draft"); code != cli.ExitUsage {
		t.Errorf("exit = %d, want %d", code, cli.ExitUsage)
	}
}

func TestDoctor(t *testing.T) {
	e := newTestEnv()
	e.report = doctor.Report{OK: true, Checks: []doctor.Check{{Name: "binary:git", OK: true}}}
	if code := e.run("doctor"); code != cli.ExitOK {
		t.Errorf("exit = %d", code)
	}
	e2 := newTestEnv()
	e2.report = doctor.Report{OK: false, Checks: []doctor.Check{{Name: "binary:gh", OK: false}}}
	if code := e2.run("doctor", "--json"); code != cli.ExitFailure {
		t.Errorf("exit = %d, want %d", code, cli.ExitFailure)
	}
	var rep doctor.Report
	if err := json.Unmarshal(e2.stdout.Bytes(), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.OK {
		t.Errorf("report = %+v", rep)
	}
}
