// Package tui implements an interactive capture wizard that writes flow.md.
package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/madddtone/flowc/internal/graph"
	"github.com/madddtone/flowc/internal/markdown"
)

var (
	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	selStyle    = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212"))
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	hintStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	focusStyle  = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("39"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("203"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("35"))
)

type mode int

const (
	modeList mode = iota
	modeForm
)

type field struct {
	label string
	value string
}

type model struct {
	path   string
	flow   *graph.Flow
	cur    int
	rcur   int
	mode   mode
	kind   string
	fields []field
	focus  int
	status string
	err    string
	width  int
	height int
	quit   bool
}

// Run launches the wizard against path.
func Run(path string) error {
	m := &model{path: path, flow: load(path)}
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		return err
	}
	return nil
}

func load(path string) *graph.Flow {
	data, err := os.ReadFile(path)
	if err != nil {
		return &graph.Flow{Name: "Untitled Flow"}
	}
	f, err := markdown.Parse(data)
	if err != nil {
		return &graph.Flow{Name: "Untitled Flow"}
	}
	return f
}

func (m *model) Init() tea.Cmd { return nil }

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		return m, nil
	case tea.KeyMsg:
		if m.mode == modeForm {
			return m.updateForm(msg)
		}
		return m.updateList(msg)
	}
	return m, nil
}

func (m *model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	n := len(m.flow.Nodes)
	switch msg.String() {
	case "q", "ctrl+c":
		m.quit = true
		return m, tea.Quit
	case "up", "k":
		if m.cur > 0 {
			m.cur--
			m.rcur = 0
		}
	case "down", "j":
		if m.cur < n-1 {
			m.cur++
			m.rcur = 0
		}
	case "left", "h":
		if m.rcur > 0 {
			m.rcur--
		}
	case "right", "l":
		if m.cur < n && m.rcur < len(m.flow.Nodes[m.cur].Routes)-1 {
			m.rcur++
		}
	case "a":
		m.openForm("node", []field{{"id", ""}, {"title", ""}, {"type", "step"}, {"actor", ""}})
	case "e":
		if m.cur < n {
			node := m.flow.Nodes[m.cur]
			m.openForm("edit", []field{{"id", node.ID}, {"title", node.Title}, {"type", node.Type}, {"actor", node.Actor}})
		}
	case "x":
		if m.cur < n {
			removed := m.flow.Nodes[m.cur].ID
			m.flow.Nodes = append(m.flow.Nodes[:m.cur], m.flow.Nodes[m.cur+1:]...)
			m.removeRoutesTo(removed)
			if m.cur >= len(m.flow.Nodes) && m.cur > 0 {
				m.cur--
			}
			m.status = "deleted " + removed
		}
	case "r":
		if m.cur < n {
			m.openForm("route", []field{{"to", ""}, {"when", ""}, {"label", ""}})
		}
	case "X":
		if m.cur < n && len(m.flow.Nodes[m.cur].Routes) > 0 {
			node := m.flow.Nodes[m.cur]
			node.Routes = append(node.Routes[:m.rcur], node.Routes[m.rcur+1:]...)
			if m.rcur >= len(node.Routes) && m.rcur > 0 {
				m.rcur--
			}
			m.syncEdges()
		}
	case "m":
		m.openForm("meta", []field{{"flow", m.flow.Name}, {"start", m.flow.Start}})
	case "s":
		if err := m.save(); err != nil {
			m.err = err.Error()
		} else {
			m.status = "saved " + m.path + " (run `flowc compile` to refresh)"
			m.err = ""
		}
	}
	return m, nil
}

func (m *model) openForm(kind string, fields []field) {
	m.mode = modeForm
	m.kind = kind
	m.fields = fields
	m.focus = 0
	m.err = ""
}

func (m *model) updateForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.mode = modeList
		return m, nil
	case "enter":
		if m.focus < len(m.fields)-1 {
			m.focus++
			return m, nil
		}
		m.commit()
		m.mode = modeList
		return m, nil
	case "tab", "down":
		if m.focus < len(m.fields)-1 {
			m.focus++
		}
	case "shift+tab", "up":
		if m.focus > 0 {
			m.focus--
		}
	case "backspace":
		v := m.fields[m.focus].value
		if len(v) > 0 {
			m.fields[m.focus].value = v[:len(v)-1]
		}
	case "ctrl+u":
		m.fields[m.focus].value = ""
	case " ":
		m.fields[m.focus].value += " "
	default:
		if len(msg.Runes) > 0 {
			m.fields[m.focus].value += string(msg.Runes)
		}
	}
	return m, nil
}

