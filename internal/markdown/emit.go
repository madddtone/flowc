package markdown

import (
	"fmt"
	"strings"

	"github.com/madddtone/flowc/internal/graph"
)

// Marshal renders a flow back to the flow.md dialect. It is used by the
// interactive TUI when saving captured input.
func Marshal(f *graph.Flow) string {
	var b strings.Builder
	b.WriteString("---\n")
	name := f.Name
	if name == "" {
		name = "Untitled Flow"
	}
	fmt.Fprintf(&b, "flow: %s\n", yamlScalar(name))
	if f.Start != "" {
		fmt.Fprintf(&b, "start: %s\n", yamlScalar(f.Start))
	}
	if len(f.Actors) > 0 {
		fmt.Fprintf(&b, "actors: [%s]\n", strings.Join(quoteAll(f.Actors), ", "))
	}
	b.WriteString("---\n\n")

	for _, n := range f.Nodes {
		fmt.Fprintf(&b, "## %s\n", n.ID)
		b.WriteString("```yaml\n")
		nt := n.Type
		if nt == "" {
			nt = graph.TypeStep
		}
		fmt.Fprintf(&b, "type: %s\n", nt)
		if n.Title != "" && n.Title != n.ID {
			fmt.Fprintf(&b, "title: %s\n", yamlScalar(n.Title))
		}
		if n.Actor != "" {
			fmt.Fprintf(&b, "actor: %s\n", yamlScalar(n.Actor))
		}
		if n.Owner != "" {
			fmt.Fprintf(&b, "owner: %s\n", yamlScalar(n.Owner))
		}
		if n.Status != "" {
			fmt.Fprintf(&b, "status: %s\n", yamlScalar(n.Status))
		}
		if n.Priority != "" {
			fmt.Fprintf(&b, "priority: %s\n", yamlScalar(n.Priority))
		}
		if n.Code != "" {
			fmt.Fprintf(&b, "code: %s\n", yamlScalar(n.Code))
		}
		if len(n.Tags) > 0 {
			fmt.Fprintf(&b, "tags: [%s]\n", strings.Join(quoteAll(n.Tags), ", "))
		}
		if n.Subflow != "" {
			fmt.Fprintf(&b, "flow: %s\n", yamlScalar(n.Subflow))
		}
		if len(n.Routes) > 0 {
			b.WriteString("routes:\n")
			for _, r := range n.Routes {
				fmt.Fprintf(&b, "  - to: %s\n", yamlScalar(r.To))
				if r.When != "" {
					fmt.Fprintf(&b, "    when: %s\n", yamlScalar(r.When))
				}
				if r.Label != "" {
					fmt.Fprintf(&b, "    label: %s\n", yamlScalar(r.Label))
				}
			}
		}
		b.WriteString("```\n")

		for _, key := range n.SectionOrder {
			val, ok := n.Sections[key]
			if !ok || strings.TrimSpace(val) == "" {
				continue
			}
			fmt.Fprintf(&b, "### %s\n%s\n\n", titleCase(key), strings.TrimRight(val, "\n"))
		}
		if len(n.SectionOrder) == 0 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func yamlScalar(s string) string {
	if s == "" {
		return `""`
	}
	if strings.ContainsAny(s, ":#{}[],&*?|<>=!%@`\"'\n") || strings.HasPrefix(s, " ") || strings.HasSuffix(s, " ") {
		return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
	}
	return s
}

func quoteAll(xs []string) []string {
	out := make([]string, len(xs))
	for i, x := range xs {
		out[i] = yamlScalar(x)
	}
	return out
}

func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if w == "" {
			continue
		}
		words[i] = strings.ToUpper(w[:1]) + w[1:]
	}
	return strings.Join(words, " ")
}
