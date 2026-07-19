package state

import (
	"errors"
	"testing"
	"time"

	"github.com/matsumoto14/agentctl/internal/core"
)

func TestStoreRoundtrip(t *testing.T) {
	s := NewStore(t.TempDir())
	sess := core.Session{ID: "issue-1", Issue: 1, Runs: 2, LastAgent: core.AgentCodex,
		CreatedAt: time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)}
	if err := s.Save(sess); err != nil {
		t.Fatal(err)
	}
	got, err := s.Load("issue-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Runs != 2 || got.LastAgent != core.AgentCodex || !got.CreatedAt.Equal(sess.CreatedAt) {
		t.Errorf("got = %+v", got)
	}
}

func TestStoreLoadNotFound(t *testing.T) {
	s := NewStore(t.TempDir())
	if _, err := s.Load("issue-9"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestStoreListSortedByCreation(t *testing.T) {
	s := NewStore(t.TempDir())
	base := time.Date(2026, 7, 19, 0, 0, 0, 0, time.UTC)
	for i, id := range []string{"issue-3", "issue-1", "issue-2"} {
		if err := s.Save(core.Session{ID: id, CreatedAt: base.Add(time.Duration(i) * time.Hour)}); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 3 || list[0].ID != "issue-3" || list[2].ID != "issue-2" {
		t.Errorf("list = %+v", list)
	}
}

func TestStoreListEmpty(t *testing.T) {
	s := NewStore(t.TempDir())
	list, err := s.List()
	if err != nil || len(list) != 0 {
		t.Errorf("list = %v, err = %v", list, err)
	}
}

func TestStoreDelete(t *testing.T) {
	s := NewStore(t.TempDir())
	if err := s.Save(core.Session{ID: "issue-1"}); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("issue-1"); err != nil {
		t.Fatal(err)
	}
	if err := s.Delete("issue-1"); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestFileLockExcludes(t *testing.T) {
	dir := t.TempDir()
	release, err := NewLock(dir).Acquire()
	if err != nil {
		t.Fatal(err)
	}
	// 同じ lock ファイルへの別 fd からの取得は失敗する（= 逐次実行の強制）
	if _, err := NewLock(dir).Acquire(); !errors.Is(err, core.ErrLocked) {
		t.Errorf("err = %v, want ErrLocked", err)
	}
	release()
	release2, err := NewLock(dir).Acquire()
	if err != nil {
		t.Errorf("解放後に取得できない: %v", err)
	} else {
		release2()
	}
}

func TestLogWriter(t *testing.T) {
	s := NewStore(t.TempDir())
	w, err := s.LogWriter("issue-1")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("line\n")); err != nil {
		t.Fatal(err)
	}
	w.Close()
}
