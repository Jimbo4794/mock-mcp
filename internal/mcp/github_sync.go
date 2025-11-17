package mcp

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// GitHubSync handles syncing config and testcases from a GitHub repository
type GitHubSync struct {
	repoURL           string
	cacheDir          string
	configDir         string
	testcasesDir      string
	repoConfigPath    string // Path to tools.yaml relative to repo root (e.g., "config/tools.yaml")
	repoTestcasesPath string // Path to testcases directory relative to repo root (e.g., "testcases")
	username          string // GitHub username for private repo access
	token             string // GitHub token/personal access token for private repo access
}

// NewGitHubSync creates a new GitHub sync instance
func NewGitHubSync(repoURL, cacheDir, repoConfigPath, repoTestcasesPath, username, token string) *GitHubSync {
	// Set defaults if not provided
	if repoConfigPath == "" {
		repoConfigPath = "config/tools.yaml"
	}
	if repoTestcasesPath == "" {
		repoTestcasesPath = "testcases"
	}

	return &GitHubSync{
		repoURL:           repoURL,
		cacheDir:          cacheDir,
		configDir:         filepath.Join(cacheDir, "config"),
		testcasesDir:      filepath.Join(cacheDir, "testcases"),
		repoConfigPath:    repoConfigPath,
		repoTestcasesPath: repoTestcasesPath,
		username:          username,
		token:             token,
	}
}

// Sync clones or pulls the repository and copies config and testcases directories
func (gs *GitHubSync) Sync() error {
	if gs.repoURL == "" {
		return fmt.Errorf("GitHub repo URL is empty")
	}

	// Ensure cache directory exists
	if err := os.MkdirAll(gs.cacheDir, 0755); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	repoDir := filepath.Join(gs.cacheDir, "repo")

	// Check if repo already exists
	_, err := os.Stat(filepath.Join(repoDir, ".git"))
	if os.IsNotExist(err) {
		// Clone the repository
		// Log URL without credentials for security
		logURL := gs.sanitizeURLForLogging(gs.repoURL)
		log.Printf("Cloning repository from %s...", logURL)
		if err := gs.cloneRepo(repoDir); err != nil {
			return fmt.Errorf("failed to clone repository: %w", err)
		}
		log.Printf("Repository cloned successfully")
	} else if err == nil {
		// Pull latest changes
		logURL := gs.sanitizeURLForLogging(gs.repoURL)
		log.Printf("Pulling latest changes from %s...", logURL)
		if err := gs.pullRepo(repoDir); err != nil {
			log.Printf("Warning: Failed to pull repository: %v. Using existing files.", err)
			// Continue with existing files if pull fails
		} else {
			log.Printf("Repository updated successfully")
		}
	} else {
		return fmt.Errorf("failed to check repository status: %w", err)
	}

	// Copy tools.yaml file if it exists in the repo
	repoConfigFile := filepath.Join(repoDir, gs.repoConfigPath)
	if _, err := os.Stat(repoConfigFile); err == nil {
		// Ensure config directory exists
		if err := os.MkdirAll(gs.configDir, 0755); err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		// Copy the file to the config directory as tools.yaml
		destConfigFile := filepath.Join(gs.configDir, "tools.yaml")
		data, err := os.ReadFile(repoConfigFile)
		if err != nil {
			return fmt.Errorf("failed to read config file from repo: %w", err)
		}
		if err := os.WriteFile(destConfigFile, data, 0644); err != nil {
			return fmt.Errorf("failed to write config file: %w", err)
		}
		log.Printf("Config file synced from %s to %s", gs.repoConfigPath, destConfigFile)
	} else {
		log.Printf("Config file not found at %s in repository, skipping", gs.repoConfigPath)
	}

	// Copy testcases directory if it exists in the repo
	repoTestcasesDir := filepath.Join(repoDir, gs.repoTestcasesPath)
	if _, err := os.Stat(repoTestcasesDir); err == nil {
		if err := gs.copyDirectory(repoTestcasesDir, gs.testcasesDir); err != nil {
			return fmt.Errorf("failed to copy testcases directory: %w", err)
		}
		log.Printf("Testcases directory synced from %s to %s", gs.repoTestcasesPath, gs.testcasesDir)
	} else {
		log.Printf("Testcases directory not found at %s in repository, skipping", gs.repoTestcasesPath)
	}

	return nil
}

