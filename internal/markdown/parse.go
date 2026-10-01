// Package markdown parses flow Markdown files into a graph.Flow.
//
// The dialect is intentionally small and AI-friendly:
//
//	---
//	flow: Checkout Flow
//	start: cart
//	actors: [customer, system]
//	---
//
//	## cart
//	```yaml
//	type: step
//	title: Cart Review
//	actor: customer
//	code: src/cart.ts:42
//	routes:
//	  - to: payment
//	    when: cart valid
//	```
//	### Logic
//	Free prose...
//	### Requirements
//	- one requirement per line
package markdown

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/madddtone/flowc/internal/graph"
	"gopkg.in/yaml.v3"
)

type frontMatter struct {
	Flow   string   `yaml:"flow"`
	Start  string   `yaml:"start"`
	Actors []string `yaml:"actors"`
}

type nodeMeta struct {
	ID       string        `yaml:"id"`
	Type     string        `yaml:"type"`
	Title    string        `yaml:"title"`
	Actor    string        `yaml:"actor"`
	Owner    string        `yaml:"owner"`
	Status   string        `yaml:"status"`
	Priority string        `yaml:"priority"`
	Code     string        `yaml:"code"`
	Tags     []string      `yaml:"tags"`
	Flow     string        `yaml:"flow"`
	Routes   []graph.Route `yaml:"routes"`
}

// ParseFile reads and parses a flow Markdown file.
func ParseFile(path string) (*graph.Flow, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	flow, err := Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}
	return flow, nil
}

// Parse parses flow Markdown source.
func Parse(data []byte) (*graph.Flow, error) {
	text := strings.ReplaceAll(string(data), "\r\n", "\n")
	lines := strings.Split(text, "\n")

	fm, bodyStart, err := parseFrontMatter(lines)
	if err != nil {
		return nil, fmt.Errorf("front matter: %w", err)
	}

	flow := &graph.Flow{
		Name:   fm.Flow,
		Start:  fm.Start,
		Actors: fm.Actors,
	}

	type rawNode struct {
		node     *graph.Node
		yamlBuf  []string
		sections map[string]*strings.Builder
		order    []string
	}

	var cur *rawNode
	var lastSection string
	inFence := false
	var fenceLang string

	flush := func() error {
		if cur == nil {
			return nil
		}
		meta := nodeMeta{}
		if yamlText := strings.TrimSpace(strings.Join(cur.yamlBuf, "\n")); yamlText != "" {
			if err := yaml.Unmarshal([]byte(yamlText), &meta); err != nil {
				return fmt.Errorf("node %q: %w", cur.node.ID, err)
			}
		}
		applyMeta(cur.node, meta)
		for _, key := range cur.order {
			if b, ok := cur.sections[key]; ok {
				val := strings.TrimRight(b.String(), "\n")
				if strings.TrimSpace(val) != "" {
					if cur.node.Sections == nil {
						cur.node.Sections = map[string]string{}
					}
					cur.node.Sections[key] = val
					cur.node.SectionOrder = append(cur.node.SectionOrder, key)
				}
			}
		}
		flow.Nodes = append(flow.Nodes, cur.node)
		cur = nil
		return nil
	}

	for i := bodyStart; i < len(lines); i++ {
		line := lines[i]
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "```") {
			if !inFence {
				inFence = true
				fenceLang = strings.TrimSpace(strings.TrimPrefix(trimmed, "```"))
				continue
			}
			inFence = false
			fenceLang = ""
			continue
		}
		if inFence {
			if cur != nil && (fenceLang == "yaml" || fenceLang == "yml" || fenceLang == "") {
				cur.yamlBuf = append(cur.yamlBuf, line)
			}
			continue
		}

		if strings.HasPrefix(line, "## ") {
			if err := flush(); err != nil {
				return nil, err
			}
			id := strings.TrimSpace(strings.TrimPrefix(line, "## "))
			id = strings.TrimPrefix(id, "Node: ")
			node := &graph.Node{ID: id}
			cur = &rawNode{node: node, sections: map[string]*strings.Builder{}}
			lastSection = ""
			continue
		}
		if strings.HasPrefix(line, "# ") && flow.Name == "" && cur == nil {
			flow.Name = strings.TrimSpace(strings.TrimPrefix(line, "# "))
			continue
		}
		if strings.HasPrefix(line, "### ") {
			label := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "### ")))
			if cur != nil {
				if _, seen := cur.sections[label]; !seen {
					cur.sections[label] = &strings.Builder{}
					cur.order = append(cur.order, label)
				}
				lastSection = label
			}
			continue
		}

		if cur != nil && lastSection != "" {
			cur.sections[lastSection].WriteString(line)
			cur.sections[lastSection].WriteString("\n")
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}

	flow.Edges = buildEdges(flow.Nodes)
	return flow, nil
}

func applyMeta(n *graph.Node, m nodeMeta) {
	if m.ID != "" {
		n.ID = m.ID
	} else {
		n.ID = Slug(n.ID)
	}
	n.Type = strings.ToLower(strings.TrimSpace(m.Type))
	if n.Type == "" {
		n.Type = graph.TypeStep
	}
	n.Title = m.Title
	if n.Title == "" {
		n.Title = n.ID
	}
	n.Actor = m.Actor
	n.Owner = m.Owner
	n.Status = m.Status
	n.Priority = m.Priority
	n.Code = m.Code
	n.Tags = m.Tags
	n.Subflow = m.Flow
	n.Routes = m.Routes
}

func buildEdges(nodes []*graph.Node) []graph.Edge {
	var edges []graph.Edge
	seen := map[string]int{}
	for _, n := range nodes {
		for _, r := range n.Routes {
			if strings.TrimSpace(r.To) == "" {
				continue
			}
			key := n.ID + "->" + r.To
			seen[key]++
			id := key
			if seen[key] > 1 {
				id = fmt.Sprintf("%s#%d", key, seen[key])
			}
			edges = append(edges, graph.Edge{
				ID:    id,
				From:  n.ID,
				To:    r.To,
				When:  r.When,
				Label: r.Label,
			})
		}
	}
	return edges
}

func parseFrontMatter(lines []string) (frontMatter, int, error) {
	var fm frontMatter
	i := 0
	for i < len(lines) && strings.TrimSpace(lines[i]) == "" {
		i++
	}
	if i >= len(lines) || strings.TrimSpace(lines[i]) != "---" {
		return fm, i, nil
	}
	start := i + 1
	j := start
	for j < len(lines) {
		t := strings.TrimSpace(lines[j])
		if t == "---" || t == "..." {
			break
		}
		j++
	}
	if j >= len(lines) {
		return fm, 0, fmt.Errorf("unterminated front matter")
	}
	block := strings.Join(lines[start:j], "\n")
	if err := yaml.Unmarshal([]byte(block), &fm); err != nil {
		return fm, 0, err
	}
	return fm, j + 1, nil
}

// Slug converts a heading into a route-safe identifier.
func Slug(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	var b strings.Builder
	prevDash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
			prevDash = false
		case r == '-' || r == '_' || r == '.' || r == '/':
			if !prevDash {
				b.WriteRune(r)
				prevDash = true
			}
		default:
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}
