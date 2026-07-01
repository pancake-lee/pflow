package session

import (
	"testing"
)

func TestSanitizeName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"my-project", "pflow-my-project"},
		{"My Project", "pflow-my-project"},
		{"hello_world", "pflow-hello_world"},
		{"foo.bar", "pflow-foo.bar"},
		{"a/b/c", "pflow-a-b-c"},
		{"UPPERCASE", "pflow-uppercase"},
		{"spaces in name", "pflow-spaces-in-name"},
		{"-leading-dash", "pflow-leading-dash"},
		{"trailing-dash-", "pflow-trailing-dash"},
		{"---", "pflow-pflow"},
		{"", "pflow-pflow"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := SanitizeName(tt.input)
			if got != tt.want {
				t.Errorf("SanitizeName(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
