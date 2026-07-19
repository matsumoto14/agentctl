package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/matsumoto14/agentctl/internal/cli"
	"github.com/matsumoto14/agentctl/internal/core"
)

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
	if !got.OK || got.ID != "myapp-issue-12" || got.Runs != 1 || got.LastAgent != core.AgentCodex {
		t.Errorf("json = %+v", got)
	}
	if e.wt.adds != 1 {
		t.Errorf("worktree adds = %d", e.wt.adds)
	}
	if len(e.agents.envs) != 1 || !contains(e.agents.envs[0], "AGENTCTL_TASK=myapp-issue-12") {
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

// --company 未指定でも AGENTCTL_COMPANY があればそれを使う（Agent からの再帰呼び出し経路）。
func TestCompanyFromEnv(t *testing.T) {
	e := newTestEnv()
	e.envs["AGENTCTL_COMPANY"] = "acme"
	var gotCompany string
	orig := e.d.loadRepo
	e.d.loadRepo = func(company, name string) (core.RepoConfig, error) {
		gotCompany = company
		return orig(company, name)
	}
	if code := e.run("task", "start", "--issue", "12"); code != cli.ExitOK {
		t.Fatalf("exit = %d", code)
	}
	if gotCompany != "acme" {
		t.Errorf("company = %q, want acme", gotCompany)
	}
	if !contains(e.agents.envs[0], "AGENTCTL_COMPANY=acme") {
		t.Errorf("agent env = %v", e.agents.envs)
	}
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
	e.agents.err = errStub("agent died")
	if code := e.run("task", "start", "--issue", "12"); code != cli.ExitFailure {
		t.Errorf("exit = %d, want %d", code, cli.ExitFailure)
	}
	// 失敗した起動も事実として記録される
	if s := e.store.m["myapp-issue-12"]; s.Runs != 1 {
		t.Errorf("Runs = %d", s.Runs)
	}
}

type errStub string

func (e errStub) Error() string { return string(e) }

// 位置引数の後ろのフラグも解析される（task resume <id> --agent claude 形式）。
func TestTaskResumeSwitchesAgentWithFlagAfterPositional(t *testing.T) {
	e := newTestEnv()
	e.store.m["myapp-issue-12"] = core.Session{ID: "myapp-issue-12", Issue: 12, Repo: "myapp", Worktree: "/wt", LastAgent: core.AgentCodex, Runs: 1}
	code := e.run("task", "resume", "myapp-issue-12", "--agent", "claude")
	if code != cli.ExitOK {
		t.Fatalf("exit = %d, stderr = %s", code, e.stderr.String())
	}
	if len(e.agents.agents) != 1 || e.agents.agents[0] != core.AgentClaude {
		t.Errorf("agents = %v", e.agents.agents)
	}
	if s := e.store.m["myapp-issue-12"]; s.Runs != 2 || s.LastAgent != core.AgentClaude {
		t.Errorf("session = %+v", s)
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
	e.store.m["myapp-issue-12"] = core.Session{ID: "myapp-issue-12", Issue: 12, Repo: "myapp", Worktree: "/no/such/dir"}
	if code := e.run("task", "status"); code != cli.ExitOK {
		t.Fatalf("exit = %d", code)
	}
	out := e.stdout.String()
	if !strings.Contains(out, "myapp-issue-12") || !strings.Contains(out, "worktree 欠落") {
		t.Errorf("stdout = %q", out)
	}
}

func TestTaskRm(t *testing.T) {
	e := newTestEnv()
	e.store.m["myapp-issue-12"] = core.Session{ID: "myapp-issue-12", RepoPath: "/repo/myapp", Worktree: "/wt"}
	if code := e.run("task", "rm", "myapp-issue-12"); code != cli.ExitOK {
		t.Fatalf("exit = %d, stderr = %s", code, e.stderr.String())
	}
	if _, ok := e.store.m["myapp-issue-12"]; ok {
		t.Error("session が残っている")
	}
	if e.comp.downs != 1 || e.wt.removes != 1 {
		t.Errorf("downs = %d, removes = %d", e.comp.downs, e.wt.removes)
	}
}
