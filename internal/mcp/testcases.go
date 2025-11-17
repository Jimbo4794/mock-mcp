package mcp

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// TestCaseManager handles loading and matching test cases
type TestCaseManager struct {
	testCasesDir string
}

// NewTestCaseManager creates a new test case manager
func NewTestCaseManager(configPath string) *TestCaseManager {
	return NewTestCaseManagerWithDir(configPath, "")
}

// NewTestCaseManagerWithDir creates a new test case manager with optional testcases directory
func NewTestCaseManagerWithDir(configPath, testcasesDir string) *TestCaseManager {
	// If testcasesDir is provided, use it directly
	if testcasesDir != "" {
		return &TestCaseManager{
			testCasesDir: testcasesDir,
		}
	}

	// Determine test cases directory based on config path location
	configDir := filepath.Dir(configPath)
	if configDir == "" || configDir == "." {
		configDir, _ = os.Getwd()
	}

	// Use testcases/ directory at the same level as config directory
	// e.g., if config is at /app/config/tools.yaml, testcases should be at /app/testcases
	// If config is at ./config/tools.yaml, testcases should be at ./testcases
	parentDir := filepath.Dir(configDir)
	testCasesDir := filepath.Join(parentDir, "testcases")

	// Fallback: if parent/testcases doesn't exist, try config/testcases (for local dev)
	if _, err := os.Stat(testCasesDir); os.IsNotExist(err) {
		fallbackDir := filepath.Join(configDir, "testcases")
		if _, err := os.Stat(fallbackDir); err == nil {
			testCasesDir = fallbackDir
		}
	}

	return &TestCaseManager{
		testCasesDir: testCasesDir,
	}
}

// FindMatchingTestCase finds a test case that matches the given tool name and arguments
// defaultTestCase: 0 = no default, 1+ = use test-case-N as default if no match found
func (tcm *TestCaseManager) FindMatchingTestCase(toolName string, args map[string]interface{}, defaultTestCase int) (*TestCaseConfig, error) {
	log.Printf("Finding test case for tool: %s with args: %v (searching in: %s, defaultTestCase: %d)", toolName, args, tcm.testCasesDir, defaultTestCase)

	// Try test cases in order (1, 2, 3, ...) up to a reasonable limit
	for i := 1; i <= 100; i++ {
		testCaseFile := filepath.Join(tcm.testCasesDir, fmt.Sprintf("%s-test-case-%d.yaml", toolName, i))

		// Check if file exists
		if _, err := os.Stat(testCaseFile); os.IsNotExist(err) {
			continue
		}

		// Load test case
		testCase, err := tcm.loadTestCase(testCaseFile)
		if err != nil {
			log.Printf("Error loading test case %s: %v", testCaseFile, err)
			continue
		}

		// Check if input arguments match
		if tcm.matchArguments(testCase.Input, args) {
			log.Printf("Matched test case: %s", testCaseFile)
			return testCase, nil
		} else {
			log.Printf("Test case %s did not match. Expected: %v, Got: %v", testCaseFile, testCase.Input, args)
		}
	}

	// If no match found and defaultTestCase is configured, use the specified default
	if defaultTestCase > 0 {
		defaultFile := filepath.Join(tcm.testCasesDir, fmt.Sprintf("%s-test-case-%d.yaml", toolName, defaultTestCase))
		if _, err := os.Stat(defaultFile); err == nil {
			testCase, err := tcm.loadTestCase(defaultFile)
			if err == nil {
				log.Printf("Using configured default test case (%d): %s", defaultTestCase, defaultFile)
				return testCase, nil
			}
		} else {
			log.Printf("Configured default test case %d not found: %s", defaultTestCase, defaultFile)
		}
	}

	return nil, fmt.Errorf("no matching test case found")
}

// loadTestCase loads a test case from a YAML file
func (tcm *TestCaseManager) loadTestCase(filePath string) (*TestCaseConfig, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read test case file: %w", err)
	}

	var testCase TestCaseConfig
	if err := yaml.Unmarshal(data, &testCase); err != nil {
		return nil, fmt.Errorf("failed to parse test case YAML: %w", err)
	}

	return &testCase, nil
}

// matchArguments checks if the expected arguments match the actual arguments
func (tcm *TestCaseManager) matchArguments(expected map[string]interface{}, actual map[string]interface{}) bool {
	// If expected is empty, match any input
	if len(expected) == 0 {
		return true
	}

	// Check if all expected keys exist in actual and match
	for key, expectedValue := range expected {
		actualValue, exists := actual[key]
		if !exists {
			// If the key doesn't exist in actual, it's a mismatch
			return false
		}

		// Compare values (handle type conversions for numbers)
		if !tcm.valuesMatch(expectedValue, actualValue) {
			return false
		}
	}

	return true
}

// valuesMatch compares two values, handling type conversions
func (tcm *TestCaseManager) valuesMatch(expected, actual interface{}) bool {
	// Convert both to float64 for numeric comparison
	expectedFloat, expectedIsNum := tcm.toFloat64(expected)
	actualFloat, actualIsNum := tcm.toFloat64(actual)

	if expectedIsNum && actualIsNum {
		return expectedFloat == actualFloat
	}

	// Handle string comparisons
	if expectedStr, ok := expected.(string); ok {
		if actualStr, ok := actual.(string); ok {
			return expectedStr == actualStr
		}
	}

	// Handle bool comparisons
	if expectedBool, ok := expected.(bool); ok {
		if actualBool, ok := actual.(bool); ok {
			return expectedBool == actualBool
		}
	}

	// Default: use == comparison
	return expected == actual
}

// toFloat64 converts numeric types to float64 for comparison
func (tcm *TestCaseManager) toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int8:
		return float64(val), true
	case int16:
		return float64(val), true
	case int32:
		return float64(val), true
	case int64:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint8:
		return float64(val), true
	case uint16:
		return float64(val), true
	case uint32:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}

// SaveTestCase saves a test case to a YAML file
func (tcm *TestCaseManager) SaveTestCase(toolName string, testCaseNumber int, testCase *TestCaseConfig) error {
	// Ensure testcases directory exists
	if err := os.MkdirAll(tcm.testCasesDir, 0755); err != nil {
		return fmt.Errorf("failed to create testcases directory: %w", err)
	}

	// Generate filename
	filename := filepath.Join(tcm.testCasesDir, fmt.Sprintf("%s-test-case-%d.yaml", toolName, testCaseNumber))

	// Marshal to YAML
	data, err := yaml.Marshal(testCase)
	if err != nil {
		return fmt.Errorf("failed to marshal test case: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write test case file: %w", err)
	}

	return nil
}

// GetTestCasesDir returns the test cases directory path
func (tcm *TestCaseManager) GetTestCasesDir() string {
	return tcm.testCasesDir
}
