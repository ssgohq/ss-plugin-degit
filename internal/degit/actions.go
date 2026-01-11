package degit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	sdk "github.com/ssgohq/ss-plugin-sdk"
)

// Action represents a degit.json action
type Action struct {
	Action  string   `json:"action"`           // "clone" or "remove"
	Src     string   `json:"src,omitempty"`    // Source repo for clone action
	Files   []string `json:"files,omitempty"`  // Files to remove for remove action
	Cache   bool     `json:"cache,omitempty"`  // Use cache for clone action
	Verbose bool     `json:"verbose,omitempty"` // Verbose output for clone action
}

// UnmarshalJSON implements custom unmarshaling to handle both string and array for files
func (a *Action) UnmarshalJSON(data []byte) error {
	type alias Action
	var raw struct {
		alias
		Files interface{} `json:"files,omitempty"`
	}

	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	*a = Action(raw.alias)

	// Handle files field being either a string or array
	if raw.Files != nil {
		switch v := raw.Files.(type) {
		case string:
			a.Files = []string{v}
		case []interface{}:
			for _, f := range v {
				if s, ok := f.(string); ok {
					a.Files = append(a.Files, s)
				}
			}
		}
	}

	return nil
}

// LoadActions loads actions from degit.json in the destination directory
func LoadActions(destDir string) ([]Action, error) {
	actionsPath := filepath.Join(destDir, "degit.json")

	data, err := os.ReadFile(actionsPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No actions file
		}
		return nil, err
	}

	var actions []Action
	if err := json.Unmarshal(data, &actions); err != nil {
		return nil, fmt.Errorf("failed to parse degit.json: %w", err)
	}

	// Remove the degit.json file after loading
	os.Remove(actionsPath)

	return actions, nil
}

// ExecuteActions executes a list of actions
func ExecuteActions(actions []Action, destDir string, degitInst *Degit) error {
	if len(actions) == 0 {
		return nil
	}

	for i, action := range actions {
		switch action.Action {
		case "clone":
			if err := executeCloneAction(action, destDir, degitInst); err != nil {
				return fmt.Errorf("action %d (clone): %w", i, err)
			}

		case "remove":
			if err := executeRemoveAction(action, destDir); err != nil {
				return fmt.Errorf("action %d (remove): %w", i, err)
			}

		default:
			sdk.Warning(fmt.Sprintf("Unknown action: %s", action.Action))
		}
	}

	return nil
}

// executeCloneAction executes a clone action (clones another repo into the same destination)
func executeCloneAction(action Action, destDir string, degitInst *Degit) error {
	if action.Src == "" {
		return fmt.Errorf("clone action requires 'src' field")
	}

	sdk.Info(fmt.Sprintf("Cloning additional source: %s", action.Src))

	// Parse the source
	src, err := ParseSource(action.Src)
	if err != nil {
		return err
	}

	// Create a new degit instance for the nested clone
	nestedDegit := New(Options{
		Force:   true, // Force for nested clones
		Cache:   action.Cache,
		Verbose: action.Verbose,
		Token:   degitInst.options.Token,
		Mode:    degitInst.options.Mode,
	})

	// Clone to the same destination (will merge)
	return nestedDegit.Clone(src, destDir)
}

// executeRemoveAction executes a remove action (removes specified files)
func executeRemoveAction(action Action, destDir string) error {
	if len(action.Files) == 0 {
		return nil
	}

	for _, file := range action.Files {
		filePath := filepath.Join(destDir, file)

		// Security check: prevent path traversal
		cleanPath := filepath.Clean(filePath)
		if !hasPrefix(cleanPath, filepath.Clean(destDir)) {
			sdk.Warning(fmt.Sprintf("Skipping path traversal attempt: %s", file))
			continue
		}

		// Check if file exists
		info, err := os.Stat(filePath)
		if err != nil {
			if os.IsNotExist(err) {
				sdk.Warning(fmt.Sprintf("File does not exist: %s", file))
				continue
			}
			return err
		}

		// Remove file or directory
		if info.IsDir() {
			if err := os.RemoveAll(filePath); err != nil {
				return fmt.Errorf("failed to remove directory %s: %w", file, err)
			}
			sdk.Info(fmt.Sprintf("Removed directory: %s", file))
		} else {
			if err := os.Remove(filePath); err != nil {
				return fmt.Errorf("failed to remove file %s: %w", file, err)
			}
			sdk.Info(fmt.Sprintf("Removed file: %s", file))
		}
	}

	return nil
}

// filepath.HasPrefix is not available in older Go versions, so we implement it
func init() {
	// This is a no-op, just a placeholder for the filepath.HasPrefix function below
}

// hasPrefix checks if path has the given prefix
func hasPrefix(path, prefix string) bool {
	path = filepath.Clean(path)
	prefix = filepath.Clean(prefix)
	return path == prefix || len(path) > len(prefix) && path[len(prefix)] == filepath.Separator && path[:len(prefix)] == prefix
}
