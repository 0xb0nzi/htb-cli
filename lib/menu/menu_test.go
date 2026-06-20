package menu

import "testing"

func TestDetectHonoursOverride(t *testing.T) {
	t.Setenv("HTB_MENU_BACKEND", "fzf")
	if got := Detect(); got != BackendFzf {
		t.Fatalf("Detect() = %q, want %q", got, BackendFzf)
	}

	t.Setenv("HTB_MENU_BACKEND", "DMENU") // case-insensitive
	if got := Detect(); got != BackendDmenu {
		t.Fatalf("Detect() = %q, want %q", got, BackendDmenu)
	}
}

func TestNewUsesExplicitBackend(t *testing.T) {
	m := New(BackendTerminal)
	if m.Backend != BackendTerminal {
		t.Fatalf("New(BackendTerminal).Backend = %q, want %q", m.Backend, BackendTerminal)
	}
}

func TestParseTerminalChoice(t *testing.T) {
	cases := []struct {
		name    string
		line    string
		count   int
		want    int
		wantErr bool
	}{
		{"first", "1", 5, 0, false},
		{"last", "5", 5, 4, false},
		{"trimmed", "  3 \n", 5, 2, false},
		{"empty is cancel", "", 5, -1, false},
		{"whitespace is cancel", "   ", 5, -1, false},
		{"zero out of range", "0", 5, -1, true},
		{"too high", "6", 5, -1, true},
		{"not a number", "abc", 5, -1, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseTerminalChoice(tc.line, tc.count)
			if tc.wantErr && err == nil {
				t.Fatalf("expected error for %q, got nil", tc.line)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tc.line, err)
			}
			if got != tc.want {
				t.Fatalf("parseTerminalChoice(%q, %d) = %d, want %d", tc.line, tc.count, got, tc.want)
			}
		})
	}
}
