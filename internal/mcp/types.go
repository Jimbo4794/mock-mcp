package mcp

import "encoding/json"

// MCP Protocol Types
type MCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type MCPResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *MCPError   `json:"error,omitempty"`
}

type MCPError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Tool Types
type Tool struct {
	Name            string      `json:"name"`
	Description     string      `json:"description"`
	InputSchema     interface{} `json:"inputSchema"`
	DefaultTestCase int         `json:"defaultTestCase,omitempty"` // 0 = no default, 1+ = use test-case-N as default
}

type ToolCall struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

type ToolResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

type ContentBlock struct {
	Type string      `json:"type"`
	Text string      `json:"text,omitempty"`
	JSON interface{} `json:"json,omitempty" yaml:"json,omitempty"`
}

// Initialize Types
type InitializeParams struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ClientInfo      map[string]interface{} `json:"clientInfo"`
}

type InitializeResult struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    map[string]interface{} `json:"capabilities"`
	ServerInfo      map[string]interface{} `json:"serverInfo"`
}

// YAML Configuration Types
type ToolConfig struct {
	Name            string                 `yaml:"name"`
	Description     string                 `yaml:"description"`
	InputSchema     map[string]interface{} `yaml:"inputSchema"`
	Handler         string                 `yaml:"handler,omitempty"`         // Optional: custom handler type
	DefaultTestCase int                    `yaml:"defaultTestCase,omitempty"` // 0 = no default, 1+ = use test-case-N as default
}

type ToolsConfig struct {
	Tools []ToolConfig `yaml:"tools"`
}

// Test Case Configuration
type TestCaseConfig struct {
	Input    map[string]interface{} `yaml:"input"`
	Response ToolResult             `yaml:"response"`
}
