package degit

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"regexp"
	"strings"

	"github.com/ssgohq/ss-plugin-degit/internal/auth"
)

// Ref represents a git reference (branch, tag, or commit)
type Ref struct {
	Type string // "HEAD", "branch", "tag", or "commit"
	Name string // Reference name (e.g., "main", "v1.0.0")
	Hash string // Commit hash
}

// refRegex parses git reference strings like "refs/heads/main" or "refs/tags/v1.0.0"
var refRegex = regexp.MustCompile(`refs/(\w+)/(.+)`)

// FetchRefs fetches all references from a remote repository
func FetchRefs(url string) ([]Ref, error) {
	cmd := exec.Command("git", "ls-remote", url)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("could not fetch refs from %s: %w", url, err)
	}

	return parseGitLsRemoteOutput(string(output))
}

// FetchRefsWithToken fetches refs using GitHub API (for private repos)
func FetchRefsWithToken(src *Source) ([]Ref, error) {
	if src.Site != "github" {
		// Fall back to git ls-remote for non-GitHub
		return FetchRefs(src.URL)
	}

	token := auth.GitHubToken()
	if token == "" {
		// No token, try git ls-remote
		return FetchRefs(src.URL)
	}

	var refs []Ref

	// Fetch default branch (HEAD)
	repoInfo, err := fetchGitHubRepoInfo(src)
	if err != nil {
		// Fall back to git ls-remote
		return FetchRefs(src.URL)
	}

	// Get default branch commit
	defaultBranch := repoInfo.DefaultBranch
	if defaultBranch == "" {
		defaultBranch = "main"
	}

	branchInfo, err := fetchGitHubBranchInfo(src, defaultBranch)
	if err == nil {
		refs = append(refs, Ref{
			Type: "HEAD",
			Name: "HEAD",
			Hash: branchInfo.Commit.SHA,
		})
		refs = append(refs, Ref{
			Type: "branch",
			Name: defaultBranch,
			Hash: branchInfo.Commit.SHA,
		})
	}

	// Fetch branches
	branches, _ := fetchGitHubBranches(src)
	for _, b := range branches {
		// Skip if already added
		if b.Name == defaultBranch {
			continue
		}
		refs = append(refs, Ref{
			Type: "branch",
			Name: b.Name,
			Hash: b.Commit.SHA,
		})
	}

	// Fetch tags
	tags, _ := fetchGitHubTags(src)
	for _, t := range tags {
		refs = append(refs, Ref{
			Type: "tag",
			Name: t.Name,
			Hash: t.Commit.SHA,
		})
	}

	if len(refs) == 0 {
		return nil, fmt.Errorf("could not fetch any refs from GitHub API")
	}

	return refs, nil
}

// GitHub API response types
type gitHubRepo struct {
	DefaultBranch string `json:"default_branch"`
}

type gitHubBranch struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

type gitHubTag struct {
	Name   string `json:"name"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
}

func fetchGitHubRepoInfo(src *Source) (*gitHubRepo, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", src.Owner, src.Repo)
	resp, err := auth.GitHubRequest(http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch repo info: %d", resp.StatusCode)
	}

	var repo gitHubRepo
	if err := json.NewDecoder(resp.Body).Decode(&repo); err != nil {
		return nil, err
	}
	return &repo, nil
}

func fetchGitHubBranchInfo(src *Source, branch string) (*gitHubBranch, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches/%s", src.Owner, src.Repo, branch)
	resp, err := auth.GitHubRequest(http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch branch info: %d", resp.StatusCode)
	}

	var b gitHubBranch
	if err := json.NewDecoder(resp.Body).Decode(&b); err != nil {
		return nil, err
	}
	return &b, nil
}

func fetchGitHubBranches(src *Source) ([]gitHubBranch, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/branches?per_page=100", src.Owner, src.Repo)
	resp, err := auth.GitHubRequest(http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch branches: %d", resp.StatusCode)
	}

	var branches []gitHubBranch
	if err := json.NewDecoder(resp.Body).Decode(&branches); err != nil {
		return nil, err
	}
	return branches, nil
}

func fetchGitHubTags(src *Source) ([]gitHubTag, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/tags?per_page=100", src.Owner, src.Repo)
	resp, err := auth.GitHubRequest(http.MethodGet, url)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch tags: %d", resp.StatusCode)
	}

	var tags []gitHubTag
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		return nil, err
	}
	return tags, nil
}

// parseGitLsRemoteOutput parses the output of git ls-remote
func parseGitLsRemoteOutput(output string) ([]Ref, error) {
	var refs []Ref
	lines := strings.Split(output, "\n")

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) != 2 {
			continue
		}

		hash := parts[0]
		refStr := parts[1]

		if refStr == "HEAD" {
			refs = append(refs, Ref{
				Type: "HEAD",
				Name: "HEAD",
				Hash: hash,
			})
			continue
		}

		match := refRegex.FindStringSubmatch(refStr)
		if match == nil {
			continue
		}

		refType := match[1]
		refName := match[2]

		switch refType {
		case "heads":
			refs = append(refs, Ref{
				Type: "branch",
				Name: refName,
				Hash: hash,
			})
		case "tags":
			refs = append(refs, Ref{
				Type: "tag",
				Name: refName,
				Hash: hash,
			})
		default:
			refs = append(refs, Ref{
				Type: refType,
				Name: refName,
				Hash: hash,
			})
		}
	}

	return refs, nil
}

// ResolveRef resolves a reference name to a commit hash
// It supports:
// - "HEAD" for default branch
// - Branch names (e.g., "main", "develop")
// - Tag names (e.g., "v1.0.0")
// - Partial commit hashes (8+ chars)
func ResolveRef(refs []Ref, refName string) (string, error) {
	if refName == "" || refName == "HEAD" {
		// Find HEAD
		for _, ref := range refs {
			if ref.Type == "HEAD" {
				return ref.Hash, nil
			}
		}
		return "", fmt.Errorf("could not find HEAD reference")
	}

	// Try to match as branch
	for _, ref := range refs {
		if ref.Type == "branch" && ref.Name == refName {
			return ref.Hash, nil
		}
	}

	// Try to match as tag
	for _, ref := range refs {
		if ref.Type == "tag" && ref.Name == refName {
			return ref.Hash, nil
		}
	}

	// Try to match as partial commit hash (8+ chars)
	if len(refName) >= 8 {
		for _, ref := range refs {
			if strings.HasPrefix(ref.Hash, refName) {
				return ref.Hash, nil
			}
		}
	}

	// If refName looks like a full commit hash, return it as-is
	if len(refName) == 40 && isHex(refName) {
		return refName, nil
	}

	return "", fmt.Errorf("could not resolve reference: %s", refName)
}

// isHex checks if a string contains only hexadecimal characters
func isHex(s string) bool {
	for _, c := range s {
		isDigit := c >= '0' && c <= '9'
		isLowerHex := c >= 'a' && c <= 'f'
		isUpperHex := c >= 'A' && c <= 'F'
		if !isDigit && !isLowerHex && !isUpperHex {
			return false
		}
	}
	return true
}

// GetDefaultBranch returns the default branch name from refs
func GetDefaultBranch(refs []Ref) string {
	// Find HEAD hash
	var headHash string
	for _, ref := range refs {
		if ref.Type == "HEAD" {
			headHash = ref.Hash
			break
		}
	}

	if headHash == "" {
		return "main" // fallback
	}

	// Find branch that matches HEAD
	for _, ref := range refs {
		if ref.Type == "branch" && ref.Hash == headHash {
			return ref.Name
		}
	}

	return "main" // fallback
}
