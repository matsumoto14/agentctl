package core

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"
)

type fakeStore struct {
	m       map[string]Session
	saveErr error
	loadErr error // ErrNotFound 以外の読込失敗を再現する
}

func newFakeStore() *fakeStore { return &fakeStore{m: map[string]Session{}} }

func (f *fakeStore) Save(s Session) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.m[s.ID] = s
	return nil
}

func (f *fakeStore) Load(id string) (Session, error) {
	if f.loadErr != nil {
		return Session{}, f.loadErr
	}
	s, ok := f.m[id]
	if !ok {
		return Session{}, ErrNotFound
	}
	return s, nil
}

func (f *fakeStore) List() ([]Session, error) {
	var out []Session
	for _, s := range f.m {
		out = append(out, s)
	}
	return out, nil
}

func (f *fakeStore) Delete(id string) error {
	if _, ok := f.m[id]; !ok {
		return ErrNotFound
	}
	delete(f.m, id)
	return nil
}

type fakeLock struct {
	err      error
	acquired int
	released int
}

func (f *fakeLock) Acquire() (func(), error) {
	if f.err != nil {
		return nil, f.err
	}
	f.acquired++
	return func() { f.released++ }, nil
}

type wtCall struct{ repo, path, branch, base string }

type fakeWT struct {
	adds      []wtCall
	removes   []wtCall
	addErr    error
	removeErr error
	branches  map[string]bool // 既存扱いにするブランチ名
}

func (f *fakeWT) Add(repo, path, branch, base string) error {
	f.adds = append(f.adds, wtCall{repo, path, branch, base})
	return f.addErr
}

func (f *fakeWT) Remove(repo, path string) error {
	f.removes = append(f.removes, wtCall{repo: repo, path: path})
	return f.removeErr
}

func (f *fakeWT) BranchExists(repo, branch string) (bool, error) {
	return f.branches[branch], nil
}

type fakeIssues struct {
	issue Issue
	err   error
}

func (f *fakeIssues) Get(repo string, n int) (Issue, error) { return f.issue, f.err }

type agentCall struct {
	agent  Agent
	dir    string
	prompt string
	env    []string
}

type fakeAgents struct {
	calls []agentCall
	err   error
	// block が真なら ctx の期限まで待つ（タイムアウト検証用）
	block bool
}

func (f *fakeAgents) Run(ctx context.Context, a Agent, dir, prompt string, env []string, out io.Writer) error {
	f.calls = append(f.calls, agentCall{a, dir, prompt, env})
	if f.block {
		<-ctx.Done()
		return ctx.Err()
	}
	return f.err
}

type composeCall struct {
	dir, project, service string
	command               []string
}

type fakeCompose struct {
	runs     []composeCall
	downs    []composeCall
	failStep string
	downErr  error
}

func (f *fakeCompose) Run(dir, project, service string, command []string, out io.Writer) error {
	f.runs = append(f.runs, composeCall{dir, project, service, command})
	if len(command) > 0 && command[0] == f.failStep {
		return errors.New("step failed")
	}
	return nil
}

func (f *fakeCompose) Down(dir, project string, out io.Writer) error {
	f.downs = append(f.downs, composeCall{dir: dir, project: project})
	return f.downErr
}

func testRepo() RepoConfig {
	return RepoConfig{
		Name: "myapp", Path: "/repo/myapp", GitHub: "owner/myapp", Base: "main",
		Service: "app",
		Checks: []CheckStep{
			{Name: "lint", Command: []string{"lint"}},
			{Name: "test", Command: []string{"test"}},
		},
	}
}

func newTasks() (*Tasks, *fakeStore, *fakeLock, *fakeWT, *fakeIssues, *fakeAgents, *fakeCompose) {
	st := newFakeStore()
	lk := &fakeLock{}
	wt := &fakeWT{branches: map[string]bool{}}
	is := &fakeIssues{issue: Issue{Number: 12, Title: "タイトル", Body: "本文"}}
	ag := &fakeAgents{}
	cp := &fakeCompose{}
	t := &Tasks{Store: st, Lock: lk, Worktrees: wt, Issues: is, Agents: ag, Compose: cp,
		Now: func() time.Time { return time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC) }}
	return t, st, lk, wt, is, ag, cp
}

func startParams() StartParams {
	return StartParams{
		Company: "personal", Repo: testRepo(), Issue: 12, Agent: AgentCodex,
		Timeout: time.Minute, WorktreePath: "/state/worktrees/myapp-issue-12", AgentOut: io.Discard,
	}
}

