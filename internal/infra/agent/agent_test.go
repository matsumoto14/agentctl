package agent

import (
	"reflect"
	"testing"

	"github.com/matsumoto14/agentctl/internal/core"
)

func TestCommandFor(t *testing.T) {
	if got := commandFor(core.AgentCodex, "p"); !reflect.DeepEqual(got, []string{"codex", "exec", "p"}) {
		t.Errorf("codex = %v", got)
	}
	if got := commandFor(core.AgentClaude, "p"); !reflect.DeepEqual(got, []string{"claude", "-p", "p"}) {
		t.Errorf("claude = %v", got)
	}
	if got := commandFor(core.Agent("other"), "p"); got != nil {
		t.Errorf("unknown = %v", got)
	}
}
