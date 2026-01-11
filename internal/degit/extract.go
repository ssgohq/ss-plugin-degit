package degit

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// ExtractOptions configures the extraction behavior
type ExtractOptions struct {
	StripComponents int    // Number of leading path components to strip
	Subdir          string // Subdirectory to extract (empty for all)
}

// ExtractTarball extracts a .tar.gz file to the destination directory
func ExtractTarball(tarballPath string, destDir string, opts ExtractOptions) error {
	// Open the tarball
	file, err := os.Open(tarballPath)
	if err != nil {
		return fmt.Errorf("failed to open tarball: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Calculate strip level
	stripLevel := opts.StripComponents
	if stripLevel == 0 {
		stripLevel = 1 // Default: strip root directory
	}

	// Add subdirectory depth to strip level
	if opts.Subdir != "" {
		subdir := strings.Trim(opts.Subdir, "/")
		stripLevel += strings.Count(subdir, "/") + 1
	}

	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		// Skip if file doesn't match subdirectory filter
		if opts.Subdir != "" {
			subdir := strings.Trim(opts.Subdir, "/")
			if !containsSubdir(header.Name, subdir) {
				continue
			}
		}

		// Strip leading path components
		name := stripPath(header.Name, stripLevel)
		if name == "" {
			continue
		}

		// Construct full destination path
		destPath := filepath.Join(destDir, name)

		// Security check: prevent path traversal
		if !strings.HasPrefix(filepath.Clean(destPath), filepath.Clean(destDir)) {
			return fmt.Errorf("path traversal detected: %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(destPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

		case tar.TypeReg:
			if err := extractFile(tarReader, destPath, header.Mode); err != nil {
				return fmt.Errorf("failed to extract file: %w", err)
			}

		case tar.TypeSymlink:
			// Security check: ensure symlink target doesn't escape destination
			linkTarget := header.Linkname
			if filepath.IsAbs(linkTarget) {
				continue // Skip absolute symlinks for security
			}
			targetPath := filepath.Join(filepath.Dir(destPath), linkTarget)
			if !strings.HasPrefix(filepath.Clean(targetPath), filepath.Clean(destDir)) {
				continue // Skip symlinks that escape destination
			}
			if err := os.Symlink(linkTarget, destPath); err != nil {
				// Ignore symlink errors (Windows compatibility)
				continue
			}
		}
	}

	return nil
}

// stripPath removes the first n path components from a path
func stripPath(path string, n int) string {
	parts := strings.Split(filepath.ToSlash(path), "/")
	if len(parts) <= n {
		return ""
	}
	return filepath.Join(parts[n:]...)
}

// containsSubdir checks if a path contains a specific subdirectory
func containsSubdir(path string, subdir string) bool {
	// Normalize paths
	path = filepath.ToSlash(path)
	subdir = filepath.ToSlash(strings.Trim(subdir, "/"))

	// Split into parts
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return false
	}

	// Skip the root directory (repo-name-hash)
	pathAfterRoot := strings.Join(parts[1:], "/")

	// Check if path starts with subdir
	return strings.HasPrefix(pathAfterRoot, subdir+"/") || pathAfterRoot == subdir
}

// extractFile extracts a single file from the tar reader
func extractFile(reader io.Reader, destPath string, mode int64) error {
	// Ensure parent directory exists
	if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
		return err
	}

	// Create destination file
	file, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, os.FileMode(mode))
	if err != nil {
		return err
	}
	defer file.Close()

	// Copy file contents
	_, err = io.Copy(file, reader)
	return err
}
