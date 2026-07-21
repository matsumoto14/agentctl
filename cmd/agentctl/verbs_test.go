package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/matsumoto14/agentctl/internal/cli"
	"github.com/matsumoto14/agentctl/internal/core"
	"github.com/matsumoto14/agentctl/internal/doctor"
)

func TestCheckPassAndFail(t *testing.T) {
	e := newTestEnv()
	e.store.m["myapp-issue-12"] = core.Session{ID: "myapp-issue-12", Repo: "myapp", Worktree: "/wt"}
	if code := e.run("check", "--task", "myapp-issue-12", "--json"); code != cli.ExitOK {
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
	e2.store.m["myapp-issue-12"] = core.Session{ID: "myapp-issue-12", Repo: "myapp", Worktree: "/wt"}
	e2.comp.failStep = "lint"
	if code := e2.run("check", "--task", "myapp-issue-12"); code != cli.ExitFailure {
		t.Errorf("exit = %d, want %d", code, cli.ExitFailure)
	}
}

func TestCheckUsesEnvTask(t *testing.T) {
	e := newTestEnv()
	e.store.m["myapp-issue-7"] = core.Session{ID: "myapp-issue-7", Repo: "myapp", Worktree: "/wt"}
	e.envs["AGENTCTL_TASK"] = "myapp-issue-7"
	if code := e.run("check"); code != cli.ExitOK {
		t.Errorf("exit = %d, stderr = %s", code, e.stderr.String())
	}
}

func TestPRDraft(t *testing.T) {
	e := newTestEnv()
	e.store.m["myapp-issue-12"] = core.Session{ID: "myapp-issue-12", Issue: 12, Repo: "myapp", IssueTitle: "タイトル", Worktree: "/wt"}
	if code := e.run("pr", "draft", "--task", "myapp-issue-12"); code != cli.ExitOK {
		t.Fatalf("exit = %d, stderr = %s", code, e.stderr.String())
	}
	if !strings.Contains(e.stdout.String(), "/pull/9") {
		t.Errorf("stdout = %q", e.stdout.String())
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
