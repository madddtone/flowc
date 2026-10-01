// Package layout assigns deterministic positions to graph nodes.
//
// It uses a layered (Sugiyama-style) approach: DFS back-edge detection,
// longest-path ranking over the resulting DAG, optional actor swimlanes,
// then cell-based placement. Output is stable for a given input so the
// canvas never reflows between compiles.
package layout

import (
	"unicode/utf8"

	"github.com/madddtone/flowc/internal/graph"
)

const (
	marginX = 90.0
	marginY = 90.0
	hGap    = 100.0
	vGap    = 46.0
	laneGap = 40.0
)

var typeHeight = map[string]float64{
	graph.TypeStart:    54,
	graph.TypeEnd:      54,
	graph.TypeStep:     66,
	graph.TypeDecision: 88,
	graph.TypeSubflow:  76,
	graph.TypeExternal: 66,
	graph.TypeParallel: 52,
	graph.TypeJoin:     52,
}

// Layout computes positions for every node and marks back-edges on f.
func Layout(f *graph.Flow, start string) {
	if len(f.Nodes) == 0 {
		return
	}
	order := map[string]int{}
	for i, n := range f.Nodes {
		order[n.ID] = i
	}

	markBackEdges(f, start, order)

	rank := computeRanks(f, order)

	// Assign lanes.
	laneOf := map[string]int{}
	actorLane := map[string]int{}
	for i, a := range f.Actors {
		actorLane[a] = i
	}
	for _, n := range f.Nodes {
		l := 0
		if n.Actor != "" {
			if li, ok := actorLane[n.Actor]; ok {
				l = li
			}
		}
		laneOf[n.ID] = l
	}

	// Size nodes.
	for _, n := range f.Nodes {
		n.Rank = rank[n.ID]
		n.Lane = laneOf[n.ID]
		n.Rect.W = nodeWidth(n)
		n.Rect.H = nodeHeight(n)
	}

	// Column widths by rank.
	maxRank := 0
	for _, n := range f.Nodes {
		if n.Rank > maxRank {
			maxRank = n.Rank
		}
	}
	colWidth := make([]float64, maxRank+1)
	for _, n := range f.Nodes {
		if n.Rect.W > colWidth[n.Rank] {
			colWidth[n.Rank] = n.Rect.W
		}
	}
	colX := make([]float64, maxRank+1)
	colX[0] = marginX
	for r := 1; r <= maxRank; r++ {
		colX[r] = colX[r-1] + colWidth[r-1] + hGap
	}

	// Group nodes into (rank, lane) cells preserving author order.
	type cellKey struct{ rank, lane int }
	cells := map[cellKey][]*graph.Node{}
	var cellKeys []cellKey
	for _, n := range f.Nodes {
		k := cellKey{n.Rank, n.Lane}
		if _, ok := cells[k]; !ok {
			cellKeys = append(cellKeys, k)
		}
		cells[k] = append(cells[k], n)
	}

	// Band height per lane: max stacked height in any single cell.
	nLanes := 1
	for _, l := range laneOf {
		if l+1 > nLanes {
			nLanes = l + 1
		}
	}
	bandHeight := make([]float64, nLanes)
	for k, list := range cells {
		total := 0.0
		for i, n := range list {
			if i > 0 {
				total += vGap
			}
			total += n.Rect.H
		}
		if total > bandHeight[k.lane] {
			bandHeight[k.lane] = total
		}
	}
	laneBase := make([]float64, nLanes)
	for l := 1; l < nLanes; l++ {
		laneBase[l] = laneBase[l-1] + bandHeight[l-1] + laneGap
	}

	// Place. Cells are processed per rank then lane so cursors stay scoped.
	cursor := map[cellKey]float64{}
	for r := 0; r <= maxRank; r++ {
		for l := 0; l < nLanes; l++ {
			k := cellKey{r, l}
			list := cells[k]
			if list == nil {
				continue
			}
			for _, n := range list {
				n.Rect.X = colX[r] + (colWidth[r]-n.Rect.W)/2
				n.Rect.Y = marginY + laneBase[l] + cursor[k]
				cursor[k] += n.Rect.H + vGap
			}
		}
	}
}

func nodeWidth(n *graph.Node) float64 {
	label := n.Title
	if label == "" {
		label = n.ID
	}
	w := 40 + float64(utf8.RuneCountInString(label))*7.5
	if w < 150 {
		w = 150
	}
	if w > 300 {
		w = 300
	}
	return w
}

func nodeHeight(n *graph.Node) float64 {
	if h, ok := typeHeight[n.Type]; ok {
		return h
	}
	return 66
}

func markBackEdges(f *graph.Flow, start string, order map[string]int) {
	type adjEdge struct {
		idx int
		to  string
	}
	adj := map[string][]adjEdge{}
	for i := range f.Edges {
		adj[f.Edges[i].From] = append(adj[f.Edges[i].From], adjEdge{i, f.Edges[i].To})
	}
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := map[string]int{}
	var dfs func(u string)
	dfs = func(u string) {
		color[u] = gray
		for _, e := range adj[u] {
			switch color[e.to] {
			case gray:
				f.Edges[e.idx].Back = true
			case white:
				dfs(e.to)
			}
		}
		color[u] = black
	}
	// Start first for stable classification, then any remaining node.
	if start != "" {
		dfs(start)
	}
	for _, n := range f.Nodes {
		if color[n.ID] == white {
			dfs(n.ID)
		}
	}
}

func computeRanks(f *graph.Flow, order map[string]int) map[string]int {
	rank := map[string]int{}
	indeg := map[string]int{}
	adj := map[string][]string{}
	for _, n := range f.Nodes {
		indeg[n.ID] = 0
	}
	for _, e := range f.Edges {
		if e.Back {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
		indeg[e.To]++
	}
	var queue []string
	for _, n := range f.Nodes {
		if indeg[n.ID] == 0 {
			queue = append(queue, n.ID)
		}
	}
	processed := 0
	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		processed++
		for _, v := range adj[u] {
			if rank[u]+1 > rank[v] {
				rank[v] = rank[u] + 1
			}
			indeg[v]--
			if indeg[v] == 0 {
				queue = append(queue, v)
			}
		}
	}
	// Residual nodes (should be none after back-edge removal) get rank 0.
	_ = order
	return rank
}
