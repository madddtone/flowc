// Package graph defines the in-memory flow model shared by the parser,
// validator, layout engine, and JSON emitter.
package graph

// Node types.
const (
	TypeStart    = "start"
	TypeEnd      = "end"
	TypeStep     = "step"
	TypeDecision = "decision"
	TypeSubflow  = "subflow"
	TypeExternal = "external"
	TypeParallel = "parallel"
	TypeJoin     = "join"
)

// KnownTypes is the set of node types accepted by the parser. Unknown types
// are preserved but reported by the validator.
var KnownTypes = map[string]bool{
	TypeStart:    true,
	TypeEnd:      true,
	TypeStep:     true,
	TypeDecision: true,
	TypeSubflow:  true,
	TypeExternal: true,
	TypeParallel: true,
	TypeJoin:     true,
}

// Rect is an axis-aligned rectangle with the origin at its top-left corner.
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Node is a single vertex in the flow.
type Node struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Title    string   `json:"title"`
	Actor    string   `json:"actor,omitempty"`
	Owner    string   `json:"owner,omitempty"`
	Status   string   `json:"status,omitempty"`
	Priority string   `json:"priority,omitempty"`
	Code     string   `json:"code,omitempty"`
	Tags     []string `json:"tags,omitempty"`
	// Subflow is a relative path to another flow Markdown file.
	Subflow string `json:"subflow,omitempty"`
	// SubflowJSON / SubflowName / SubflowNodes are filled by the compiler when
	// a subflow reference resolves, so the canvas can drill into it.
	SubflowJSON  string `json:"subflowJson,omitempty"`
	SubflowName  string `json:"subflowName,omitempty"`
	SubflowNodes int    `json:"subflowNodes,omitempty"`
	// Sections holds long-form prose attributes keyed by lowercase label
	// (logic, requirements, prerequisites, notes, inputs, outputs,
	// failure modes, and any custom label).
	Sections map[string]string `json:"sections,omitempty"`
	// SectionOrder preserves author ordering for the inspector.
	SectionOrder []string `json:"sectionOrder,omitempty"`
	// Routes are the outgoing edges declared on this node.
	Routes []Route `json:"-"`

	// Layout fields, populated by the layout engine.
	Rank int  `json:"rank"`
	Lane int  `json:"lane"`
	Rect Rect `json:"rect"`
}

// Route is an outgoing edge as authored on a node.
type Route struct {
	To    string `yaml:"to" json:"to"`
	When  string `yaml:"when" json:"when,omitempty"`
	Label string `yaml:"label" json:"label,omitempty"`
}

// Edge is a resolved, emitted connection between two nodes.
type Edge struct {
	ID    string `json:"id"`
	From  string `json:"from"`
	To    string `json:"to"`
	When  string `json:"when,omitempty"`
	Label string `json:"label,omitempty"`
	// Back marks a back-edge (part of a cycle) so renderers can route it
	// around the graph instead of through it.
	Back bool `json:"back,omitempty"`
}

// Flow is a complete parsed flow, ready to validate and lay out.
type Flow struct {
	Name   string
	Start  string
	Actors []string
	Nodes  []*Node
	Edges  []Edge
}

// NodeIndex returns a lookup map from id to node.
func (f *Flow) NodeIndex() map[string]*Node {
	m := make(map[string]*Node, len(f.Nodes))
	for _, n := range f.Nodes {
		m[n.ID] = n
	}
	return m
}
