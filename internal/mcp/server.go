package mcp

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/gorilla/websocket"
)

// MockMCPServer handles HTTP requests and MCP protocol
type MockMCPServer struct {
	toolManager     *ToolManager
	testCaseManager *TestCaseManager
	upgrader        websocket.Upgrader
	webhookHandler  *WebhookHandler
}

// NewMockMCPServer creates a new MCP server instance
func NewMockMCPServer(configPath string) (*MockMCPServer, error) {
	return NewMockMCPServerWithTestcases(configPath, "")
}

// NewMockMCPServerWithTestcases creates a new MCP server instance with optional testcases directory
func NewMockMCPServerWithTestcases(configPath, testcasesDir string) (*MockMCPServer, error) {
	return NewMockMCPServerWithWebhook(configPath, testcasesDir, nil, "")
}

// NewMockMCPServerWithWebhook creates a new MCP server instance with optional webhook support
func NewMockMCPServerWithWebhook(configPath, testcasesDir string, githubSync *GitHubSync, webhookSecret string) (*MockMCPServer, error) {
	toolManager, err := NewToolManager(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create tool manager: %w", err)
	}

	testCaseManager := NewTestCaseManagerWithDir(configPath, testcasesDir)

	var webhookHandler *WebhookHandler
	if githubSync != nil {
		webhookHandler = NewWebhookHandler(githubSync, webhookSecret)
	}

	server := &MockMCPServer{
		toolManager:     toolManager,
		testCaseManager: testCaseManager,
		webhookHandler:  webhookHandler,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // Allow all origins for development
			},
		},
	}

	return server, nil
}

// HandleWebhook handles GitHub webhook requests
func (s *MockMCPServer) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if s.webhookHandler == nil {
		http.Error(w, "Webhook handler not configured", http.StatusNotImplemented)
		return
	}
	s.webhookHandler.HandleWebhook(w, r)
}

// Close closes the server and cleans up resources
func (s *MockMCPServer) Close() error {
	return s.toolManager.Close()
}

// HandleRequest handles incoming HTTP requests
func (s *MockMCPServer) HandleRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		s.handleHTTPRequest(w, r)
	} else if r.Header.Get("Upgrade") == "websocket" {
		s.handleWebSocketRequest(w, r)
	} else {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleHTTPRequest handles standard HTTP POST requests
func (s *MockMCPServer) handleHTTPRequest(w http.ResponseWriter, r *http.Request) {
	var req MCPRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.sendError(w, nil, -32700, "Parse error", err.Error())
		return
	}

	// Check if streaming is requested
	if r.Header.Get("Accept") == "text/event-stream" || r.URL.Query().Get("stream") == "true" {
		s.handleStreamingRequest(w, r, &req)
		return
	}

	response := s.processRequest(&req)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleStreamingRequest handles Server-Sent Events streaming requests
func (s *MockMCPServer) handleStreamingRequest(w http.ResponseWriter, r *http.Request, req *MCPRequest) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Send initial response
	response := s.processRequest(req)
	data, _ := json.Marshal(response)
	fmt.Fprintf(w, "data: %s\n\n", data)
	flusher.Flush()

	// If it's a tool call, stream progress updates
	if req.Method == "tools/call" {
		var toolCall ToolCall
		if err := json.Unmarshal(req.Params, &toolCall); err == nil {
			s.streamToolCallProgress(w, flusher, &toolCall)
		}
	}
}

// streamToolCallProgress sends progress updates for tool calls
func (s *MockMCPServer) streamToolCallProgress(w http.ResponseWriter, flusher http.Flusher, toolCall *ToolCall) {
	// Send progress updates
	for i := 0; i < 3; i++ {
		time.Sleep(500 * time.Millisecond)
		progress := map[string]interface{}{
			"type":    "progress",
			"message": fmt.Sprintf("Processing %s... (%d/3)", toolCall.Name, i+1),
		}
		data, _ := json.Marshal(progress)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

// handleWebSocketRequest handles WebSocket connections
func (s *MockMCPServer) handleWebSocketRequest(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}
	defer conn.Close()

	for {
		var req MCPRequest
		if err := conn.ReadJSON(&req); err != nil {
			log.Printf("WebSocket read error: %v", err)
			break
		}

		response := s.processRequest(&req)
		if err := conn.WriteJSON(response); err != nil {
			log.Printf("WebSocket write error: %v", err)
			break
		}
	}
}

// processRequest processes MCP protocol requests
func (s *MockMCPServer) processRequest(req *MCPRequest) *MCPResponse {
	switch req.Method {
	case "initialize":
		return s.handleInitialize(req)
	case "tools/list":
		return s.handleListTools(req)
	case "tools/call":
		return s.handleCallTool(req)
	default:
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Method not found",
			},
		}
	}
}

