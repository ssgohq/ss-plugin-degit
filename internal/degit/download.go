package degit

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	sdk "github.com/ssgohq/ss-plugin-sdk"

	"github.com/ssgohq/ss-plugin-degit/internal/auth"
)

// DownloadOptions configures the download behavior
type DownloadOptions struct {
	Token   string // GitHub token for private repos
	Verbose bool   // Enable verbose output
}

// DownloadTarball downloads a repository tarball to the specified path
func DownloadTarball(src *Source, hash string, destPath string, opts DownloadOptions) error {
	// For GitHub, try API-based download first (works for both public and private)
	if src.Site == "github" {
		err := downloadGitHubTarball(src, hash, destPath, opts)
		if err == nil {
			return nil
		}

		if opts.Verbose {
			sdk.Warning(fmt.Sprintf("API download failed: %v", err))
		}

		// Always try direct URL as fallback (might work for public repos)
		if opts.Verbose {
			sdk.Info("Trying direct URL download...")
		}
		directErr := downloadPublic(src.TarballURL(hash), destPath)
		if directErr == nil {
			return nil
		}

		// Return original API error for private repos, direct error for public
		if opts.Token != "" {
			return fmt.Errorf("API download failed: %w (direct download also failed: %v)", err, directErr)
		}
		return directErr
	}

	// For non-GitHub, use direct URL
	return downloadPublic(src.TarballURL(hash), destPath)
}

// downloadGitHubTarball downloads a GitHub repository tarball using the API
func downloadGitHubTarball(src *Source, hash string, destPath string, opts DownloadOptions) error {
	// Use GitHub API tarball endpoint
	apiURL := src.APITarballURL(hash)

	if opts.Verbose {
		sdk.Info(fmt.Sprintf("Requesting tarball from API: %s", apiURL))
	}

	// Create request
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers - use application/vnd.github+json for API
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ss-plugin-degit")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")

	// Add authorization if token is available
	token := auth.GitHubToken()
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		if opts.Verbose {
			sdk.Info(fmt.Sprintf("Using GitHub token for authentication (token starts with: %s...)", token[:20]))
		}
	} else if opts.Verbose {
		sdk.Warning("No GitHub token found - private repos will not be accessible")
	}

	// Create client with redirect handler that preserves auth for same-host redirects
	client := &http.Client{
		CheckRedirect: func(r *http.Request, via []*http.Request) error {
			if len(via) >= 10 {
				return fmt.Errorf("too many redirects")
			}
			// Only preserve auth header for GitHub domains
			if isGitHubDomain(r.URL.Host) {
				if auth := via[0].Header.Get("Authorization"); auth != "" {
					r.Header.Set("Authorization", auth)
				}
			}
			r.Header.Set("User-Agent", "ss-plugin-degit")
			return nil
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to request tarball: %w", err)
	}
	defer resp.Body.Close()

	if opts.Verbose {
		sdk.Info(fmt.Sprintf("Response status: %d", resp.StatusCode))
	}

	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("repository not found or not accessible (404)")
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("unauthorized: invalid or missing GitHub token (401)")
	}

	if resp.StatusCode == http.StatusForbidden {
		return fmt.Errorf("forbidden: check your GitHub token permissions (403)")
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	return saveResponse(resp, destPath)
}

// isGitHubDomain checks if a host is a GitHub domain
func isGitHubDomain(host string) bool {
	return host == "api.github.com" ||
		host == "github.com" ||
		host == "codeload.github.com" ||
		host == "objects.githubusercontent.com"
}

// downloadPublic downloads a file without authentication
func downloadPublic(url string, destPath string) error {
	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	return saveResponse(resp, destPath)
}

// saveResponse saves an HTTP response body to a file
func saveResponse(resp *http.Response, destPath string) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Create destination file
	file, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Copy response body to file
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// CheckAccess checks if a repository is accessible (returns true if accessible)
func CheckAccess(src *Source, token string) (bool, error) {
	if src.Site != "github" {
		// For non-GitHub, try a simple HEAD request
		resp, err := http.Head(src.URL)
		if err != nil {
			return false, err
		}
		defer resp.Body.Close()
		return resp.StatusCode == http.StatusOK, nil
	}

	// For GitHub, check via API
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/%s", src.Owner, src.Repo)

	var resp *http.Response
	var err error

	if token != "" {
		resp, err = auth.GitHubRequest(http.MethodGet, apiURL)
	} else {
		resp, err = http.Get(apiURL)
	}

	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}
