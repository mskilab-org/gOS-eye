package dag

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
)

// --- Unexported parser internals ---

// dotNode represents a raw DOT node declaration.
// ID is the DOT identifier (e.g. "v7").
// Label is the node label (e.g. "BWA_MEM" for processes, "" for operators).
// Shape is "point", "circle", or "" (default = process node).
type dotNode struct {
	ID    string
	Label string
	Shape string
}

// dotEdge represents a raw DOT directed edge declaration.
// From and To are DOT node IDs.
type dotEdge struct {
	From string
	To   string
}

// --- Exported types ---

// Edge represents a directed process-to-process dependency.
// From and To are process names (not DOT IDs).
type Edge struct {
	From string
	To   string
}

// DAG represents the full process-level dependency graph.
type DAG struct {
	// Processes lists process names in topological order (roots first).
	Processes []string
	// Edges contains deduplicated process-to-process edges.
	Edges []Edge
	// Children maps each process to its downstream process names.
	Children map[string][]string
	// Parents maps each process to its upstream process names.
	Parents map[string][]string
}

// NodeLayout holds the visual position for one process node.
type NodeLayout struct {
	// Name is the process name.
	Name string
	// Layer is the topological depth (0 = root processes with no dependencies).
	Layer int
	// Index is the horizontal position within the layer (0-based).
	Index int
}

// Layout holds all computed positions for rendering a DAG.
type Layout struct {
	// Nodes contains one NodeLayout per process, ordered by layer then index.
	Nodes []NodeLayout
	// Edges are the same edges as DAG, for drawing connectors.
	Edges []Edge
	// LayerCount is the total number of layers.
	LayerCount int
	// MaxWidth is the maximum number of nodes in any single layer.
	MaxWidth int
}

// --- Public API ---

// ParseDOT parses a Nextflow DOT file into a process-level dependency graph.
// It regex-parses node and edge declarations, identifies process nodes,
// BFS-resolves through operator nodes to find process-to-process edges,
// deduplicates, topologically sorts, and builds adjacency maps.
func ParseDOT(r io.Reader) (*DAG, error) {
	// 1. Parse raw DOT nodes and edges.
	nodes, dotEdges, err := parseDotFile(r)
	if err != nil {
		return nil, err
	}

	// 2. Filter process nodes; build processIDs and nodesByID maps.
	processIDs := make(map[string]string)   // DOT ID → label
	nodesByID := make(map[string]dotNode)    // DOT ID → dotNode
	var processNames []string               // unique names in discovery order
	seenName := make(map[string]bool)

	for _, n := range nodes {
		nodesByID[n.ID] = n
		if isProcessNode(n) {
			processIDs[n.ID] = n.Label
			if !seenName[n.Label] {
				seenName[n.Label] = true
				processNames = append(processNames, n.Label)
			}
		}
	}

	// 3. Resolve process-to-process edges through operator nodes.
	edges := resolveProcessEdges(processIDs, nodesByID, dotEdges)

	// 4. Deduplicate edges.
	edges = deduplicateEdges(edges)

	// 5. Topological sort.
	sorted, err := topoSort(processNames, edges)
	if err != nil {
		return nil, err
	}

	// 6. Build adjacency maps.
	children, parents := buildAdjacency(edges)

	// Ensure maps are non-nil even when there are no edges.
	if children == nil {
		children = make(map[string][]string)
	}
	if parents == nil {
		parents = make(map[string][]string)
	}

	return &DAG{
		Processes: sorted,
		Edges:     edges,
		Children:  children,
		Parents:   parents,
	}, nil
}

