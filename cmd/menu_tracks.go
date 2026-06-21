package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/0xb0nzi/htb-cli/config"
	"github.com/0xb0nzi/htb-cli/lib/menu"
	"github.com/0xb0nzi/htb-cli/lib/utils"
)

// fetchTrackJSON GETs a tracks endpoint and returns the decoded JSON. The tracks
// API returns bare arrays/objects (no {"data":...} envelope), so we decode the
// body directly rather than via ParseJsonMessage.
func fetchTrackJSON(url string) (interface{}, error) {
	resp, err := utils.HtbRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var v interface{}
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, fmt.Errorf("unexpected tracks response")
	}
	return v, nil
}

// truthy coerces a JSON bool/number ("complete" fields use both) to a bool.
func truthy(v interface{}) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t != 0
	}
	return false
}

// enrolledTrackIDs returns the set of track ids the user is enrolled in,
// from GET /user/tracks (a cheap [{id, complete}] list).
func enrolledTrackIDs() map[int]bool {
	set := map[int]bool{}
	v, err := fetchTrackJSON(config.BaseHackTheBoxAPIURL + "/user/tracks")
	if err != nil {
		return set // best-effort: no markers rather than a hard failure
	}
	arr, ok := v.([]interface{})
	if !ok {
		return set
	}
	for _, it := range arr {
		if mp, ok := it.(map[string]interface{}); ok {
			set[asInt(mp["id"])] = true
		}
	}
	return set
}

// menuTracks lists Tracks, marks the ones you're enrolled in, and opens a
// selected track's details.
func menuTracks(m *menu.Menu) {
	v, err := fetchTrackJSON(config.BaseHackTheBoxAPIURL + "/tracks")
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to load tracks: %v", err))
		return
	}
	arr, ok := v.([]interface{})
	if !ok || len(arr) == 0 {
		m.Notify("No tracks found")
		return
	}
	enrolled := enrolledTrackIDs()

	type entry struct {
		id   int
		name string
	}
	var items []entry
	var labels []string
	for _, it := range arr {
		mp, ok := it.(map[string]interface{})
		if !ok {
			continue
		}
		id := asInt(mp["id"])
		name, _ := mp["name"].(string)
		if name == "" {
			continue
		}
		diff, _ := mp["difficulty"].(string)
		mark := "  "
		if enrolled[id] {
			mark = "✓ " // already enrolled
		}
		items = append(items, entry{id, name})
		labels = append(labels, fmt.Sprintf("%s%s (%s)", mark, name, diff))
	}

	idx, _, err := m.Select("Track  (✓ = enrolled)", labels)
	if err != nil || idx < 0 {
		return
	}
	menuShowTrack(m, items[idx].id)
}

// menuShowTrack shows a track's description, status and its items (what to do)
// with per-item completion.
func menuShowTrack(m *menu.Menu, id int) {
	v, err := fetchTrackJSON(fmt.Sprintf("%s/tracks/%d", config.BaseHackTheBoxAPIURL, id))
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to load track: %v", err))
		return
	}
	d, ok := v.(map[string]interface{})
	if !ok || d["name"] == nil {
		m.Notify("Track not found")
		return
	}

	name, _ := d["name"].(string)
	diff, _ := d["difficulty"].(string)
	desc, _ := d["description"].(string)
	status := "not enrolled"
	if truthy(d["completed"]) {
		status = "completed"
	} else if truthy(d["enrolled"]) {
		status = "enrolled"
	}

	header := fmt.Sprintf("%s  [%s] — %s", name, diff, status)
	if desc != "" {
		header += "\n\n" + desc
	}
	m.Notify(header)

	// Build the selectable item list so you can drill in and act on each item
	// (start a machine, open a sherlock, view/submit a challenge) right here.
	items, _ := d["items"].([]interface{})
	if len(items) == 0 {
		return
	}
	for {
		labels := make([]string, 0, len(items)+1)
		for _, it := range items {
			mp, _ := it.(map[string]interface{})
			mark := "·"
			if truthy(mp["complete"]) {
				mark = "✓"
			}
			itemType, _ := mp["type"].(string)
			itemName, _ := mp["name"].(string)
			itemDiff, _ := mp["difficulty"].(string)
			labels = append(labels, fmt.Sprintf("[%s] %s: %s (%s)", mark, itemType, itemName, itemDiff))
		}
		labels = append(labels, "Back")

		idx, _, err := m.Select(name+" items", labels)
		if err != nil || idx < 0 || idx >= len(items) {
			return // Back or cancel
		}
		mp, _ := items[idx].(map[string]interface{})
		menuOpenTrackItem(m, mp)
	}
}

// menuOpenTrackItem routes a track item to the right handler by type.
func menuOpenTrackItem(m *menu.Menu, mp map[string]interface{}) {
	id := asInt(mp["id"])
	name, _ := mp["name"].(string)
	switch t, _ := mp["type"].(string); t {
	case "machine":
		os, _ := mp["os"].(string)
		if os != "" {
			os = strings.ToUpper(os[:1]) + strings.ToLower(os[1:])
		}
		diff, _ := mp["difficulty"].(string)
		menuShowMachine(m, machineEntry{name: name, id: id, os: os, difficulty: diff})
	case "challenge":
		menuShowChallenge(m, id)
	case "sherlock":
		menuSherlockActions(m, name, strconv.Itoa(id))
	default:
		m.Notify(fmt.Sprintf("%s (%s) — not openable from the menu yet", name, t))
	}
}
