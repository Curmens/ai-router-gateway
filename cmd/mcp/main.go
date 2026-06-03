package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/user1024/auto-router/internal/config"
	"github.com/user1024/auto-router/internal/graph"
)

// JSON-RPC 2.0 standard structs
type JSONRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type JSONRPCResponse struct {
	JSONRPC string `json:"jsonrpc"`
	ID      any    `json:"id"`
	Result  any    `json:"result,omitempty"`
	Error   *Error `json:"error,omitempty"`
}

type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

// MCP Initialize standard structs
type InitializeParams struct {
	ProtocolVersion string `json:"protocolVersion"`
}

type InitializeResult struct {
	ProtocolVersion string     `json:"protocolVersion"`
	Capabilities    ServerCaps `json:"capabilities"`
	ServerInfo      ServerInfo `json:"serverInfo"`
}

type ServerCaps struct {
	Tools map[string]any `json:"tools"`
}

type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// MCP Tools standard structs
type Tool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InputSchema Schema `json:"inputSchema"`
}

type Schema struct {
	Type       string         `json:"type"`
	Properties map[string]any `json:"properties"`
	Required   []string       `json:"required,omitempty"`
}

type ToolListResult struct {
	Tools []Tool `json:"tools"`
}

type ToolCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type ToolCallResult struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

type Content struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func main() {
	configPath := flag.String("config", "", "Path to config file")
	flag.Parse()

	// Direct all diagnostic outputs/logs to stderr to keep stdout strictly clean for JSON-RPC stdio
	fmt.Fprintln(os.Stderr, "Starting Auto-Router Go MCP Server...")

	cfg, err := config.LoadConfig(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: Failed to load config, using local defaults: %v\n", err)
		// Set minimal config defaults if missing
		cfg = &config.Config{}
		cfg.Server.Port = 8080
		cfg.Server.APIKeys = []config.APIKeyConfig{{Key: "sk-router-admin-12345"}}
	}

	port := cfg.Server.Port
	if port == 0 {
		port = 8080
	}

	apiKey := "sk-router-admin-12345"
	if len(cfg.Server.APIKeys) > 0 {
		apiKey = cfg.Server.APIKeys[0].Key
	}

	fmt.Fprintf(os.Stderr, "Go MCP server loaded API gateway: http://localhost:%d\n", port)

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}

		var req JSONRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			sendError(nil, -32700, "Parse error", err.Error())
			continue
		}

		handleRequest(req, port, apiKey)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Stdin scan error: %v\n", err)
	}
}

