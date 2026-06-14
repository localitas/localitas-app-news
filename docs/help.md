---
title: News
description: RSS feed reader and article manager
---

# News

Subscribe to RSS feeds, organize them into collections, and read articles.

## Collections

Group feeds into named collections for organization.

**GET /api/collections** - List all collections
**POST /api/collections** - Create a new collection
**DELETE /api/collections/{id}** - Delete a collection

## Feeds

Add RSS/Atom feeds and sync them to pull new articles.

**GET /api/feeds** - List all subscribed feeds
**POST /api/feeds** - Subscribe to a new feed
**DELETE /api/feeds/{id}** - Unsubscribe from a feed
**POST /api/feeds/sync** - Refresh feeds and pull new articles

## Articles

Browse and manage synced articles. Mark articles as read, star favorites, and tag them for organization.

**GET /api/articles** - List articles (filter by feed, collection, read status)
**POST /api/articles/{id}/read** - Mark an article as read
**POST /api/articles/{id}/star** - Toggle star on an article

## Tags

Create tags and apply them to articles for custom categorization.

**GET /api/tags** - List all tags
**POST /api/tags** - Create a new tag
**DELETE /api/tags/{id}** - Delete a tag
**POST /api/articles/{id}/tag** - Tag an article
**DELETE /api/articles/{id}/tags/{tag_id}** - Remove a tag from an article
**GET /api/tags/{id}/articles** - List articles with a specific tag

## Search

**GET /api/search** - Full-text search across article titles and content

## Default Feeds

**GET /api/defaults** - List built-in default feed suggestions to get started quickly

## Build & Deploy

### Version

```bash
./news-server --version
```

### Build from source

```bash
# Development (native)
cd apps/news && go build -o bin/news-server ./cmd/news-server

# Cross-compile for Linux
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "-s -w" -trimpath -o bin/news-server-linux-amd64 ./cmd/news-server
```

### Docker

Build a Docker image directly from the binary:

```bash
# Default base image (debian:12-slim)
./news-server docker-build

# Custom base image
./news-server docker-build --base ubuntu:24.04

# Custom Dockerfile
./news-server docker-build --dockerfile ./my.Dockerfile

# Tag and push to registry
./news-server docker-build --tag ghcr.io/localitas/news:latest --push
```

The `docker-build` command requires a Linux amd64 binary in the same directory. Run `make deploy-build` from the project root first.

### Download

Pre-built binaries are available on the [GitHub releases page](https://github.com/localitas/localitas/releases).

Each release includes three builds per app:
- `news-server-darwin-arm64` (macOS Apple Silicon)
- `news-server-linux-amd64` (Linux x86_64)
- `news-server-linux-arm64` (Linux ARM64)

Download with the GitHub CLI:

    gh release download --repo localitas/localitas --pattern 'news-server-*'

### Release

All app binaries are published to GitHub releases as part of `make deploy-upload-image`.
