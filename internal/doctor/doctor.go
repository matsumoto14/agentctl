// Package doctor は環境の非破壊チェックを行う。報告のみで、修復はしない。
package doctor

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/matsumoto14/agentctl/internal/core"
)

type Check struct {
	Name   string `json:"name"`
	OK     bool   `json:"ok"`
	Detail string `json:"detail,omitempty"`
}

type Report struct {
	OK     bool    `json:"ok"`
	Checks []Check `json:"checks"`
}

type Options struct {
	Binaries     []string
	StateDir     string
	ConfigDir    string
	LoadConfig   func() error // 設定の検証は config パッケージの責務なので注入する
	Sessions     []core.Session
	WorktreesDir string

	// テストから外部コマンド依存を差し替えるための間接化
	LookPath       func(string) (string, error)
	ComposeVersion func() (string, error)
}

func Run(o Options) Report {
	lookPath := o.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	composeVersion := o.ComposeVersion
	if composeVersion == nil {
		composeVersion = func() (string, error) {
			out, err := exec.Command("docker", "compose", "version").Output()
			return strings.TrimSpace(string(out)), err
		}
	}

	var r Report
	add := func(name string, ok bool, detail string) {
		r.Checks = append(r.Checks, Check{Name: name, OK: ok, Detail: detail})
	}

	for _, b := range o.Binaries {
		if p, err := lookPath(b); err != nil {
			add("binary:"+b, false, "PATH に見つからない")
		} else {
			add("binary:"+b, true, p)
		}
	}
	// compose は CLI プラグインのため LookPath では確認できない
	if v, err := composeVersion(); err != nil {
		add("docker-compose", false, "docker compose version が失敗")
	} else {
		add("docker-compose", true, v)
	}

	if err := os.MkdirAll(o.StateDir, 0o755); err != nil {
		add("state-dir", false, err.Error())
	} else if f, err := os.CreateTemp(o.StateDir, ".doctor-*"); err != nil {
		add("state-dir", false, fmt.Sprintf("書き込み不可: %v", err))
	} else {
		f.Close()
		os.Remove(f.Name())
		add("state-dir", true, o.StateDir)
	}

	if o.LoadConfig != nil {
		if err := o.LoadConfig(); err != nil {
			add("config", false, err.Error())
		} else {
			add("config", true, o.ConfigDir)
		}
	}

	// session ↔ worktree の drift を報告する。片付けは task rm の責務。
	known := map[string]bool{}
	for _, s := range o.Sessions {
		known[filepath.Base(s.Worktree)] = true
		if _, err := os.Stat(s.Worktree); err != nil {
			add("session:"+s.ID, false, "worktree が存在しない（task rm で片付ける）")
		} else {
			add("session:"+s.ID, true, s.Worktree)
		}
	}
	if entries, err := os.ReadDir(o.WorktreesDir); err == nil {
		for _, e := range entries {
			if e.IsDir() && !known[e.Name()] {
				add("worktree:"+e.Name(), false, "session の無い worktree（task rm 済みの残骸か手動作成物）")
			}
		}
	}

	r.OK = true
	for _, c := range r.Checks {
		if !c.OK {
			r.OK = false
			break
		}
	}
	return r
}
