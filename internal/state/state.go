// Package state manages the active-flow pointer that tells the canvas
// which repo's flow.json to display.
package state

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// Active points at the currently selected flow.
type Active struct {
	// ProjectID is set when the active flow belongs to the central store.
	ProjectID string `json:"projectId,omitempty"`
	Name      string `json:"name,omitempty"`
	FlowFile  string `json:"flowFile"`
	JSONFile  string `json:"jsonFile"`
	RepoRoot  string `json:"repoRoot,omitempty"`
	UpdatedAt string `json:"updatedAt"`
	// Error carries the latest watcher compile error, if any, so the canvas
	// can surface it without probing for a missing side file.
	Error string `json:"error,omitempty"`
}

// Dir returns the flow-tracker state directory.
func Dir() string {
	if x := os.Getenv("XDG_STATE_HOME"); x != "" {
		return filepath.Join(x, "flow-tracker")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "flow-tracker")
	}
	return filepath.Join(home, ".local", "state", "flow-tracker")
}

// ActivePath is the pointer file path.
func ActivePath() string { return filepath.Join(Dir(), "active.json") }

// ErrorPath is where the watcher writes the latest compile error, if any.
func ErrorPath(jsonFile string) string { return jsonFile + ".error" }

// WriteActive persists the active flow pointer.
func WriteActive(a Active) error {
	a.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := os.MkdirAll(Dir(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(a, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(ActivePath(), append(data, '\n'), 0o644)
}

// ReadActive loads the active flow pointer.
func ReadActive() (Active, error) {
	var a Active
	data, err := os.ReadFile(ActivePath())
	if err != nil {
		return a, err
	}
	err = json.Unmarshal(data, &a)
	return a, err
}
