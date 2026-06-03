package graph

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestProjectResolver(t *testing.T) {
	// Create mock project structures
	tmpRoot := t.TempDir()
	projAPath := filepath.Join(tmpRoot, "projectA")
	projBPath := filepath.Join(tmpRoot, "projectB")

	err := os.MkdirAll(filepath.Join(projAPath, "graphify-out"), 0755)
	if err != nil {
		t.Fatalf("failed to create projectA dir: %v", err)
	}
	err = os.MkdirAll(filepath.Join(projBPath, "graphify-out"), 0755)
	if err != nil {
		t.Fatalf("failed to create projectB dir: %v", err)
	}

	graphA := CodebaseGraph{
		Directed:   true,
		Multigraph: false,
		Nodes: []GraphNode{
			{ID: "db_go", Label: "db.go", FileType: "code", SourceFile: "db.go", Description: "Database driver"},
			{ID: "db_getuser", Label: "GetUser()", FileType: "code", SourceFile: "db.go", Description: "Retrieve user from database"},
		},
	}
	graphB := CodebaseGraph{
		Directed:   true,
		Multigraph: false,
		Nodes: []GraphNode{
			{ID: "server_go", Label: "server.go", FileType: "code", SourceFile: "server.go", Description: "API Server"},
			{ID: "server_start", Label: "StartServer()", FileType: "code", SourceFile: "server.go", Description: "Launch server listener"},
		},
	}

	bytesA, _ := json.Marshal(graphA)
	bytesB, _ := json.Marshal(graphB)

	_ = os.WriteFile(filepath.Join(projAPath, "graphify-out", "graph.json"), bytesA, 0644)
	_ = os.WriteFile(filepath.Join(projBPath, "graphify-out", "graph.json"), bytesB, 0644)

	// Initialize Resolver
	defaultPath := filepath.Join(tmpRoot, "defaultProj")
	InitResolver(defaultPath)

	// Test scanning
	err = ActiveResolver.ScanProjects(tmpRoot)
	if err != nil {
		t.Fatalf("ScanProjects failed: %v", err)
	}

	if len(ActiveResolver.Projects) != 2 {
		t.Errorf("expected 2 projects, got %d", len(ActiveResolver.Projects))
	}

	// Test project resolution based on file/symbol keywords
	// 1. Match project A
	path, g := ActiveResolver.Resolve("Update database parameters inside db.go or GetUser method")
	if path != projAPath {
		t.Errorf("expected resolution to projectA (%s), got: %s", projAPath, path)
	}
	if g == nil || len(g.Nodes) != 2 {
		t.Errorf("expected valid graph with 2 nodes, got: %v", g)
	}

	// 2. Match project B
	path, g = ActiveResolver.Resolve("StartServer has some port conflict errors in server.go")
	if path != projBPath {
		t.Errorf("expected resolution to projectB (%s), got: %s", projBPath, path)
	}

	// 3. Fallback to default
	path, _ = ActiveResolver.Resolve("Create a generic template for a new python app")
	if path != defaultPath {
		t.Errorf("expected fallback path %s, got: %s", defaultPath, path)
	}
}

func TestContextHelpers(t *testing.T) {
	ctx := context.Background()
	
	// Test empty path retrieval
	if path := GetProjectPath(ctx); path != "" {
		t.Errorf("expected empty path from empty context, got: %s", path)
	}

	// Test setting and getting project path
	ctx = WithProjectPath(ctx, "/path/to/project")
	if path := GetProjectPath(ctx); path != "/path/to/project" {
		t.Errorf("expected /path/to/project, got: %s", path)
	}
}
