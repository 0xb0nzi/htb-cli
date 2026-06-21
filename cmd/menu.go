package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xb0nzi/htb-cli/config"
	"github.com/0xb0nzi/htb-cli/lib/challenge"
	"github.com/0xb0nzi/htb-cli/lib/hosts"
	"github.com/0xb0nzi/htb-cli/lib/menu"
	"github.com/0xb0nzi/htb-cli/lib/submit"
	"github.com/0xb0nzi/htb-cli/lib/utils"
	"github.com/0xb0nzi/htb-cli/lib/vpn"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// machineEntry is a single pickable machine in a browse menu.
type machineEntry struct {
	name string
	id   int
}

// fetchAllPages walks a paginated HTB "data"+"meta" listing and returns every
// row across all pages. Handing the full set to rofi/fzf means you can scroll
// and filter the entire list instead of just the first page. Endpoints without
// "meta" are treated as single-page (no behaviour change).
func fetchAllPages(baseURL string) ([]map[string]interface{}, error) {
	var all []map[string]interface{}
	for page := 1; page <= 100; page++ { // hard cap guards against a server that ignores ?page
		sep := "?"
		if strings.Contains(baseURL, "?") {
			sep = "&"
		}
		resp, err := utils.HtbRequest(http.MethodGet, fmt.Sprintf("%s%spage=%d", baseURL, sep, page), nil)
		if err != nil {
			return nil, err
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}
		var root map[string]interface{}
		if err := json.Unmarshal(body, &root); err != nil {
			return nil, fmt.Errorf("unexpected list response")
		}
		data, _ := root["data"].([]interface{})
		for _, it := range data {
			if mp, ok := it.(map[string]interface{}); ok {
				all = append(all, mp)
			}
		}

		meta, ok := root["meta"].(map[string]interface{})
		if !ok {
			break // no pagination metadata -> single page
		}
		last := asInt(meta["last_page"])
		if last == 0 || asInt(meta["current_page"]) >= last {
			break
		}
	}
	return all, nil
}

// fetchMachineList returns the name/id pairs across ALL pages of a machine
// listing endpoint so they can be presented (and filtered) in a picker.
func fetchMachineList(url string) ([]machineEntry, error) {
	rows, err := fetchAllPages(url)
	if err != nil {
		return nil, err
	}
	var entries []machineEntry
	for _, mp := range rows {
		name, _ := mp["name"].(string)
		if name == "" {
			continue
		}
		entries = append(entries, machineEntry{name: name, id: asInt(mp["id"])})
	}
	return entries, nil
}

// menuBrowseAndStart lists machines from url, lets the user pick one, and starts it.
func menuBrowseAndStart(m *menu.Menu, label, url string) {
	entries, err := fetchMachineList(url)
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to load %s machines: %v", label, err))
		return
	}
	if len(entries) == 0 {
		m.Notify(fmt.Sprintf("No %s machines found", label))
		return
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}

	idx, chosen, err := m.Select(label+" machine", names)
	if err != nil {
		return // cancelled
	}

	// Resolve the id: a normal pick gives an index; a free-typed rofi entry
	// (idx == -1) falls back to a name search inside coreStartCmd.
	id := 0
	name := chosen
	if idx >= 0 {
		id = entries[idx].id
		name = entries[idx].name
	}

	out, err := coreStartCmd(name, id)
	if err != nil {
		m.Notify(fmt.Sprintf("Start failed: %v", err))
		return
	}
	m.Notify(out)
	menuMaybeAddHost(m)
}

// menuMaybeAddHost offers to map the freshly spawned machine's IP to a
// <name>.htb hostname in /etc/hosts. It reads the canonical IP/name from the
// active machine so it works regardless of how the machine was started. The
// underlying lib/hosts add is idempotent, so re-running is harmless.
func menuMaybeAddHost(m *menu.Menu) {
	data, err := utils.GetInformationsFromActiveMachine()
	if err != nil || data == nil {
		return
	}
	ip, _ := data["ip"].(string)
	name, _ := data["name"].(string)
	if ip == "" || ip == "Undefined" || name == "" {
		return
	}

	host := strings.ToLower(name) + ".htb"
	if !m.Confirm(fmt.Sprintf("Add '%s  %s' to /etc/hosts?", ip, host)) {
		return
	}
	if err := hosts.AddEntryToHosts(ip, host); err != nil {
		m.Notify(fmt.Sprintf("hosts update failed: %v", err))
		return
	}
	m.Notify(fmt.Sprintf("Added %s  %s to /etc/hosts", ip, host))
}

