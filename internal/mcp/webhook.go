package mcp

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
)

// GitHubWebhookPayload represents a GitHub webhook payload
type GitHubWebhookPayload struct {
	Ref        string `json:"ref"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
	Commits []struct {
		ID       string   `json:"id"`
		Message  string   `json:"message"`
		Added    []string `json:"added"`
		Removed  []string `json:"removed"`
		Modified []string `json:"modified"`
	} `json:"commits"`
}

// WebhookHandler handles GitHub webhook events
type WebhookHandler struct {
	githubSync    *GitHubSync
	webhookSecret string
}

// NewWebhookHandler creates a new webhook handler
func NewWebhookHandler(githubSync *GitHubSync, webhookSecret string) *WebhookHandler {
	return &WebhookHandler{
		githubSync:    githubSync,
		webhookSecret: webhookSecret,
	}
}

// HandleWebhook processes incoming GitHub webhook requests
func (wh *WebhookHandler) HandleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read the payload first (needed for signature verification)
	payload, err := io.ReadAll(r.Body)
	if err != nil {
		log.Printf("Error reading webhook payload: %v", err)
		http.Error(w, "Failed to read payload", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Verify webhook signature if secret is configured
	if wh.webhookSecret != "" {
		if !wh.verifySignatureWithBody(r, payload) {
			log.Printf("Webhook signature verification failed")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Parse the webhook event type
	eventType := r.Header.Get("X-GitHub-Event")
	if eventType == "" {
		log.Printf("Missing X-GitHub-Event header")
		http.Error(w, "Missing event type", http.StatusBadRequest)
		return
	}

	log.Printf("Received GitHub webhook event: %s", eventType)

	// Handle push events
	if eventType == "push" {
		if err := wh.handlePushEvent(payload); err != nil {
			log.Printf("Error handling push event: %v", err)
			http.Error(w, fmt.Sprintf("Error processing webhook: %v", err), http.StatusInternalServerError)
			return
		}
	} else {
		log.Printf("Ignoring webhook event type: %s", eventType)
	}

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// handlePushEvent processes a GitHub push event
func (wh *WebhookHandler) handlePushEvent(payload []byte) error {
	var webhookPayload GitHubWebhookPayload
	if err := json.Unmarshal(payload, &webhookPayload); err != nil {
		return fmt.Errorf("failed to parse webhook payload: %w", err)
	}

	// Get configured paths from GitHubSync
	repoConfigPath := wh.githubSync.GetRepoConfigPath()
	repoTestcasesPath := wh.githubSync.GetRepoTestcasesPath()

	// Extract directory paths for prefix matching
	// For config path like "config/tools.yaml", check for "config/"
	// For testcases path like "testcases", check for "testcases/"
	var configDirPrefix string
	if idx := strings.LastIndex(repoConfigPath, "/"); idx != -1 {
		configDirPrefix = repoConfigPath[:idx+1] // Include trailing slash
	} else {
		// If config is at root (e.g., "tools.yaml"), check for exact match
		configDirPrefix = repoConfigPath
	}

	testcasesDirPrefix := repoTestcasesPath
	if !strings.HasSuffix(testcasesDirPrefix, "/") {
		testcasesDirPrefix = testcasesDirPrefix + "/"
	}

	// Check if config or testcases directories were modified
	shouldSync := false
	for _, commit := range webhookPayload.Commits {
		for _, file := range append(append(commit.Added, commit.Modified...), commit.Removed...) {
			// Check config path (either exact match for root files or prefix match for directories)
			configMatches := false
			if strings.Contains(repoConfigPath, "/") {
				configMatches = strings.HasPrefix(file, configDirPrefix)
			} else {
				configMatches = file == configDirPrefix
			}

			// Check testcases path (always prefix match since it's a directory)
			testcasesMatches := strings.HasPrefix(file, testcasesDirPrefix)

			if configMatches || testcasesMatches {
				shouldSync = true
				break
			}
		}
		if shouldSync {
			break
		}
	}

	// If no relevant files changed, skip sync
	if !shouldSync {
		log.Printf("No changes to %s or %s, skipping sync", configDirPrefix, testcasesDirPrefix)
		return nil
	}

	log.Printf("Changes detected in %s or %s, triggering sync...", configDirPrefix, testcasesDirPrefix)
	log.Printf("Repository: %s, Ref: %s", webhookPayload.Repository.FullName, webhookPayload.Ref)

	// Trigger sync
	if err := wh.githubSync.Sync(); err != nil {
		return fmt.Errorf("failed to sync repository: %w", err)
	}

	log.Printf("Repository synced successfully via webhook")
	return nil
}

// verifySignatureWithBody verifies the GitHub webhook signature with a pre-read body
func (wh *WebhookHandler) verifySignatureWithBody(r *http.Request, body []byte) bool {
	signature := r.Header.Get("X-Hub-Signature-256")
	if signature == "" {
		return false
	}

	// Calculate expected signature
	mac := hmac.New(sha256.New, []byte(wh.webhookSecret))
	mac.Write(body)
	expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	// Use constant-time comparison
	return hmac.Equal([]byte(signature), []byte(expectedSignature))
}
