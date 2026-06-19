package utils

import (
	"encoding/json"
	"testing"
)

// These tests feed real (trimmed) HackTheBox API responses through the parsers
// so that a future API response-shape change is caught here rather than at
// runtime. Every bug found while maintaining this fork compiled cleanly and
// only broke against live JSON — these tests close that gap.

// Captured from GET /api/v4/fortresses on 2026-06-19. "data" is a JSON array.
const fortressesJSON = `{
  "status": true,
  "data": [
    {"id": 7, "name": "AWS"},
    {"id": 6, "name": "Faraday"}
  ]
}`

func TestExtractNamesAndIDs_Fortresses(t *testing.T) {
	got, err := extractNamesAndIDs(fortressesJSON)
	if err != nil {
		t.Fatalf("extractNamesAndIDs returned error: %v", err)
	}
	want := map[string]int{"AWS": 7, "Faraday": 6}
	if len(got) != len(want) {
		t.Fatalf("got %d fortresses, want %d: %v", len(got), len(want), got)
	}
	for name, id := range want {
		if got[name] != id {
			t.Errorf("fortress %q: got id %d, want %d", name, got[name], id)
		}
	}
}

// Regression guard for the map[string]Item -> []Item fix: a JSON array under
// "data" must not error or silently parse to nothing.
func TestExtractNamesAndIDs_RejectsEmpty(t *testing.T) {
	got, err := extractNamesAndIDs(`{"status": true, "data": []}`)
	if err != nil {
		t.Fatalf("unexpected error on empty data: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected no fortresses, got %v", got)
	}
}

// Captured from GET /api/v4/prolabs on 2026-06-19. "data" is an object whose
// "labs" key holds the array (distinct from the fortresses shape above).
const prolabsJSON = `{
  "status": true,
  "data": {
    "count": 33,
    "labs": [
      {"id": 25, "name": "Mythical"},
      {"id": 22, "name": "Puppet"}
    ]
  }
}`

func TestExtractProlabsNamesAndIDs(t *testing.T) {
	got, err := extractProlabsNamesAndIDs(prolabsJSON)
	if err != nil {
		t.Fatalf("extractProlabsNamesAndIDs returned error: %v", err)
	}
	want := map[string]int{"Mythical": 25, "Puppet": 22}
	if len(got) != len(want) {
		t.Fatalf("got %d prolabs, want %d: %v", len(got), len(want), got)
	}
	for name, id := range want {
		if got[name] != id {
			t.Errorf("prolab %q: got id %d, want %d", name, got[name], id)
		}
	}
}

// Captured from GET /api/v4/machine/recommended/ on 2026-06-19. The "id" is a
// JSON number, which json.Unmarshal into interface{} yields as float64 — the
// exact condition that made the original int comparison always false.
const recommendedCard1JSON = `{"id": 909, "name": "Checkpoint"}`

func TestIsReleaseMachine(t *testing.T) {
	var card map[string]interface{}
	if err := json.Unmarshal([]byte(recommendedCard1JSON), &card); err != nil {
		t.Fatalf("failed to unmarshal fixture: %v", err)
	}

	if _, ok := card["id"].(float64); !ok {
		t.Fatalf("precondition: card[\"id\"] should decode to float64, got %T", card["id"])
	}

	if !isReleaseMachine(card, 909) {
		t.Error("isReleaseMachine(card, 909) = false, want true (float64 id must match int)")
	}
	if isReleaseMachine(card, 910) {
		t.Error("isReleaseMachine(card, 910) = true, want false")
	}
	if isReleaseMachine(map[string]interface{}{}, 909) {
		t.Error("isReleaseMachine on card without id = true, want false")
	}
}