func handleRequest(req JSONRPCRequest, port int, apiKey string) {
	switch req.Method {
	case "initialize":
		res := InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: ServerCaps{
				Tools: make(map[string]any),
			},
			ServerInfo: ServerInfo{
				Name:    "auto-router-mcp",
				Version: "1.0.0",
			},
		}
		sendResult(req.ID, res)

	case "notifications/initialized":
		// Standard notifications have no response

	case "tools/list":
		tools := []Tool{
			{
				Name:        "execute_routed_chat",
				Description: "Prompt the gateway router to execute an LLM completions request. The prompt is dynamically analyzed by the classifier, routed to the best model, and telemetry logs are updated.",
				InputSchema: Schema{
					Type: "object",
					Properties: map[string]any{
						"prompt": map[string]any{
							"type":        "string",
							"description": "The prompt or instruction to send to the router",
						},
						"model": map[string]any{
							"type":        "string",
							"description": "Target model name (default is 'auto')",
							"enum":        []string{"auto", "claude-3-7-sonnet", "gemini-3.1-pro", "gpt-4o", "qwen3:latest"},
						},
					},
					Required: []string{"prompt"},
				},
			},
			{
				Name:        "get_router_metrics",
				Description: "Get live gateway usage telemetry summary (throughput, average latency, prompt/completion tokens, and total cost).",
				InputSchema: Schema{
					Type:       "object",
					Properties: make(map[string]any),
				},
			},
			{
				Name:        "list_active_providers",
				Description: "Retrieve current health status and average latencies for all registered upstream provider endpoints.",
				InputSchema: Schema{
					Type:       "object",
					Properties: make(map[string]any),
				},
			},
			{
				Name:        "get_recent_traces",
				Description: "Fetch the most recent request traces detailing the classifier decisions, complexity levels, routed targets, and costs.",
				InputSchema: Schema{
					Type: "object",
					Properties: map[string]any{
						"limit": map[string]any{
							"type":        "integer",
							"description": "Number of traces to pull (default is 5, max is 50)",
						},
					},
				},
			},
			{
				Name:        "query_codebase_graph",
				Description: "Search the codebase knowledge graph (nodes, labels, types, files, descriptions, and edges) using query keywords.",
				InputSchema: Schema{
					Type: "object",
					Properties: map[string]any{
						"query": map[string]any{
							"type":        "string",
							"description": "Query string or keywords to match against file names, symbols, descriptions, or file paths",
						},
					},
					Required: []string{"query"},
				},
			},
			{
				Name:        "find_impact_path",
				Description: "Perform pathfinding in the codebase knowledge graph from a source node to a target node to check dependency impact flows.",
				InputSchema: Schema{
					Type: "object",
					Properties: map[string]any{
						"start": map[string]any{
							"type":        "string",
							"description": "The unique ID/label of the source node (e.g., cmd_router_main)",
						},
						"end": map[string]any{
							"type":        "string",
							"description": "The unique ID/label of the target node (e.g., internal_db_db)",
						},
					},
					Required: []string{"start", "end"},
				},
			},
			{
				Name:        "refresh_codebase_graph",
				Description: "Trigger an incremental re-extraction of codebase AST structures and rebuild the local graph.json files.",
				InputSchema: Schema{
					Type:       "object",
					Properties: make(map[string]any),
				},
			},
		}
		sendResult(req.ID, ToolListResult{Tools: tools})

	case "tools/call":
		var params ToolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			sendError(req.ID, -32602, "Invalid params", err.Error())
			return
		}

		result, isErr := callTool(params.Name, params.Arguments, port, apiKey)
		sendResult(req.ID, ToolCallResult{
			Content: []Content{{Type: "text", Text: result}},
			IsError: isErr,
		})

	default:
		sendError(req.ID, -32601, "Method not found", "Unknown method: "+req.Method)
	}
}