// menuStartingPoint lets the user pick a Starting Point tier, then browse and
// start a machine from it. The v5 spTier listing shares the shape used by
// menuBrowseAndStart, so the spawn path is reused as-is.
func menuStartingPoint(m *menu.Menu) {
	_, tier, err := m.Select("Starting Point tier", []string{"Tier 1", "Tier 2", "Tier 3"})
	if err != nil {
		return
	}
	n := strings.TrimSpace(strings.TrimPrefix(tier, "Tier "))
	url := fmt.Sprintf("%s/machines?spTier=%s", config.BaseHackTheBoxAPIURLv5, n)
	menuBrowseAndStart(m, "Starting Point", url)
}

// menuStartSeason launches the current Season / Release Arena machine, mirroring
// `htb-cli start` with no machine argument.
func menuStartSeason(m *menu.Menu) {
	id, err := utils.SearchLastReleaseArenaMachine()
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to find Season machine: %v", err))
		return
	}
	out, err := coreStartCmd("", id)
	if err != nil {
		m.Notify(fmt.Sprintf("Start failed: %v", err))
		return
	}
	m.Notify(out)
	menuMaybeAddHost(m)
}

// menuStartByName prompts for a machine name and starts it.
func menuStartByName(m *menu.Menu) {
	name, err := m.Input("Machine name")
	if err != nil || strings.TrimSpace(name) == "" {
		return
	}
	out, err := coreStartCmd(name, 0)
	if err != nil {
		m.Notify(fmt.Sprintf("Start failed: %v", err))
		return
	}
	m.Notify(out)
	menuMaybeAddHost(m)
}

// menuActiveInfo shows details about the currently running machine.
func menuActiveInfo(m *menu.Menu) {
	data, err := utils.GetInformationsFromActiveMachine()
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to get active machine: %v", err))
		return
	}
	if data == nil {
		m.Notify("No machine is running")
		return
	}

	field := func(key string) string {
		if v, ok := data[key]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return "-"
	}
	msg := fmt.Sprintf("Name: %s\nIP: %s\nOS: %s\nDifficulty: %s\nPoints: %s",
		field("name"), field("ip"), field("os"), field("difficultyText"), field("points"))
	m.Notify(msg)
}

// menuConnectVPN lets the user pick a VPN product and connects to it.
func menuConnectVPN(m *menu.Menu) {
	// Display label -> (mode, glob pattern) used by lib/vpn config file naming.
	type vpnChoice struct{ mode, pattern string }
	labels := []string{"Labs", "Starting Point", "Fortresses", "Release Arena"}
	choices := map[string]vpnChoice{
		"Labs":           {"labs", "Labs"},
		"Starting Point": {"sp", "StartingPoint"},
		"Fortresses":     {"fortresses", "Fortress"},
		"Release Arena":  {"competitive", "Release_Arena"},
	}

	_, chosen, err := m.Select("Connect VPN", labels)
	if err != nil {
		return
	}
	choice, ok := choices[chosen]
	if !ok {
		return
	}

	filename := config.BaseDirectory + "/*" + choice.pattern + "*"
	out, err := vpn.Start(filename)
	if err != nil {
		m.Notify(fmt.Sprintf("VPN connect failed: %v", err))
		return
	}
	m.Notify(out)
}

// challengeEntry is a single pickable challenge in a browse menu.
type challengeEntry struct {
	name string
	id   int
}