// cloneRepo clones the repository to the specified directory
func (gs *GitHubSync) cloneRepo(destDir string) error {
	// Remove destination if it exists
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("failed to remove existing directory: %w", err)
	}

	// Use authenticated URL if credentials are provided
	cloneURL := gs.getAuthenticatedURL()
	cmd := exec.Command("git", "clone", "--depth", "1", cloneURL, destDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone failed: %w", err)
	}

	return nil
}

// pullRepo pulls the latest changes from the repository
func (gs *GitHubSync) pullRepo(repoDir string) error {
	// For pull, we need to update the remote URL if credentials are provided
	if gs.username != "" && gs.token != "" {
		// Update the remote URL with credentials
		authenticatedURL := gs.getAuthenticatedURL()
		cmd := exec.Command("git", "-C", repoDir, "remote", "set-url", "origin", authenticatedURL)
		if err := cmd.Run(); err != nil {
			log.Printf("Warning: Failed to update remote URL: %v", err)
		}
	}

	cmd := exec.Command("git", "-C", repoDir, "pull")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull failed: %w", err)
	}

	return nil
}

// copyDirectory copies a directory recursively
func (gs *GitHubSync) copyDirectory(srcDir, destDir string) error {
	// Remove destination directory if it exists
	if err := os.RemoveAll(destDir); err != nil {
		return fmt.Errorf("failed to remove existing destination directory: %w", err)
	}

	// Create destination directory
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Use cp command for simplicity (works on Unix-like systems)
	// For cross-platform, we could use filepath.Walk, but cp is simpler
	cmd := exec.Command("cp", "-r", srcDir+"/.", destDir+"/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		// Fallback to manual copy if cp fails
		return gs.copyDirectoryManual(srcDir, destDir)
	}

	return nil
}

// copyDirectoryManual manually copies files using filepath.Walk
func (gs *GitHubSync) copyDirectoryManual(srcDir, destDir string) error {
	return filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Calculate relative path from source
		relPath, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}

		destPath := filepath.Join(destDir, relPath)

		if info.IsDir() {
			// Create directory
			return os.MkdirAll(destPath, info.Mode())
		}

		// Read source file
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		// Write destination file
		return os.WriteFile(destPath, data, info.Mode())
	})
}

// GetConfigPath returns the path to the tools.yaml file in the synced config directory
func (gs *GitHubSync) GetConfigPath() string {
	return filepath.Join(gs.configDir, "tools.yaml")
}

// GetTestcasesDir returns the path to the synced testcases directory
func (gs *GitHubSync) GetTestcasesDir() string {
	return gs.testcasesDir
}

// GetRepoConfigPath returns the path to tools.yaml relative to repo root
func (gs *GitHubSync) GetRepoConfigPath() string {
	return gs.repoConfigPath
}

// GetRepoTestcasesPath returns the path to testcases directory relative to repo root
func (gs *GitHubSync) GetRepoTestcasesPath() string {
	return gs.repoTestcasesPath
}

// getAuthenticatedURL returns the repository URL with embedded credentials if provided
func (gs *GitHubSync) getAuthenticatedURL() string {
	if gs.username == "" || gs.token == "" {
		return gs.repoURL
	}

	// Parse the URL and inject credentials
	url := gs.repoURL

	// Remove existing credentials if present (format: https://user:pass@host/path)
	if protocolIdx := strings.Index(url, "://"); protocolIdx != -1 {
		// Find the @ symbol after the protocol
		afterProtocol := url[protocolIdx+3:]
		if atIdx := strings.Index(afterProtocol, "@"); atIdx != -1 {
			// Remove existing credentials
			url = url[:protocolIdx+3] + afterProtocol[atIdx+1:]
		}

		// Insert new credentials after protocol
		afterProtocol = url[protocolIdx+3:]
		url = url[:protocolIdx+3] + fmt.Sprintf("%s:%s@", gs.username, gs.token) + afterProtocol
	}

	return url
}