func (m *model) commit() {
	val := func(i int) string { return strings.TrimSpace(m.fields[i].value) }
	switch m.kind {
	case "node", "edit":
		if val(0) == "" {
			m.err = "id is required"
			return
		}
		id := markdown.Slug(val(0))
		var node *graph.Node
		if m.kind == "edit" && m.cur < len(m.flow.Nodes) {
			node = m.flow.Nodes[m.cur]
			old := node.ID
			node.ID = id
			if old != id {
				for _, o := range m.flow.Nodes {
					for i := range o.Routes {
						if o.Routes[i].To == old {
							o.Routes[i].To = id
						}
					}
				}
			}
		} else {
			for _, e := range m.flow.Nodes {
				if e.ID == id {
					m.err = "duplicate id " + id
					return
				}
			}
			node = &graph.Node{ID: id}
			m.flow.Nodes = append(m.flow.Nodes, node)
		}
		node.Title = val(1)
		if node.Title == "" {
			node.Title = node.ID
		}
		if val(2) != "" {
			node.Type = strings.ToLower(val(2))
		}
		node.Actor = val(3)
		m.status = "node " + id
	case "route":
		if m.cur >= len(m.flow.Nodes) || val(0) == "" {
			m.err = "target is required"
			return
		}
		m.flow.Nodes[m.cur].Routes = append(m.flow.Nodes[m.cur].Routes, graph.Route{To: markdown.Slug(val(0)), When: val(1), Label: val(2)})
		m.syncEdges()
	case "meta":
		m.flow.Name = val(0)
		m.flow.Start = markdown.Slug(val(1))
	}
	if m.err == "" {
		m.syncEdges()
	}
}

func (m *model) removeRoutesTo(id string) {
	for _, n := range m.flow.Nodes {
		var kept []graph.Route
		for _, r := range n.Routes {
			if r.To != id {
				kept = append(kept, r)
			}
		}
		n.Routes = kept
	}
	m.syncEdges()
}

func (m *model) syncEdges() {
	var edges []graph.Edge
	seen := map[string]int{}
	for _, n := range m.flow.Nodes {
		for _, r := range n.Routes {
			if r.To == "" {
				continue
			}
			key := n.ID + "->" + r.To
			seen[key]++
			id := key
			if seen[key] > 1 {
				id = fmt.Sprintf("%s#%d", key, seen[key])
			}
			edges = append(edges, graph.Edge{ID: id, From: n.ID, To: r.To, When: r.When, Label: r.Label})
		}
	}
	m.flow.Edges = edges
}

func (m *model) save() error {
	if m.flow.Name == "" {
		m.flow.Name = "Untitled Flow"
	}
	return os.WriteFile(m.path, []byte(markdown.Marshal(m.flow)), 0o644)
}

func (m *model) View() string {
	if m.quit {
		return ""
	}
	var b strings.Builder
	b.WriteString(titleStyle.Render("flowc tui") + dimStyle.Render("  "+m.path) + "\n\n")

	if m.mode == modeForm {
		b.WriteString(m.formView())
		return b.String()
	}

	if len(m.flow.Nodes) == 0 {
		b.WriteString(hintStyle.Render("(no nodes yet - press 'a' to add one)") + "\n")
	}
	for i, n := range m.flow.Nodes {
		marker := "  "
		line := fmt.Sprintf("%-16s %-10s %s", n.ID, n.Type, n.Title)
		if i == m.cur {
			marker = selStyle.Render("> ")
			line = selStyle.Render(line)
		}
		b.WriteString(marker + line + "\n")
		if i == m.cur && len(n.Routes) > 0 {
			for j, r := range n.Routes {
				rm := "    -> "
				route := r.To
				if r.When != "" {
					route += "  when: " + r.When
				}
				if j == m.rcur {
					rm = selStyle.Render("    -> ")
					route = selStyle.Render(route)
				}
				b.WriteString(rm + route + "\n")
			}
		}
	}

	b.WriteString("\n")
	if m.err != "" {
		b.WriteString(errStyle.Render("error: "+m.err) + "\n")
	} else if m.status != "" {
		b.WriteString(statusStyle.Render(m.status) + "\n")
	}
	b.WriteString(hintStyle.Render("a add  e edit  x delete  r route  X del route  m meta  s save  q quit"))
	return b.String()
}

func (m *model) formView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("new "+m.kind) + "\n\n")
	for i, f := range m.fields {
		label := fmt.Sprintf("%-6s", f.label)
		val := f.value
		if i == m.focus {
			b.WriteString(focusStyle.Render("> "+label) + val + "_\n")
		} else {
			b.WriteString("  " + label + val + "\n")
		}
	}
	b.WriteString("\n" + hintStyle.Render("enter next/commit   tab move   esc cancel"))
	return b.String()
}