// fetchChallengeList returns the active challenge name/id pairs for a picker.
func fetchChallengeList() ([]challengeEntry, error) {
	resp, err := utils.HtbRequest(http.MethodGet, config.BaseHackTheBoxAPIURL+"/challenge/list", nil)
	if err != nil {
		return nil, err
	}
	arr, ok := utils.ParseJsonMessage(resp, "challenges").([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response from challenge list")
	}

	var entries []challengeEntry
	for _, item := range arr {
		mp, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := mp["name"].(string)
		idf, _ := mp["id"].(float64)
		if name == "" {
			continue
		}
		entries = append(entries, challengeEntry{name: name, id: int(idf)})
	}
	return entries, nil
}

// fetchChallengeInfo returns the /challenge/info/{id} object.
func fetchChallengeInfo(id int) (map[string]interface{}, error) {
	resp, err := utils.HtbRequest(http.MethodGet, fmt.Sprintf("%s/challenge/info/%d", config.BaseHackTheBoxAPIURL, id), nil)
	if err != nil {
		return nil, err
	}
	data, ok := utils.ParseJsonMessage(resp, "challenge").(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response from challenge info")
	}
	return data, nil
}

// formatChallengeInfo renders a challenge summary including description + first blood.
func formatChallengeInfo(data map[string]interface{}) string {
	field := func(key string) string {
		if v, ok := data[key]; ok && v != nil {
			return fmt.Sprintf("%v", v)
		}
		return "-"
	}
	solved := "No"
	if v, ok := data["authUserSolve"].(bool); ok && v {
		solved = "Yes"
	}

	msg := fmt.Sprintf("%s  [%s]\nDifficulty: %s   Points: %s   Solves: %s   Solved: %s",
		field("name"), field("category_name"), field("difficulty"), field("points"), field("solves"), solved)
	if blood := field("first_blood_user"); blood != "-" {
		msg += "\nFirst blood: " + blood
	}
	if desc, ok := data["description"].(string); ok && desc != "" {
		msg += "\n\n" + desc
	}
	return msg
}

// menuShowChallenge displays a challenge's info, then loops on per-challenge
// actions (submit a flag, download files when available).
func menuShowChallenge(m *menu.Menu, id int) {
	data, err := fetchChallengeInfo(id)
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to load challenge: %v", err))
		return
	}
	m.Notify(formatChallengeInfo(data))

	name, _ := data["name"].(string)
	downloadable, _ := data["download"].(bool)

	for {
		opts := []string{"Submit flag"}
		if downloadable {
			opts = append(opts, "Download files")
		}
		opts = append(opts, "Back")

		_, action, err := m.Select(name, opts)
		if err != nil {
			return
		}
		switch action {
		case "Submit flag":
			menuSubmitChallengeByID(m, id)
		case "Download files":
			menuDownloadChallenge(m, id, name)
		case "Back":
			return
		}
	}
}

// menuDownloadChallenge downloads a challenge's files, defaulting to ~/<name>.zip.
func menuDownloadChallenge(m *menu.Menu, id int, name string) {
	home, _ := os.UserHomeDir()
	safe := strings.ReplaceAll(name, " ", "_")
	def := filepath.Join(home, safe+".zip")

	path, err := m.Input(fmt.Sprintf("Save to (blank = %s)", def))
	if err != nil {
		return
	}
	if strings.TrimSpace(path) == "" {
		path = def
	}
	saved, err := challenge.Download(id, path)
	if err != nil {
		m.Notify(fmt.Sprintf("Download failed: %v", err))
		return
	}
	m.Notify("Downloaded to " + saved)
}

// menuBrowseChallenges lists challenges, lets the user pick one and shows its info.
func menuBrowseChallenges(m *menu.Menu) {
	entries, err := fetchChallengeList()
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to load challenges: %v", err))
		return
	}
	if len(entries) == 0 {
		m.Notify("No challenges found")
		return
	}

	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.name
	}
	idx, _, err := m.Select("Challenge", names)
	if err != nil || idx < 0 {
		return
	}
	menuShowChallenge(m, entries[idx].id)
}

// askChallengeDifficulty prompts for the required 1-10 challenge rating.
func askChallengeDifficulty(m *menu.Menu) (int, bool) {
	raw, err := m.Input("Rate difficulty 1-10")
	if err != nil {
		return 0, false
	}
	d, err := strconv.Atoi(strings.TrimSpace(raw))
	if err != nil || d < 1 || d > 10 {
		m.Notify("Difficulty must be a number from 1 to 10")
		return 0, false
	}
	return d, true
}

// menuSubmitChallengeByID submits a flag for a known challenge id.
func menuSubmitChallengeByID(m *menu.Menu, id int) {
	flag, err := m.Password("Flag")
	if err != nil || strings.TrimSpace(flag) == "" {
		return
	}
	difficulty, ok := askChallengeDifficulty(m)
	if !ok {
		return
	}
	out, _, err := submit.CoreSubmitCmd(difficulty, "challenge-id", strconv.Itoa(id), flag)
	if err != nil {
		m.Notify(fmt.Sprintf("Submit failed: %v", err))
		return
	}
	m.Notify(out)
}

// menuSubmitChallenge submits a challenge flag by challenge name.
func menuSubmitChallenge(m *menu.Menu) {
	name, err := m.Input("Challenge name")
	if err != nil || strings.TrimSpace(name) == "" {
		return
	}
	flag, err := m.Password("Flag")
	if err != nil || strings.TrimSpace(flag) == "" {
		return
	}
	difficulty, ok := askChallengeDifficulty(m)
	if !ok {
		return
	}
	out, _, err := submit.CoreSubmitCmd(difficulty, "challenge", name, flag)
	if err != nil {
		m.Notify(fmt.Sprintf("Submit failed: %v", err))
		return
	}
	m.Notify(out)
}

