package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const valid = `
path: /home/user/git/myapp
github: owner/myapp
base: main
compose:
  service: app
checks:
  - name: lint
    run: [npm, run, lint]
  - name: test
    run: [npm, test]
`

func writeConfig(t *testing.T, dir, company, name, content string) string {
	t.Helper()
	d := filepath.Join(dir, "repos", company)
	if err := os.MkdirAll(d, 0o755); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(d, name+".yaml")
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoad(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "personal", "myapp", valid)
	r, err := Load(p)
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "myapp" || r.GitHub != "owner/myapp" || r.Compose.Service != "app" {
		t.Errorf("repo = %+v", r)
	}
	if len(r.Checks) != 2 || r.Checks[0].Run[0] != "npm" {
		t.Errorf("checks = %+v", r.Checks)
	}
}

func TestLoadRejectsUnknownField(t *testing.T) {
	dir := t.TempDir()
	p := writeConfig(t, dir, "personal", "myapp", valid+"\nunknown_key: 1\n")
	if _, err := Load(p); err == nil {
		t.Error("未知キーがエラーにならない")
	}
}

func TestLoadValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{"path なし", strings.Replace(valid, "path: /home/user/git/myapp", "", 1), "path"},
		{"github 形式", strings.Replace(valid, "github: owner/myapp", "github: myapp", 1), "owner/name"},
		{"checks なし", valid[:strings.Index(valid, "checks:")], "checks"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			p := writeConfig(t, dir, "personal", "x", tt.content)
			_, err := Load(p)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want %q を含む", err, tt.wantErr)
			}
		})
	}
}

func TestLoadRepoDefaultsToSingle(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "personal", "myapp", valid)
	r, err := LoadRepo(dir, "personal", "")
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "myapp" {
		t.Errorf("name = %q", r.Name)
	}
}

func TestLoadRepoAmbiguous(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "personal", "a", valid)
	writeConfig(t, dir, "personal", "b", valid)
	_, err := LoadRepo(dir, "personal", "")
	if err == nil || !strings.Contains(err.Error(), "--repo") {
		t.Errorf("err = %v", err)
	}
}

func TestLoadRepoMissingDir(t *testing.T) {
	if _, err := LoadRepo(t.TempDir(), "personal", ""); err == nil {
		t.Error("設定ディレクトリ欠如がエラーにならない")
	}
}
