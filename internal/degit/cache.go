package degit

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// CacheDir returns the cache directory path for degit
// Default: ~/.ss/cache/degit/
func GetCacheDir() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "degit-cache")
	}
	return filepath.Join(homeDir, ".ss", "cache", "degit")
}

// GetRepoCacheDir returns the cache directory for a specific repository
func GetRepoCacheDir(src *Source) string {
	return filepath.Join(GetCacheDir(), src.Site, src.Owner, src.Repo)
}

// RefMap stores the mapping from ref names to commit hashes
type RefMap map[string]string

// AccessLog stores access timestamps for refs
type AccessLog map[string]string

// CacheEntry represents a cached repository entry
type CacheEntry struct {
	Source      *Source
	Hash        string
	TarballPath string
	AccessTime  time.Time
}

// LoadRefMap loads the ref->hash mapping from cache
func LoadRefMap(cacheDir string) (RefMap, error) {
	mapPath := filepath.Join(cacheDir, "map.json")
	data, err := os.ReadFile(mapPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(RefMap), nil
		}
		return nil, err
	}

	var refMap RefMap
	if err := json.Unmarshal(data, &refMap); err != nil {
		return nil, err
	}

	return refMap, nil
}

// SaveRefMap saves the ref->hash mapping to cache
func SaveRefMap(cacheDir string, refMap RefMap) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	mapPath := filepath.Join(cacheDir, "map.json")
	data, err := json.MarshalIndent(refMap, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(mapPath, data, 0644)
}

// LoadAccessLog loads access timestamps from cache
func LoadAccessLog(cacheDir string) (AccessLog, error) {
	accessPath := filepath.Join(cacheDir, "access.json")
	data, err := os.ReadFile(accessPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(AccessLog), nil
		}
		return nil, err
	}

	var accessLog AccessLog
	if err := json.Unmarshal(data, &accessLog); err != nil {
		return nil, err
	}

	return accessLog, nil
}

// SaveAccessLog saves access timestamps to cache
func SaveAccessLog(cacheDir string, accessLog AccessLog) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	accessPath := filepath.Join(cacheDir, "access.json")
	data, err := json.MarshalIndent(accessLog, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(accessPath, data, 0644)
}

// UpdateCache updates the cache with a new ref->hash mapping
func UpdateCache(cacheDir string, ref string, hash string) error {
	// Load current mappings
	refMap, err := LoadRefMap(cacheDir)
	if err != nil {
		refMap = make(RefMap)
	}

	accessLog, err := LoadAccessLog(cacheDir)
	if err != nil {
		accessLog = make(AccessLog)
	}

	// Update access timestamp
	accessLog[ref] = time.Now().UTC().Format(time.RFC3339)
	if err := SaveAccessLog(cacheDir, accessLog); err != nil {
		return err
	}

	// Check if ref already points to this hash
	if refMap[ref] == hash {
		return nil
	}

	// Check if old hash is still in use by other refs
	oldHash := refMap[ref]
	if oldHash != "" {
		hashInUse := false
		for _, h := range refMap {
			if h == oldHash {
				hashInUse = true
				break
			}
		}

		// Clean up old tarball if no longer in use
		if !hashInUse {
			oldTarball := filepath.Join(cacheDir, oldHash+".tar.gz")
			_ = os.Remove(oldTarball)
		}
	}

	// Update mapping
	refMap[ref] = hash
	return SaveRefMap(cacheDir, refMap)
}

// GetCachedTarball returns the path to a cached tarball if it exists
func GetCachedTarball(cacheDir string, hash string) string {
	tarballPath := filepath.Join(cacheDir, hash+".tar.gz")
	if _, err := os.Stat(tarballPath); err == nil {
		return tarballPath
	}
	return ""
}

// GetCachedHash returns the cached hash for a ref, if any
func GetCachedHash(cacheDir string, ref string) string {
	refMap, err := LoadRefMap(cacheDir)
	if err != nil {
		return ""
	}
	return refMap[ref]
}

// UpdateCacheAccess updates only the access log (for git mode clones without tarballs)
// This ensures repos cloned via git mode appear in interactive mode
func UpdateCacheAccess(cacheDir string, ref string) error {
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return err
	}

	accessLog, err := LoadAccessLog(cacheDir)
	if err != nil {
		accessLog = make(AccessLog)
	}

	accessLog[ref] = time.Now().UTC().Format(time.RFC3339)
	return SaveAccessLog(cacheDir, accessLog)
}

// GetCachedRepos returns a list of all cached repository paths
// Format: "site/owner/repo"
func GetCachedRepos() []string {
	cacheDir := GetCacheDir()
	var repos []string

	// Walk through cache directory structure
	_ = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		// Look for map.json files (indicates a cached repo)
		if info.Name() == "map.json" {
			// Get the repo path relative to cache dir
			dir := filepath.Dir(path)
			rel, err := filepath.Rel(cacheDir, dir)
			if err == nil && rel != "." {
				repos = append(repos, rel)
			}
		}
		return nil
	})

	return repos
}

// GetCachedReposByRecency returns cached repos sorted by most recent access
func GetCachedReposByRecency() []string {
	cacheDir := GetCacheDir()

	type repoAccess struct {
		path       string
		accessTime time.Time
	}

	var repos []repoAccess

	_ = filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.Name() == "access.json" {
			dir := filepath.Dir(path)
			rel, err := filepath.Rel(cacheDir, dir)
			if err != nil || rel == "." {
				return nil
			}

			accessLog, err := LoadAccessLog(dir)
			if err != nil {
				return nil
			}

			// Find most recent access time
			var mostRecent time.Time
			for _, ts := range accessLog {
				t, err := time.Parse(time.RFC3339, ts)
				if err == nil && t.After(mostRecent) {
					mostRecent = t
				}
			}

			repos = append(repos, repoAccess{
				path:       rel,
				accessTime: mostRecent,
			})
		}
		return nil
	})

	// Sort by most recent first
	sort.Slice(repos, func(i, j int) bool {
		return repos[i].accessTime.After(repos[j].accessTime)
	})

	// Extract paths
	result := make([]string, len(repos))
	for i, r := range repos {
		result[i] = r.path
	}

	return result
}
