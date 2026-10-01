// Package compile turns a flow Markdown file into the flow.json document
// consumed by the canvas plugin.
package compile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/madddtone/flowc/internal/graph"
	"github.com/madddtone/flowc/internal/layout"
	"github.com/madddtone/flowc/internal/markdown"
	"github.com/madddtone/flowc/internal/validate"
)

// Version is the flow.json schema version.
const Version = 1

// Meta carries provenance for the generated document.
type Meta struct {
	GeneratedAt string `json:"generatedAt"`
	Source      string `json:"source"`
	SourceHash  string `json:"sourceHash"`
	Layout      string `json:"layout"`
	Stats       Stats  `json:"stats"`
}

// Stats summarizes the graph.
type Stats struct {
	Nodes     int `json:"nodes"`
	Edges     int `json:"edges"`
	Decisions int `json:"decisions"`
	Subflows  int `json:"subflows"`
}

// Output is the compiled flow document.
type Output struct {
	Version int           `json:"version"`
	Flow    string        `json:"flow"`
	Start   string        `json:"start"`
	Actors  []string      `json:"actors,omitempty"`
	Nodes   []*graph.Node `json:"nodes"`
	Edges   []graph.Edge  `json:"edges"`
	Meta    Meta          `json:"meta"`
}

// Result bundles the document with validation diagnostics.
type Result struct {
	Doc   *Output
	Diags []validate.Diagnostic
}

// Forbidden reports whether validation found errors.
func (r *Result) Forbidden() bool {
	for _, d := range r.Diags {
		if d.Severity == validate.Error {
			return true
		}
	}
	return false
}

// Compile parses, validates, and lays out the flow at path.
func Compile(path string) (*Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	flow, err := markdown.Parse(data)
	if err != nil {
		return nil, err
	}
	vr := validate.Validate(flow)
	vr.Diagnostics = append(vr.Diagnostics, checkSubflows(flow, path)...)
	if vr.Start != "" {
		flow.Start = vr.Start
	}
	layout.Layout(flow, flow.Start)

	doc := &Output{
		Version: Version,
		Flow:    flow.Name,
		Start:   flow.Start,
		Actors:  flow.Actors,
		Nodes:   flow.Nodes,
		Edges:   flow.Edges,
		Meta: Meta{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Source:      filepath.Base(path),
			SourceHash:  hash(data),
			Layout:      "layered",
			Stats:       stats(flow),
		},
	}
	if doc.Flow == "" {
		doc.Flow = filepath.Base(path)
	}
	return &Result{Doc: doc, Diags: vr.Diagnostics}, nil
}

// Write compiles path and writes the JSON document to outPath atomically.
func Write(path, outPath string) (*Result, error) {
	res, err := Compile(path)
	if err != nil {
		return nil, err
	}
	if res.Forbidden() {
		return res, fmt.Errorf("validation failed")
	}
	data, err := json.MarshalIndent(res.Doc, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	tmp := outPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return nil, err
	}
	if err := os.Rename(tmp, outPath); err != nil {
		return nil, err
	}
	return res, nil
}

func checkSubflows(f *graph.Flow, path string) []validate.Diagnostic {
	dir := filepath.Dir(path)
	var diags []validate.Diagnostic
	for _, n := range f.Nodes {
		if n.Subflow == "" {
			continue
		}
		ref := n.Subflow
		if !filepath.IsAbs(ref) {
			ref = filepath.Join(dir, ref)
		}
		if _, err := os.Stat(ref); err != nil {
			diags = append(diags, validate.Diagnostic{
				Severity: validate.Warning,
				Node:     n.ID,
				Message:  fmt.Sprintf("subflow file not found: %s", n.Subflow),
			})
		}
	}
	return diags
}

func stats(f *graph.Flow) Stats {
	s := Stats{Nodes: len(f.Nodes), Edges: len(f.Edges)}
	for _, n := range f.Nodes {
		switch n.Type {
		case graph.TypeDecision:
			s.Decisions++
		case graph.TypeSubflow:
			s.Subflows++
		}
	}
	return s
}

func hash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:12]
}
