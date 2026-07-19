// Package config は基盤側に置くリポジトリ設定の読み込みと検証を行う。
// 正規コマンドの正本を対象リポジトリではなくここに置くのは、依存方向を
// 「基盤 → リポ」の一方向に保つため（C1、設計書 §9.1）。
package config

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type Repo struct {
	Name    string  `yaml:"-"`
	Path    string  `yaml:"path"`
	GitHub  string  `yaml:"github"`
	Base    string  `yaml:"base"`
	Compose Compose `yaml:"compose"`
	Checks  []Check `yaml:"checks"`
}

type Compose struct {
	Service string `yaml:"service"`
}

type Check struct {
	Name string   `yaml:"name"`
	Run  []string `yaml:"run"`
}

func Load(path string) (Repo, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return Repo{}, err
	}
	var r Repo
	dec := yaml.NewDecoder(bytes.NewReader(b))
	// 綴りの誤った設定キーを黙って無視しない
	dec.KnownFields(true)
	if err := dec.Decode(&r); err != nil {
		return Repo{}, fmt.Errorf("%s: %w", path, err)
	}
	r.Name = strings.TrimSuffix(filepath.Base(path), ".yaml")
	if home, err := os.UserHomeDir(); err == nil && strings.HasPrefix(r.Path, "~/") {
		r.Path = filepath.Join(home, r.Path[2:])
	}
	if err := r.validate(); err != nil {
		return Repo{}, fmt.Errorf("%s: %w", path, err)
	}
	return r, nil
}

func (r Repo) validate() error {
	var errs []error
	if r.Path == "" {
		errs = append(errs, errors.New("path（ローカル clone の場所）が必要"))
	}
	if p := strings.Split(r.GitHub, "/"); len(p) != 2 || p[0] == "" || p[1] == "" {
		errs = append(errs, fmt.Errorf("github は owner/name 形式で指定する: %q", r.GitHub))
	}
	if r.Base == "" {
		errs = append(errs, errors.New("base（PR のベースブランチ）が必要"))
	}
	if r.Compose.Service == "" {
		errs = append(errs, errors.New("compose.service が必要"))
	}
	if len(r.Checks) == 0 {
		errs = append(errs, errors.New("checks に少なくとも 1 つの検証を定義する"))
	}
	for i, c := range r.Checks {
		if c.Name == "" || len(c.Run) == 0 {
			errs = append(errs, fmt.Errorf("checks[%d] に name と run が必要", i))
		}
	}
	return errors.Join(errs...)
}

// LoadAll は <configDir>/repos/<company>/ 配下の全設定を読み、検証する。
func LoadAll(configDir, company string) ([]Repo, error) {
	dir := filepath.Join(configDir, "repos", company)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("設定ディレクトリがない: %s", dir)
	}
	if err != nil {
		return nil, err
	}
	var repos []Repo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		r, err := Load(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		repos = append(repos, r)
	}
	if len(repos) == 0 {
		return nil, fmt.Errorf("config/repos/%s/ に設定がない", company)
	}
	sort.Slice(repos, func(i, j int) bool { return repos[i].Name < repos[j].Name })
	return repos, nil
}

// LoadRepo は名前を指定して 1 件読む。name が空で候補が 1 つだけならそれを使う。
func LoadRepo(configDir, company, name string) (Repo, error) {
	if name != "" {
		return Load(filepath.Join(configDir, "repos", company, name+".yaml"))
	}
	repos, err := LoadAll(configDir, company)
	if err != nil {
		return Repo{}, err
	}
	switch len(repos) {
	case 1:
		return repos[0], nil
	default:
		var names []string
		for _, r := range repos {
			names = append(names, r.Name)
		}
		return Repo{}, fmt.Errorf("設定が複数あるため --repo で指定する: %s", strings.Join(names, ", "))
	}
}
