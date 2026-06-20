package cmd

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/0xb0nzi/htb-cli/config"
	"github.com/0xb0nzi/htb-cli/lib/menu"
	"github.com/0xb0nzi/htb-cli/lib/submit"
	"github.com/0xb0nzi/htb-cli/lib/utils"
)

// asInt coerces a JSON number (float64) to int, defaulting to 0.
func asInt(v interface{}) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

// menuSubmitFlagFor prompts for a flag and submits it for the given mode/value.
func menuSubmitFlagFor(m *menu.Menu, mode, value, label string) {
	if !m.Confirm("Submit a flag for " + label + "?") {
		return
	}
	flag, err := m.Password("Flag")
	if err != nil || strings.TrimSpace(flag) == "" {
		return
	}
	out, _, err := submit.CoreSubmitCmd(0, mode, value, strings.TrimSpace(flag))
	if err != nil {
		m.Notify(fmt.Sprintf("Submit failed: %v", err))
		return
	}
	m.Notify(out)
}

// menuFortresses lists fortresses with flag progress and offers to submit a flag.
func menuFortresses(m *menu.Menu) {
	resp, err := utils.HtbRequest(http.MethodGet, config.BaseHackTheBoxAPIURL+"/fortresses", nil)
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to load fortresses: %v", err))
		return
	}
	arr, ok := utils.ParseJsonMessage(resp, "data").([]interface{})
	if !ok || len(arr) == 0 {
		m.Notify("No fortresses found")
		return
	}

	type entry struct {
		name         string
		owned, total int
	}
	var items []entry
	var names []string
	for _, it := range arr {
		mp, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := mp["name"].(string)
		if name == "" {
			continue
		}
		items = append(items, entry{name, asInt(mp["owned_flags"]), asInt(mp["number_of_flags"])})
		names = append(names, name)
	}

	idx, _, err := m.Select("Fortress", names)
	if err != nil || idx < 0 {
		return
	}
	f := items[idx]
	m.Notify(fmt.Sprintf("%s\nFlags owned: %d / %d", f.name, f.owned, f.total))
	menuSubmitFlagFor(m, "fortress", f.name, f.name)
}

// menuProlabs lists pro labs with progress and offers to submit a flag.
func menuProlabs(m *menu.Menu) {
	resp, err := utils.HtbRequest(http.MethodGet, config.BaseHackTheBoxAPIURL+"/prolabs", nil)
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to load prolabs: %v", err))
		return
	}
	data, ok := utils.ParseJsonMessage(resp, "data").(map[string]interface{})
	if !ok {
		m.Notify("Unexpected prolabs response")
		return
	}
	labs, ok := data["labs"].([]interface{})
	if !ok || len(labs) == 0 {
		m.Notify("No prolabs found")
		return
	}

	type entry struct {
		name            string
		machines, flags int
		state           string
	}
	var items []entry
	var names []string
	for _, it := range labs {
		mp, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := mp["name"].(string)
		if name == "" {
			continue
		}
		state, _ := mp["state"].(string)
		items = append(items, entry{name, asInt(mp["pro_machines_count"]), asInt(mp["pro_flags_count"]), state})
		names = append(names, name)
	}

	idx, _, err := m.Select("Pro Lab", names)
	if err != nil || idx < 0 {
		return
	}
	p := items[idx]
	m.Notify(fmt.Sprintf("%s\nMachines: %d   Flags: %d   Access: %s", p.name, p.machines, p.flags, p.state))
	menuSubmitFlagFor(m, "prolab", p.name, p.name)
}

// menuPwnbox manages the pwnbox. Start is intentionally omitted: HTB gates it
// behind a browser recaptcha, so it can't be triggered from the API.
func menuPwnbox(m *menu.Menu) {
	for {
		_, action, err := m.Select("Pwnbox (start needs the web UI)", []string{"Status", "Stop", "Back"})
		if err != nil {
			return
		}
		switch action {
		case "Status":
			resp, err := utils.HtbRequest(http.MethodGet, config.BaseHackTheBoxAPIURL+"/pwnbox/status", nil)
			if err != nil {
				m.Notify(fmt.Sprintf("Failed: %v", err))
				continue
			}
			if msg, ok := utils.ParseJsonMessage(resp, "message").(string); ok && msg != "" {
				m.Notify(msg)
			} else {
				m.Notify("Pwnbox appears to be active")
			}
		case "Stop":
			resp, err := utils.HtbRequest(http.MethodPost, config.BaseHackTheBoxAPIURL+"/pwnbox/terminate", nil)
			if err != nil {
				m.Notify(fmt.Sprintf("Failed: %v", err))
				continue
			}
			if msg, ok := utils.ParseJsonMessage(resp, "message").(string); ok {
				m.Notify(msg)
			} else {
				m.Notify("Pwnbox terminate requested")
			}
		case "Back":
			return
		}
	}
}

// menuProfile shows the authenticated user's identity, plan and rank/owns.
func menuProfile(m *menu.Menu) {
	resp, err := utils.HtbRequest(http.MethodGet, config.BaseHackTheBoxAPIURL+"/user/info", nil)
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to load profile: %v", err))
		return
	}
	info, ok := utils.ParseJsonMessage(resp, "info").(map[string]interface{})
	if !ok {
		m.Notify("Unexpected user info response")
		return
	}

	name, _ := info["name"].(string)
	plan, _ := info["subscription_plan"].(string)
	if plan == "" {
		if v, ok := info["subscriptionType"].(string); ok {
			plan = v
		}
	}
	id := asInt(info["id"])

	msg := fmt.Sprintf("%s\nPlan: %s", name, plan)
	if id != 0 {
		resp2, err := utils.HtbRequest(http.MethodGet, fmt.Sprintf("%s/user/profile/basic/%d", config.BaseHackTheBoxAPIURL, id), nil)
		if err == nil {
			if p, ok := utils.ParseJsonMessage(resp2, "profile").(map[string]interface{}); ok {
				rank, _ := p["rank"].(string)
				msg += fmt.Sprintf("\nRank: %s   Points: %d   Global rank: #%d\nOwns: %d user / %d system",
					rank, asInt(p["points"]), asInt(p["ranking"]), asInt(p["user_owns"]), asInt(p["system_owns"]))
			}
		}
	}
	m.Notify(msg)
}
