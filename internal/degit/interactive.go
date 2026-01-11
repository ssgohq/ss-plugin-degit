package degit

import (
	"errors"
	"fmt"
	"strings"

	"github.com/manifoldco/promptui"
	"github.com/sahilm/fuzzy"
)

// ErrNoCachedRepos is returned when no cached repositories are found
var ErrNoCachedRepos = errors.New("no cached repositories")

// ErrUserCancelled is returned when the user cancels the selection
var ErrUserCancelled = errors.New("user cancelled")

// RunInteractive shows an interactive fuzzy search prompt for cached repos
func RunInteractive() (string, error) {
	// Get cached repos sorted by recency
	repos := GetCachedReposByRecency()

	if len(repos) == 0 {
		return "", ErrNoCachedRepos
	}

	// Convert to user-friendly format (remove "github/" prefix, use "/" separator)
	items := make([]string, len(repos))
	for i, repo := range repos {
		// Convert "github/owner/repo" to "owner/repo"
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) == 2 {
			items[i] = parts[1] // "owner/repo"
		} else {
			items[i] = repo
		}
	}

	// Create a searcher function for fuzzy matching
	searcher := func(input string, index int) bool {
		if input == "" {
			return true
		}
		matches := fuzzy.Find(input, []string{items[index]})
		return len(matches) > 0
	}

	// Configure the prompt
	prompt := promptui.Select{
		Label:             "Select a repository",
		Items:             items,
		Size:              10,
		Searcher:          searcher,
		StartInSearchMode: true,
		Templates: &promptui.SelectTemplates{
			Label:    "{{ . }}",
			Active:   "\U0001F449 {{ . | cyan }}",
			Inactive: "  {{ . }}",
			Selected: "\U0001F4E6 {{ . | green }}",
		},
	}

	// Run the prompt
	idx, _, err := prompt.Run()
	if err != nil {
		if err == promptui.ErrInterrupt || err == promptui.ErrEOF {
			return "", ErrUserCancelled
		}
		return "", fmt.Errorf("prompt failed: %w", err)
	}

	// Return the selected repo in source format
	return items[idx], nil
}

// RepoSearchResult represents a search result for completion
type RepoSearchResult struct {
	Repo  string
	Score int
}

// SearchCachedRepos performs fuzzy search on cached repos
func SearchCachedRepos(query string) []RepoSearchResult {
	repos := GetCachedReposByRecency()

	// Convert to user-friendly format
	items := make([]string, len(repos))
	for i, repo := range repos {
		parts := strings.SplitN(repo, "/", 2)
		if len(parts) == 2 {
			items[i] = parts[1]
		} else {
			items[i] = repo
		}
	}

	if query == "" {
		// Return all repos
		results := make([]RepoSearchResult, len(items))
		for i, item := range items {
			results[i] = RepoSearchResult{Repo: item, Score: 0}
		}
		return results
	}

	// Perform fuzzy search
	matches := fuzzy.Find(query, items)

	results := make([]RepoSearchResult, len(matches))
	for i, match := range matches {
		results[i] = RepoSearchResult{
			Repo:  match.Str,
			Score: match.Score,
		}
	}

	return results
}
