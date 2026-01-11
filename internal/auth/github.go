// Package auth provides authentication utilities for GitHub and other git hosts.
package auth

import (
	"bytes"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// GlobalConfig represents the global ss-cli configuration
type GlobalConfig struct {
	PluginDir   string `yaml:"plugin_dir,omitempty"`
	GitHubToken string `yaml:"github_token,omitempty"`
}

// GitHubToken returns a token from config, gh CLI, or env (in that priority).
// This mirrors the logic in ss-cli/internal/plugin/discovery.go
func GitHubToken() string {
	// 1) Config file (~/.ss/config.yaml)
	if cfg, err := loadGlobalConfig(); err == nil && cfg != nil {
		if token := strings.TrimSpace(cfg.GitHubToken); token != "" {
			return token
		}
	}

	// 2) gh CLI
	if _, err := exec.LookPath("gh"); err == nil {
		var stdout, stderr bytes.Buffer
		cmd := exec.Command("gh", "auth", "token")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil {
			if token := strings.TrimSpace(stdout.String()); token != "" {
				return token
			}
		}
	}

	// 3) Environment variable
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token
	}

	return ""
}

// loadGlobalConfig loads the global config from ~/.ss/config.yaml
func loadGlobalConfig() (*GlobalConfig, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".ss", "config.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var cfg GlobalConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

// GitHubRequest issues an HTTP request with optional Authorization header.
func GitHubRequest(method, rawURL string) (*http.Response, error) {
	return GitHubRequestWithHeaders(method, rawURL, nil)
}

// GitHubRequestWithHeaders issues an HTTP request with optional headers and Authorization.
// It also preserves headers (including Authorization) across redirects.
func GitHubRequestWithHeaders(method, rawURL string, headers map[string]string) (*http.Response, error) {
	req, err := http.NewRequest(method, rawURL, nil)
	if err != nil {
		return nil, err
	}

	// Set default headers
	defaultHeaders := map[string]string{
		"Accept":     "application/vnd.github+json",
		"User-Agent": "ss-plugin-degit",
	}
	for k, v := range defaultHeaders {
		req.Header.Set(k, v)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Add authorization if token is available
	token := GitHubToken()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	// Create client with redirect handler that preserves headers
	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			// Re-apply headers (Authorization is not copied automatically)
			for k, v := range req.Header {
				r.Header[k] = v
			}
			return nil
		},
	}

	return client.Do(req)
}

// HasToken returns true if a GitHub token is available
func HasToken() bool {
	return GitHubToken() != ""
}
