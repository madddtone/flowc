// Package compile turns a flow Markdown file into the flow.json document
// consumed by the canvas plugin. Subflow references are compiled recursively
// and annotated on their node so the canvas can drill in.
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

// Result bundles the document with validation diagnostics and every compiled
// subflow (keyed by absolute json path).
type Result struct {
	Doc      *Output
	Diags    []validate.Diagnostic
	children map[string]*Output
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

// Compile parses, validates, lays out, and recursively compiles subflows.
func Compile(path string) (*Result, error) {
	return compileTree(path, map[string]bool{})
}

func compileTree(path string, seen map[string]bool) (*Result, error) {
	abs := absPath(path)
	if seen[abs] {
		return nil, fmt.Errorf("subflow cycle: %s", abs)
	}
	seen[abs] = true

	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	flow, err := markdown.Parse(data)
	if err != nil {
		return nil, err
	}
	vr := validate.Validate(flow)
	if vr.Start != "" {
		flow.Start = vr.Start
	}
	layout.Layout(flow, flow.Start)

	children := map[string]*Output{}
	var subDiags []validate.Diagnostic
	for _, n := range flow.Nodes {
		if n.Subflow == "" {
			continue
		}
		ref := resolveRef(filepath.Dir(abs), n.Subflow)
		if _, err := os.Stat(ref); err != nil {
			subDiags = append(subDiags, validate.Diagnostic{
				Severity: validate.Warning,
				Node:     n.ID,
				Message:  fmt.Sprintf("subflow file not found: %s", n.Subflow),
			})
			continue
		}
		child, err := compileTree(ref, seen)
		if err != nil {
			subDiags = append(subDiags, validate.Diagnostic{
				Severity: validate.Warning,
				Node:     n.ID,
				Message:  fmt.Sprintf("subflow %s: %v", n.Subflow, err),
			})
			continue
		}
		childPath := jsonPathFor(ref)
		children[childPath] = child.Doc
		for k, v := range child.children {
			children[k] = v
		}
		subDiags = append(subDiags, child.Diags...)
		n.SubflowJSON = childPath
		n.SubflowName = child.Doc.Flow
		n.SubflowNodes = len(child.Doc.Nodes)
	}

	doc := &Output{
		Version: Version,
		Flow:    flow.Name,
		Start:   flow.Start,
		Actors:  flow.Actors,
		Nodes:   flow.Nodes,
		Edges:   flow.Edges,
		Meta: Meta{
			GeneratedAt: time.Now().UTC().Format(time.RFC3339),
			Source:      filepath.Base(abs),
			SourceHash:  hash(data),
			Layout:      "layered",
			Stats:       stats(flow),
		},
	}
	if doc.Flow == "" {
		doc.Flow = filepath.Base(abs)
	}
	return &Result{Doc: doc, Diags: append(vr.Diagnostics, subDiags...), children: children}, nil
}

// Write compiles path and writes the JSON document (and every subflow JSON)
// atomically.
func Write(path, outPath string) (*Result, error) {
	res, err := Compile(path)
	if err != nil {
		return nil, err
	}
	if res.Forbidden() {
		return res, fmt.Errorf("validation failed")
	}
	// Subflows first, then the top-level document.
	for childPath, doc := range res.children {
		if err := writeJSON(childPath, doc); err != nil {
			return res, err
		}
	}
	if err := writeJSON(outPath, res.Doc); err != nil {
		return res, err
	}
	return res, nil
}

func writeJSON(outPath string, doc *Output) error {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	tmp := outPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, outPath)
}

func resolveRef(dir, ref string) string {
	if !filepath.IsAbs(ref) {
		ref = filepath.Join(dir, ref)
	}
	return absPath(ref)
}

func jsonPathFor(md string) string {
	base := md[:len(md)-len(filepath.Ext(md))]
	return base + ".json"
}

func absPath(p string) string {
	if a, err := filepath.Abs(p); err == nil {
		return filepath.Clean(a)
	}
	return filepath.Clean(p)
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
