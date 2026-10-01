// Package store manages the central Flow Tracker project store: one
// directory per project holding the authored flow.md and compiled flow.json,
// plus an index.json the canvas reads to list/switch projects.
//
// Layout (under $XDG_DATA_HOME/flow-tracker, default ~/.local/share/flow-tracker):
//
//	index.json                 aggregate for the canvas project picker
//	projects/<id>/flow.md      authored source (the AI/human edits this)
//	projects/<id>/flow.json    compiled graph
//	projects/<id>/meta.json    name, repo, description, stats
package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/madddtone/flowc/internal/schema"
)

// Root returns the central data directory for Flow Tracker.
func Root() string {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "flow-tracker")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "flow-tracker")
	}
	return filepath.Join(home, ".local", "share", "flow-tracker")
}

// ProjectsDir is where per-project folders live.
func ProjectsDir() string { return filepath.Join(Root(), "projects") }

// IndexPath is the aggregate file the canvas reads.
func IndexPath() string { return filepath.Join(Root(), "index.json") }

// ProjectDir returns a project's directory.
func ProjectDir(id string) string { return filepath.Join(ProjectsDir(), id) }

// FlowFile returns a project's authored Markdown path.
func FlowFile(id string) string { return filepath.Join(ProjectDir(id), "flow.md") }

// JSONFile returns a project's compiled graph path.
func JSONFile(id string) string { return filepath.Join(ProjectDir(id), "flow.json") }

func metaPath(id string) string { return filepath.Join(ProjectDir(id), "meta.json") }

// Meta is per-project metadata.
type Meta struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Repo        string `json:"repo,omitempty"`
	Description string `json:"description,omitempty"`
	Nodes       int    `json:"nodes"`
	Edges       int    `json:"edges"`
	CreatedAt   string `json:"createdAt"`
	UpdatedAt   string `json:"updatedAt"`
}

// Entry is a project plus its resolved paths, used by the index.
type Entry struct {
	Meta
	FlowFile string `json:"flowFile"`
	JSONFile string `json:"jsonFile"`
}

// Index is the aggregate document the canvas reads.
type Index struct {
	Version   int     `json:"version"`
	UpdatedAt string  `json:"updatedAt"`
	Projects  []Entry `json:"projects"`
}

// List scans the store and returns every project, newest first.
func List() ([]Entry, error) {
	dirs, err := os.ReadDir(ProjectsDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []Entry
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		m, err := readMeta(d.Name())
		if err != nil {
			continue
		}
		out = append(out, entryFor(m))
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt > out[j].UpdatedAt
	})
	return out, nil
}

// Find resolves a project by id or (case-insensitive) name.
func Find(ref string) (Entry, error) {
	entries, err := List()
	if err != nil {
		return Entry{}, err
	}
	ref = strings.TrimSpace(ref)
	for _, e := range entries {
		if e.ID == ref {
			return e, nil
		}
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name, ref) {
			return e, nil
		}
	}
	return Entry{}, fmt.Errorf("no project named %q (see `flowc list`)", ref)
}

// New creates a project with a scaffolded flow.md and returns it.
func New(name, repo, description string) (Entry, error) {
	if strings.TrimSpace(name) == "" {
		return Entry{}, fmt.Errorf("project name is required")
	}
	id := uniqueID(Slug(name))
	dir := ProjectDir(id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Entry{}, err
	}
	now := time.Now().UTC().Format(time.RFC3339)
	if repo != "" {
		repo = abs(repo)
	}
	m := Meta{
		ID:          id,
		Name:        name,
		Repo:        repo,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := os.WriteFile(FlowFile(id), []byte(schema.SampleFor(name)), 0o644); err != nil {
		return Entry{}, err
	}
	if err := writeMeta(m); err != nil {
		return Entry{}, err
	}
	if err := WriteIndex(); err != nil {
		return Entry{}, err
	}
	return entryFor(m), nil
}

// UpdateStats records node/edge counts after a compile.
func UpdateStats(id string, nodes, edges int) error {
	m, err := readMeta(id)
	if err != nil {
		return err
	}
	m.Nodes, m.Edges = nodes, edges
	m.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := writeMeta(m); err != nil {
		return err
	}
	return WriteIndex()
}

// Remove deletes a project directory and refreshes the index.
func Remove(ref string) (Entry, error) {
	e, err := Find(ref)
	if err != nil {
		return Entry{}, err
	}
	if err := os.RemoveAll(ProjectDir(e.ID)); err != nil {
		return Entry{}, err
	}
	if err := WriteIndex(); err != nil {
		return Entry{}, err
	}
	return e, nil
}

// WriteIndex regenerates index.json from the project directories.
func WriteIndex() error {
	entries, err := List()
	if err != nil {
		return err
	}
	idx := Index{
		Version:   1,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
		Projects:  entries,
	}
	if idx.Projects == nil {
		idx.Projects = []Entry{}
	}
	if err := os.MkdirAll(Root(), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(idx, "", "  ")
	if err != nil {
		return err
	}
	tmp := IndexPath() + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, IndexPath()); err != nil {
		return err
	}
	// Keep the authoring guide discoverable next to the flows.
	_ = os.WriteFile(filepath.Join(Root(), "GUIDE.md"), []byte(schema.Guide), 0o644)
	return nil
}

func entryFor(m Meta) Entry {
	return Entry{
		Meta:     m,
		FlowFile: FlowFile(m.ID),
		JSONFile: JSONFile(m.ID),
	}
}

func readMeta(id string) (Meta, error) {
	var m Meta
	data, err := os.ReadFile(metaPath(id))
	if err != nil {
		return m, err
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return m, err
	}
	if m.ID == "" {
		m.ID = id
	}
	return m, nil
}

func writeMeta(m Meta) error {
	if err := os.MkdirAll(ProjectDir(m.ID), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath(m.ID), append(data, '\n'), 0o644)
}

func uniqueID(base string) string {
	if base == "" {
		base = "project"
	}
	id := base
	for i := 2; ; i++ {
		if _, err := os.Stat(ProjectDir(id)); os.IsNotExist(err) {
			return id
		}
		id = fmt.Sprintf("%s-%d", base, i)
	}
}

// Slug converts a project name into a filesystem-safe id.
func Slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

func abs(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return a
	}
	return p
}
