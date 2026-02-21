# CloudTerminal — Rename + Ship Design

## Context

The existing `claude-deck-spec.md` defines a terminal multiplexer for Claude Code sessions. This design covers renaming to CloudTerminal and setting up automated distribution so users can download prebuilt binaries and run immediately.

## Branding

| What | Old | New |
|------|-----|-----|
| Binary name | `claude-deck` | `cloudterminal` |
| Top bar brand | `claude-deck` | `CloudTerminal` |
| Config directory | `claude-deck/` | `cloudterminal/` |
| Session ID prefix | `deck-{runid}-{name}-{shortid}` | `ct-{runid}-{name}-{shortid}` |
| Go module | `claude-deck` | `github.com/BlinkingLine/cloudterminal` |

## Project Structure

```
cloudterminal/
├── main.go
├── go.mod
├── go.sum
├── Makefile
├── .goreleaser.yml
├── .github/workflows/release.yml
├── claude-deck-spec.md
└── internal/
    ├── session/
    │   ├── session.go
    │   ├── proc_unix.go
    │   ├── proc_windows.go
    │   └── mock.go
    ├── queue/
    │   └── queue.go
    ├── ui/
    │   ├── model.go
    │   ├── styles.go
    │   ├── view_normal.go
    │   ├── view_focus.go
    │   ├── input.go
    │   └── overlay.go
    └── config/
        └── config.go
```

## Distribution

- GoReleaser builds for: darwin/amd64, darwin/arm64, linux/amd64, linux/arm64, windows/amd64
- Archives: .tar.gz for macOS/Linux, .zip for Windows
- GitHub Actions triggers on tags matching `v*`
- Binaries published to GitHub Releases automatically

## Release Process

```bash
git tag v0.1.0
git push origin v0.1.0
# Binaries appear on GitHub Releases in ~2 minutes
```

## User Experience

User downloads binary, runs:
```bash
cloudterminal "auth:refactor auth" "tests:write tests"
```

Only prerequisite: `claude` CLI in PATH. Auto-detects missing CLI and falls back to mock mode. Config auto-created on first run.