// handleInitialize handles the initialize MCP method
func (s *MockMCPServer) handleInitialize(req *MCPRequest) *MCPResponse {
	var params InitializeParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    err.Error(),
			},
		}
	}

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: InitializeResult{
			ProtocolVersion: "2024-11-05",
			Capabilities: map[string]interface{}{
				"tools": map[string]interface{}{
					"listChanged": true,
				},
			},
			ServerInfo: map[string]interface{}{
				"name":    "mock-mcp-server",
				"version": "1.0.0",
			},
		},
	}
}

// handleListTools handles the tools/list MCP method
func (s *MockMCPServer) handleListTools(req *MCPRequest) *MCPResponse {
	tools := s.toolManager.GetAllTools()

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result: map[string]interface{}{
			"tools": tools,
		},
	}
}

// handleCallTool handles the tools/call MCP method
func (s *MockMCPServer) handleCallTool(req *MCPRequest) *MCPResponse {
	var toolCall struct {
		Name      string                 `json:"name"`
		Arguments map[string]interface{} `json:"arguments,omitempty"`
	}

	if err := json.Unmarshal(req.Params, &toolCall); err != nil {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32602,
				Message: "Invalid params",
				Data:    err.Error(),
			},
		}
	}

	// Check if tool exists
	if _, exists := s.toolManager.GetTool(toolCall.Name); !exists {
		return &MCPResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error: &MCPError{
				Code:    -32601,
				Message: "Tool not found",
			},
		}
	}

	// Execute mock tool using test cases
	result := s.executeMockTool(toolCall.Name, toolCall.Arguments)

	return &MCPResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	}
}

// executeMockTool executes a tool by finding and returning a matching test case
func (s *MockMCPServer) executeMockTool(name string, args map[string]interface{}) ToolResult {
	// Get tool configuration to check default test case setting
	tool, exists := s.toolManager.GetTool(name)
	defaultTestCase := 0
	if exists {
		defaultTestCase = tool.DefaultTestCase
	}

	// Look for matching test case files
	testCase, err := s.testCaseManager.FindMatchingTestCase(name, args, defaultTestCase)
	if err != nil {
		log.Printf("Error finding test case for tool %s: %v", name, err)
		// Return a default response if no test case found
		return ToolResult{
			Content: []ContentBlock{
				{
					Type: "text",
					Text: fmt.Sprintf("No test case found for tool: %s with args: %v", name, args),
				},
			},
			IsError: true,
		}
	}

	return testCase.Response
}

// sendError sends an error response
func (s *MockMCPServer) sendError(w http.ResponseWriter, id interface{}, code int, message string, data interface{}) {
	response := &MCPResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error: &MCPError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(response)
}

// HandleTestCaseBuilder serves the test case builder HTML page
func (s *MockMCPServer) HandleTestCaseBuilder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	tools := s.toolManager.GetAllTools()
	toolsJSON, _ := json.Marshal(tools)

	// Find template file
	templatePath := s.findTemplatePath()
	if templatePath == "" {
		log.Printf("Error: Template file not found")
		http.Error(w, "Template file not found", http.StatusInternalServerError)
		return
	}

	// Read and parse template
	tmpl, err := template.ParseFiles(templatePath)
	if err != nil {
		log.Printf("Error parsing template: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Tools":     tools,
		"ToolsJSON": template.JS(toolsJSON),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := tmpl.Execute(w, data); err != nil {
		log.Printf("Error rendering test case builder: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
}

// findTemplatePath finds the template file using similar logic to config path resolution
func (s *MockMCPServer) findTemplatePath() string {
	wd, _ := os.Getwd()
	possiblePaths := []string{
		"/app/templates/testcase-builder.html",                  // Docker default
		filepath.Join(wd, "templates", "testcase-builder.html"), // Local dev
		filepath.Join(wd, "..", "templates", "testcase-builder.html"),
		filepath.Join(wd, "..", "..", "templates", "testcase-builder.html"),
		"templates/testcase-builder.html",
	}

	for _, path := range possiblePaths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	// If still not found, use Docker default
	return "/app/templates/testcase-builder.html"
}

// HandleSaveTestCase handles saving a test case via API
func (s *MockMCPServer) HandleSaveTestCase(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ToolName       string                 `json:"toolName"`
		TestCaseNumber int                    `json:"testCaseNumber"`
		Input          map[string]interface{} `json:"input"`
		Response       ToolResult             `json:"response"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
		return
	}

	// Validate tool exists
	if _, exists := s.toolManager.GetTool(req.ToolName); !exists {
		http.Error(w, fmt.Sprintf("Tool not found: %s", req.ToolName), http.StatusBadRequest)
		return
	}

	// Create test case config
	testCase := &TestCaseConfig{
		Input:    req.Input,
		Response: req.Response,
	}

	// Save test case
	if err := s.testCaseManager.SaveTestCase(req.ToolName, req.TestCaseNumber, testCase); err != nil {
		log.Printf("Error saving test case: %v", err)
		http.Error(w, fmt.Sprintf("Failed to save test case: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"message": fmt.Sprintf("Test case saved: %s-test-case-%d.yaml", req.ToolName, req.TestCaseNumber),
	})
}
