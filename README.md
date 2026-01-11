# ss-plugin-degit

Fast git repository scaffolding without history. Inspired by [degit](https://github.com/Rich-Harris/degit).

## Features

- **Fast cloning** - Downloads tarball instead of full git history
- **Private repository support** - Works with private GitHub repos using ss-cli token system
- **Automatic fallback** - Falls back to git clone when tarball download fails
- **Caching** - Caches downloads for offline use
- **Subdirectory support** - Clone specific subdirectories
- **Reference support** - Clone specific branches, tags, or commits

## Installation

```bash
ss plugin install github.com/ssgohq/ss-plugin-degit
```

## Usage

```bash
# Clone a repository
ss degit user/repo

# Clone to specific directory
ss degit user/repo my-project

# Clone specific branch/tag/commit
ss degit user/repo#dev
ss degit user/repo#v1.0.0
ss degit user/repo#abc1234

# Clone subdirectory only
ss degit user/repo/src/components

# Force clone to non-empty directory
ss degit user/repo --force

# Use git clone instead of tarball
ss degit user/repo --mode=git

# Offline mode (use cache only)
ss degit user/repo --offline
```

## Private Repository Support

ss-plugin-degit supports private GitHub repositories. Authentication is resolved in this order:

1. `github_token` in `~/.ss/config.yaml`
2. `gh auth token` (GitHub CLI)
3. `GITHUB_TOKEN` environment variable
4. Git credential helper (automatic fallback)

```yaml
# ~/.ss/config.yaml
github_token: ghp_xxxxxxxxxxxx
```

If tarball download fails (e.g., token lacks repo access), the plugin automatically falls back to git clone using your system's git credentials.

## Credits

Inspired by [degit](https://github.com/Rich-Harris/degit) by Rich Harris.
