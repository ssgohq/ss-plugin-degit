package degit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	sdk "github.com/ssgohq/ss-plugin-sdk"
)

// Options configures the Degit behavior
type Options struct {
	Force   bool   // Allow cloning to non-empty directory
	Cache   bool   // Only use cached files (offline mode)
	Mode    string // "tar" or "git"
	Verbose bool   // Enable verbose output
	Token   string // GitHub token for private repos
}

// Degit is the main struct for degit operations
type Degit struct {
	options Options
}

// New creates a new Degit instance
func New(opts Options) *Degit {
	if opts.Mode == "" {
		opts.Mode = "tar"
	}
	return &Degit{options: opts}
}

// Clone clones a repository to the destination directory
func (d *Degit) Clone(src *Source, dest string) error {
	// Check if destination is empty
	if !d.options.Force {
		if err := d.checkDestEmpty(dest); err != nil {
			return err
		}
	}

	// Get cache directory
	cacheDir := GetRepoCacheDir(src)

	// Clone based on mode
	var err error
	if d.options.Mode == "git" {
		err = d.cloneWithGit(src, dest)
	} else {
		err = d.cloneWithTar(src, dest, cacheDir)
		// If tar mode fails, automatically try git mode as fallback
		if err != nil {
			if d.options.Verbose {
				sdk.Warning(fmt.Sprintf("Tarball download failed: %v", err))
				sdk.Info("Falling back to git clone mode...")
			}
			// Clean up any partial extraction
			os.RemoveAll(dest)
			gitErr := d.cloneWithGit(src, dest)
			if gitErr != nil {
				return fmt.Errorf("tarball download failed (%v) and git clone also failed (%v)", err, gitErr)
			}
			err = nil // Git clone succeeded
		}
	}

	if err != nil {
		return err
	}

	// Execute actions from degit.json if present
	actions, loadErr := LoadActions(dest)
	if loadErr != nil {
		sdk.Warning(fmt.Sprintf("Failed to load degit.json: %v", loadErr))
	} else if len(actions) > 0 {
		if d.options.Verbose {
			sdk.Info(fmt.Sprintf("Executing %d actions from degit.json", len(actions)))
		}
		if execErr := ExecuteActions(actions, dest, d); execErr != nil {
			return fmt.Errorf("failed to execute actions: %w", execErr)
		}
	}

	return nil
}

// cloneWithTar clones using tarball download (fast, no git history)
func (d *Degit) cloneWithTar(src *Source, dest string, cacheDir string) error {
	var hash string
	var err error

	// Try to resolve ref to hash
	if d.options.Cache {
		// Only use cache, don't fetch refs
		hash = GetCachedHash(cacheDir, src.Ref)
		if hash == "" {
			return fmt.Errorf("ref %s not found in cache (offline mode)", src.Ref)
		}
	} else {
		// Fetch refs from remote (use API for GitHub if token available)
		var refs []Ref
		var fetchErr error

		if src.Site == "github" {
			refs, fetchErr = FetchRefsWithToken(src)
		} else {
			refs, fetchErr = FetchRefs(src.URL)
		}

		if fetchErr != nil {
			// Try fallback to cached hash
			hash = GetCachedHash(cacheDir, src.Ref)
			if hash == "" {
				return fmt.Errorf("could not fetch refs and no cache available: %w", fetchErr)
			}
			if d.options.Verbose {
				sdk.Warning("Could not fetch refs, using cached version")
			}
		} else {
			// Resolve ref to hash
			hash, err = ResolveRef(refs, src.Ref)
			if err != nil {
				return fmt.Errorf("could not resolve ref %s: %w", src.Ref, err)
			}
		}
	}

	if d.options.Verbose {
		sdk.Info(fmt.Sprintf("Resolved %s to %s", src.Ref, hash[:8]))
	}

	// Check for cached tarball
	tarballPath := GetCachedTarball(cacheDir, hash)

	if tarballPath == "" {
		if d.options.Cache {
			return fmt.Errorf("tarball for %s not found in cache (offline mode)", hash[:8])
		}

		// Download tarball
		tarballPath = filepath.Join(cacheDir, hash+".tar.gz")
		if d.options.Verbose {
			sdk.Info(fmt.Sprintf("Downloading %s", src.TarballURL(hash)))
		}

		err = DownloadTarball(src, hash, tarballPath, DownloadOptions{
			Token:   d.options.Token,
			Verbose: d.options.Verbose,
		})
		if err != nil {
			return fmt.Errorf("failed to download tarball: %w", err)
		}
	} else if d.options.Verbose {
		sdk.Info("Using cached tarball")
	}

	// Update cache
	if err := UpdateCache(cacheDir, src.Ref, hash); err != nil && d.options.Verbose {
		sdk.Warning(fmt.Sprintf("Failed to update cache: %v", err))
	}

	// Extract tarball
	if d.options.Verbose {
		sdk.Info(fmt.Sprintf("Extracting to %s", dest))
	}

	extractOpts := ExtractOptions{
		StripComponents: 1,
		Subdir:          src.Subdir,
	}

	if err := ExtractTarball(tarballPath, dest, extractOpts); err != nil {
		return fmt.Errorf("failed to extract tarball: %w", err)
	}

	return nil
}

