# Contract: Release Artifacts

Each tagged release (`v*`) MUST publish the following assets to GitHub Releases via GoReleaser.

## Assets Per Release

| Asset | Description |
|-------|-------------|
| `mcp-proxy_<version>_darwin_arm64.tar.gz` | macOS Apple Silicon binary |
| `mcp-proxy_<version>_darwin_amd64.tar.gz` | macOS Intel binary |
| `checksums.txt` | SHA-256 checksums for all archives |

## Binary Contents

Each `.tar.gz` contains:
```
mcp-proxy          # the binary
```

## Version Embedding

Every binary MUST respond to:
```bash
mcp-proxy --version
# Output: mcp-proxy v1.2.3
```

The version string matches the git tag that triggered the release (injected via `ldflags -X main.version={{.Version}}`).

## Release Trigger

Releases are triggered by pushing a semver tag:
```bash
git tag v1.2.3
git push origin v1.2.3
```

The `release.yml` GitHub Actions workflow runs GoReleaser on `push: tags: ["v*.*.*"]`.

## GoReleaser Config (`.goreleaser.yml`)

```yaml
version: 2

builds:
  - main: ./cmd/server
    binary: mcp-proxy
    env:
      - CGO_ENABLED=0
    goos:
      - darwin
    goarch:
      - arm64
      - amd64
    ldflags:
      - -s -w -X main.version={{.Version}}

archives:
  - format: tar.gz
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"

checksum:
  name_template: "checksums.txt"
  algorithm: sha256

release:
  github:
    owner: rayjohnson
    name: mcp-proxy
```

## GitHub Actions Workflow (`.github/workflows/release.yml`)

```yaml
name: Release

on:
  push:
    tags:
      - "v*.*.*"

jobs:
  release:
    name: Release
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version-file: go.mod
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```
