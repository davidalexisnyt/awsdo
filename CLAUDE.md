# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

`awsdo` is a Go CLI tool that simplifies AWS operations — SSO login, EC2 instance discovery, SSM terminal sessions, and database bastion tunneling. It wraps the AWS CLI (no SDK) and stores state in a JSON config file alongside the executable.

## Build Commands

```bash
# Build for current platform
task build
# or: go build -ldflags="-s -w"

# Cross-platform builds (Windows, Linux amd64, macOS amd64/arm64)
task buildall
```

No automated tests exist in this project — testing is manual.

## Code Architecture

**Single-package flat structure** — all files are `package main` in the root. No subdirectories for Go code.

**Key files by concern:**
- `main.go` — CLI entry point and command router; version string lives here
- `config.go` — `Configuration` struct, JSON serialization, `loadConfiguration`/`saveConfiguration`
- `bastion.go` — largest file (~980 lines); bastion tunnel CRUD and port forwarding
- `instances.go` — EC2 discovery, filtering, caching (~1,382 lines)
- `init.go` — interactive setup wizard (~843 lines)
- `repl.go` — interactive REPL with custom line editing and history
- `utils_unix.go` / `utils_windows.go` — platform-specific signal handling

**Configuration model:**
```
Configuration
└── Profiles[profileName]
    ├── DefaultInstance
    ├── Instances[name]
    └── Bastions[name] → { Instance, Host, Port, LocalPort }
```
Config is stored as `awsdo_config.json` next to the executable. Every command follows: load config → execute → save config.

**AWS CLI wrapper pattern** — the tool shells out to `aws` CLI via `exec.Command()` rather than using the AWS SDK. This means AWS CLI must be installed and configured on the host.

**Embedded help** — `help/*.txt` files are embedded via `//go:embed` into the binary. The `docs` command renders the README to terminal.

**Port allocation** — bastion tunnels auto-discover free local ports starting at 7000 via TCP listener checks.

## Release Process

Version is tracked as a string constant in `main.go`. The GitHub Actions workflow (`.github/workflows/release.yml`) auto-detects version bumps, increments the patch version, builds all platforms, and publishes a GitHub Release. To cut a release, update the version in `main.go`.
