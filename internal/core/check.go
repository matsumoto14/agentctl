package core

import "io"

type CheckResult struct {
	Passed bool              `json:"passed"`
	Steps  []CheckStepResult `json:"steps"`
}

type CheckStepResult struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Error  string `json:"error,omitempty"`
}

type Checker struct {
	Compose ComposeRunner
}

// Run は設定された検証を順に実行し、最初の失敗で打ち切る。
// 検証コマンドの正本は基盤側設定であり、Markdown 等から推測しない（設計書 §9.1）。
func (c *Checker) Run(repo RepoConfig, dir, project string, out io.Writer) CheckResult {
	r := CheckResult{Passed: true}
	for _, step := range repo.Checks {
		err := c.Compose.Run(dir, project, repo.Service, step.Command, out)
		sr := CheckStepResult{Name: step.Name, Passed: err == nil}
		if err != nil {
			sr.Error = err.Error()
			r.Passed = false
		}
		r.Steps = append(r.Steps, sr)
		if err != nil {
			break
		}
	}
	return r
}
