// Package state はローカルの一時実行状態（session / logs / lock）を扱う。
// 永続記録の正本は GitHub Issue/PR であり、ここは再作成可能なデータのみ置く（C5）。
package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"

	"github.com/matsumoto14/agentctl/internal/core"
)

// Store は 1 会社分の state ディレクトリ（companies/<company>）を扱う。
type Store struct {
	dir string
}

func NewStore(companyDir string) *Store { return &Store{dir: companyDir} }

func (s *Store) sessionsDir() string { return filepath.Join(s.dir, "sessions") }

func (s *Store) sessionPath(id string) string {
	return filepath.Join(s.sessionsDir(), id+".json")
}

func (s *Store) Save(sess core.Session) error {
	if err := os.MkdirAll(s.sessionsDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(sess, "", "  ")
	if err != nil {
		return err
	}
	// 書き込み途中で落ちても壊れた JSON を残さない
	tmp := s.sessionPath(sess.ID) + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.sessionPath(sess.ID))
}

func (s *Store) Load(id string) (core.Session, error) {
	b, err := os.ReadFile(s.sessionPath(id))
	if errors.Is(err, fs.ErrNotExist) {
		return core.Session{}, fmt.Errorf("%w: task %s", core.ErrNotFound, id)
	}
	if err != nil {
		return core.Session{}, err
	}
	var sess core.Session
	if err := json.Unmarshal(b, &sess); err != nil {
		return core.Session{}, fmt.Errorf("session %s の読み込み: %w", id, err)
	}
	return sess, nil
}

// List は読める session だけを返す。壊れたファイル 1 つで status や doctor の
// 全体が失敗すると、生きているタスクまで orphan と誤診されるため隔離する。
// 壊れたものは ListBroken で別途報告する。
func (s *Store) List() ([]core.Session, error) {
	entries, err := os.ReadDir(s.sessionsDir())
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []core.Session
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		sess, err := s.Load(strings.TrimSuffix(e.Name(), ".json"))
		if err != nil {
			continue
		}
		out = append(out, sess)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out, nil
}

// ListBroken は読み込みに失敗する session の id を返す（doctor の報告用）。
func (s *Store) ListBroken() []string {
	entries, err := os.ReadDir(s.sessionsDir())
	if err != nil {
		return nil
	}
	var broken []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		id := strings.TrimSuffix(e.Name(), ".json")
		if _, err := s.Load(id); err != nil {
			broken = append(broken, id)
		}
	}
	return broken
}

func (s *Store) Delete(id string) error {
	err := os.Remove(s.sessionPath(id))
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: task %s", core.ErrNotFound, id)
	}
	return err
}

// LogWriter は生ログの書き込み先を返す。生ログはローカル限りで Issue/PR には貼らない（C5）。
func (s *Store) LogWriter(id string) (io.WriteCloser, error) {
	dir := filepath.Join(s.dir, "logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, id+".log"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
}

func (s *Store) WorktreesDir() string { return filepath.Join(s.dir, "worktrees") }

func (s *Store) WorktreePath(id string) string { return filepath.Join(s.WorktreesDir(), id) }

// FileLock は state dir の flock で逐次実行を機構的に強制する（設計書 §10）。
type FileLock struct {
	path string
}

func NewLock(companyDir string) *FileLock {
	return &FileLock{path: filepath.Join(companyDir, "lock")}
}

func (l *FileLock) Acquire() (func(), error) {
	if err := os.MkdirAll(filepath.Dir(l.path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(l.path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("%w: 別のタスクが実行中（%s）", core.ErrLocked, l.path)
	}
	return func() {
		syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
	}, nil
}
