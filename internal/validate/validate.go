// Package validate applies structural lint rules to a parsed flow.
package validate

import (
	"fmt"
	"sort"

	"github.com/madddtone/flowc/internal/graph"
)

// Severity ranks a diagnostic.
type Severity int

const (
	Info Severity = iota
	Warning
	Error
)

func (s Severity) String() string {
	switch s {
	case Error:
		return "error"
	case Warning:
		return "warning"
	default:
		return "info"
	}
}

// Diagnostic is a single finding.
type Diagnostic struct {
	Severity Severity
	Node     string
	Message  string
}

func (d Diagnostic) String() string {
	loc := d.Node
	if loc == "" {
		loc = "flow"
	}
	return fmt.Sprintf("%s: %s: %s", d.Severity, loc, d.Message)
}

// Result is the outcome of validation.
type Result struct {
	Diagnostics []Diagnostic
	Start       string
}

// HasErrors reports whether any diagnostic is an error.
func (r Result) HasErrors() bool {
	for _, d := range r.Diagnostics {
		if d.Severity == Error {
			return true
		}
	}
	return false
}

// Validate checks the flow and resolves its entry node.
func Validate(f *graph.Flow) Result {
	var diags []Diagnostic
	if len(f.Nodes) == 0 {
		diags = append(diags, Diagnostic{Severity: Error, Message: "flow has no nodes"})
		return Result{Diagnostics: diags}
	}

	index := map[string]*graph.Node{}
	for _, n := range f.Nodes {
		if _, dup := index[n.ID]; dup {
			diags = append(diags, Diagnostic{Severity: Error, Node: n.ID, Message: "duplicate node id"})
			continue
		}
		index[n.ID] = n
		if !graph.KnownTypes[n.Type] {
			diags = append(diags, Diagnostic{Severity: Warning, Node: n.ID, Message: fmt.Sprintf("unknown node type %q", n.Type)})
		}
		if n.Type == graph.TypeSubflow && n.Subflow == "" {
			diags = append(diags, Diagnostic{Severity: Warning, Node: n.ID, Message: "subflow node has no `flow:` reference"})
		}
	}

	// Route targets.
	adj := map[string][]string{}
	for _, e := range f.Edges {
		if _, ok := index[e.To]; !ok {
			diags = append(diags, Diagnostic{Severity: Error, Node: e.From, Message: fmt.Sprintf("route points to unknown node %q", e.To)})
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
	}

	// Resolve entry node.
	start := f.Start
	if start != "" {
		if _, ok := index[start]; !ok {
			diags = append(diags, Diagnostic{Severity: Error, Message: fmt.Sprintf("start node %q does not exist", start)})
			start = ""
		}
	}
	if start == "" {
		for _, n := range f.Nodes {
			if n.Type == graph.TypeStart {
				start = n.ID
				break
			}
		}
	}
	if start == "" {
		start = f.Nodes[0].ID
		diags = append(diags, Diagnostic{Severity: Info, Message: fmt.Sprintf("no start declared; using first node %q", start)})
	}

	// Reachability from start.
	reachable := map[string]bool{}
	var stack []string
	if start != "" {
		stack = append(stack, start)
	}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if reachable[cur] {
			continue
		}
		reachable[cur] = true
		stack = append(stack, adj[cur]...)
	}
	var unreachable []string
	for _, n := range f.Nodes {
		if !reachable[n.ID] {
			unreachable = append(unreachable, n.ID)
		}
	}
	sort.Strings(unreachable)
	for _, id := range unreachable {
		diags = append(diags, Diagnostic{Severity: Warning, Node: id, Message: "unreachable from start"})
	}

	// Dead ends and branch sanity.
	for _, n := range f.Nodes {
		out := adj[n.ID]
		if len(out) == 0 && n.Type != graph.TypeEnd {
			diags = append(diags, Diagnostic{Severity: Warning, Node: n.ID, Message: "no outgoing routes"})
		}
		if n.Type == graph.TypeDecision && len(out) < 2 {
			diags = append(diags, Diagnostic{Severity: Warning, Node: n.ID, Message: "decision node has fewer than two routes"})
		}
	}

	return Result{Diagnostics: diags, Start: start}
}