func TestStart(t *testing.T) {
	tasks, st, lk, wt, _, ag, _ := newTasks()
	s, err := tasks.Start(context.Background(), startParams())
	if err != nil {
		t.Fatal(err)
	}
	if s.ID != "myapp-issue-12" || s.Branch != "agentctl/myapp-issue-12" || s.Runs != 1 || s.LastAgent != AgentCodex {
		t.Errorf("session = %+v", s)
	}
	if got := st.m["myapp-issue-12"]; got.Runs != 1 {
		t.Errorf("保存された session の Runs = %d", got.Runs)
	}
	if len(wt.adds) != 1 || wt.adds[0].branch != "agentctl/myapp-issue-12" || wt.adds[0].base != "main" {
		t.Errorf("worktree add = %+v", wt.adds)
	}
	if len(ag.calls) != 1 {
		t.Fatalf("agent 起動回数 = %d, want 1（1回起動が規約）", len(ag.calls))
	}
	call := ag.calls[0]
	if call.dir != "/state/worktrees/myapp-issue-12" {
		t.Errorf("agent dir = %q", call.dir)
	}
	if !strings.Contains(call.prompt, "Issue #12") || !strings.Contains(call.prompt, "agentctl check") {
		t.Errorf("prompt に必要な指示がない: %q", call.prompt)
	}
	for _, want := range []string{"AGENTCTL_TASK=myapp-issue-12", "AGENTCTL_COMPANY=personal"} {
		found := false
		for _, e := range call.env {
			if e == want {
				found = true
			}
		}
		if !found {
			t.Errorf("%s が env にない: %v", want, call.env)
		}
	}
	if lk.acquired != 1 || lk.released != 1 {
		t.Errorf("lock acquired=%d released=%d", lk.acquired, lk.released)
	}
}

func TestStartValidation(t *testing.T) {
	tasks, _, _, _, _, _, _ := newTasks()
	p := startParams()
	p.Issue = 0
	if _, err := tasks.Start(context.Background(), p); !errors.Is(err, ErrInvalid) {
		t.Errorf("err = %v, want ErrInvalid", err)
	}
}

func TestStartLocked(t *testing.T) {
	tasks, _, lk, wt, _, _, _ := newTasks()
	lk.err = ErrLocked
	if _, err := tasks.Start(context.Background(), startParams()); !errors.Is(err, ErrLocked) {
		t.Errorf("err = %v, want ErrLocked", err)
	}
	if len(wt.adds) != 0 {
		t.Error("lock 取得失敗後に worktree が作られた")
	}
}

func TestStartExisting(t *testing.T) {
	tasks, st, _, _, _, ag, _ := newTasks()
	st.m["myapp-issue-12"] = Session{ID: "myapp-issue-12"}
	if _, err := tasks.Start(context.Background(), startParams()); !errors.Is(err, ErrExists) {
		t.Errorf("err = %v, want ErrExists", err)
	}
	if len(ag.calls) != 0 {
		t.Error("既存タスクに対して agent が起動された")
	}
}

// 「読めない」を「存在しない」と扱うと既存作業を上書きするため、必ず中断する。
func TestStartAbortsOnUnreadableSession(t *testing.T) {
	tasks, st, _, wt, _, _, _ := newTasks()
	st.loadErr = errors.New("permission denied")
	if _, err := tasks.Start(context.Background(), startParams()); err == nil || !strings.Contains(err.Error(), "既存 session の確認") {
		t.Errorf("err = %v", err)
	}
	if len(wt.adds) != 0 {
		t.Error("session が読めないのに worktree が作られた")
	}
}

// session の保存が失敗したら worktree を作らない（write-ahead）。
// 逆順だと「記録のない worktree」が残り、task rm で片付けられなくなる。
func TestStartSaveFailureCreatesNothing(t *testing.T) {
	tasks, st, _, wt, _, ag, _ := newTasks()
	st.saveErr = errors.New("disk full")
	_, err := tasks.Start(context.Background(), startParams())
	if err == nil || !strings.Contains(err.Error(), "session の保存") {
		t.Errorf("err = %v", err)
	}
	if len(wt.adds) != 0 {
		t.Error("保存失敗なのに worktree が作られた")
	}
	if len(ag.calls) != 0 {
		t.Error("保存失敗なのに agent が起動された")
	}
}

