package githubx

import (
	"strings"
	"testing"
)

func TestGet(t *testing.T) {
	c := &Client{run: func(dir, name string, args ...string) (string, error) {
		if name != "gh" || args[0] != "issue" {
			t.Errorf("cmd = %s %v", name, args)
		}
		return `{"number":12,"title":"タイトル","body":"本文","url":"https://example.com/12"}`, nil
	}}
	issue, err := c.Get("owner/myapp", 12)
	if err != nil {
		t.Fatal(err)
	}
	if issue.Number != 12 || issue.Title != "タイトル" || issue.Body != "本文" {
		t.Errorf("issue = %+v", issue)
	}
}

func TestCreateDraftPushesThenCreates(t *testing.T) {
	var calls []string
	c := &Client{run: func(dir, name string, args ...string) (string, error) {
		calls = append(calls, name+" "+args[0])
		if name == "gh" {
			if dir != "/wt" {
				t.Errorf("dir = %q", dir)
			}
			return "note\nhttps://github.com/owner/myapp/pull/9\n", nil
		}
		return "", nil
	}}
	url, err := c.CreateDraft("/wt", "main", "t", "b")
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://github.com/owner/myapp/pull/9" {
		t.Errorf("url = %q", url)
	}
	if strings.Join(calls, ",") != "git push,gh pr" {
		t.Errorf("calls = %v（push が先）", calls)
	}
}
