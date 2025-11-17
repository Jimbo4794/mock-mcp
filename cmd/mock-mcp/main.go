package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/Jibmo4794/mock-mcp/internal/mcp"
)

func main() {
	var configPath string
	var testcasesDir string
	var githubSync *mcp.GitHubSync

	// Check if GitHub sync is enabled
	githubRepoURL := os.Getenv("GITHUB_REPO_URL")
	if githubRepoURL != "" {
		log.Printf("GitHub sync enabled. Syncing from: %s", githubRepoURL)
		syncedConfigPath, syncedTestcasesDir, sync, err := mcp.SyncFromGitHub(githubRepoURL)
		if err != nil {
			log.Fatalf("Failed to sync from GitHub: %v", err)
		}
		configPath = syncedConfigPath
		testcasesDir = syncedTestcasesDir
		githubSync = sync
		log.Printf("Synced config from GitHub: %s", configPath)
		log.Printf("Synced testcases from GitHub: %s", testcasesDir)
	} else {
		// Default config path, can be overridden via environment variable
		configPath = os.Getenv("TOOLS_CONFIG")
		if configPath == "" {
			// Default paths to try (works for both local dev and Docker)
			wd, _ := os.Getwd()
			possiblePaths := []string{
				"/app/config/tools.yaml",                  // Docker default
				filepath.Join(wd, "config", "tools.yaml"), // Local dev
				filepath.Join(wd, "..", "config", "tools.yaml"),
				filepath.Join(wd, "..", "..", "config", "tools.yaml"),
				"config/tools.yaml",
			}

			for _, path := range possiblePaths {
				if _, err := os.Stat(path); err == nil {
					configPath = path
					break
				}
			}

			// If still not found, use Docker default
			if configPath == "" {
				configPath = "/app/config/tools.yaml"
			}
		}
	}

	// Get webhook secret from environment variable (optional)
	webhookSecret := os.Getenv("GITHUB_WEBHOOK_SECRET")

	server, err := mcp.NewMockMCPServerWithWebhook(configPath, testcasesDir, githubSync, webhookSecret)
	if err != nil {
		log.Fatal("Failed to create server:", err)
	}
	defer server.Close()

	http.HandleFunc("/mcp", server.HandleRequest)
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
	http.HandleFunc("/testcase/builder", server.HandleTestCaseBuilder)
	http.HandleFunc("/api/testcase/save", server.HandleSaveTestCase)

	// Register webhook endpoint if GitHub sync is enabled
	if githubSync != nil {
		http.HandleFunc("/webhook/github", server.HandleWebhook)
	}

	port := ":8080"
	log.Printf("Starting Mock MCP Server on port %s", port)
	log.Printf("Watching config file: %s", configPath)
	log.Printf("Endpoints:")
	log.Printf("  POST /mcp - MCP protocol endpoint")
	log.Printf("  GET /mcp?stream=true - Streaming MCP endpoint")
	log.Printf("  WS /mcp - WebSocket MCP endpoint")
	log.Printf("  GET /health - Health check")
	log.Printf("  GET /testcase/builder - Test case builder UI")
	log.Printf("  POST /api/testcase/save - Save test case API")
	if githubSync != nil {
		log.Printf("  POST /webhook/github - GitHub webhook endpoint (for auto-sync)")
		if webhookSecret != "" {
			log.Printf("    Webhook signature verification: ENABLED")
		} else {
			log.Printf("    Webhook signature verification: DISABLED (set GITHUB_WEBHOOK_SECRET to enable)")
		}
	}
	log.Printf("")
	log.Printf("Edit %s to add/remove tools. Changes will be reloaded automatically.", configPath)

	if err := http.ListenAndServe(port, nil); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
