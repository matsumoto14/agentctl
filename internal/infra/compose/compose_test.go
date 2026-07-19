package compose

import (
	"reflect"
	"testing"
)

func TestRunArgs(t *testing.T) {
	got := runArgs("agentctl-issue-12", "app", []string{"npm", "test"})
	want := []string{"compose", "-p", "agentctl-issue-12", "run", "--rm", "app", "npm", "test"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}

func TestDownArgs(t *testing.T) {
	got := downArgs("agentctl-issue-12")
	want := []string{"compose", "-p", "agentctl-issue-12", "down", "--remove-orphans"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("args = %v, want %v", got, want)
	}
}
