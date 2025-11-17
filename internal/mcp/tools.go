package mcp

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"gopkg.in/yaml.v3"
)

// ToolManager handles tool loading, configuration, and file watching
type ToolManager struct {
	tools      map[string]Tool
	toolsMutex sync.RWMutex
	configPath string
	watcher    *fsnotify.Watcher
}

// NewToolManager creates a new tool manager and loads tools from YAML
func NewToolManager(configPath string) (*ToolManager, error) {
	tm := &ToolManager{
		tools:      make(map[string]Tool),
		configPath: configPath,
	}

	// Load tools from YAML if file exists, otherwise use defaults
	if _, err := os.Stat(configPath); err == nil {
		if err := tm.loadToolsFromYAML(); err != nil {
			log.Printf("Warning: Failed to load tools from YAML: %v. Using defaults.", err)
			tm.registerDefaultTools()
		} else {
			log.Printf("Loaded tools from %s", configPath)
		}
	} else {
		log.Printf("Config file %s not found. Using default tools.", configPath)
		tm.registerDefaultTools()
		// Create example YAML file
		tm.createExampleYAML()
	}

	// Start watching the YAML file for changes
	if err := tm.startFileWatcher(); err != nil {
		log.Printf("Warning: Failed to start file watcher: %v", err)
	}

	return tm, nil
}

// GetTool retrieves a tool by name (thread-safe)
func (tm *ToolManager) GetTool(name string) (Tool, bool) {
	tm.toolsMutex.RLock()
	defer tm.toolsMutex.RUnlock()
	tool, exists := tm.tools[name]
	return tool, exists
}

// GetAllTools returns all registered tools (thread-safe)
func (tm *ToolManager) GetAllTools() []Tool {
	tm.toolsMutex.RLock()
	defer tm.toolsMutex.RUnlock()

	tools := make([]Tool, 0, len(tm.tools))
	for _, tool := range tm.tools {
		tools = append(tools, tool)
	}
	return tools
}

// loadToolsFromYAML loads tools from the YAML configuration file
func (tm *ToolManager) loadToolsFromYAML() error {
	data, err := os.ReadFile(tm.configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	var config ToolsConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}

	tm.toolsMutex.Lock()
	defer tm.toolsMutex.Unlock()

	// Clear existing tools
	tm.tools = make(map[string]Tool)

	// Load tools from YAML
	for _, toolConfig := range config.Tools {
		tool := Tool{
			Name:            toolConfig.Name,
			Description:     toolConfig.Description,
			InputSchema:     toolConfig.InputSchema,
			DefaultTestCase: toolConfig.DefaultTestCase,
		}
		tm.tools[toolConfig.Name] = tool
		log.Printf("Loaded tool: %s (defaultTestCase: %d)", toolConfig.Name, toolConfig.DefaultTestCase)
	}

	return nil
}

// createExampleYAML creates an example tools.yaml file
func (tm *ToolManager) createExampleYAML() {
	exampleConfig := ToolsConfig{
		Tools: []ToolConfig{
			{
				Name:        "mock_echo",
				Description: "Echoes back the input message",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"message": map[string]interface{}{
							"type":        "string",
							"description": "The message to echo",
						},
					},
					"required": []string{"message"},
				},
			},
			{
				Name:        "mock_calculator",
				Description: "Performs basic arithmetic operations",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"operation": map[string]interface{}{
							"type":        "string",
							"description": "The operation to perform (add, subtract, multiply, divide)",
							"enum":        []string{"add", "subtract", "multiply", "divide"},
						},
						"a": map[string]interface{}{
							"type":        "number",
							"description": "First number",
						},
						"b": map[string]interface{}{
							"type":        "number",
							"description": "Second number",
						},
					},
					"required": []string{"operation", "a", "b"},
				},
			},
			{
				Name:        "mock_delay",
				Description: "Simulates a delayed operation",
				InputSchema: map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"seconds": map[string]interface{}{
							"type":        "number",
							"description": "Number of seconds to delay",
						},
					},
					"required": []string{"seconds"},
				},
			},
		},
	}

	data, err := yaml.Marshal(&exampleConfig)
	if err != nil {
		log.Printf("Warning: Failed to create example YAML: %v", err)
		return
	}

	if err := os.WriteFile(tm.configPath, data, 0644); err != nil {
		log.Printf("Warning: Failed to write example YAML: %v", err)
		return
	}

	log.Printf("Created example configuration file: %s", tm.configPath)
}

// registerDefaultTools registers default mock tools
func (tm *ToolManager) registerDefaultTools() {
	tm.toolsMutex.Lock()
	defer tm.toolsMutex.Unlock()

	tm.tools["mock_echo"] = Tool{
		Name:        "mock_echo",
		Description: "Echoes back the input message",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"message": map[string]interface{}{
					"type":        "string",
					"description": "The message to echo",
				},
			},
			"required": []string{"message"},
		},
	}

	tm.tools["mock_calculator"] = Tool{
		Name:        "mock_calculator",
		Description: "Performs basic arithmetic operations",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"operation": map[string]interface{}{
					"type":        "string",
					"description": "The operation to perform (add, subtract, multiply, divide)",
					"enum":        []string{"add", "subtract", "multiply", "divide"},
				},
				"a": map[string]interface{}{
					"type":        "number",
					"description": "First number",
				},
				"b": map[string]interface{}{
					"type":        "number",
					"description": "Second number",
				},
			},
			"required": []string{"operation", "a", "b"},
		},
	}

	tm.tools["mock_delay"] = Tool{
		Name:        "mock_delay",
		Description: "Simulates a delayed operation",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"seconds": map[string]interface{}{
					"type":        "number",
					"description": "Number of seconds to delay",
				},
			},
			"required": []string{"seconds"},
		},
	}
}

// startFileWatcher starts watching the config file for changes
func (tm *ToolManager) startFileWatcher() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}

	tm.watcher = watcher

	// Watch the directory containing the config file
	configDir := filepath.Dir(tm.configPath)
	if err := watcher.Add(configDir); err != nil {
		watcher.Close()
		return err
	}

	go tm.watchFileChanges()

	return nil
}

// watchFileChanges monitors the config file for changes and reloads tools
func (tm *ToolManager) watchFileChanges() {
	for {
		select {
		case event, ok := <-tm.watcher.Events:
			if !ok {
				return
			}

			// Only reload on write/rename events for the config file
			if (event.Op&fsnotify.Write == fsnotify.Write || event.Op&fsnotify.Rename == fsnotify.Rename) &&
				event.Name == tm.configPath {
				log.Printf("Config file changed, reloading tools...")
				// Small delay to ensure file write is complete
				time.Sleep(100 * time.Millisecond)
				if err := tm.loadToolsFromYAML(); err != nil {
					log.Printf("Error reloading tools: %v", err)
				} else {
					log.Printf("Tools reloaded successfully")
				}
			}

		case err, ok := <-tm.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("File watcher error: %v", err)
		}
	}
}

// Close closes the file watcher
func (tm *ToolManager) Close() error {
	if tm.watcher != nil {
		return tm.watcher.Close()
	}
	return nil
}