// menuSubmitFlag prompts for a target and flag, then submits it.
func menuSubmitFlag(m *menu.Menu) {
	name, err := m.Input("Machine name (blank = release arena)")
	if err != nil {
		return
	}
	flag, err := m.Password("Flag")
	if err != nil || strings.TrimSpace(flag) == "" {
		return
	}

	modeType, modeValue := "release-arena", ""
	if strings.TrimSpace(name) != "" {
		modeType, modeValue = "machine", name
	}

	out, _, err := submit.CoreSubmitCmd(0, modeType, modeValue, flag)
	if err != nil {
		m.Notify(fmt.Sprintf("Submit failed: %v", err))
		return
	}
	m.Notify(out)
}

var menuCmd = &cobra.Command{
	Use:   "menu",
	Short: "Interactive rofi/fzf/dmenu control panel for HackTheBox",
	Long: `Launches an interactive menu to drive HackTheBox without the web UI.

The front-end is auto-detected (rofi on a graphical session, fzf in a terminal,
dmenu, or a plain numbered prompt) and can be forced with --backend or the
HTB_MENU_BACKEND environment variable. Designed to be bound to an i3 keybind:

    bindsym $mod+h exec --no-startup-id htb-cli menu`,
	Run: func(cmd *cobra.Command, args []string) {
		backend, err := cmd.Flags().GetString("backend")
		if err != nil {
			config.GlobalConfig.Logger.Error("", zap.Error(err))
			os.Exit(1)
		}

		// The menu has no controlling TTY when launched from a keybind, so
		// suppress interactive survey prompts by auto-confirming fuzzy matches.
		config.GlobalConfig.BatchParam = true

		m := menu.New(menu.Backend(backend))

		actions := []string{
			"Start machine (by name)",
			"Browse & start (active)",
			"Browse & start (retired)",
			"Starting Point",
			"Season machine",
			"Active machine info",
			"Submit machine flag",
			"Browse challenges",
			"Submit challenge flag",
			"Sherlocks",
			"Tracks",
			"Fortresses",
			"Pro Labs",
			"Reset machine",
			"Stop machine",
			"Connect VPN",
			"Stop VPN",
			"Pwnbox",
			"My profile",
			"Quit",
		}

		for {
			_, choice, err := m.Select("HackTheBox", actions)
			if err != nil {
				return // cancelled -> exit menu
			}

			switch choice {
			case "Start machine (by name)":
				menuStartByName(m)
			case "Browse & start (active)":
				menuBrowseAndStart(m, "active", machineURL)
			case "Browse & start (retired)":
				menuBrowseAndStart(m, "retired", retiredURL)
			case "Starting Point":
				menuStartingPoint(m)
			case "Season machine":
				menuStartSeason(m)
			case "Active machine info":
				menuActiveInfo(m)
			case "Submit machine flag":
				menuSubmitFlag(m)
			case "Browse challenges":
				menuBrowseChallenges(m)
			case "Submit challenge flag":
				menuSubmitChallenge(m)
			case "Sherlocks":
				menuSherlocks(m)
			case "Tracks":
				menuTracks(m)
			case "Fortresses":
				menuFortresses(m)
			case "Pro Labs":
				menuProlabs(m)
			case "Pwnbox":
				menuPwnbox(m)
			case "My profile":
				menuProfile(m)
			case "Reset machine":
				out, err := coreResetCmd()
				if err != nil {
					m.Notify(fmt.Sprintf("Reset failed: %v", err))
					break
				}
				m.Notify(out)
			case "Stop machine":
				out, err := coreStopCmd()
				if err != nil {
					m.Notify(fmt.Sprintf("Stop failed: %v", err))
					break
				}
				m.Notify(out)
			case "Connect VPN":
				menuConnectVPN(m)
			case "Stop VPN":
				out, err := vpn.Stop()
				if err != nil {
					m.Notify(fmt.Sprintf("VPN stop failed: %v", err))
					break
				}
				m.Notify(out)
			case "Quit":
				return
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(menuCmd)
	menuCmd.Flags().String("backend", "", "Force menu backend: rofi | dmenu | fzf | terminal (default: auto-detect)")
}
