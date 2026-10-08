package compile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/madddtone/flowc/internal/graph"
)

const sample = `---
flow: Test Flow
start: a
actors: [human, system]
---

## a
` + "```yaml" + `
type: start
title: Begin
actor: human
routes:
  - to: b
` + "```" + `
### Logic
Start here.

## b
` + "```yaml" + `
type: decision
title: Choose
actor: system
routes:
  - to: c
    when: yes
  - to: a
    when: no
` + "```" + `

## c
` + "```yaml" + `
type: end
title: Done
` + "```" + `
`

func writeTemp(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "flow.md")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestCompile(t *testing.T) {
	res, err := Compile(writeTemp(t, sample))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	doc := res.Doc
	if doc.Flow != "Test Flow" || doc.Start != "a" {
		t.Fatalf("unexpected header: %q start=%q", doc.Flow, doc.Start)
	}
	if len(doc.Nodes) != 3 || len(doc.Edges) != 3 {
		t.Fatalf("expected 3 nodes/3 edges, got %d/%d", len(doc.Nodes), len(doc.Edges))
	}

	byID := map[string]*graph.Node{}
	for _, n := range doc.Nodes {
		byID[n.ID] = n
	}
	if byID["a"].Sections["logic"] == "" {
		t.Fatalf("expected logic section on a")
	}
	if byID["b"].Type != graph.TypeDecision {
		t.Fatalf("expected b to be decision, got %q", byID["b"].Type)
	}

	// Back edge b->a must be flagged, and must not push a to rank > 0.
	for _, e := range doc.Edges {
		if e.From == "b" && e.To == "a" && !e.Back {
			t.Fatalf("expected b->a to be a back edge")
		}
	}
	if byID["a"].Rank != 0 {
		t.Fatalf("start should rank 0, got %d", byID["a"].Rank)
	}
	if byID["b"].Rank != 1 || byID["c"].Rank != 2 {
		t.Fatalf("unexpected ranks: b=%d c=%d", byID["b"].Rank, byID["c"].Rank)
	}
	if byID["a"].Rect.X >= byID["b"].Rect.X {
		t.Fatalf("expected a left of b")
	}
	if doc.Meta.Stats.Decisions != 1 {
		t.Fatalf("expected 1 decision, got %d", doc.Meta.Stats.Decisions)
	}
}

func TestTableNode(t *testing.T) {
	body := `---
flow: Data
start: orders
---
## orders
` + "```yaml" + `
type: table
title: orders
schema: public
columns:
  - name: id
    type: bigint
    pk: true
  - "user_id bigint FK users.id"
  - name: total
    type: numeric
    nullable: false
  - name: a
  - name: b
  - name: c
  - name: d
routes:
  - to: done
    data: order total
` + "```" + `
## done
` + "```yaml" + `
type: end
` + "```" + `
`
	res, err := Compile(writeTemp(t, body))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	n := res.Doc.Nodes[0]
	if n.Type != graph.TypeTable {
		t.Fatalf("expected table, got %q", n.Type)
	}
	if len(n.Columns) != 7 {
		t.Fatalf("expected 7 columns, got %d", len(n.Columns))
	}
	if !n.Columns[0].PK {
		t.Fatalf("expected first column PK")
	}
	if n.Columns[1].FK != "users.id" {
		t.Fatalf("compact FK not parsed: %+v", n.Columns[1])
	}
	if n.Schema != "public" {
		t.Fatalf("schema not parsed: %q", n.Schema)
	}
	// Height should reflect only the first 5 shown columns + a "+N more" row.
	shown := 5
	wantH := 24.0 + float64(shown)*18.0 + 6.0 + 16.0
	if n.Rect.H != wantH {
		t.Fatalf("table height = %v, want %v", n.Rect.H, wantH)
	}
	// data: alias becomes the edge label.
	if res.Doc.Edges[0].Label != "order total" {
		t.Fatalf("data alias not applied: %q", res.Doc.Edges[0].Label)
	}
}

func TestValidateDanglingRoute(t *testing.T) {
	body := `---
flow: Bad
start: a
---
## a
` + "```yaml" + `
type: start
routes:
  - to: missing
` + "```" + `
`
	res, err := Compile(writeTemp(t, body))
	if err != nil {
		t.Fatalf("compile: %v", err)
	}
	if !res.Forbidden() {
		t.Fatalf("expected validation error for dangling route")
	}
}
