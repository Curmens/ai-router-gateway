package graph

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadGraph(t *testing.T) {
	// Create a temp graph.json file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "graph.json")

	mockGraph := CodebaseGraph{
		Directed:   true,
		Multigraph: false,
		Nodes: []GraphNode{
			{ID: "node1", Label: "main.go", FileType: "code", Description: "Main application entry"},
			{ID: "node2", Label: "db.go", FileType: "code", Description: "Database connections"},
		},
		Links: []GraphLink{
			{Source: "node1", Target: "node2", Relation: "imports"},
		},
	}

	bytes, err := json.Marshal(mockGraph)
	if err != nil {
		t.Fatalf("failed to marshal mock graph: %v", err)
	}

	if err := os.WriteFile(tmpFile, bytes, 0644); err != nil {
		t.Fatalf("failed to write mock graph to file: %v", err)
	}

	// Test loading
	g, err := LoadGraph(tmpFile)
	if err != nil {
		t.Fatalf("failed to load graph: %v", err)
	}

	if !g.Directed {
		t.Errorf("expected Directed to be true")
	}
	if len(g.Nodes) != 2 {
		t.Errorf("expected 2 nodes, got %d", len(g.Nodes))
	}
	if len(g.Links) != 1 {
		t.Errorf("expected 1 link, got %d", len(g.Links))
	}
}

func TestQueryNodes(t *testing.T) {
	g := &CodebaseGraph{
		Nodes: []GraphNode{
			{ID: "internal_db_db", Label: "db.go", FileType: "code", Description: "Database interface"},
			{ID: "internal_logger_logger", Label: "logger.go", FileType: "code", Description: "Structured logger helper"},
			{ID: "internal_telemetry_telemetry", Label: "telemetry.go", FileType: "code", Description: "Prometheus stats endpoint"},
		},
	}

	// Match query "database" in description
	res := g.QueryNodes("database")
	if len(res) != 1 || res[0].ID != "internal_db_db" {
		t.Errorf("expected to match internal_db_db, got: %v", res)
	}

	// Match query "logger" in ID
	res = g.QueryNodes("logger")
	if len(res) != 1 || res[0].ID != "internal_logger_logger" {
		t.Errorf("expected to match internal_logger_logger, got: %v", res)
	}

	// Match multiple words
	res = g.QueryNodes("prometheus logger")
	if len(res) != 2 {
		t.Errorf("expected to match 2 nodes, got: %d", len(res))
	}

	// No match
	res = g.QueryNodes("something_unrelated")
	if len(res) != 0 {
		t.Errorf("expected 0 matches, got: %d", len(res))
	}
}

func TestFindImpactPath(t *testing.T) {
	g := &CodebaseGraph{
		Nodes: []GraphNode{
			{ID: "A"},
			{ID: "B"},
			{ID: "C"},
			{ID: "D"},
		},
		Links: []GraphLink{
			{Source: "A", Target: "B"},
			{Source: "B", Target: "C"},
			{Source: "C", Target: "D"},
			{Source: "A", Target: "C"},
		},
	}

	// Test direct connection/shortest path
	path, err := g.FindImpactPath("A", "D")
	if err != nil {
		t.Fatalf("expected path to exist: %v", err)
	}

	// Expected shortest path from A to D: A -> C -> D
	expected := []string{"A", "C", "D"}
	if len(path) != len(expected) {
		t.Fatalf("expected path length %d, got %d (path: %v)", len(expected), len(path), path)
	}
	for i, v := range path {
		if v != expected[i] {
			t.Errorf("at index %d expected %s, got %s", i, expected[i], v)
		}
	}

	// Test no connection path
	path, err = g.FindImpactPath("D", "A")
	if err == nil {
		t.Errorf("expected error for backward search, got path: %v", path)
	}
}

func TestQueryContext(t *testing.T) {
	g := &CodebaseGraph{
		Nodes: []GraphNode{
			{ID: "internal_db_db", Label: "db.go", FileType: "code", SourceFile: "internal/db/db.go", SourceLocation: "L1", Description: "Database configuration helper"},
		},
		Links: []GraphLink{
			{Source: "internal_db_db", Target: "database_sql", Relation: "imports"},
		},
	}

	ctxText := g.QueryContext("helper database")
	if !strings.Contains(ctxText, "Codebase Graph Context") {
		t.Errorf("expected context title, got: %s", ctxText)
	}
	if !strings.Contains(ctxText, "internal_db_db") {
		t.Errorf("expected node ID inside context, got: %s", ctxText)
	}
	if !strings.Contains(ctxText, "database_sql (imports)") {
		t.Errorf("expected outgoing link dependency inside context, got: %s", ctxText)
	}
}
