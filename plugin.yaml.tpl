# Plugin manifest for ss-plugin-degit
# This file is generated from plugin.yaml.tpl during build
# Version is injected by goreleaser during build (replaces __VERSION__)

apiVersion: v1
kind: Plugin

metadata:
  name: degit
  version: "__VERSION__"  # Replaced by goreleaser
  description: Fast git repository scaffolding without history
  author: ssgo team
  homepage: https://github.com/ssgohq/ss-plugin-degit

commands:
  - name: degit
    description: Clone a git repository without history
    usage: ss degit <source> [dest] [flags]
    flags:
      - name: force
        short: f
        description: Allow cloning to non-empty directory
        type: bool
      - name: offline
        short: o
        description: Only use cached files (offline mode)
        type: bool
      - name: mode
        short: m
        description: Clone mode (tar or git)
        type: string
      - name: verbose
        short: v
        description: Enable verbose output
        type: bool

# Runtime configuration with platform-specific binaries
runtime:
  # Default command (fallback if no platform match)
  command: ss-plugin-degit

  # Platform-specific binary paths
  platforms:
    - os: linux
      arch: amd64
      command: ss-plugin-degit
    - os: linux
      arch: arm64
      command: ss-plugin-degit
    - os: darwin
      arch: amd64
      command: ss-plugin-degit
    - os: darwin
      arch: arm64
      command: ss-plugin-degit
    - os: windows
      arch: amd64
      command: ss-plugin-degit.exe

completions:
  dynamic: true