func callTool(name string, argsRaw json.RawMessage, port int, apiKey string) (string, bool) {
	client := &http.Client{Timeout: 60 * time.Second}

	switch name {
	case "execute_routed_chat":
		var args struct {
			Prompt string `json:"prompt"`
			Model  string `json:"model"`
		}
		if err := json.Unmarshal(argsRaw, &args); err != nil {
			return fmt.Sprintf("Failed to parse arguments: %v", err), true
		}

		if args.Prompt == "" {
			return "Argument 'prompt' is required.", true
		}

		model := args.Model
		if model == "" {
			model = "auto"
		}

		// Execute chat completions request via HTTP
		url := fmt.Sprintf("http://localhost:%d/v1/chat/completions", port)
		body := map[string]any{
			"model": model,
			"messages": []map[string]string{
				{"role": "user", "content": args.Prompt},
			},
		}

		jsonBody, _ := json.Marshal(body)
		httpReq, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
		httpReq.Header.Set("Content-Type", "application/json")
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		fmt.Fprintf(os.Stderr, "Calling HTTP completions to port %d...\n", port)
		resp, err := client.Do(httpReq)
		if err != nil {
			return fmt.Sprintf("Gateway offline: Failed to connect to local auto-router on port %d. Please start the router server first!", port), true
		}
		defer resp.Body.Close()

		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Sprintf("API Gateway error (HTTP %d): %s", resp.StatusCode, string(respBytes)), true
		}

		var chatResp struct {
			Choices []struct {
				Message struct {
					Content string `json:"content"`
				} `json:"message"`
			} `json:"choices"`
		}
		_ = json.Unmarshal(respBytes, &chatResp)

		content := "No choices returned from LLM."
		if len(chatResp.Choices) > 0 {
			content = chatResp.Choices[0].Message.Content
		}

		// Retrieve latest trace stats
		traceInfo := ""
		traceUrl := fmt.Sprintf("http://localhost:%d/v1/logs?limit=1", port)
		traceReq, _ := http.NewRequest("GET", traceUrl, nil)
		traceReq.Header.Set("Authorization", "Bearer "+apiKey)
		
		if traceResp, traceErr := client.Do(traceReq); traceErr == nil {
			defer traceResp.Body.Close()
			if traceBytes, err := io.ReadAll(traceResp.Body); err == nil {
				var traces struct {
					Logs []map[string]any `json:"logs"`
				}
				if err := json.Unmarshal(traceBytes, &traces); err == nil && len(traces.Logs) > 0 {
					t := traces.Logs[0]
					traceInfo = fmt.Sprintf(
						"\n\n### ⚡ Gateway Trace Inspector:\n- **Provider:** %v\n- **Routed Model:** %v\n- **Classification Reason:** %v\n- **Latency:** %vms\n- **Cost:** $%v",
						t["ChosenProvider"], t["ChosenModel"], t["Reason"], t["LatencyMs"], t["Cost"],
					)
				}
			}
		}

		return content + traceInfo, false

	case "get_router_metrics":
		url := fmt.Sprintf("http://localhost:%d/v1/usage/logs?limit=1", port)
		httpReq, _ := http.NewRequest("GET", url, nil)
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(httpReq)
		if err != nil {
			return fmt.Sprintf("Gateway offline: Failed to connect to local auto-router on port %d.", port), true
		}
		defer resp.Body.Close()

		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Sprintf("API Gateway error (HTTP %d): %s", resp.StatusCode, string(respBytes)), true
		}

		var metricsResp struct {
			Summary struct {
				TotalPromptTokens     int64   `json:"total_prompt_tokens"`
				TotalCompletionTokens int64   `json:"total_completion_tokens"`
				TotalTokens           int64   `json:"total_tokens"`
				TotalCost             float64 `json:"total_cost"`
				RequestCount          int64   `json:"request_count"`
			} `json:"summary"`
		}
		_ = json.Unmarshal(respBytes, &metricsResp)
		s := metricsResp.Summary

		md := fmt.Sprintf(
			"### 📊 Go Router Live Telemetry Metrics:\n\n- **Total Requests Handled:** %d\n- **Total Cost:** $%f\n- **Total Tokens Consumed:** %d tokens\n  - **Prompt Tokens:** %d tokens\n  - **Completion Tokens:** %d tokens\n",
			s.RequestCount, s.TotalCost, s.TotalTokens, s.TotalPromptTokens, s.TotalCompletionTokens,
		)
		return md, false

	case "list_active_providers":
		url := fmt.Sprintf("http://localhost:%d/v1/providers", port)
		httpReq, _ := http.NewRequest("GET", url, nil)
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(httpReq)
		if err != nil {
			return fmt.Sprintf("Gateway offline: Failed to connect to local auto-router on port %d.", port), true
		}
		defer resp.Body.Close()

		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Sprintf("API Gateway error (HTTP %d): %s", resp.StatusCode, string(respBytes)), true
		}

		var providers []struct {
			Name       string `json:"name"`
			IsHealthy  bool   `json:"is_healthy"`
			AvgLatency int    `json:"avg_latency"`
		}
		_ = json.Unmarshal(respBytes, &providers)

		var sb strings.Builder
		sb.WriteString("### 🔌 Upstream LLM Providers & Endpoint Statuses:\n\n")
		sb.WriteString("| Provider | Status | Average Latency |\n")
		sb.WriteString("| :--- | :--- | :--- |\n")
		for _, p := range providers {
			status := "🟢 Healthy"
			if !p.IsHealthy {
				status = "🔴 Unhealthy"
			}
			sb.WriteString(fmt.Sprintf("| **%s** | %s | %dms |\n", strings.Title(p.Name), status, p.AvgLatency))
		}
		return sb.String(), false

	case "get_recent_traces":
		var args struct {
			Limit int `json:"limit"`
		}
		_ = json.Unmarshal(argsRaw, &args)
		limit := args.Limit
		if limit <= 0 {
			limit = 5
		}

		url := fmt.Sprintf("http://localhost:%d/v1/logs?limit=%d", port, limit)
		httpReq, _ := http.NewRequest("GET", url, nil)
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(httpReq)
		if err != nil {
			return fmt.Sprintf("Gateway offline: Failed to connect to local auto-router on port %d.", port), true
		}
		defer resp.Body.Close()

		respBytes, _ := io.ReadAll(resp.Body)
		if resp.StatusCode != http.StatusOK {
			return fmt.Sprintf("API Gateway error (HTTP %d): %s", resp.StatusCode, string(respBytes)), true
		}

		var traces struct {
			Logs []map[string]any `json:"logs"`
		}
		_ = json.Unmarshal(respBytes, &traces)

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("### ⚡ Recent Request Traces (Last %d calls):\n\n", len(traces.Logs)))
		sb.WriteString("| Trace ID | Target Route | Status | Latency | Cost | Complexity | Reason |\n")
		sb.WriteString("| :--- | :--- | :--- | :--- | :--- | :--- | :--- |\n")
		
		for _, t := range traces.Logs {
			id := "N/A"
			if t["RequestID"] != nil {
				idStr := fmt.Sprintf("%v", t["RequestID"])
				if len(idStr) > 10 {
					id = fmt.Sprintf("`%s`", idStr[:10])
				} else {
					id = fmt.Sprintf("`%s`", idStr)
				}
			}
			routingType := t["RoutingType"]
			if routingType == nil || routingType == "" {
				routingType = "explicit"
			}
			sb.WriteString(fmt.Sprintf(
				"| %s | **%v** / %v | %v | %vms | $%v | %v | %v |\n",
				id, t["ChosenProvider"], t["ChosenModel"], t["Status"], t["LatencyMs"], t["Cost"], routingType, t["Reason"],
			))
		}
		return sb.String(), false

	case "query_codebase_graph":
		var args struct {
			Query string `json:"query"`
		}
		if err := json.Unmarshal(argsRaw, &args); err != nil {
			return fmt.Sprintf("Failed to parse arguments: %v", err), true
		}
		if args.Query == "" {
			return "Argument 'query' is required.", true
		}

		graphPath := "graphify-out/graph.json"
		if config.ActiveConfig != nil && config.ActiveConfig.Routing.GraphPath != "" {
			graphPath = config.ActiveConfig.Routing.GraphPath
		}

		g, err := graph.LoadGraph(graphPath)
		if err != nil {
			return fmt.Sprintf("Failed to load codebase graph: %v. Please ensure graphify has run on this codebase.", err), true
		}

		ctxText := g.QueryContext(args.Query)
		if ctxText == "" {
			return fmt.Sprintf("No relevant nodes found in codebase graph matching query: %s", args.Query), false
		}
		return ctxText, false

	case "find_impact_path":
		var args struct {
			Start string `json:"start"`
			End   string `json:"end"`
		}
		if err := json.Unmarshal(argsRaw, &args); err != nil {
			return fmt.Sprintf("Failed to parse arguments: %v", err), true
		}
		if args.Start == "" || args.End == "" {
			return "Arguments 'start' and 'end' are required.", true
		}

		graphPath := "graphify-out/graph.json"
		if config.ActiveConfig != nil && config.ActiveConfig.Routing.GraphPath != "" {
			graphPath = config.ActiveConfig.Routing.GraphPath
		}

		g, err := graph.LoadGraph(graphPath)
		if err != nil {
			return fmt.Sprintf("Failed to load codebase graph: %v. Please ensure graphify has run on this codebase.", err), true
		}

		path, err := g.FindImpactPath(args.Start, args.End)
		if err != nil {
			return fmt.Sprintf("Could not trace path: %v", err), false
		}

		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("### 🔍 Codebase Impact Flow Path from %s to %s:\n\n", args.Start, args.End))
		for i, p := range path {
			if i > 0 {
				sb.WriteString(" ➔ ")
			}
			sb.WriteString(fmt.Sprintf("`%s`", p))
		}
		return sb.String(), false

	case "refresh_codebase_graph":
		fmt.Fprintln(os.Stderr, "Triggering codebase graph refresh...")
		binary := "graphify"
		if _, err := exec.LookPath(binary); err != nil {
			binary = "/home/user1024/.local/bin/graphify"
		}
		cmd := exec.Command(binary, "update", "/home/user1024/Projects/auto-router")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("Failed to refresh codebase graph: %v. Output:\n%s", err, string(output)), true
		}
		return fmt.Sprintf("Successfully refreshed codebase graph!\n\n%s", string(output)), false

	default:
		return "Unknown tool name: " + name, true
	}
}

func sendResult(id any, result any) {
	res := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
	send(res)
}

func sendError(id any, code int, message string, data any) {
	res := JSONRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &Error{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	send(res)
}

func send(resp any) {
	bytes, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "JSON marshal error: %v\n", err)
		return
	}
	// Stdio-based JSON-RPC sends every message as a single line ended with a newline character
	os.Stdout.Write(bytes)
	os.Stdout.Write([]byte("\n"))
}
