package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xb0nzi/htb-cli/lib/menu"
	"github.com/0xb0nzi/htb-cli/lib/sherlocks"
)

// menuSherlocks drives the Sherlocks (blue-team) flow: pick a Sherlock, then act
// on it (info / download / tasks & submit) in a sub-loop.
func menuSherlocks(m *menu.Menu) {
	items, err := sherlocks.List()
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to load Sherlocks: %v", err))
		return
	}
	if len(items) == 0 {
		m.Notify("No Sherlocks found")
		return
	}

	names := make([]string, len(items))
	for i, s := range items {
		names[i] = s.Name
	}
	idx, _, err := m.Select("Sherlock", names)
	if err != nil || idx < 0 {
		return
	}
	s := items[idx]
	id := strconv.Itoa(s.ID)

	for {
		_, action, err := m.Select(s.Name, []string{"Info", "Download archive", "Tasks / submit", "Back"})
		if err != nil {
			return
		}
		switch action {
		case "Info":
			txt, err := sherlocks.GeneralInfoText(id)
			if err != nil {
				m.Notify(fmt.Sprintf("Failed: %v", err))
				continue
			}
			m.Notify(txt)
		case "Download archive":
			menuSherlockDownload(m, s.Name, id)
		case "Tasks / submit":
			menuSherlockTasks(m, id)
		case "Back":
			return
		}
	}
}

// menuSherlockDownload downloads the Sherlock archive, defaulting to ~/<name>.zip.
func menuSherlockDownload(m *menu.Menu, name, id string) {
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
	if err := sherlocks.Download(id, path); err != nil {
		m.Notify(fmt.Sprintf("Download failed: %v", err))
		return
	}
	m.Notify(fmt.Sprintf("Downloaded to %s\nArchive password: hacktheblue", path))
}

// menuSherlockTasks lists a Sherlock's tasks, shows the selected task, and
// optionally submits a flag for it.
func menuSherlockTasks(m *menu.Menu, id string) {
	tasks, err := sherlocks.GetTasks(id)
	if err != nil {
		m.Notify(fmt.Sprintf("Failed to load tasks: %v", err))
		return
	}
	if tasks == nil || len(tasks.Tasks) == 0 {
		m.Notify("No tasks for this Sherlock")
		return
	}

	titles := make([]string, len(tasks.Tasks))
	for i, t := range tasks.Tasks {
		mark := " "
		if t.Completed {
			mark = "✓"
		}
		titles[i] = fmt.Sprintf("[%s] %s", mark, t.Title)
	}
	idx, _, err := m.Select("Task", titles)
	if err != nil || idx < 0 {
		return
	}
	task := tasks.Tasks[idx]

	detail := fmt.Sprintf("%s\n\n%s\n\nMasked flag: %s", task.Title, task.Description, task.MaskedFlag)
	if task.Hint != "" {
		detail += "\nHint: " + task.Hint
	}
	m.Notify(detail)

	if !m.Confirm("Submit a flag for this task?") {
		return
	}
	flag, err := m.Password("Flag")
	if err != nil || strings.TrimSpace(flag) == "" {
		return
	}
	msg, err := sherlocks.SubmitTaskFlag(id, task.ID, strings.TrimSpace(flag))
	if err != nil {
		m.Notify(fmt.Sprintf("Submit failed: %v", err))
		return
	}
	m.Notify(msg)
}
