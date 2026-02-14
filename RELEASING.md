# Releasing

This document describes how to create a new release of agent-evals.

## Prerequisites

The release workflow requires two secrets configured in the GitHub repository settings:

- `GITHUB_TOKEN` (automatic) -- used by GoReleaser to create the GitHub release and upload artifacts.
- `HOMEBREW_TAP_GITHUB_TOKEN` -- a personal access token with write access to the `thinkwright/homebrew-tap` repository. GoReleaser uses this to push the updated Homebrew formula on each release.

## Version Strategy

The project uses semantic versioning with git tags as the source of truth. GoReleaser reads the tag and injects the version into the binary via `-ldflags -X main.version={{.Version}}`. There is no separate VERSION file.

- **Patch** (0.0.x): Bug fixes, documentation updates, refusal pattern improvements.
- **Minor** (0.x.0): New features, new domains, new output formats, new provider support.
- **Major** (x.0.0): Breaking changes to CLI flags, configuration schema, or output format structure.

The project starts at v0.1.0. Pre-1.0 releases may include breaking changes in minor versions.

## Release Process

### 1. Verify the main branch is clean

Run the full test suite and confirm everything passes.

```sh
go test ./...
go vet ./...
```

### 2. Update the changelog

Move the `[Unreleased]` section in `CHANGELOG.md` to a versioned heading. Add a new empty `[Unreleased]` section at the top.

```markdown
## [Unreleased]

## [0.1.0] - 2025-02-14

### Added
- Initial release with static analysis and live boundary testing
...
```

Commit the changelog update to main.

### 3. Tag the release

```sh
git tag -a v0.1.0 -m "Release v0.1.0"
git push origin v0.1.0
```

Pushing the tag triggers `.github/workflows/release.yaml`, which runs the test suite and then GoReleaser.

### 4. Verify the release

GoReleaser builds binaries for linux/amd64, linux/arm64, darwin/amd64, and darwin/arm64, packages them as tar.gz archives with checksums, creates a GitHub release, and updates the Homebrew tap formula.

After the workflow completes, verify:

1. The [GitHub release](https://github.com/thinkwright/agent-evals/releases) has the correct binaries and checksums.
2. The [Homebrew tap](https://github.com/thinkwright/homebrew-tap) has an updated formula.
3. Installation works:

```sh
# go install
go install github.com/thinkwright/agent-evals/cmd/agent-evals@v0.1.0

# Homebrew
brew update && brew install thinkwright/tap/agent-evals

# curl
curl -sSL https://thinkwright.ai/install | sh

# Verify version
agent-evals --version
```

## Build Targets

GoReleaser produces the following archives:

| Archive | OS | Architecture |
|---------|-----|-------------|
| `agent-evals_linux_amd64.tar.gz` | Linux | x86_64 |
| `agent-evals_linux_arm64.tar.gz` | Linux | ARM64 |
| `agent-evals_darwin_amd64.tar.gz` | macOS | x86_64 |
| `agent-evals_darwin_arm64.tar.gz` | macOS | ARM64 (Apple Silicon) |

## Rollback

If a release has issues, delete the tag and the GitHub release, then fix and re-release.

```sh
git tag -d v0.1.0
git push origin --delete v0.1.0
```

Then delete the release from the GitHub UI, fix the issue, and tag again.