// ComputeLayout computes layered positions from DAG topology for rendering.
// Each process gets layer = max(parent layers) + 1 (roots get layer 0).
// Within each layer, processes are assigned index 0..N-1 maintaining topological order.
func ComputeLayout(dag *DAG) *Layout {
	layers := assignLayers(dag)

	// Group processes by layer, preserving topological order from dag.Processes.
	groups := make(map[int][]string)
	for _, proc := range dag.Processes {
		layer := layers[proc]
		groups[layer] = append(groups[layer], proc)
	}

	// Compute LayerCount and MaxWidth.
	layerCount := 0
	maxWidth := 0
	for layer, procs := range groups {
		if layer+1 > layerCount {
			layerCount = layer + 1
		}
		if len(procs) > maxWidth {
			maxWidth = len(procs)
		}
	}

	// Build nodes ordered by (layer, index).
	var nodes []NodeLayout
	for l := 0; l < layerCount; l++ {
		for idx, proc := range groups[l] {
			nodes = append(nodes, NodeLayout{
				Name:  proc,
				Layer: l,
				Index: idx,
			})
		}
	}

	// Copy edges from DAG.
	edges := make([]Edge, len(dag.Edges))
	copy(edges, dag.Edges)

	return &Layout{
		Nodes:      nodes,
		Edges:      edges,
		LayerCount: layerCount,
		MaxWidth:   maxWidth,
	}
}

// --- Internal helpers ---

// parseDotFile reads DOT-format input and returns all node declarations and edge declarations.
// Parses lines matching node pattern (vN [attrs]) and edge pattern (vN -> vN).
// Node pattern: `vN [label="...",shape=...]` or `vN [shape=point,label="",...,xlabel="..."]`
// Edge pattern: `vN -> vN` with optional `[label="..."]` suffix.
// Deduplicates nodes by ID (keeps last declaration seen for each ID).
func parseDotFile(r io.Reader) ([]dotNode, []dotEdge, error) {
	// Node pattern: vN [attrs]  — captures ID and the attribute string
	nodeRe := regexp.MustCompile(`^\s*(v\d+)\s+\[(.+)\]\s*;?\s*$`)
	// Edge pattern: vN -> vN with optional [label="..."] suffix
	edgeRe := regexp.MustCompile(`^\s*(v\d+)\s*->\s*(v\d+)`)
	// Attribute extractors
	labelRe := regexp.MustCompile(`\blabel="([^"]*)"`)
	shapeRe := regexp.MustCompile(`\bshape=(\w+)`)

	// Use ordered map: track insertion order + last-wins dedup
	nodeOrder := []string{}          // IDs in first-seen order
	nodeMap := map[string]dotNode{}  // ID → last declaration

	var edges []dotEdge

	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()

		if m := nodeRe.FindStringSubmatch(line); m != nil {
			id := m[1]
			attrs := m[2]

			var label, shape string
			if lm := labelRe.FindStringSubmatch(attrs); lm != nil {
				label = lm[1]
			}
			if sm := shapeRe.FindStringSubmatch(attrs); sm != nil {
				shape = sm[1]
			}

			if _, exists := nodeMap[id]; !exists {
				nodeOrder = append(nodeOrder, id)
			}
			nodeMap[id] = dotNode{ID: id, Label: label, Shape: shape}
			continue
		}

		if m := edgeRe.FindStringSubmatch(line); m != nil {
			edges = append(edges, dotEdge{From: m[1], To: m[2]})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, nil, err
	}

	// Build deduplicated node slice in first-seen order
	nodes := make([]dotNode, 0, len(nodeOrder))
	for _, id := range nodeOrder {
		nodes = append(nodes, nodeMap[id])
	}

	return nodes, edges, nil
}

// isProcessNode returns true if the dotNode represents a Nextflow process
// (non-empty Label AND Shape is not "point" or "circle").
func isProcessNode(n dotNode) bool {
	return n.Label != "" && n.Shape != "point" && n.Shape != "circle"
}

// resolveProcessEdges finds process-to-process edges by BFS through non-process
// (operator/channel) nodes. For each process node, follows outgoing edges
// through non-process nodes until reaching another process node, producing
// one Edge per reachable downstream process.
// processIDs maps DOT node ID → process name for process nodes only.
// nodesByID maps DOT node ID → dotNode for all nodes.
// dotEdges is the complete list of raw DOT edges.
func resolveProcessEdges(processIDs map[string]string, nodesByID map[string]dotNode, dotEdges []dotEdge) []Edge {
	// 1. Build adjacency list: node ID → list of target IDs
	adj := make(map[string][]string)
	for _, e := range dotEdges {
		adj[e.From] = append(adj[e.From], e.To)
	}

	// 2. BFS from each process node
	var result []Edge
	for sourceID, sourceName := range processIDs {
		// BFS queue starts with direct neighbors of the source process
		queue := adj[sourceID]
		visited := map[string]bool{sourceID: true}

		for i := 0; i < len(queue); i++ {
			nid := queue[i]
			if visited[nid] {
				continue
			}
			visited[nid] = true

			if targetName, isProcess := processIDs[nid]; isProcess {
				// Reached another process → emit edge, don't continue through it
				result = append(result, Edge{From: sourceName, To: targetName})
			} else {
				// Operator/channel node → continue BFS through it
				queue = append(queue, adj[nid]...)
			}
		}
	}

	return result
}

