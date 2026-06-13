package project

import (
	"testing"
)

func TestMatchRootFromList(t *testing.T) {
	roots := []Root{
		{Path: "/home/user/code/pflow", Priority: "primary"},
		{Path: "/home/user/code/hermes", Priority: "secondary"},
		{Path: "/home/user", Priority: "normal"},
	}

	tests := []struct {
		name    string
		cwd     string
		want    string // expected matched path, empty = nil
	}{
		{"exact match", "/home/user/code/pflow", "/home/user/code/pflow"},
		{"subdirectory match", "/home/user/code/pflow/internal/api", "/home/user/code/pflow"},
		{"deeper match wins", "/home/user/code/hermes/src", "/home/user/code/hermes"},
		{"shorter match", "/home/user/projects", "/home/user"},
		{"no match", "/other/path", ""},
		{"empty cwd", "", ""},
		{"root cwd", "/", ""},
		{"question mark", "?", ""},
		{"trailing slash", "/home/user/code/pflow/", "/home/user/code/pflow"},
		{"no partial component match", "/home/userx", ""},          // /home/userx should NOT match /home/user
		{"subdir should not match partial", "/home/user/code/pflow-ext", "/home/user"}, // /pflow-ext ≠ /pflow, but /home/user still matches
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchRootFromList(roots, tt.cwd)
			if tt.want == "" {
				if got != nil {
					t.Errorf("MatchRootFromList(%q) = %v, want nil", tt.cwd, got)
				}
			} else {
				if got == nil {
					t.Errorf("MatchRootFromList(%q) = nil, want %q", tt.cwd, tt.want)
				} else if got.Path != tt.want {
					t.Errorf("MatchRootFromList(%q) = %q, want %q", tt.cwd, got.Path, tt.want)
				}
			}
		})
	}
}
