package graph

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

type GraphNode struct {
	ID             string `json:"id"`
	Label          string `json:"label,omitempty"`
	FileType       string `json:"file_type,omitempty"`
	SourceFile     string `json:"source_file,omitempty"`
	SourceLocation string `json:"source_location,omitempty"`
	Community      int    `json:"community,omitempty"`
	NormLabel      string `json:"norm_label,omitempty"`
	Description    string `json:"description,omitempty"`
}

type GraphLink struct {
	Source          string  `json:"source"`
	Target          string  `json:"target"`
	Relation        string  `json:"relation,omitempty"`
	Confidence      string  `json:"confidence,omitempty"`
	Weight          float64 `json:"weight,omitempty"`
	ConfidenceScore float64 `json:"confidence_score,omitempty"`
}

type CodebaseGraph struct {
	Directed   bool        `json:"directed"`
	Multigraph bool        `json:"multigraph"`
	Nodes      []GraphNode `json:"nodes"`
	Links      []GraphLink `json:"links"`
	FilePath   string      `json:"-"`
}

func LoadGraph(path string) (*CodebaseGraph, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read graph file at %s: %w", path, err)
	}

	var g CodebaseGraph
	if err := json.Unmarshal(data, &g); err != nil {
		return nil, fmt.Errorf("failed to parse graph JSON: %w", err)
	}

	g.FilePath = path
	return &g, nil
}

// QueryNodes searches nodes by ID or description containing any query terms.
func (g *CodebaseGraph) QueryNodes(query string) []GraphNode {
	words := strings.Fields(strings.ToLower(query))
	if len(words) == 0 {
		return nil
	}

	var results []GraphNode
	for _, node := range g.Nodes {
		idLower := strings.ToLower(node.ID)
		descLower := strings.ToLower(node.Description)
		labelLower := strings.ToLower(node.Label)
		sourceLower := strings.ToLower(node.SourceFile)

		matchCount := 0
		for _, w := range words {
			if len(w) < 3 {
				continue
			}
			if strings.Contains(idLower, w) || 
				strings.Contains(descLower, w) || 
				strings.Contains(labelLower, w) || 
				strings.Contains(sourceLower, w) {
				matchCount++
			}
		}

		if matchCount > 0 {
			results = append(results, node)
		}
	}
	return results
}

// FindImpactPath performs a BFS to find the shortest path from start to end.
func (g *CodebaseGraph) FindImpactPath(start, end string) ([]string, error) {
	// Build an adjacency list
	adj := make(map[string][]string)
	for _, link := range g.Links {
		adj[link.Source] = append(adj[link.Source], link.Target)
	}

	// Verify if both start and end exist
	startExists := false
	endExists := false
	for _, node := range g.Nodes {
		if node.ID == start {
			startExists = true
		}
		if node.ID == end {
			endExists = true
		}
	}
	if !startExists || !endExists {
		return nil, fmt.Errorf("start or end node not found in graph")
	}

	// BFS traversal
	queue := [][]string{{start}}
	visited := map[string]bool{start: true}

	for len(queue) > 0 {
		path := queue[0]
		queue = queue[1:]

		node := path[len(path)-1]
		if node == end {
			return path, nil
		}

		for _, neighbor := range adj[node] {
			if !visited[neighbor] {
				visited[neighbor] = true
				newPath := make([]string, len(path)+1)
				copy(newPath, path)
				newPath[len(path)] = neighbor
				queue = append(queue, newPath)
			}
		}
	}

	return nil, errors.New("no path found between the nodes")
}

// QueryContext builds a text representation of the matched nodes and their dependencies for LLM context.
func (g *CodebaseGraph) QueryContext(query string) string {
	matched := g.QueryNodes(query)
	if len(matched) == 0 {
		return ""
	}

	// Cap at top 15 results to prevent prompt bloat
	if len(matched) > 15 {
		matched = matched[:15]
	}

	var sb strings.Builder
	sb.WriteString("Codebase Graph Context (Relevant Entities & Dependencies):\n")
	for _, node := range matched {
		sb.WriteString(fmt.Sprintf("- Node: %s\n", node.ID))
		if node.Label != "" {
			sb.WriteString(fmt.Sprintf("  Label: %s\n", node.Label))
		}
		if node.FileType != "" {
			sb.WriteString(fmt.Sprintf("  Type: %s\n", node.FileType))
		}
		if node.SourceFile != "" {
			sb.WriteString(fmt.Sprintf("  File: %s:%s\n", node.SourceFile, node.SourceLocation))
		}
		if node.Description != "" {
			sb.WriteString(fmt.Sprintf("  Description: %s\n", node.Description))
		}

		// Find dependencies (outgoing links)
		var deps []string
		for _, link := range g.Links {
			if link.Source == node.ID {
				deps = append(deps, fmt.Sprintf("%s (%s)", link.Target, link.Relation))
			}
		}
		if len(deps) > 0 {
			sb.WriteString(fmt.Sprintf("  Outbound Edges: %s\n", strings.Join(deps, ", ")))
		}
	}
	return sb.String()
}
