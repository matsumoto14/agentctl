package cli

import (
	"bytes"
	"testing"
)

// 終了コードは Gateway コントラクトの一部であり、値の変更は破壊的変更になる。
func TestExitCodesAreStable(t *testing.T) {
	tests := []struct {
		name string
		got  int
		want int
	}{
		{"ExitOK", ExitOK, 0},
		{"ExitFailure", ExitFailure, 1},
		{"ExitUsage", ExitUsage, 2},
		{"ExitPrecondition", ExitPrecondition, 3},
		{"ExitUnimplemented", ExitUnimplemented, 10},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

func TestWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, map[string]any{"command": "doctor"}); err != nil {
		t.Fatal(err)
	}
	want := "{\"command\":\"doctor\"}\n"
	if buf.String() != want {
		t.Errorf("WriteJSON = %q, want %q", buf.String(), want)
	}
}

func TestWriteJSONError(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteJSON(&buf, func() {}); err == nil {
		t.Error("エンコード不能な値でエラーが返らない")
	}
}
