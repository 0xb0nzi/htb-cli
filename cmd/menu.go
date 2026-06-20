package cmd

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/0xb0nzi/htb-cli/config"
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

// fetchMachineList returns the name/id pairs from a machine listing endpoint so
// they can be presented in a picker.
func fetchMachineList(url string) ([]machineEntry, error) {
	resp, err := utils.HtbRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	data := utils.ParseJsonMessage(resp, "data")
	arr, ok := data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("unexpected response format from %s", url)
	}

	var entries []machineEntry
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
		entries = append(entries, machineEntry{name: name, id: int(idf)})
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
			"Active machine info",
			"Submit flag",
			"Reset machine",
			"Stop machine",
			"Connect VPN",
			"Stop VPN",
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
			case "Active machine info":
				menuActiveInfo(m)
			case "Submit flag":
				menuSubmitFlag(m)
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