// sanitizeURLForLogging removes credentials from URL for safe logging
func (gs *GitHubSync) sanitizeURLForLogging(url string) string {
	// Remove credentials from URL if present (format: https://user:pass@host/path)
	if protocolIdx := strings.Index(url, "://"); protocolIdx != -1 {
		afterProtocol := url[protocolIdx+3:]
		if atIdx := strings.Index(afterProtocol, "@"); atIdx != -1 {
			// Replace credentials with ***
			url = url[:protocolIdx+3] + "***@" + afterProtocol[atIdx+1:]
		}
	}
	return url
}

// Cleanup removes the cache directory (optional, for cleanup operations)
func (gs *GitHubSync) Cleanup() error {
	return os.RemoveAll(gs.cacheDir)
}

// SyncFromGitHub syncs config and testcases from a GitHub repository
// Returns the config path, testcases directory path, and GitHubSync instance, or error
func SyncFromGitHub(repoURL string) (configPath string, testcasesDir string, githubSync *GitHubSync, err error) {
	if repoURL == "" {
		return "", "", nil, fmt.Errorf("GitHub repo URL is empty")
	}

	// Normalize repo URL (add https:// if missing, ensure .git suffix)
	normalizedURL := normalizeRepoURL(repoURL)

	// Get repo-relative paths from environment variables with defaults
	repoConfigPath := os.Getenv("GITHUB_TOOLS_CONFIG_PATH")
	if repoConfigPath == "" {
		repoConfigPath = "config/tools.yaml"
	}
	repoTestcasesPath := os.Getenv("GITHUB_TESTCASES_PATH")
	if repoTestcasesPath == "" {
		repoTestcasesPath = "testcases"
	}

	// Get authentication credentials from environment variables
	username := os.Getenv("GITHUB_USERNAME")
	token := os.Getenv("GITHUB_TOKEN")

	// Use a cache directory in the system temp or current directory
	cacheBase := filepath.Join(os.TempDir(), "mock-mcp-github-sync")
	cacheDir := filepath.Join(cacheBase, sanitizeRepoName(normalizedURL))

	sync := NewGitHubSync(normalizedURL, cacheDir, repoConfigPath, repoTestcasesPath, username, token)
	if err := sync.Sync(); err != nil {
		return "", "", nil, fmt.Errorf("failed to sync from GitHub: %w", err)
	}

	configPath = sync.GetConfigPath()
	testcasesDir = sync.GetTestcasesDir()

	return configPath, testcasesDir, sync, nil
}

// normalizeRepoURL normalizes a GitHub repository URL
func normalizeRepoURL(url string) string {
	url = strings.TrimSpace(url)

	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")

	// Add https:// if no protocol is specified
	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		// If it looks like a GitHub repo (user/repo format), add https://github.com/
		if strings.Contains(url, "/") && !strings.Contains(url, "@") {
			url = "https://github.com/" + strings.TrimPrefix(url, "github.com/")
		} else {
			url = "https://" + url
		}
	}

	// Add .git suffix for git clone
	if !strings.HasSuffix(url, ".git") {
		url = url + ".git"
	}

	return url
}

// sanitizeRepoName creates a safe directory name from a repo URL
func sanitizeRepoName(url string) string {
	// Remove protocol
	url = strings.TrimPrefix(url, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Remove .git suffix
	url = strings.TrimSuffix(url, ".git")

	// Replace slashes and other special chars with underscores
	url = strings.ReplaceAll(url, "/", "_")
	url = strings.ReplaceAll(url, ":", "_")
	url = strings.ReplaceAll(url, "@", "_")

	return url
}