// worktree 作成に失敗しても session は残り、task rm で一組として回収できる。
func TestStartWorktreeFailureIsRecoverableByRm(t *testing.T) {
	tasks, st, _, wt, _, ag, _ := newTasks()
	wt.addErr = errors.New("path exists")
	_, err := tasks.Start(context.Background(), startParams())
	if err == nil || !strings.Contains(err.Error(), "task rm") {
		t.Errorf("err = %v（rm への誘導がない）", err)
	}
	if _, ok := st.m["myapp-issue-12"]; !ok {
		t.Fatal("session が残っていない（rm で回収できない）")
	}
	if len(ag.calls) != 0 {
		t.Error("worktree 作成失敗なのに agent が起動された")
	}
	// 回収経路: rm が session ごと片付けられる
	if _, err := tasks.Remove("myapp-issue-12", io.Discard); err != nil {
		t.Errorf("rm での回収に失敗: %v", err)
	}
	if _, ok := st.m["myapp-issue-12"]; ok {
		t.Error("rm 後も session が残っている")
	}
}

// 過去の試行のブランチが残っていても、強制リセットせず未使用の名前を選ぶ。
func TestStartPicksUnusedBranch(t *testing.T) {
	tasks, _, _, wt, _, _, _ := newTasks()
	wt.branches["agentctl/myapp-issue-12"] = true
	wt.branches["agentctl/myapp-issue-12-2"] = true
	s, err := tasks.Start(context.Background(), startParams())
	if err != nil {
		t.Fatal(err)
	}
	if s.Branch != "agentctl/myapp-issue-12-3" {
		t.Errorf("branch = %q", s.Branch)
	}
}

func TestStartTimeout(t *testing.T) {
	tasks, st, _, _, _, ag, _ := newTasks()
	ag.block = true
	p := startParams()
	p.Timeout = 10 * time.Millisecond
	s, err := tasks.Start(context.Background(), p)
	if err == nil || !strings.Contains(err.Error(), "--max-minutes") {
		t.Errorf("err = %v, want タイムアウト", err)
	}
	if s.Runs != 1 {
		t.Errorf("Runs = %d（失敗した起動も事実として記録する）", s.Runs)
	}
	if got := st.m["myapp-issue-12"]; got.Runs != 1 {
		t.Errorf("保存された Runs = %d", got.Runs)
	}
}