// cloneWithGit clones using git (slower, but works when tarball download fails)
func (d *Degit) cloneWithGit(src *Source, dest string) error {
	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git not found in PATH")
	}

	// Try HTTPS first (works with credential helpers), fallback to SSH
	cloneURL := src.URL + ".git"
	if d.options.Verbose {
		sdk.Info(fmt.Sprintf("Cloning with git (HTTPS): %s", cloneURL))
	}

	cmd := exec.Command("git", "clone", "--depth", "1", cloneURL, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		// Try SSH as fallback
		if d.options.Verbose {
			sdk.Warning("HTTPS clone failed, trying SSH...")
		}
		os.RemoveAll(dest) // Clean up failed clone

		cmd = exec.Command("git", "clone", "--depth", "1", src.SSH, dest)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if sshErr := cmd.Run(); sshErr != nil {
			return fmt.Errorf("git clone failed (HTTPS: %v, SSH: %v)", err, sshErr)
		}
	}

	// Checkout specific ref if not HEAD
	if src.Ref != "HEAD" && src.Ref != "" {
		checkoutCmd := exec.Command("git", "-C", dest, "checkout", src.Ref)
		if err := checkoutCmd.Run(); err != nil {
			// Try fetching the ref first
			fetchCmd := exec.Command("git", "-C", dest, "fetch", "origin", src.Ref)
			if fetchErr := fetchCmd.Run(); fetchErr == nil {
				checkoutCmd = exec.Command("git", "-C", dest, "checkout", src.Ref)
				if err := checkoutCmd.Run(); err != nil {
					sdk.Warning(fmt.Sprintf("Could not checkout ref %s: %v", src.Ref, err))
				}
			}
		}
	}

	// Remove .git directory
	gitDir := filepath.Join(dest, ".git")
	if err := os.RemoveAll(gitDir); err != nil {
		sdk.Warning(fmt.Sprintf("Failed to remove .git directory: %v", err))
	}

	// Handle subdirectory extraction for git mode
	if src.Subdir != "" {
		subdir := filepath.Join(dest, src.Subdir)

		// Create temp directory
		tempDir, err := os.MkdirTemp("", "degit-subdir-")
		if err != nil {
			return fmt.Errorf("failed to create temp directory: %w", err)
		}
		defer os.RemoveAll(tempDir)

		// Move subdirectory contents to temp
		entries, err := os.ReadDir(subdir)
		if err != nil {
			return fmt.Errorf("subdirectory %s not found: %w", src.Subdir, err)
		}

		for _, entry := range entries {
			srcPath := filepath.Join(subdir, entry.Name())
			dstPath := filepath.Join(tempDir, entry.Name())
			if err := os.Rename(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to move file: %w", err)
			}
		}

		// Clear destination
		entries, _ = os.ReadDir(dest)
		for _, entry := range entries {
			os.RemoveAll(filepath.Join(dest, entry.Name()))
		}

		// Move temp contents to destination
		entries, _ = os.ReadDir(tempDir)
		for _, entry := range entries {
			srcPath := filepath.Join(tempDir, entry.Name())
			dstPath := filepath.Join(dest, entry.Name())
			if err := os.Rename(srcPath, dstPath); err != nil {
				return fmt.Errorf("failed to move file: %w", err)
			}
		}
	}

	return nil
}

// checkDestEmpty checks if the destination directory is empty
func (d *Degit) checkDestEmpty(dest string) error {
	entries, err := os.ReadDir(dest)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, that's fine
		}
		return err
	}

	if len(entries) > 0 {
		return fmt.Errorf("destination %s is not empty (use --force to override)", dest)
	}

	return nil
}
