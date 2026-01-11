// Package degit provides git repository scaffolding functionality.
package degit

import (
	"fmt"
	"regexp"
	"strings"
)

// Source represents a parsed repository source
type Source struct {
	Site   string // e.g., "github", "gitlab", "bitbucket"
	Owner  string // repository owner/user
	Repo   string // repository name
	Ref    string // branch, tag, or commit (default: "HEAD")
	Subdir string // subdirectory path (optional)
	URL    string // HTTPS URL
	SSH    string // SSH URL
}

// supportedHosts lists the supported git hosting platforms
var supportedHosts = map[string]bool{
	"github":    true,
	"gitlab":    true,
	"bitbucket": true,
	"git.sr.ht": true,
}

// sourceRegex parses repository source strings in various formats:
// - user/repo
// - github:user/repo
// - gitlab:user/repo
// - bitbucket:user/repo
// - https://github.com/user/repo
// - git@github.com:user/repo
// - user/repo#branch
// - user/repo/subdir#ref
var sourceRegex = regexp.MustCompile(
	`^(?:(?:https:\/\/)?([^:/]+\.[^:/]+)\/|git@([^:/]+)[:/]|([^/]+):)?` +
		`([^/\s]+)\/([^/\s#]+)(?:((?:\/[^/\s#]+)+))?(?:\/)?(?:#(.+))?$`,
)

// ParseSource parses a repository source string into a Source struct
func ParseSource(src string) (*Source, error) {
	match := sourceRegex.FindStringSubmatch(src)
	if match == nil {
		return nil, fmt.Errorf("could not parse source: %s", src)
	}

	// Determine the site (host)
	site := match[1]
	if site == "" {
		site = match[2]
	}
	if site == "" {
		site = match[3]
	}
	if site == "" {
		site = "github"
	}

	// Remove common TLD suffixes
	site = strings.TrimSuffix(site, ".com")
	site = strings.TrimSuffix(site, ".org")

	// Check if the site is supported
	if !supportedHosts[site] {
		return nil, fmt.Errorf("unsupported host: %s (supported: github, gitlab, bitbucket, git.sr.ht)", site)
	}

	owner := match[4]
	repo := strings.TrimSuffix(match[5], ".git")
	subdir := match[6]
	ref := match[7]
	if ref == "" {
		ref = "HEAD"
	}

	// Build URLs
	domain := getDomain(site)
	url := fmt.Sprintf("https://%s/%s/%s", domain, owner, repo)
	ssh := fmt.Sprintf("git@%s:%s/%s", domain, owner, repo)

	return &Source{
		Site:   site,
		Owner:  owner,
		Repo:   repo,
		Ref:    ref,
		Subdir: subdir,
		URL:    url,
		SSH:    ssh,
	}, nil
}

// getDomain returns the full domain for a site
func getDomain(site string) string {
	switch site {
	case "github":
		return "github.com"
	case "gitlab":
		return "gitlab.com"
	case "bitbucket":
		return "bitbucket.org"
	case "git.sr.ht":
		return "git.sr.ht"
	default:
		return site + ".com"
	}
}

// String returns a human-readable representation of the source
func (s *Source) String() string {
	result := fmt.Sprintf("%s/%s", s.Owner, s.Repo)
	if s.Subdir != "" {
		result += s.Subdir
	}
	if s.Ref != "HEAD" {
		result += "#" + s.Ref
	}
	return result
}

// CacheKey returns a unique key for caching this source
func (s *Source) CacheKey() string {
	return fmt.Sprintf("%s/%s/%s", s.Site, s.Owner, s.Repo)
}

// TarballURL returns the URL for downloading the repository tarball
func (s *Source) TarballURL(hash string) string {
	switch s.Site {
	case "github":
		return fmt.Sprintf("%s/archive/%s.tar.gz", s.URL, hash)
	case "gitlab":
		return fmt.Sprintf("%s/-/archive/%s/%s-%s.tar.gz", s.URL, hash, s.Repo, hash)
	case "bitbucket":
		return fmt.Sprintf("%s/get/%s.tar.gz", s.URL, hash)
	case "git.sr.ht":
		return fmt.Sprintf("%s/archive/%s.tar.gz", s.URL, hash)
	default:
		return fmt.Sprintf("%s/archive/%s.tar.gz", s.URL, hash)
	}
}

// APITarballURL returns the GitHub API URL for downloading private repo tarballs
func (s *Source) APITarballURL(ref string) string {
	if s.Site != "github" {
		return ""
	}
	return fmt.Sprintf("https://api.github.com/repos/%s/%s/tarball/%s", s.Owner, s.Repo, ref)
}
