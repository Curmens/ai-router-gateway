package graph

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/user1024/auto-router/internal/logger"
	"go.uber.org/zap"
)

type contextKey string

const ProjectPathKey contextKey = "project_path"

func WithProjectPath(ctx context.Context, path string) context.Context {
	return context.WithValue(ctx, ProjectPathKey, path)
}

func GetProjectPath(ctx context.Context) string {
	if val, ok := ctx.Value(ProjectPathKey).(string); ok {
		return val
	}
	return ""
}

type ProjectInfo struct {
	Path       string `json:"path"`
	NodesCount int    `json:"nodes_count"`
	EdgesCount int    `json:"edges_count"`
}

type ProjectResolver struct {
	mu           sync.RWMutex
	Projects     map[string]*CodebaseGraph
	symbolToProj map[string]string
	DefaultPath  string
}

var ActiveResolver *ProjectResolver

func InitResolver(defaultPath string) {
	ActiveResolver = &ProjectResolver{
		Projects:     make(map[string]*CodebaseGraph),
		symbolToProj: make(map[string]string),
		DefaultPath:  defaultPath,
	}
}

// ScanProjects scans the root directory up to depth 2 looking for graphify-out/graph.json
func (pr *ProjectResolver) ScanProjects(rootPath string) error {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	logger.Log.Info("Scanning projects root path for graphify codebases", zap.String("root", rootPath))

	// Ensure rootPath is absolute
	absRoot, err := filepath.Abs(rootPath)
	if err != nil {
		absRoot = rootPath
	}

	entries, err := os.ReadDir(absRoot)
	if err != nil {
		return fmt.Errorf("failed to read projects root directory: %w", err)
	}

	foundProjects := make(map[string]*CodebaseGraph)

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		projPath := filepath.Join(absRoot, entry.Name())
		graphFile := filepath.Join(projPath, "graphify-out", "graph.json")

		// Check if graph.json exists
		if _, err := os.Stat(graphFile); err == nil {
			g, err := LoadGraph(graphFile)
			if err == nil {
				foundProjects[projPath] = g
				logger.Log.Info("Registered project codebase graph", zap.String("path", projPath), zap.Int("nodes", len(g.Nodes)))
			} else {
				logger.Log.Warn("Failed to load project graph", zap.String("path", graphFile), zap.Error(err))
			}
		}

		// Also check subdirectories up to depth 2 (e.g. root/group/project)
		subEntries, err := os.ReadDir(projPath)
		if err == nil {
			for _, subEntry := range subEntries {
				if !subEntry.IsDir() {
					continue
				}
				subProjPath := filepath.Join(projPath, subEntry.Name())
				subGraphFile := filepath.Join(subProjPath, "graphify-out", "graph.json")
				if _, err := os.Stat(subGraphFile); err == nil {
					g, err := LoadGraph(subGraphFile)
					if err == nil {
						foundProjects[subProjPath] = g
						logger.Log.Info("Registered project codebase graph", zap.String("path", subProjPath), zap.Int("nodes", len(g.Nodes)))
					}
				}
			}
		}
	}

	pr.Projects = foundProjects
	pr.rebuildIndexLocked()

	return nil
}

// rebuildIndexLocked compiles the symbol-to-project lookup map
func (pr *ProjectResolver) rebuildIndexLocked() {
	pr.symbolToProj = make(map[string]string)
	for path, g := range pr.Projects {
		for _, node := range g.Nodes {
			// Index normalized lowercased node labels and filenames
			if node.Label != "" {
				label := strings.ToLower(node.Label)
				// Remove common characters to clean up match targets
				label = strings.TrimSuffix(label, "()")
				pr.symbolToProj[label] = path
			}
			if node.SourceFile != "" {
				base := strings.ToLower(filepath.Base(node.SourceFile))
				pr.symbolToProj[base] = path
			}
		}
	}
	logger.Log.Info("Rebuilt project symbol index", zap.Int("indexed_symbols", len(pr.symbolToProj)))
}

// Resolve analyzes a prompt and returns the best matched project path and graph
func (pr *ProjectResolver) Resolve(prompt string) (string, *CodebaseGraph) {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	if len(pr.Projects) == 0 {
		return pr.DefaultPath, nil
	}

	words := strings.Fields(strings.ToLower(prompt))

	// Count occurrences of matching symbols per project
	counts := make(map[string]int)
	for _, w := range words {
		// Clean punctuation
		w = strings.Trim(w, `.,:;?!"'()[]{}`)
		if len(w) < 3 {
			continue
		}

		if path, ok := pr.symbolToProj[w]; ok {
			counts[path]++
		}
	}

	bestPath := ""
	maxCount := 0
	for path, count := range counts {
		if count > maxCount {
			maxCount = count
			bestPath = path
		}
	}

	if bestPath != "" && maxCount >= 1 {
		logger.Log.Info("Resolved project context automatically", zap.String("project", bestPath), zap.Int("match_score", maxCount))
		return bestPath, pr.Projects[bestPath]
	}

	// Fallback to default
	logger.Log.Debug("No unique project context matched, using default fallback path", zap.String("default", pr.DefaultPath))
	return pr.DefaultPath, pr.Projects[pr.DefaultPath]
}

// GetScannedProjects returns a summary list of all registered projects
func (pr *ProjectResolver) GetScannedProjects() []ProjectInfo {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	var list []ProjectInfo
	for path, g := range pr.Projects {
		list = append(list, ProjectInfo{
			Path:       path,
			NodesCount: len(g.Nodes),
			EdgesCount: len(g.Links),
		})
	}
	return list
}
