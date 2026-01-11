// Package main provides a fast git repository scaffolding plugin for ss-cli.
//
// ss-plugin-degit clones git repositories without history, similar to degit,
// but with support for private GitHub repositories using ss-cli's token system.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	sdk "github.com/ssgohq/ss-plugin-sdk"

	"github.com/ssgohq/ss-plugin-degit/internal/auth"
	"github.com/ssgohq/ss-plugin-degit/internal/degit"
)

// Build-time variables (injected by goreleaser via ldflags)
var (
	version = "dev"
	commit  = "none"    //nolint:unused
	date    = "unknown" //nolint:unused
)

// DegitPlugin implements the sdk.Plugin interface
type DegitPlugin struct {
	source  string
	dest    string
	force   bool
	cache   bool
	mode    string
	verbose bool
}

// Metadata returns plugin information
func (p *DegitPlugin) Metadata() sdk.Metadata {
	return sdk.Metadata{
		Name:        "degit",
		Version:     version,
		Description: "Fast git repository scaffolding without history",
		Commands: []sdk.Command{
			{
				Name:        "degit",
				Description: "Clone a git repository without history",
				Usage:       "ss degit <source> [dest] [flags]",
			},
		},
	}
}

// Init parses arguments and flags
func (p *DegitPlugin) Init(ctx *sdk.Context) error {
	// Parse flags
	p.force = ctx.Flags["force"] == "true"
	p.cache = ctx.Flags["offline"] == "true"
	p.mode = ctx.Flags["mode"]
	p.verbose = ctx.Flags["verbose"] == "true"

	// Default mode to tar
	if p.mode == "" {
		p.mode = "tar"
	}

	// Parse positional arguments
	if len(ctx.Args) > 0 {
		p.source = ctx.Args[0]
	}
	if len(ctx.Args) > 1 {
		p.dest = ctx.Args[1]
	}

	return nil
}

// Execute runs the plugin's main logic
func (p *DegitPlugin) Execute(ctx *sdk.Context) error {
	// If no source provided, run interactive mode
	if p.source == "" {
		return p.runInteractive(ctx)
	}

	// Parse the source URL
	src, err := degit.ParseSource(p.source)
	if err != nil {
		return fmt.Errorf("invalid source: %w", err)
	}

	// Determine destination
	dest := p.dest
	if dest == "" {
		dest = src.Repo
	}

	// Make destination absolute
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(ctx.WorkingDir, dest)
	}

	// Get GitHub token for private repos
	token := auth.GitHubToken()

	// Create degit instance
	d := degit.New(degit.Options{
		Force:   p.force,
		Cache:   p.cache,
		Mode:    p.mode,
		Verbose: p.verbose,
		Token:   token,
	})

	// Clone the repository
	if p.verbose {
		sdk.Info(fmt.Sprintf("Cloning %s to %s", p.source, dest))
	}

	if err := d.Clone(src, dest); err != nil {
		return err
	}

	sdk.Success(fmt.Sprintf("Cloned %s to %s", p.source, dest))
	return nil
}

// runInteractive shows a fuzzy-searchable list of cached repos
func (p *DegitPlugin) runInteractive(ctx *sdk.Context) error {
	selected, err := degit.RunInteractive()
	if err != nil {
		if err == degit.ErrNoCachedRepos {
			sdk.Warning("No cached repositories found")
			sdk.Info("Usage: ss degit <source> [dest]")
			sdk.Info("Example: ss degit user/repo")
			return nil
		}
		return err
	}

	// Re-run with selected source
	p.source = selected
	return p.Execute(ctx)
}

// Cleanup is called after Execute
func (p *DegitPlugin) Cleanup(ctx *sdk.Context) error {
	return nil
}

// Complete handles completion requests (implements sdk.Completer)
func (p *DegitPlugin) Complete(ctx *sdk.Context) {
	// Get cached repos for completion
	repos := degit.GetCachedRepos()
	toComplete := ctx.GetCompletionToComplete()
	filtered := sdk.FilterCompletions(repos, toComplete)
	sdk.PrintCompletions(filtered)
}

func main() {
	// Ensure cache directory exists
	cacheDir := degit.GetCacheDir()
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create cache directory: %v\n", err)
	}

	sdk.Run(&DegitPlugin{})
}
