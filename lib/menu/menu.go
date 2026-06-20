// Package menu provides a small, backend-agnostic interactive picker used by the
// `htb-cli menu` command. It auto-detects the best available front-end:
//
//	rofi     - graphical dmenu replacement (X11/Wayland), ideal for i3 keybinds
//	dmenu    - classic X11 menu
//	fzf      - fuzzy finder, for use inside a terminal
//	terminal - plain numbered prompts, the universal fallback (works over SSH)
//
// The same Menu API (Select / Input / Password / Confirm / Notify) works across
// all backends, so callers never branch on the environment.
package menu

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"golang.org/x/term"
)

// Backend identifies a concrete UI front-end.
type Backend string

const (
	BackendRofi     Backend = "rofi"
	BackendDmenu    Backend = "dmenu"
	BackendFzf      Backend = "fzf"
	BackendTerminal Backend = "terminal"
)

// ErrCancelled is returned when the user dismisses a prompt (Esc in rofi/fzf,
// empty selection, EOF on the terminal).
var ErrCancelled = fmt.Errorf("selection cancelled")

// Menu wraps a chosen backend.
type Menu struct {
	Backend Backend
}

// graphical reports whether a graphical session is available for rofi/dmenu.
func graphical() bool {
	return os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != ""
}

func inPath(bin string) bool {
	_, err := exec.LookPath(bin)
	return err == nil
}

// rofiThemeArgs returns the rofi -theme flag when HTB_ROFI_THEME points at a
// theme file, letting the distribution layer ship a consistent look without
// touching the user's global rofi config.
func rofiThemeArgs() []string {
	if theme := os.Getenv("HTB_ROFI_THEME"); theme != "" {
		return []string{"-theme", theme}
	}
	return nil
}

// Detect picks the best backend for the current environment. It honours an
// explicit override via the HTB_MENU_BACKEND environment variable.
func Detect() Backend {
	if forced := Backend(strings.ToLower(os.Getenv("HTB_MENU_BACKEND"))); forced != "" {
		return forced
	}
	if graphical() && inPath("rofi") {
		return BackendRofi
	}
	stdoutTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if stdoutTTY && inPath("fzf") {
		return BackendFzf
	}
	if graphical() && inPath("dmenu") {
		return BackendDmenu
	}
	return BackendTerminal
}

// New builds a Menu, using backend if non-empty or auto-detecting otherwise.
func New(backend Backend) *Menu {
	if backend == "" {
		backend = Detect()
	}
	return &Menu{Backend: backend}
}

// Select presents the items and returns the chosen index and label. It returns
// ErrCancelled if the user dismisses the menu.
func (m *Menu) Select(prompt string, items []string) (int, string, error) {
	if len(items) == 0 {
		return -1, "", fmt.Errorf("no items to choose from")
	}
	switch m.Backend {
	case BackendRofi:
		return m.selectExternal("rofi", append([]string{"-dmenu", "-i", "-p", prompt}, rofiThemeArgs()...), items)
	case BackendDmenu:
		return m.selectExternal("dmenu", []string{"-i", "-p", prompt}, items)
	case BackendFzf:
		return m.selectExternal("fzf", []string{"--prompt", prompt + "> ", "--height", "40%", "--reverse"}, items)
	default:
		return m.selectTerminal(prompt, items)
	}
}

// selectExternal feeds items on stdin to a dmenu-style binary and maps the
// returned line back to its index.
func (m *Menu) selectExternal(bin string, args []string, items []string) (int, string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader(strings.Join(items, "\n"))
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		// dmenu/rofi/fzf exit non-zero on cancel.
		return -1, "", ErrCancelled
	}
	choice := strings.TrimRight(string(out), "\n")
	if choice == "" {
		return -1, "", ErrCancelled
	}
	for i, item := range items {
		if item == choice {
			return i, item, nil
		}
	}
	// Backend returned free text not in the list (rofi allows custom entries).
	return -1, choice, nil
}

func (m *Menu) selectTerminal(prompt string, items []string) (int, string, error) {
	for i, item := range items {
		fmt.Fprintf(os.Stderr, "  %2d) %s\n", i+1, item)
	}
	fmt.Fprintf(os.Stderr, "%s [1-%d]: ", prompt, len(items))
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return -1, "", ErrCancelled
	}
	idx, err := parseTerminalChoice(line, len(items))
	if err != nil {
		return -1, "", err
	}
	if idx < 0 {
		return -1, "", ErrCancelled
	}
	return idx, items[idx], nil
}

// parseTerminalChoice converts a typed line into a zero-based index in [0,count).
// It returns -1 for an empty line (cancellation) and an error for anything that
// isn't a number inside the valid range.
func parseTerminalChoice(line string, count int) (int, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return -1, nil
	}
	n, err := strconv.Atoi(line)
	if err != nil || n < 1 || n > count {
		return -1, fmt.Errorf("invalid selection: %s", line)
	}
	return n - 1, nil
}

// Input asks for free-text input.
func (m *Menu) Input(prompt string) (string, error) {
	switch m.Backend {
	case BackendRofi:
		return m.inputExternal("rofi", append([]string{"-dmenu", "-p", prompt, "-l", "0"}, rofiThemeArgs()...))
	case BackendDmenu:
		return m.inputExternal("dmenu", []string{"-p", prompt})
	default:
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
		if err != nil {
			return "", ErrCancelled
		}
		return strings.TrimSpace(line), nil
	}
}

// inputExternal runs a dmenu-style binary with no candidate list so the user
// types a value.
func (m *Menu) inputExternal(bin string, args []string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = strings.NewReader("")
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", ErrCancelled
	}
	return strings.TrimRight(string(out), "\n"), nil
}

// Password asks for a secret. rofi masks input natively; the terminal backend
// uses term.ReadPassword. (fzf/dmenu fall back to a masked rofi or terminal.)
func (m *Menu) Password(prompt string) (string, error) {
	if m.Backend == BackendRofi {
		cmd := exec.Command("rofi", append([]string{"-dmenu", "-password", "-p", prompt, "-l", "0"}, rofiThemeArgs()...)...)
		cmd.Stdin = strings.NewReader("")
		cmd.Stderr = os.Stderr
		out, err := cmd.Output()
		if err != nil {
			return "", ErrCancelled
		}
		return strings.TrimRight(string(out), "\n"), nil
	}
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, "%s: ", prompt)
		b, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Fprintln(os.Stderr)
		if err != nil {
			return "", ErrCancelled
		}
		return string(b), nil
	}
	return m.Input(prompt)
}

// Confirm asks a yes/no question, defaulting to No on cancel.
func (m *Menu) Confirm(prompt string) bool {
	_, choice, err := m.Select(prompt, []string{"No", "Yes"})
	if err != nil {
		return false
	}
	return choice == "Yes"
}

// Notify shows a short informational message to the user through the active
// backend, so menu results are visible even when launched from a keybind with
// no terminal attached.
func (m *Menu) Notify(message string) {
	switch m.Backend {
	case BackendRofi:
		_ = exec.Command("rofi", "-e", message).Run()
	case BackendDmenu, BackendFzf:
		if inPath("notify-send") {
			_ = exec.Command("notify-send", "HackTheBox", message).Run()
			return
		}
		fmt.Fprintln(os.Stderr, message)
	default:
		fmt.Fprintln(os.Stderr, message)
	}
}
