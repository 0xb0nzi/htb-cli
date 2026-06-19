package sherlocks

import (
	"encoding/json"
	"testing"
)

// These tests feed real (trimmed) HackTheBox API responses through the sherlock
// models so a future response-shape change is caught here rather than at
// runtime.

// Captured from GET /api/v4/sherlocks/631/tasks on 2026-06-19. Note "type" is
// an object the model ignores; the flag-relevant fields must still bind.
const sherlockTasksJSON = `{
  "data": [
    {
      "id": 2880,
      "title": "Task 1",
      "description": "Analyze the auth.log. What is the IP address used by the attacker?",
      "hint": "Searching for brute force keywords may help.",
      "type": {"id": 0, "text": "task"},
      "task_type": {"id": 0, "text": "text"},
      "completed": true,
      "masked_flag": "x.x.x.x"
    }
  ]
}`

func TestSherlockTasksUnmarshal(t *testing.T) {
	var data SherlockDataTasks
	if err := json.Unmarshal([]byte(sherlockTasksJSON), &data); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(data.Tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(data.Tasks))
	}
	task := data.Tasks[0]
	if task.ID != 2880 {
		t.Errorf("task ID: got %d, want 2880", task.ID)
	}
	if task.Title != "Task 1" {
		t.Errorf("task Title: got %q, want %q", task.Title, "Task 1")
	}
	if !task.Completed {
		t.Error("task Completed: got false, want true")
	}
	if task.MaskedFlag != "x.x.x.x" {
		t.Errorf("task MaskedFlag: got %q, want %q", task.MaskedFlag, "x.x.x.x")
	}
}

// Captured from GET /api/v4/sherlocks on 2026-06-19 — drives name->id lookup.
const sherlockListJSON = `{
  "data": [
    {"id": 631, "name": "Brutus"},
    {"id": 1200, "name": "FortySeven-1"}
  ]
}`

func TestSherlockListUnmarshal(t *testing.T) {
	var data SherlockData
	if err := json.Unmarshal([]byte(sherlockListJSON), &data); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if len(data.Data) != 2 {
		t.Fatalf("got %d sherlocks, want 2", len(data.Data))
	}
	if data.Data[0].ID != 631 || data.Data[0].Name != "Brutus" {
		t.Errorf("first sherlock: got {%d, %q}, want {631, Brutus}", data.Data[0].ID, data.Data[0].Name)
	}
}

// Captured shape from GET /api/v4/sherlocks/{id}/download_link.
const downloadLinkJSON = `{"url": "https://labs.hackthebox.com/api/v4/sherlocks/631/cdn/redirect", "expires_in": 3600}`

func TestDownloadFileUnmarshal(t *testing.T) {
	var d DownloadFile
	if err := json.Unmarshal([]byte(downloadLinkJSON), &d); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if d.URL == "" {
		t.Error("DownloadFile.URL is empty; download_link response shape changed")
	}
}