// deduplicateEdges removes duplicate edges, preserving order of first occurrence.
// Two edges are equal if both From and To match.
func deduplicateEdges(edges []Edge) []Edge {
	seen := make(map[string]bool)
	var result []Edge
	for _, e := range edges {
		key := e.From + "->" + e.To
		if !seen[key] {
			seen[key] = true
			result = append(result, e)
		}
	}
	return result
}

// topoSort returns process names in topological order (roots first) using Kahn's algorithm.
// Returns error if the graph contains a cycle.
func topoSort(processes []string, edges []Edge) ([]string, error) {
	// Build index map for stable ordering: process name → position in input slice
	posOf := make(map[string]int, len(processes))
	for i, p := range processes {
		posOf[p] = i
	}

	// Build in-degree map and adjacency list
	inDeg := make(map[string]int, len(processes))
	adj := make(map[string][]string, len(processes))
	for _, p := range processes {
		inDeg[p] = 0 // ensure all processes have entries
	}
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
		inDeg[e.To]++
	}

	// Seed queue with zero-in-degree nodes, sorted by input position
	queue := make([]string, 0, len(processes))
	for _, p := range processes {
		if inDeg[p] == 0 {
			queue = append(queue, p)
		}
	}

	result := make([]string, 0, len(processes))
	for len(queue) > 0 {
		// Pick the node with the smallest input position (stable ordering)
		bestIdx := 0
		for i := 1; i < len(queue); i++ {
			if posOf[queue[i]] < posOf[queue[bestIdx]] {
				bestIdx = i
			}
		}
		node := queue[bestIdx]
		queue[bestIdx] = queue[len(queue)-1]
		queue = queue[:len(queue)-1]

		result = append(result, node)

		for _, neighbor := range adj[node] {
			inDeg[neighbor]--
			if inDeg[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(result) != len(processes) {
		return nil, fmt.Errorf("cycle detected in process graph")
	}

	return result, nil
}

// buildAdjacency constructs Children and Parents adjacency maps from a set of edges.
// Children: process name → list of downstream process names.
// Parents: process name → list of upstream process names.
func buildAdjacency(edges []Edge) (children map[string][]string, parents map[string][]string) {
	children = make(map[string][]string)
	parents = make(map[string][]string)

	// Initialize every node mentioned in edges with an empty slice
	for _, e := range edges {
		if _, ok := children[e.From]; !ok {
			children[e.From] = []string{}
		}
		if _, ok := children[e.To]; !ok {
			children[e.To] = []string{}
		}
		if _, ok := parents[e.From]; !ok {
			parents[e.From] = []string{}
		}
		if _, ok := parents[e.To]; !ok {
			parents[e.To] = []string{}
		}
	}

	// Populate adjacency lists
	for _, e := range edges {
		children[e.From] = append(children[e.From], e.To)
		parents[e.To] = append(parents[e.To], e.From)
	}

	return children, parents
}

// assignLayers assigns a layer number to each process.
// Layer = max(parent layers) + 1; roots (no parents) get layer 0.
// Processes must be in topological order so parents are always assigned before children.
func assignLayers(dag *DAG) map[string]int {
	layers := make(map[string]int, len(dag.Processes))
	for _, proc := range dag.Processes {
		parents := dag.Parents[proc]
		if len(parents) == 0 {
			layers[proc] = 0
			continue
		}
		maxParent := 0
		for _, p := range parents {
			if layers[p] > maxParent {
				maxParent = layers[p]
			}
		}
		layers[proc] = maxParent + 1
	}
	return layers
}
