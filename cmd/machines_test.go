package cmd

import "testing"

// getOSEmoji must be case-insensitive: the v4 machine lists return "Windows"/
// "Linux" while the v5 unreleased list returns "windows"/"linux" (and both
// casings can even appear in the same v5 response). A regression to a
// case-sensitive switch silently drops the OS emoji for half the entries.
func TestGetOSEmoji_CaseInsensitive(t *testing.T) {
	cases := []struct {
		os   string
		want string
	}{
		{"Linux", Penguin},
		{"linux", Penguin},
		{"LINUX", Penguin},
		{"Windows", Computer},
		{"windows", Computer},
		{"FreeBSD", ""},
		{"", ""},
	}
	for _, c := range cases {
		if got := getOSEmoji(c.os); got != c.want {
			t.Errorf("getOSEmoji(%q) = %q, want %q", c.os, got, c.want)
		}
	}
}

func TestGetColorFromDifficultyText(t *testing.T) {
	cases := map[string]string{
		"Easy":      "[green]",
		"Medium":    "[orange]",
		"Hard":      "[red]",
		"Insane":    "[purple]",
		"Unknown":   "[-]",
		"VeryEasy?": "[-]",
	}
	for difficulty, want := range cases {
		if got := getColorFromDifficultyText(difficulty); got != want {
			t.Errorf("getColorFromDifficultyText(%q) = %q, want %q", difficulty, got, want)
		}
	}
}