func TestResumeKeepsLastAgent(t *testing.T) {
	tasks, st, _, _, _, ag, _ := newTasks()
	st.m["myapp-issue-12"] = Session{ID: "myapp-issue-12", Issue: 12, Company: "personal", Worktree: "/wt", LastAgent: AgentClaude, Runs: 1}
	s, err := tasks.Resume(context.Background(), ResumeParams{ID: "myapp-issue-12", Repo: testRepo(), Timeout: time.Minute, AgentOut: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if s.LastAgent != AgentClaude || s.Runs != 2 {
		t.Errorf("session = %+v", s)
	}
	if ag.calls[0].agent != AgentClaude {
		t.Errorf("agent = %v, want 前回と同じ claude", ag.calls[0].agent)
	}
}

func TestResumeSwitchesAgent(t *testing.T) {
	tasks, st, _, _, _, ag, _ := newTasks()
	st.m["myapp-issue-12"] = Session{ID: "myapp-issue-12", Issue: 12, Worktree: "/wt", LastAgent: AgentCodex, Runs: 3}
	s, err := tasks.Resume(context.Background(), ResumeParams{ID: "myapp-issue-12", Repo: testRepo(), Agent: AgentClaude, Timeout: time.Minute, AgentOut: io.Discard})
	if err != nil {
		t.Fatal(err)
	}
	if s.LastAgent != AgentClaude || s.Runs != 4 {
		t.Errorf("session = %+v", s)
	}
	if ag.calls[0].agent != AgentClaude {
		t.Errorf("agent = %v", ag.calls[0].agent)
	}
}

func TestResumeNotFound(t *testing.T) {
	tasks, _, _, _, _, _, _ := newTasks()
	if _, err := tasks.Resume(context.Background(), ResumeParams{ID: "myapp-issue-99", Repo: testRepo(), Timeout: time.Minute}); !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestRemoveUnwindsAll(t *testing.T) {
	tasks, st, lk, wt, _, _, cp := newTasks()
	st.m["myapp-issue-12"] = Session{ID: "myapp-issue-12", RepoPath: "/repo/myapp", Worktree: "/wt/myapp-issue-12"}
	if _, err := tasks.Remove("myapp-issue-12", io.Discard); err != nil {
		t.Fatal(err)
	}
	if len(cp.downs) != 1 || cp.downs[0].project != "agentctl-myapp-issue-12" {
		t.Errorf("compose down = %+v", cp.downs)
	}
	if len(wt.removes) != 1 || wt.removes[0].path != "/wt/myapp-issue-12" {
		t.Errorf("worktree remove = %+v", wt.removes)
	}
	if _, ok := st.m["myapp-issue-12"]; ok {
		t.Error("session が削除されていない")
	}
	if lk.acquired != 1 || lk.released != 1 {
		t.Errorf("rm が lock を取っていない: acquired=%d", lk.acquired)
	}
}

// Agent 実行中（lock 保持中）の rm は破壊を防ぐため拒否される。
func TestRemoveLocked(t *testing.T) {
	tasks, st, lk, wt, _, _, _ := newTasks()
	st.m["myapp-issue-12"] = Session{ID: "myapp-issue-12"}
	lk.err = ErrLocked
	if _, err := tasks.Remove("myapp-issue-12", io.Discard); !errors.Is(err, ErrLocked) {
		t.Errorf("err = %v, want ErrLocked", err)
	}
	if len(wt.removes) != 0 {
		t.Error("lock 未取得で worktree が削除された")
	}
}

// unwind が完遂できないときは session を残す。消すと task rm を再実行できなくなる。
func TestRemoveKeepsSessionOnFailure(t *testing.T) {
	tasks, st, _, wt, _, _, _ := newTasks()
	st.m["myapp-issue-12"] = Session{ID: "myapp-issue-12", RepoPath: "/repo/myapp", Worktree: "/wt/myapp-issue-12"}
	wt.removeErr = errors.New("worktree locked")
	_, err := tasks.Remove("myapp-issue-12", io.Discard)
	if err == nil || !strings.Contains(err.Error(), "worktree remove") {
		t.Errorf("err = %v", err)
	}
	if _, ok := st.m["myapp-issue-12"]; !ok {
		t.Error("失敗した unwind で session が削除された")
	}
}

func TestCheckerStopsAtFirstFailure(t *testing.T) {
	cp := &fakeCompose{failStep: "lint"}
	c := &Checker{Compose: cp}
	r := c.Run(testRepo(), "/wt", "agentctl-myapp-issue-12", io.Discard)
	if r.Passed {
		t.Error("Passed = true")
	}
	if len(r.Steps) != 1 || r.Steps[0].Name != "lint" || r.Steps[0].Passed {
		t.Errorf("steps = %+v（最初の失敗で打ち切る）", r.Steps)
	}
	if len(cp.runs) != 1 {
		t.Errorf("compose 実行回数 = %d", len(cp.runs))
	}
}

func TestCheckerAllPass(t *testing.T) {
	cp := &fakeCompose{}
	c := &Checker{Compose: cp}
	r := c.Run(testRepo(), "/wt", "agentctl-myapp-issue-12", io.Discard)
	if !r.Passed || len(r.Steps) != 2 {
		t.Errorf("result = %+v", r)
	}
	if cp.runs[0].project != "agentctl-myapp-issue-12" || cp.runs[0].service != "app" {
		t.Errorf("compose run = %+v", cp.runs[0])
	}
}

type fakePRs struct {
	dir, base, title, body string
}

func (f *fakePRs) CreateDraft(dir, base, title, body string) (string, error) {
	f.dir, f.base, f.title, f.body = dir, base, title, body
	return "https://github.com/owner/myapp/pull/9", nil
}

func TestPRDraft(t *testing.T) {
	prs := &fakePRs{}
	d := &PRDrafter{PRs: prs}
	s := Session{ID: "myapp-issue-12", Issue: 12, IssueTitle: "タイトル", Worktree: "/wt/myapp-issue-12"}
	url, err := d.Draft(s, testRepo())
	if err != nil {
		t.Fatal(err)
	}
	if url == "" || prs.base != "main" || prs.dir != "/wt/myapp-issue-12" {
		t.Errorf("draft = %+v", prs)
	}
	if !strings.Contains(prs.title, "#12") || !strings.Contains(prs.body, "Closes #12") {
		t.Errorf("title/body = %q / %q", prs.title, prs.body)
	}
}
